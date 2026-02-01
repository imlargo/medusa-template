package bootstrap

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/internal/config"
	"github.com/imlargo/medusa/pkg/medusa/core/app"
	"github.com/imlargo/medusa/pkg/medusa/core/server/http"
	"github.com/imlargo/medusa/pkg/medusa/tools"
)

// App wraps the Medusa app with its container.
type App struct {
	*app.App
	Container *Container
}

// New creates a fully configured application.
// Uses DefaultOptions (all features enabled).
func New(name string) (*App, error) {
	return NewWithOptions(name, DefaultOptions())
}

// NewMinimal creates a lightweight application.
// Only database and JWT, no Redis/Storage/Metrics.
func NewMinimal(name string) (*App, error) {
	return NewWithOptions(name, MinimalOptions())
}

// NewWithOptions creates an application with custom options.
func NewWithOptions(name string, opts Options) (*App, error) {
	// Load config
	cfg := config.LoadConfig()

	// Build container with all dependencies
	container, err := NewContainer(&cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	// Setup router
	router := gin.Default()
	SetupRoutes(router, container)

	// Create HTTP server
	srv := http.NewServer(
		router,
		container.Logger,
		http.WithServerHost(cfg.Server.Host),
		http.WithServerPort(cfg.Server.Port),
	)

	// Create Medusa app
	medusaApp := app.NewApp(
		app.WithName(name),
		app.WithServer(srv),
	)

	// Print banner
	printBanner(name, &cfg)

	return &App{
		App:       medusaApp,
		Container: container,
	}, nil
}

func printBanner(name string, cfg *config.Config) {
	fmt.Println("")
	fmt.Println("🪼 Medusa")
	fmt.Println("─────────────────────────────────")
	fmt.Printf("   App:    %s\n", name)
	fmt.Printf("   Server: %s\n", tools.GetFullAppUrl(cfg.Server.Host, cfg.Server.Port))
	fmt.Printf("   Docs:   %s\n", tools.GetFullDocsUrl(cfg.Server.Host, cfg.Server.Port))
	fmt.Println("─────────────────────────────────")
	fmt.Println("")
}
