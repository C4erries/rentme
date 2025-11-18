package ginserver

import (
	"net/http"

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

type Handlers struct {
	Booking      BookingHTTP
	Availability AvailabilityHTTP
}

func NewServer(cfg config.Config, obsMW obs.Middleware, health obs.HealthHandlers, h Handlers) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(obsMW.RequestID())
	router.Use(obsMW.LoggerMiddleware())

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

	return &http.Server{Addr: cfg.HTTPAddr, Handler: router}
}
