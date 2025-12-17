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

type ReviewsHTTP interface {
	Submit(c *gin.Context)
	ListByListing(c *gin.Context)
}

type HostListingHTTP interface {
	List(c *gin.Context)
	Create(c *gin.Context)
	Get(c *gin.Context)
	Update(c *gin.Context)
	Publish(c *gin.Context)
	Unpublish(c *gin.Context)
	PriceSuggestion(c *gin.Context)
	UploadPhoto(c *gin.Context)
}

type Handlers struct {
	Booking        BookingHTTP
	Availability   AvailabilityHTTP
	Listing        ListingHTTP
	HostListing    HostListingHTTP
	Chat           ChatHTTP
	Auth           AuthHTTP
	Reviews        ReviewsHTTP
	Me             MeHTTP
	Admin          AdminHTTP
	AuthMiddleware gin.HandlerFunc
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
	router.MaxMultipartMemory = 16 << 20 // 16 MiB guardrail for uploads
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
	if h.AuthMiddleware != nil {
		router.Use(h.AuthMiddleware)
	}

	registerSwaggerRoutes(router)

	router.GET("/livez", health.Livez)
	router.GET("/readyz", health.Readyz)

	api := router.Group("/api/v1")
	if h.Auth != nil {
		api.POST("/auth/register", h.Auth.Register)
		api.POST("/auth/login", h.Auth.Login)
		api.POST("/auth/logout", h.Auth.Logout)
		api.GET("/auth/me", h.Auth.Me)
	}
	if h.Booking != nil {
		api.POST("/bookings", h.Booking.Create)
		api.POST("/bookings/:id/accept", h.Booking.Accept)
	}
	if h.Reviews != nil {
		api.POST("/bookings/:id/review", h.Reviews.Submit)
		api.GET("/listings/:id/reviews", h.Reviews.ListByListing)
	}
	if h.Availability != nil {
		api.GET("/listings/:id/calendar", h.Availability.Calendar)
	}
	if h.Listing != nil {
		api.GET("/listings", h.Listing.Catalog)
		api.GET("/listings/:id/overview", h.Listing.Overview)
	}
	if h.Chat != nil {
		api.POST("/chats", h.Chat.CreateDirectConversation)
		api.GET("/me/chats", h.Chat.ListMyConversations)
		api.GET("/chats/:id/messages", h.Chat.ListMessages)
		api.POST("/chats/:id/messages", h.Chat.SendMessage)
		api.POST("/chats/:id/read", h.Chat.MarkRead)
		api.POST("/listings/:id/chat", h.Chat.CreateListingConversation)
	}
	if h.HostListing != nil {
		hostGroup := api.Group("/host/listings")
		hostGroup.GET("", h.HostListing.List)
		hostGroup.POST("", h.HostListing.Create)
		hostGroup.GET("/:id", h.HostListing.Get)
		hostGroup.PUT("/:id", h.HostListing.Update)
		hostGroup.POST("/:id/publish", h.HostListing.Publish)
		hostGroup.POST("/:id/unpublish", h.HostListing.Unpublish)
		hostGroup.POST("/:id/price-suggestion", h.HostListing.PriceSuggestion)
		hostGroup.POST("/:id/photos", h.HostListing.UploadPhoto)
	}
	if h.Me != nil {
		meGroup := api.Group("/me")
		meGroup.GET("/bookings", h.Me.ListBookings)
	}
	if h.Admin != nil {
		adminGroup := api.Group("/admin")
		adminGroup.GET("/users", h.Admin.ListUsers)
		adminGroup.GET("/ml/metrics", h.Admin.MLMetrics)
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
