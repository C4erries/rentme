package ginserver

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	gin "github.com/gin-gonic/gin"

	"rentme/internal/infra/config"
	"rentme/internal/infra/obs"
)

type BookingHTTP interface {
	Create(c *gin.Context)
	Accept(c *gin.Context)
}

type AvailabilityHTTP interface {
	Calendar(c *gin.Context)
}

type ListingHTTP interface {
	Catalog(c *gin.Context)
	Overview(c *gin.Context)
}

type Handlers struct {
	Booking      BookingHTTP
	Availability AvailabilityHTTP
	Listing      ListingHTTP
}

func NewServer(cfg config.Config, obsMW obs.Middleware, health obs.HealthHandlers, h Handlers) *http.Server {
	mode := configureGinMode(cfg.Env)
	if obsMW.Logger != nil {
		obsMW.Logger.Info("gin initialized", "mode", mode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(obsMW.RequestID())
	router.Use(obsMW.LoggerMiddleware())
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "Idempotency-Key"},
		ExposeHeaders: []string{
			"Content-Length",
			"Content-Type",
			"X-Request-ID",
		},
		MaxAge: 12 * time.Hour,
	}))

	registerSwaggerRoutes(router)

	router.GET("/livez", health.Livez)
	router.GET("/readyz", health.Readyz)

	api := router.Group("/api/v1")
	if h.Booking != nil {
		api.POST("/bookings", h.Booking.Create)
		api.POST("/bookings/:id/accept", h.Booking.Accept)
	}
	if h.Availability != nil {
		api.GET("/listings/:id/calendar", h.Availability.Calendar)
	}
	if h.Listing != nil {
		api.GET("/listings", h.Listing.Catalog)
		api.GET("/listings/:id/overview", h.Listing.Overview)
	}

	return &http.Server{Addr: cfg.HTTPAddr, Handler: router}
}

func configureGinMode(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "debug":
		gin.SetMode(gin.DebugMode)
		return gin.DebugMode
	case "test", "testing":
		gin.SetMode(gin.TestMode)
		return gin.TestMode
	default:
		gin.SetMode(gin.ReleaseMode)
		return gin.ReleaseMode
	}
}
