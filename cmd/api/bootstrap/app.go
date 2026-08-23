// Package bootstrap assembles the API: it loads the configuration, wires every
// dependency into a Container, builds the router and returns a runnable App.
package bootstrap

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/imlargo/medusa/internal/config"
	"github.com/imlargo/medusa/pkg/medusa/core/app"
	"github.com/imlargo/medusa/pkg/medusa/core/server/http"
	"github.com/imlargo/medusa/pkg/medusa/tools"
)

// App is a Medusa application together with the container holding its
// dependencies. Close releases them once Run returns.
type App struct {
	*app.App

	Container *Container
}

// New builds a fully featured application: database, Redis, object storage and
// metrics, as far as the environment configures them.
//
// ctx bounds the connectivity checks made while dialing dependencies, so a
// canceled context aborts a slow startup instead of hanging.
func New(ctx context.Context, name string) (*App, error) {
	return NewWithOptions(ctx, name, DefaultOptions())
}

// NewMinimal builds a lightweight application with only the database and JWT.
func NewMinimal(ctx context.Context, name string) (*App, error) {
	return NewWithOptions(ctx, name, MinimalOptions())
}

// NewWithOptions builds an application with an explicit set of optional
// components. It returns an error if the configuration is invalid or any
// required dependency is unreachable.
func NewWithOptions(ctx context.Context, name string, opts Options) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	container, err := NewContainer(ctx, cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("initialize %s: %w", name, err)
	}

	server := http.NewServer(
		newRouter(container),
		container.Logger,
		http.WithServerHost(cfg.Server.Host),
		http.WithServerPort(cfg.Server.Port),
	)

	printBanner(os.Stdout, name, cfg)

	return &App{
		App: app.NewApp(
			app.WithName(name),
			app.WithServer(server),
			app.WithLogger(container.Logger),
			// Runs before the HTTP server stops, so open SSE streams end on
			// their own instead of holding the shutdown open.
			app.WithOnStop(container.DrainEventStreams),
		),
		Container: container,
	}, nil
}

// Close releases every resource held by the application. Defer it right after
// construction; calling it more than once is safe.
func (a *App) Close() error {
	if a == nil || a.Container == nil {
		return nil
	}
	return a.Container.Close()
}

// printBanner writes a short startup summary with the URLs the app serves.
func printBanner(w io.Writer, name string, cfg *config.Config) {
	const separator = "─────────────────────────────────"

	fields := [][2]string{
		{"App", name},
		{"Env", string(cfg.Environment)},
		{"Server", tools.GetFullAppUrl(cfg.Server.Host, cfg.Server.Port)},
		{"Docs", tools.GetFullDocsUrl(cfg.Server.Host, cfg.Server.Port)},
	}

	var banner strings.Builder
	fmt.Fprintf(&banner, "\n🪼 Medusa\n%s\n", separator)
	for _, field := range fields {
		fmt.Fprintf(&banner, "   %-7s %s\n", field[0]+":", field[1])
	}
	fmt.Fprintf(&banner, "%s\n\n", separator)

	// Nothing actionable if the banner cannot be written.
	_, _ = io.WriteString(w, banner.String())
}
