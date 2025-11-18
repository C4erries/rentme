package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"rentme/internal/app/commands"
	availabilityapp "rentme/internal/app/handlers/availability"
	bookingapp "rentme/internal/app/handlers/booking"
	listingapp "rentme/internal/app/handlers/listings"
	"rentme/internal/app/middleware"
	"rentme/internal/app/outbox"
	"rentme/internal/app/queries"
	"rentme/internal/domain/listings"
	"rentme/internal/infra/config"
	ginserver "rentme/internal/infra/http/gin"
	"rentme/internal/infra/obs"
	"rentme/internal/infra/storage/memory"
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
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	app := buildApplication(logger)
	server := ginserver.NewServer(cfg, obs.Middleware{Logger: logger}, obs.HealthHandlers{
		Ready: func() error { return nil },
	}, app.handlers)

	fixturesPath := getenv("LISTINGS_FIXTURES", "")
	if fixturesPath == "" {
		fixturesPath = defaultListingFixturesPath()
	}
	if err := app.loadListingFixtures(ctx, fixturesPath, logger); err != nil {
		logger.Warn("listing fixtures load failed", "error", err, "path", fixturesPath)
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
	}
}

func buildApplication(logger *slog.Logger) application {
	listingsRepo := memory.NewListingRepository()
	availabilityRepo := memory.NewAvailabilityRepository()
	bookingRepo := memory.NewBookingRepository()
	reviewsRepo := memory.NewReviewsRepository()
	pricingCalc := memory.NewPricingEngine()
	pricingPort := memory.PricingPortAdapter{Calculator: pricingCalc}
	outboxStore := memory.NewOutbox()
	idStore := memory.NewIdempotencyStore()

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
			Listing: ginserver.ListingHandler{
				Queries: queryBusWithMiddleware,
			},
		},
		repos: struct {
			listings     *memory.ListingRepository
			availability *memory.AvailabilityRepository
		}{
			listings:     listingsRepo,
			availability: availabilityRepo,
		},
	}
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
			ID:          listings.ListingID(fx.ID),
			Host:        listings.HostID(fx.Host),
			Title:       fx.Title,
			Description: fx.Description,
			Address: listings.Address{
				Line1:   fx.Address.Line1,
				Line2:   fx.Address.Line2,
				City:    fx.Address.City,
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
			NightlyRateCents:     fx.NightlyRateCents,
			Bedrooms:             fx.Bedrooms,
			Bathrooms:            fx.Bathrooms,
			AreaSquareMeters:     fx.AreaSquareMeters,
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
	Address              fixtureAddress `json:"address"`
	Amenities            []string       `json:"amenities"`
	GuestsLimit          int            `json:"guests_limit"`
	MinNights            int            `json:"min_nights"`
	MaxNights            int            `json:"max_nights"`
	HouseRules           []string       `json:"house_rules"`
	CancellationPolicyID string         `json:"cancellation_policy_id"`
	Tags                 []string       `json:"tags"`
	Highlights           []string       `json:"highlights"`
	NightlyRateCents     int64          `json:"nightly_rate_cents"`
	Bedrooms             int            `json:"bedrooms"`
	Bathrooms            int            `json:"bathrooms"`
	AreaSquareMeters     float64        `json:"area_sq_m"`
	ThumbnailURL         string         `json:"thumbnail_url"`
	Rating               float64        `json:"rating"`
	AvailableFrom        string         `json:"available_from"`
}

type fixtureAddress struct {
	Line1   string  `json:"line1"`
	Line2   string  `json:"line2"`
	City    string  `json:"city"`
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

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
