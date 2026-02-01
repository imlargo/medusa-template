package bootstrap

import (
	"github.com/gin-gonic/gin"
	medusadocs "github.com/imlargo/medusa/pkg/medusa/core/docs"
	"github.com/imlargo/medusa/pkg/medusa/middleware"
)

// SetupRoutes configures all application routes.
func SetupRoutes(router *gin.Engine, c *Container) {
	// Global middleware
	router.Use(middleware.CreateRequestIDMiddleware())
	router.Use(middleware.NewCorsMiddleware(c.Config.Server.Host, []string{}))

	// Docs
	medusadocs.RegisterDocs(router, c.Config.Server.Host, c.Config.Server.Port)

	// Health (public)
	router.GET("/health", c.Handlers.Health.Health)
	router.GET("/ready", c.Handlers.Health.Ready)

	// API v1
	v1 := router.Group("/v1")
	setupPublicRoutes(v1, c)
	setupProtectedRoutes(v1, c)
}

func setupPublicRoutes(rg *gin.RouterGroup, c *Container) {
	auth := rg.Group("/auth")
	{
		auth.POST("/login", c.Handlers.Auth.LoginWithPassword)
		auth.POST("/register", c.Handlers.Auth.Register)
	}
}

func setupProtectedRoutes(rg *gin.RouterGroup, c *Container) {
	// Middleware stack
	authMiddleware := middleware.NewJWTAuthMiddleware(c.JWT)

	protected := rg.Group("")
	protected.Use(authMiddleware)

	// Optional middleware
	if c.Metrics != nil {
		protected.Use(middleware.NewMetricsMiddleware(c.Metrics))
	}
	if c.RateLimiter != nil {
		protected.Use(middleware.NewRateLimiterMiddleware(c.RateLimiter))
	}

	// Protected auth routes
	protected.GET("/auth/user", c.Handlers.Auth.GetUser)

	// Add more route groups here:
	// setupUserRoutes(protected, c)
	// setupProductRoutes(protected, c)
}

// Template for adding new route groups:
//
// func setupProductRoutes(rg *gin.RouterGroup, c *Container) {
//     products := rg.Group("/products")
//     {
//         products.GET("", c.Handlers.Product.List)
//         products.GET("/:id", c.Handlers.Product.Get)
//         products.POST("", c.Handlers.Product.Create)
//         products.PUT("/:id", c.Handlers.Product.Update)
//         products.DELETE("/:id", c.Handlers.Product.Delete)
//     }
// }
