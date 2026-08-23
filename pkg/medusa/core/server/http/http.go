// Package http provides an HTTP server implementation on top of Gin.
//
// The server wraps a Gin engine with lifecycle management, graceful shutdown and
// structured logging, and implements server.Server so the app framework can
// manage it alongside other servers.
//
//	log := logger.NewLogger()
//	router := gin.New()
//	router.GET("/health", healthHandler)
//
//	srv := http.NewServer(router, log,
//	    http.WithServerHost("0.0.0.0"),
//	    http.WithServerPort(8080),
//	    http.WithShutdownTimeout(15*time.Second),
//	)
//
//	if err := srv.Start(ctx); err != nil {
//	    return err
//	}
//
// Start returns its error rather than terminating the process: deciding whether
// a failed listen is fatal belongs to the application, not to this package.
package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa/core/logger"
	"go.uber.org/zap"
)

// Defaults applied when the matching option is not supplied.
const (
	// DefaultShutdownTimeout bounds how long in-flight requests get to finish.
	DefaultShutdownTimeout = 10 * time.Second

	// DefaultReadHeaderTimeout caps how long a client may take to send its
	// request headers. Without it a handful of connections trickling headers one
	// byte at a time can hold the server open indefinitely (Slowloris).
	DefaultReadHeaderTimeout = 10 * time.Second
)

// Server is an HTTP server wrapping a Gin engine. It embeds *gin.Engine, so
// every routing method remains available directly.
//
// Route configuration is safe from multiple goroutines before Start; Start
// itself must be called once.
type Server struct {
	*gin.Engine

	host              string
	port              int
	shutdownTimeout   time.Duration
	readHeaderTimeout time.Duration
	logger            *logger.Logger

	// mu guards httpSrv, which Start publishes and Stop reads, possibly from
	// another goroutine.
	mu       sync.Mutex
	httpSrv  *http.Server
	stopOnce sync.Once
}

// Option configures a Server.
type Option func(s *Server)

// NewServer creates an HTTP server for the given engine and logger.
// It listens on port 8080 of every interface unless told otherwise.
func NewServer(engine *gin.Engine, log *logger.Logger, opts ...Option) *Server {
	s := &Server{
		Engine:            engine,
		port:              8080,
		shutdownTimeout:   DefaultShutdownTimeout,
		readHeaderTimeout: DefaultReadHeaderTimeout,
		logger:            log,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// WithServerHost sets the bind address. Use "0.0.0.0" for every interface, or
// "localhost" to accept only local connections. Empty means every interface.
func WithServerHost(host string) Option {
	return func(s *Server) { s.host = host }
}

// WithServerPort sets the TCP port to listen on.
func WithServerPort(port int) Option {
	return func(s *Server) { s.port = port }
}

// WithShutdownTimeout sets how long Stop waits for in-flight requests before
// closing connections. It should exceed your slowest normal request and stay
// below your orchestrator's termination grace period.
func WithShutdownTimeout(d time.Duration) Option {
	return func(s *Server) { s.shutdownTimeout = d }
}

// WithReadHeaderTimeout caps how long a client may take to send request headers.
// Set it to a negative duration to disable the limit, which is only reasonable
// behind a proxy that already enforces one.
func WithReadHeaderTimeout(d time.Duration) Option {
	return func(s *Server) { s.readHeaderTimeout = d }
}

// Addr returns the address the server listens on.
func (s *Server) Addr() string {
	return net.JoinHostPort(s.host, strconv.Itoa(s.port))
}

// Start serves HTTP requests and blocks until the server is stopped, the
// context is canceled, or serving fails.
//
// It returns nil on a graceful shutdown and an error if the listener cannot be
// opened or serving fails unexpectedly. It never terminates the process; the
// caller decides what a failure means.
func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.Addr(),
		Handler:           s.Engine,
		ReadHeaderTimeout: s.readHeaderTimeout,
	}
	if s.readHeaderTimeout < 0 {
		srv.ReadHeaderTimeout = 0
	}

	s.mu.Lock()
	s.httpSrv = srv
	s.mu.Unlock()

	// The Server contract promises Start also returns when ctx is done, not only
	// when Stop is called, so cancellation has to reach Shutdown. The watcher
	// exits either way: whichever of the two happens first wins.
	returned := make(chan struct{})
	defer close(returned)
	go func() {
		select {
		case <-ctx.Done():
			_ = s.Stop(context.WithoutCancel(ctx))
		case <-returned:
		}
	}()

	s.logger.Info("http server listening", zap.String("addr", srv.Addr))

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http on %s: %w", srv.Addr, err)
	}

	return nil
}

// Stop gracefully shuts the server down, waiting up to the configured shutdown
// timeout for in-flight requests to finish before closing connections.
//
// Calling it more than once, or before Start, is safe and does nothing.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	srv := s.httpSrv
	s.mu.Unlock()

	if srv == nil {
		return nil
	}

	var err error
	s.stopOnce.Do(func() {
		s.logger.Info("shutting down http server", zap.Duration("timeout", s.shutdownTimeout))

		// The parent's cancellation is dropped deliberately: shutdown is often
		// triggered *by* a canceled context, and inheriting it would abort
		// in-flight requests immediately instead of draining them. The timeout
		// below is what bounds the wait.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.shutdownTimeout)
		defer cancel()

		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			err = fmt.Errorf("shutdown http server: %w", shutdownErr)
			s.logger.Error("graceful shutdown failed, connections were closed", zap.Error(shutdownErr))
			return
		}

		s.logger.Info("http server stopped")
	})

	return err
}
