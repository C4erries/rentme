package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rentme/internal/app/commands"
	availabilityapp "rentme/internal/app/handlers/availability"
	bookingapp "rentme/internal/app/handlers/booking"
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

	app.seedDemoData(ctx, logger)

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

func (a application) seedDemoData(ctx context.Context, logger *slog.Logger) {
	listing, err := listings.NewListing(listings.CreateListingParams{
		ID:          "listing-demo-1",
		Host:        "host-demo",
		Title:       "Demo Loft Downtown",
		Description: "A minimal listing used to demonstrate the booking API.",
		Address: listings.Address{
			Line1:   "Main Street 1",
			City:    "Prague",
			Country: "CZ",
		},
		Amenities:            []string{"wifi", "kitchen"},
		GuestsLimit:          4,
		MinNights:            1,
		MaxNights:            30,
		HouseRules:           []string{"no parties"},
		CancellationPolicyID: "flexible",
		Now:                  time.Now(),
	})
	if err != nil {
		logger.Error("failed to create demo listing", "error", err)
		return
	}
	if err := listing.Activate(time.Now()); err != nil {
		logger.Error("failed to activate demo listing", "error", err)
		return
	}
	if err := a.repos.listings.Save(ctx, listing); err != nil {
		logger.Error("cannot seed listing repository", "error", err)
		return
	}
	if _, err := a.repos.availability.Calendar(ctx, listing.ID); err != nil {
		logger.Error("cannot seed availability repository", "error", err)
		return
	}
	logger.Info("demo listing ready", "listing_id", listing.ID)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
