// Package app provides application lifecycle management for Medusa applications.
// It handles graceful startup and shutdown of servers, signal handling, and context propagation.
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/imlargo/medusa/pkg/medusa/core/server"
)

// App represents a Medusa application with one or more servers.
// It manages the application lifecycle including graceful shutdown.
type App struct {
	name    string
	servers []server.Server
	onStart []func(ctx context.Context) error
	onStop  []func(ctx context.Context) error
}

// Option is a functional option for configuring an App.
type Option func(a *App)

// NewApp creates a new Medusa application with the provided options.
func NewApp(opts ...Option) *App {
	a := &App{
		onStart: make([]func(ctx context.Context) error, 0),
		onStop:  make([]func(ctx context.Context) error, 0),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// WithServer adds one or more servers to the application.
// Multiple servers can run concurrently and will be managed together.
func WithServer(servers ...server.Server) Option {
	return func(a *App) {
		a.servers = append(a.servers, servers...)
	}
}

// WithName sets the application name.
func WithName(name string) Option {
	return func(a *App) {
		a.name = name
	}
}

// WithOnStart registers a hook to be executed before servers start.
// Hooks are executed sequentially in the order they were registered.
// If any hook returns an error, the application startup is aborted.
func WithOnStart(fn func(ctx context.Context) error) Option {
	return func(a *App) {
		a.onStart = append(a.onStart, fn)
	}
}

// WithOnStop registers a hook to be executed during graceful shutdown.
// Hooks are executed sequentially in the order they were registered.
// All hooks are executed even if some return errors.
func WithOnStop(fn func(ctx context.Context) error) Option {
	return func(a *App) {
		a.onStop = append(a.onStop, fn)
	}
}

// Run starts the application and all configured servers.
// It blocks until a termination signal (SIGINT, SIGTERM) is received or the context is canceled.
// All servers are started concurrently and stopped gracefully on shutdown.
//
// Lifecycle flow:
// 1. Execute all onStart hooks sequentially
// 2. Start all servers concurrently
// 3. Wait for termination signal or context cancellation
// 4. Execute all onStop hooks sequentially
// 5. Stop all servers gracefully
func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Execute onStart hooks
	if a.name != "" {
		log.Printf("Starting application: %s", a.name)
	}

	for i, fn := range a.onStart {
		if err := fn(ctx); err != nil {
			return fmt.Errorf("onStart hook %d failed: %w", i, err)
		}
	}

	// Channel to collect server errors
	errChan := make(chan error, len(a.servers))
	var wg sync.WaitGroup

	// Start all servers
	for _, srv := range a.servers {
		wg.Add(1)
		go func(srv server.Server) {
			defer wg.Done()
			if err := srv.Start(ctx); err != nil {
				log.Printf("Server start error: %v", err)
				errChan <- err
			}
		}(srv)
	}

	// Setup signal handling
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	// Wait for shutdown trigger
	select {
	case <-signals:
		log.Println("Received termination signal, shutting down gracefully...")
	case <-ctx.Done():
		log.Println("Context canceled, shutting down...")
	case err := <-errChan:
		log.Printf("Server error occurred: %v, initiating shutdown...", err)
		cancel()
	}

	// Execute onStop hooks
	log.Println("Executing shutdown hooks...")
	for i, fn := range a.onStop {
		if err := fn(ctx); err != nil {
			log.Printf("onStop hook %d error: %v", i, err)
		}
	}

	// Gracefully stop all servers
	log.Println("Stopping servers...")
	for _, srv := range a.servers {
		if err := srv.Stop(ctx); err != nil {
			log.Printf("Server stop error: %v", err)
		}
	}

	// Wait for all server goroutines to finish
	wg.Wait()

	if a.name != "" {
		log.Printf("Application %s stopped", a.name)
	}

	return nil
}
