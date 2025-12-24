package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"rentme/internal/app/commands"
	availabilityapp "rentme/internal/app/handlers/availability"
	bookingapp "rentme/internal/app/handlers/booking"
	listingapp "rentme/internal/app/handlers/listings"
	meapp "rentme/internal/app/handlers/me"
	reviewsapp "rentme/internal/app/handlers/reviews"
	"rentme/internal/app/middleware"
	"rentme/internal/app/outbox"
	"rentme/internal/app/queries"
	authsvc "rentme/internal/app/services/auth"
	domainbooking "rentme/internal/domain/booking"
	"rentme/internal/domain/listings"
	domainpricing "rentme/internal/domain/pricing"
	domainreviews "rentme/internal/domain/reviews"
	domainrange "rentme/internal/domain/shared/daterange"
	"rentme/internal/domain/shared/money"
	domainuser "rentme/internal/domain/user"
	"rentme/internal/infra/config"
	ginserver "rentme/internal/infra/http/gin"
	infraMessaging "rentme/internal/infra/messaging"
	"rentme/internal/infra/obs"
	mlpricing "rentme/internal/infra/pricing"
	"rentme/internal/infra/security"
	"rentme/internal/infra/storage/memory"
	storages3 "rentme/internal/infra/storage/s3"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	env := getenv("APP_ENV", "dev")
	logger := obs.NewLogger(env)

	cfg, err := config.Load()
	if err != nil {
		logger.Warn("using fallback configuration", "error", err)
		cfg.Env = env
		cfg.HTTPAddr = getenv("HTTP_ADDR", ":8080")
		cfg.MongoURI = getenv("MONGO_URI", "mongodb://localhost:27017")
		cfg.MongoDB = getenv("MONGO_DB", "rentals")
		if brokers := strings.TrimSpace(getenv("KAFKA_BROKERS", "")); brokers != "" {
			cfg.KafkaBrokers = strings.Split(brokers, ",")
		}
		cfg.KafkaTopicPrefix = getenv("KAFKA_TOPIC_PREFIX", "")
		cfg.IdempotencyTTL = 168 * time.Hour
		cfg.OutboxPollInterval = 500 * time.Millisecond
		cfg.RetryBackoff = []time.Duration{time.Second, 5 * time.Second, 30 * time.Second}
		cfg.PricingMode = strings.ToLower(getenv("PRICING_MODE", "memory"))
		cfg.MLPricingURL = getenv("ML_PRICING_URL", "http://localhost:8000/predict")
		cfg.S3Endpoint = getenv("S3_ENDPOINT", "http://localhost:9000")
		cfg.S3PublicEndpoint = getenv("S3_PUBLIC_ENDPOINT", cfg.S3Endpoint)
		cfg.S3AccessKey = getenv("S3_ACCESS_KEY", "minioadmin")
		cfg.S3SecretKey = getenv("S3_SECRET_KEY", "minioadmin")
		cfg.S3Bucket = getenv("S3_BUCKET", "rentme-photos")
		cfg.S3UseSSL = parseBoolWithDefault(getenv("S3_USE_SSL", "false"), false)
		cfg.MessagingGRPCAddr = getenv("MESSAGING_GRPC_ADDR", "localhost:9000")
		if d, err := time.ParseDuration(getenv("MESSAGING_GRPC_DIAL_TIMEOUT", "")); err == nil && d > 0 {
			cfg.MessagingGRPCDial = d
		} else {
			cfg.MessagingGRPCDial = 3 * time.Second
		}
		if d, err := time.ParseDuration(getenv("MESSAGING_GRPC_TIMEOUT", "")); err == nil && d > 0 {
			cfg.MessagingGRPCTime = d
		} else {
			cfg.MessagingGRPCTime = 5 * time.Second
		}
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	app := buildApplication(logger, cfg)
	server := ginserver.NewServer(cfg, obs.Middleware{Logger: logger}, obs.HealthHandlers{
		Ready: func() error { return nil },
	}, app.handlers)
	defer app.close()

	fixturesPath := getenv("LISTINGS_FIXTURES", "")
	if fixturesPath == "" {
		fixturesPath = defaultListingFixturesPath()
	}
	if err := app.loadListingFixtures(ctx, fixturesPath, logger); err != nil {
		logger.Warn("listing fixtures load failed", "error", err, "path", fixturesPath)
	}
	if err := app.seedDemoGuestHistory(ctx, env, logger); err != nil {
		logger.Warn("demo guest history seed failed", "error", err)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("http shutdown failed", "error", err)
		}
	}()

	logger.Info("HTTP server starting", "addr", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server failed", "error", err)
		os.Exit(1)
	}
	logger.Info("HTTP server stopped")
}

type application struct {
	handlers ginserver.Handlers
	repos    struct {
		listings     *memory.ListingRepository
		availability *memory.AvailabilityRepository
		booking      *memory.BookingRepository
		reviews      *memory.ReviewsRepository
	}
	cleanup []func()
}

func buildApplication(logger *slog.Logger, cfg config.Config) application {
	var cleanup []func()
	listingsRepo := memory.NewListingRepository()
	availabilityRepo := memory.NewAvailabilityRepository()
	bookingRepo := memory.NewBookingRepository()
	reviewsRepo := memory.NewReviewsRepository()
	httpClient := &http.Client{Timeout: 5 * time.Second}
	pricingCalc := resolvePricingCalculator(cfg, httpClient, listingsRepo, logger)
	pricingPort := memory.PricingPortAdapter{Calculator: pricingCalc}
	uploader := resolveUploader(cfg, logger)
	outboxStore := memory.NewOutbox()
	idStore := memory.NewIdempotencyStore()
	userRepo := memory.NewUserRepository()
	sessionStore := memory.NewSessionStore()
	passwordHasher := security.BcryptHasher{}
	authService := &authsvc.Service{
		Users:      userRepo,
		Sessions:   sessionStore,
		Passwords:  passwordHasher,
		Tokens:     security.RandomTokenGenerator{Size: 48},
		SessionTTL: 24 * time.Hour,
		Logger:     logger,
	}
	seedDevAdmin(cfg.Env, userRepo, passwordHasher, logger)
	seedDemoUsers(cfg.Env, userRepo, passwordHasher, logger)
	messagingClient, msgCleanup := resolveMessagingClient(cfg, logger)
	if msgCleanup != nil {
		cleanup = append(cleanup, msgCleanup)
	}

	uowFactory := memory.Factory{
		ListingsRepo:     listingsRepo,
		AvailabilityRepo: availabilityRepo,
		BookingRepo:      bookingRepo,
		PricingSvc:       pricingCalc,
		ReviewsRepo:      reviewsRepo,
	}

	commandBus := commands.NewInMemoryBus()
	bookingHandler := &bookingapp.RequestBookingHandler{
		UoWFactory: uowFactory,
		Pricing:    pricingPort,
		Outbox:     outboxStore,
		Encoder:    outbox.JSONEventEncoder{},
	}
	commands.RegisterHandler(commandBus, bookingapp.RequestBookingCommand{}.Key(), bookingHandler)
	confirmBookingHandler := &bookingapp.ConfirmHostBookingHandler{Logger: logger}
	commands.RegisterHandler(commandBus, bookingapp.ConfirmHostBookingCommand{}.Key(), confirmBookingHandler)
	declineBookingHandler := &bookingapp.DeclineHostBookingHandler{Logger: logger}
	commands.RegisterHandler(commandBus, bookingapp.DeclineHostBookingCommand{}.Key(), declineBookingHandler)
	reviewSubmitHandler := &reviewsapp.SubmitReviewHandler{
		UoWFactory: uowFactory,
		Logger:     logger,
	}
	commands.RegisterHandler(commandBus, reviewsapp.SubmitReviewCommand{}.Key(), reviewSubmitHandler)
	reviewUpdateHandler := &reviewsapp.UpdateReviewHandler{
		UoWFactory: uowFactory,
		Logger:     logger,
	}
	commands.RegisterHandler(commandBus, reviewsapp.UpdateReviewCommand{}.Key(), reviewUpdateHandler)

	createListingHandler := &listingapp.CreateHostListingHandler{Logger: logger}
	commands.RegisterHandler(commandBus, listingapp.CreateHostListingCommand{}.Key(), createListingHandler)
	updateListingHandler := &listingapp.UpdateHostListingHandler{Logger: logger}
	commands.RegisterHandler(commandBus, listingapp.UpdateHostListingCommand{}.Key(), updateListingHandler)
	publishListingHandler := &listingapp.PublishHostListingHandler{Logger: logger}
	commands.RegisterHandler(commandBus, listingapp.PublishHostListingCommand{}.Key(), publishListingHandler)
	unpublishListingHandler := &listingapp.UnpublishHostListingHandler{Logger: logger}
	commands.RegisterHandler(commandBus, listingapp.UnpublishHostListingCommand{}.Key(), unpublishListingHandler)
	uploadPhotoHandler := &listingapp.UploadHostListingPhotoHandler{
		Logger:   logger,
		Uploader: uploader,
	}
	commands.RegisterHandler(commandBus, listingapp.UploadHostListingPhotoCommand{}.Key(), uploadPhotoHandler)

	queryBus := queries.NewInMemoryBus()
	availabilityHandler := &availabilityapp.GetCalendarHandler{
		UoWFactory: uowFactory,
	}
	queries.RegisterHandler(queryBus, availabilityapp.GetCalendarQuery{}.Key(), availabilityHandler)
	listingOverviewHandler := &listingapp.GetOverviewHandler{
		UoWFactory: uowFactory,
	}
	queries.RegisterHandler(queryBus, listingapp.GetOverviewQuery{}.Key(), listingOverviewHandler)
	catalogHandler := &listingapp.SearchCatalogHandler{
		UoWFactory: uowFactory,
	}
	queries.RegisterHandler(queryBus, listingapp.SearchCatalogQuery{}.Key(), catalogHandler)
	hostCatalogHandler := &listingapp.ListHostListingsHandler{
		UoWFactory: uowFactory,
		Logger:     logger,
	}
	queries.RegisterHandler(queryBus, listingapp.ListHostListingsQuery{}.Key(), hostCatalogHandler)
	hostDetailHandler := &listingapp.GetHostListingHandler{
		UoWFactory: uowFactory,
		Logger:     logger,
	}
	queries.RegisterHandler(queryBus, listingapp.GetHostListingQuery{}.Key(), hostDetailHandler)
	priceSuggestionHandler := &listingapp.HostListingPriceSuggestionHandler{
		UoWFactory: uowFactory,
		Pricing:    pricingPort,
		Logger:     logger,
	}
	queries.RegisterHandler(queryBus, listingapp.HostListingPriceSuggestionQuery{}.Key(), priceSuggestionHandler)
	meBookingsHandler := &meapp.ListGuestBookingsHandler{
		UoWFactory: uowFactory,
		Logger:     logger,
	}
	queries.RegisterHandler(queryBus, meapp.ListGuestBookingsQuery{}.Key(), meBookingsHandler)
	hostBookingsHandler := &bookingapp.ListHostBookingsHandler{
		UoWFactory: uowFactory,
		Logger:     logger,
	}
	queries.RegisterHandler(queryBus, bookingapp.ListHostBookingsQuery{}.Key(), hostBookingsHandler)
	listingReviewsHandler := &reviewsapp.ListListingReviewsHandler{
		UoWFactory: uowFactory,
		Logger:     logger,
	}
	queries.RegisterHandler(queryBus, reviewsapp.ListListingReviewsQuery{}.Key(), listingReviewsHandler)

	commandBusWithMiddleware := middleware.ChainCommands(
		commandBus,
		middleware.Idempotency(idStore, nil),
		middleware.Transaction(uowFactory, nil),
		middleware.OutboxFlush(outboxStore),
	)

	queryBusWithMiddleware := middleware.ChainQueries(queryBus)

	return application{
		handlers: ginserver.Handlers{
			Booking: ginserver.BookingHandler{
				Commands: commandBusWithMiddleware,
			},
			Availability: ginserver.AvailabilityHandler{
				Queries: queryBusWithMiddleware,
			},
			Reviews: ginserver.ReviewsHandler{
				Commands: commandBusWithMiddleware,
				Queries:  queryBusWithMiddleware,
				Logger:   logger,
			},
			Listing: ginserver.ListingHandler{
				Queries: queryBusWithMiddleware,
			},
			HostListing: ginserver.HostListingHandler{
				Commands: commandBusWithMiddleware,
				Queries:  queryBusWithMiddleware,
				Logger:   logger,
			},
			HostBooking: ginserver.HostBookingHandler{
				Commands: commandBusWithMiddleware,
				Queries:  queryBusWithMiddleware,
				Logger:   logger,
			},
			Auth: ginserver.AuthHandler{
				Service: authService,
				Logger:  logger,
			},
			Me: ginserver.MeHandler{
				Queries: queryBusWithMiddleware,
				Logger:  logger,
			},
			Chat: ginserver.ChatHandler{
				Messaging:  messagingClient,
				UoWFactory: uowFactory,
				Logger:     logger,
			},
			Admin: ginserver.AdminHandler{
				Users:    userRepo,
				Sessions: sessionStore,
				Metrics:  buildMLMetricsClient(cfg, httpClient, logger),
				Logger:   logger,
			},
			AuthMiddleware: ginserver.AuthMiddleware{
				Service: authService,
				Logger:  logger,
			}.Handle,
		},
		repos: struct {
			listings     *memory.ListingRepository
			availability *memory.AvailabilityRepository
			booking      *memory.BookingRepository
			reviews      *memory.ReviewsRepository
		}{
			listings:     listingsRepo,
			availability: availabilityRepo,
			booking:      bookingRepo,
			reviews:      reviewsRepo,
		},
		cleanup: cleanup,
	}
}

func resolvePricingCalculator(cfg config.Config, httpClient *http.Client, listingsRepo *memory.ListingRepository, logger *slog.Logger) domainpricing.Calculator {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.PricingMode))
	switch mode {
	case "ml":
		endpoint := cfg.MLPricingURL
		if endpoint == "" {
			endpoint = "http://localhost:8000/predict"
		}
		return &mlpricing.MLPricingEngine{
			Client:   httpClient,
			Endpoint: endpoint,
			Listings: listingsRepo,
			Logger:   logger,
			Clamps:   mlpricing.LoadClampConfig(cfg.MLPriceClamps, logger),
		}
	default:
		return memory.NewPricingEngine()
	}
}

func resolveMessagingClient(cfg config.Config, logger *slog.Logger) (*infraMessaging.Client, func()) {
	addr := strings.TrimSpace(cfg.MessagingGRPCAddr)
	if addr == "" {
		return nil, nil
	}
	client, err := infraMessaging.NewClient(context.Background(), infraMessaging.Config{
		Addr:        addr,
		DialTimeout: cfg.MessagingGRPCDial,
		CallTimeout: cfg.MessagingGRPCTime,
	}, logger)
	if err != nil {
		if logger != nil {
			logger.Warn("messaging grpc client init failed", "error", err, "addr", addr)
		}
		return nil, nil
	}
	return client, func() {
		_ = client.Close()
	}
}

func resolveUploader(cfg config.Config, logger *slog.Logger) storages3.Uploader {
	uploader, err := storages3.NewClient(cfg.S3Endpoint, cfg.S3UseSSL, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3PublicEndpoint, logger)
	if err != nil {
		if logger != nil {
			logger.Warn("s3 uploader disabled; falling back to noop", "error", err)
		}
		return storages3.NoopUploader{}
	}
	return uploader
}

func buildMLMetricsClient(cfg config.Config, httpClient *http.Client, logger *slog.Logger) *mlpricing.MetricsClient {
	endpoint := deriveMLMetricsEndpoint(cfg.MLPricingURL)
	if endpoint == "" {
		return nil
	}
	const metricsTimeout = 15 * time.Second
	if httpClient == nil || httpClient.Timeout < metricsTimeout {
		httpClient = &http.Client{Timeout: metricsTimeout}
	}
	return &mlpricing.MetricsClient{
		Endpoint: endpoint,
		Client:   httpClient,
		Logger:   logger,
	}
}

func deriveMLMetricsEndpoint(predictURL string) string {
	raw := strings.TrimSpace(predictURL)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Path = "/metrics"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func seedDevAdmin(env string, repo domainuser.Repository, hasher security.BcryptHasher, logger *slog.Logger) {
	email := strings.TrimSpace(getenv("ADMIN_EMAIL", ""))
	password := getenv("ADMIN_PASSWORD", "")
	if email == "" || password == "" {
		if strings.ToLower(strings.TrimSpace(env)) != "dev" {
			return
		}
		email = "admin@rentme.dev"
		password = "adminadmin"
	}
	ctx := context.Background()
	user, err := repo.ByEmail(ctx, email)
	if err == nil && user != nil {
		if user.HasRole("admin") {
			return
		}
		if err := user.EnsureRole("admin", time.Now()); err == nil {
			if saveErr := repo.Save(ctx, user); saveErr != nil && logger != nil {
				logger.Warn("cannot update dev admin user", "error", saveErr)
			} else if logger != nil {
				logger.Info("dev admin role added", "user_id", user.ID, "email", user.Email)
			}
		}
		return
	}
	if err != nil && !errors.Is(err, domainuser.ErrNotFound) {
		if logger != nil {
			logger.Warn("cannot check dev admin user", "error", err)
		}
		return
	}

	hash, err := hasher.Hash(password)
	if err != nil {
		if logger != nil {
			logger.Warn("cannot hash admin password", "error", err)
		}
		return
	}
	now := time.Now()
	adminUser, err := domainuser.NewUser(domainuser.CreateParams{
		ID:           domainuser.ID(uuid.NewString()),
		Email:        email,
		Name:         "Admin",
		PasswordHash: hash,
		Roles:        []domainuser.Role{"admin"},
		CreatedAt:    now,
	})
	if err != nil {
		if logger != nil {
			logger.Warn("cannot create dev admin user", "error", err)
		}
		return
	}
	if err := repo.Save(ctx, adminUser); err != nil {
		if logger != nil {
			logger.Warn("cannot save dev admin user", "error", err)
		}
		return
	}
	if logger != nil {
		logger.Info("dev admin seeded", "email", adminUser.Email)
	}
}

func seedDemoUsers(env string, repo domainuser.Repository, hasher security.BcryptHasher, logger *slog.Logger) {
	seed := parseBoolWithDefault(getenv("DEMO_SEED", ""), strings.ToLower(strings.TrimSpace(env)) == "dev")
	if !seed || repo == nil {
		return
	}
	type demoUser struct {
		ID       string
		Email    string
		Name     string
		Password string
		Roles    []domainuser.Role
	}
	accounts := []demoUser{
		{ID: "demo-admin", Email: "demo-admin@rentme.dev", Name: "Demo Admin", Password: "demo1234", Roles: []domainuser.Role{"admin", "host", "guest"}},
		{ID: "host-demo", Email: "host-demo@rentme.dev", Name: "Demo Host", Password: "demo1234", Roles: []domainuser.Role{"host", "guest"}},
		{ID: "host-lakeside", Email: "host-lakeside@rentme.dev", Name: "Host Lakeside", Password: "demo1234", Roles: []domainuser.Role{"host", "guest"}},
		{ID: "host-townhouse", Email: "host-townhouse@rentme.dev", Name: "Host Townhouse", Password: "demo1234", Roles: []domainuser.Role{"host"}},
		{ID: "host-nordic", Email: "host-nordic@rentme.dev", Name: "Host Nordic", Password: "demo1234", Roles: []domainuser.Role{"host"}},
		{ID: "host-botanical", Email: "host-botanical@rentme.dev", Name: "Host Botanical", Password: "demo1234", Roles: []domainuser.Role{"host"}},
		{ID: "guest-olga", Email: "guest-olga@rentme.dev", Name: "Ольга (гость)", Password: "demo1234", Roles: []domainuser.Role{"guest"}},
		{ID: "guest-ivan", Email: "guest-ivan@rentme.dev", Name: "Иван (гость)", Password: "demo1234", Roles: []domainuser.Role{"guest"}},
		{ID: "guest-marina", Email: "guest-marina@rentme.dev", Name: "Марина (гость)", Password: "demo1234", Roles: []domainuser.Role{"guest"}},
	}

	ctx := context.Background()
	for _, acc := range accounts {
		existing, err := repo.ByEmail(ctx, acc.Email)
		if err == nil && existing != nil {
			updated := false
			if acc.ID != "" && string(existing.ID) != acc.ID {
				if logger != nil {
					logger.Warn("demo user email already used by different id", "email", acc.Email, "existing_id", existing.ID, "expected_id", acc.ID)
				}
			}
			for _, role := range acc.Roles {
				if ensureErr := existing.EnsureRole(role, time.Now()); ensureErr == nil {
					updated = true
				}
			}
			if updated {
				if saveErr := repo.Save(ctx, existing); saveErr != nil && logger != nil {
					logger.Warn("cannot update demo user roles", "email", acc.Email, "error", saveErr)
				}
			}
			continue
		}
		if err != nil && !errors.Is(err, domainuser.ErrNotFound) {
			if logger != nil {
				logger.Warn("cannot check demo user", "email", acc.Email, "error", err)
			}
			continue
		}

		hash, err := hasher.Hash(acc.Password)
		if err != nil {
			if logger != nil {
				logger.Warn("cannot hash demo password", "email", acc.Email, "error", err)
			}
			continue
		}
		userModel, err := domainuser.NewUser(domainuser.CreateParams{
			ID:           domainuser.ID(acc.ID),
			Email:        acc.Email,
			Name:         acc.Name,
			PasswordHash: hash,
			Roles:        acc.Roles,
			CreatedAt:    time.Now(),
		})
		if err != nil {
			if logger != nil {
				logger.Warn("cannot build demo user", "email", acc.Email, "error", err)
			}
			continue
		}
		if err := repo.Save(ctx, userModel); err != nil {
			if logger != nil {
				logger.Warn("cannot save demo user", "email", acc.Email, "error", err)
			}
			continue
		}
		if logger != nil {
			logger.Info("demo user seeded", "email", acc.Email, "roles", acc.Roles)
		}
	}
}

func (a application) seedDemoGuestHistory(ctx context.Context, env string, logger *slog.Logger) error {
	seed := parseBoolWithDefault(getenv("DEMO_SEED", ""), strings.ToLower(strings.TrimSpace(env)) == "dev")
	if !seed {
		return nil
	}
	if a.repos.booking == nil || a.repos.reviews == nil || a.repos.listings == nil {
		return nil
	}

	type demoReviewSeed struct {
		ID     string
		Rating int
		Text   string
	}
	type demoBookingSeed struct {
		ID               string
		ListingID        string
		PriceUnit        string
		Months           int
		Nights           int
		Guests           int
		RateRub          int64
		CheckInOffsetDay int
		Review           demoReviewSeed
	}

	now := time.Now().UTC()
	guestID := "guest-marina"
	seeds := []demoBookingSeed{
		{
			ID:               "booking-demo-marina-1",
			ListingID:        "listing-demo-10",
			PriceUnit:        "night",
			Nights:           4,
			Guests:           2,
			RateRub:          5200,
			CheckInOffsetDay: -40,
			Review: demoReviewSeed{
				ID:     "review-demo-marina-1",
				Rating: 5,
				Text:   "Очень уютная квартира и отличный район. Заселение прошло без проблем.",
			},
		},
		{
			ID:               "booking-demo-marina-2",
			ListingID:        "listing-demo-11",
			PriceUnit:        "month",
			Months:           3,
			Guests:           3,
			RateRub:          65000,
			CheckInOffsetDay: -210,
			Review: demoReviewSeed{
				ID:     "review-demo-marina-2",
				Rating: 5,
				Text:   "Тихий дом, удобное расположение и комфортная планировка. Спасибо хосту!",
			},
		},
	}

	for _, seed := range seeds {
		if _, err := a.repos.booking.ByID(ctx, domainbooking.BookingID(seed.ID)); err == nil {
			continue
		} else if err != nil && !errors.Is(err, domainbooking.ErrBookingNotFound) {
			return err
		}

		listing, err := a.repos.listings.ByID(ctx, listings.ListingID(seed.ListingID))
		if err != nil {
			if logger != nil {
				logger.Warn("demo booking listing missing", "listing_id", seed.ListingID, "error", err)
			}
			continue
		}

		checkIn := now.AddDate(0, 0, seed.CheckInOffsetDay)
		checkOut := checkIn
		switch seed.PriceUnit {
		case "month":
			months := seed.Months
			if months < 1 {
				months = 1
			}
			checkOut = checkIn.AddDate(0, months, 0)
		default:
			nights := seed.Nights
			if nights < 1 {
				nights = 1
			}
			checkOut = checkIn.AddDate(0, 0, nights)
		}

		dr, err := domainrange.New(checkIn, checkOut)
		if err != nil {
			if logger != nil {
				logger.Warn("demo booking range invalid", "booking_id", seed.ID, "error", err)
			}
			continue
		}

		units := dr.Nights()
		months := 0
		if seed.PriceUnit == "month" {
			months = seed.Months
			units = months
		}

		price, err := buildSeedPrice(seed.RateRub, units)
		if err != nil {
			if logger != nil {
				logger.Warn("demo booking price invalid", "booking_id", seed.ID, "error", err)
			}
			continue
		}

		createdAt := checkIn.AddDate(0, 0, -7)
		booking, err := domainbooking.NewBooking(domainbooking.CreateParams{
			ID:        domainbooking.BookingID(seed.ID),
			ListingID: listing.ID,
			GuestID:   guestID,
			Range:     dr,
			Guests:    seed.Guests,
			Months:    months,
			PriceUnit: seed.PriceUnit,
			Price:     price,
			Policy: domainbooking.CancellationPolicySnapshot{
				PolicyID: listing.CancellationPolicyID,
			},
			CreatedAt: createdAt,
		})
		if err != nil {
			if logger != nil {
				logger.Warn("demo booking build failed", "booking_id", seed.ID, "error", err)
			}
			continue
		}

		if err := booking.Accept(createdAt.Add(2 * time.Hour)); err != nil {
			return err
		}
		if err := booking.Confirm("demo-hold", createdAt.Add(4*time.Hour)); err != nil {
			return err
		}
		if err := booking.CheckIn(checkIn); err != nil {
			return err
		}
		if err := booking.CheckOut(checkOut); err != nil {
			return err
		}
		if err := a.repos.booking.Save(ctx, booking); err != nil {
			return err
		}

		if seed.Review.ID != "" {
			if _, err := a.repos.reviews.ByBooking(ctx, booking.ID, guestID); err == nil {
				continue
			} else if err != nil && !errors.Is(err, domainreviews.ErrNotFound) {
				return err
			}

			review, err := domainreviews.Submit(domainreviews.SubmitParams{
				ID:        domainreviews.ReviewID(seed.Review.ID),
				BookingID: booking.ID,
				AuthorID:  guestID,
				ListingID: booking.ListingID,
				Rating:    seed.Review.Rating,
				Text:      seed.Review.Text,
				CreatedAt: checkOut.AddDate(0, 0, 2),
			})
			if err != nil {
				return err
			}
			if err := a.repos.reviews.Save(ctx, review); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a application) loadListingFixtures(ctx context.Context, path string, logger *slog.Logger) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Info("listing fixtures file not found, skipping", "path", path)
			return nil
		}
		return fmt.Errorf("read fixtures: %w", err)
	}
	if len(data) == 0 {
		logger.Warn("listing fixtures file empty", "path", path)
		return nil
	}
	// Be tolerant to UTF-8 BOM in fixtures (common when edited on Windows).
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var fixtures []listingFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		return fmt.Errorf("decode fixtures: %w", err)
	}
	if len(fixtures) == 0 {
		return nil
	}

	now := time.Now()
	for _, fx := range fixtures {
		params := listings.CreateListingParams{
			ID:           listings.ListingID(fx.ID),
			Host:         listings.HostID(fx.Host),
			Title:        fx.Title,
			Description:  fx.Description,
			PropertyType: fx.PropertyType,
			Address: listings.Address{
				Line1: fx.Address.Line1,
				Line2: fx.Address.Line2,
				City:  fx.Address.City,
				Region: func() string {
					r := strings.TrimSpace(fx.Address.Region)
					if r != "" {
						return r
					}
					return fx.Address.Country
				}(),
				Country: fx.Address.Country,
				Lat:     fx.Address.Lat,
				Lon:     fx.Address.Lon,
			},
			Amenities:            append([]string(nil), fx.Amenities...),
			GuestsLimit:          fx.GuestsLimit,
			MinNights:            fx.MinNights,
			MaxNights:            fx.MaxNights,
			HouseRules:           append([]string(nil), fx.HouseRules...),
			CancellationPolicyID: fx.CancellationPolicyID,
			Tags:                 append([]string(nil), fx.Tags...),
			Highlights:           append([]string(nil), fx.Highlights...),
			RateRub:              fx.RateRub,
			Bedrooms:             fx.Bedrooms,
			Bathrooms:            fx.Bathrooms,
			Floor:                fx.Floor,
			FloorsTotal:          fx.FloorsTotal,
			RenovationScore:      fx.RenovationScore,
			BuildingAgeYears:     fx.BuildingAgeYears,
			AreaSquareMeters:     fx.AreaSquareMeters,
			RentalTermType:       listings.RentalTermType(strings.TrimSpace(strings.ToLower(fx.RentalTerm))),
			ThumbnailURL:         fx.ThumbnailURL,
			Rating:               fx.Rating,
			AvailableFrom:        parseFixtureTime(fx.AvailableFrom, now),
			Now:                  now,
		}

		listing, err := listings.NewListing(params)
		if err != nil {
			logger.Error("fixture invalid", "listing_id", fx.ID, "error", err)
			continue
		}
		if err := listing.Activate(now); err != nil {
			logger.Error("fixture activation failed", "listing_id", fx.ID, "error", err)
			continue
		}
		if err := a.repos.listings.Save(ctx, listing); err != nil {
			logger.Error("cannot store fixture listing", "listing_id", fx.ID, "error", err)
			continue
		}
		if _, err := a.repos.availability.Calendar(ctx, listing.ID); err != nil {
			logger.Error("cannot prepare availability for fixture", "listing_id", fx.ID, "error", err)
			continue
		}
		logger.Info("listing fixture imported", "listing_id", listing.ID)
	}
	return nil
}

type listingFixture struct {
	ID                   string         `json:"id"`
	Host                 string         `json:"host"`
	Title                string         `json:"title"`
	Description          string         `json:"description"`
	PropertyType         string         `json:"property_type"`
	Address              fixtureAddress `json:"address"`
	Amenities            []string       `json:"amenities"`
	GuestsLimit          int            `json:"guests_limit"`
	MinNights            int            `json:"min_nights"`
	MaxNights            int            `json:"max_nights"`
	HouseRules           []string       `json:"house_rules"`
	CancellationPolicyID string         `json:"cancellation_policy_id"`
	Tags                 []string       `json:"tags"`
	Highlights           []string       `json:"highlights"`
	RateRub              int64          `json:"rate_rub"`
	PriceUnit            string         `json:"price_unit"`
	Bedrooms             int            `json:"bedrooms"`
	Bathrooms            int            `json:"bathrooms"`
	Floor                int            `json:"floor"`
	FloorsTotal          int            `json:"floors_total"`
	RenovationScore      int            `json:"renovation_score"`
	BuildingAgeYears     int            `json:"building_age_years"`
	AreaSquareMeters     float64        `json:"area_sq_m"`
	RentalTerm           string         `json:"rental_term"`
	ThumbnailURL         string         `json:"thumbnail_url"`
	Rating               float64        `json:"rating"`
	AvailableFrom        string         `json:"available_from"`
}

type fixtureAddress struct {
	Line1   string  `json:"line1"`
	Line2   string  `json:"line2"`
	City    string  `json:"city"`
	Region  string  `json:"region"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

func parseFixtureTime(value string, fallback time.Time) time.Time {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return fallback
}

func buildSeedPrice(rateRub int64, units int) (domainpricing.PriceBreakdown, error) {
	if units <= 0 {
		return domainpricing.PriceBreakdown{}, errors.New("seed: units must be positive")
	}
	breakdown := domainpricing.PriceBreakdown{
		Nights:  units,
		Nightly: money.Must(rateRub, "RUB"),
	}
	if err := breakdown.RecalculateTotal(); err != nil {
		return domainpricing.PriceBreakdown{}, err
	}
	return breakdown, nil
}

func defaultListingFixturesPath() string {
	candidates := []string{
		filepath.Join("data", "listings.json"),
		filepath.Join("backend", "data", "listings.json"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func parseBoolWithDefault(raw string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	case "0", "f", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (a application) close() {
	for _, fn := range a.cleanup {
		if fn != nil {
			fn()
		}
	}
}
