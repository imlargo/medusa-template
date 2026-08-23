package bootstrap

import (
	"github.com/gin-gonic/gin"
	medusadocs "github.com/imlargo/medusa/pkg/medusa/core/docs"
	"github.com/imlargo/medusa/pkg/medusa/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// apiGroups are the two authorization scopes every feature registers routes
// into. Both already carry the shared middleware stack.
type apiGroups struct {
	// public needs no credentials.
	public *gin.RouterGroup
	// protected requires a valid JWT access token.
	protected *gin.RouterGroup
}

// newRouter builds the Gin engine: router mode, global middleware and routes.
func newRouter(c *Container) *gin.Engine {
	gin.SetMode(ginMode(c))

	router := gin.New()
	router.Use(gin.Recovery(), accessLogMiddleware(c.Logger))

	registerRoutes(router, c)

	return router
}

// ginMode maps the deployment environment onto Gin's own mode. Release mode
// disables Gin's debug output and startup warnings.
func ginMode(c *Container) string {
	if c.Config.Environment.IsProduction() {
		return gin.ReleaseMode
	}
	return gin.DebugMode
}

// registerRoutes installs the global middleware and every route of the API.
//
// Middleware order matters: the request ID comes first so everything downstream
// can log it, CORS runs before any middleware that may abort so preflight
// responses keep their headers, and metrics wrap the rest to measure the real
// end-to-end latency of a request.
func registerRoutes(router *gin.Engine, c *Container) {
	router.Use(
		middleware.CreateRequestIDMiddleware(),
		middleware.NewCorsMiddleware(c.Config.Server.Host, c.Config.CORS.AllowedOrigins),
	)

	if c.Metrics != nil {
		router.Use(middleware.NewMetricsMiddleware(c.Metrics))

		// Prometheus scrape endpoint. It exposes internal request paths and
		// latencies, so restrict it at the ingress or network layer.
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	medusadocs.RegisterDocs(router, c.Config.Server.Host, c.Config.Server.Port)

	// Probes sit outside the API group on purpose: no auth and no rate limit, so
	// orchestrators keep getting an answer even while the app sheds load.
	router.GET("/health", c.Handlers.Health.Health)
	router.GET("/ready", c.Handlers.Health.Ready)

	v1 := router.Group("/v1")

	// Rate limiting covers public endpoints too: unauthenticated login and
	// register calls are precisely what needs throttling.
	if c.RateLimiter != nil {
		v1.Use(middleware.NewRateLimiterMiddleware(c.RateLimiter))
	}

	groups := apiGroups{
		public:    v1.Group(""),
		protected: v1.Group("", middleware.NewJWTAuthMiddleware(c.JWT)),
	}

	registerAuthRoutes(groups, c)

	// Add more feature route groups here:
	// registerProductRoutes(groups, c)
}

func registerAuthRoutes(g apiGroups, c *Container) {
	public := g.public.Group("/auth")
	public.POST("/login", c.Handlers.Auth.LoginWithPassword)
	public.POST("/register", c.Handlers.Auth.Register)

	protected := g.protected.Group("/auth")
	protected.GET("/user", c.Handlers.Auth.GetUser)
}

// Template for adding a new feature:
//
//	func registerProductRoutes(g apiGroups, c *Container) {
//	    products := g.protected.Group("/products")
//	    products.GET("", c.Handlers.Product.List)
//	    products.GET("/:id", c.Handlers.Product.Get)
//	    products.POST("", c.Handlers.Product.Create)
//	    products.PUT("/:id", c.Handlers.Product.Update)
//	    products.DELETE("/:id", c.Handlers.Product.Delete)
//	}
