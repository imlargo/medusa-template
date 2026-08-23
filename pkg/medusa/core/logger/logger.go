// Package logger provides structured logging capabilities using Uber's Zap library.
//
// The Logger wraps zap.Logger to provide high-performance, structured, and leveled logging
// suitable for production environments. It supports multiple log levels (Debug, Info, Warn, Error)
// and automatically adds structured fields to log entries.
//
// Example usage:
//
//	log := logger.NewLogger()
//	defer log.Sync() // Flush any buffered log entries
//
//	log.Info("Server started",
//	    zap.String("host", "localhost"),
//	    zap.Int("port", 8080),
//	)
//
//	log.Error("Failed to connect to database",
//	    zap.Error(err),
//	    zap.String("database", "postgres"),
//	)
//
// For more information on zap fields and usage, see: https://pkg.go.dev/go.uber.org/zap
package logger

import (
	"fmt"

	"go.uber.org/zap"
)

// Logger wraps zap.Logger to provide structured logging capabilities.
// It embeds *zap.Logger, making all zap methods directly available.
//
// The logger is safe for concurrent use by multiple goroutines.
type Logger struct {
	*zap.Logger
}

// NewLogger returns a production logger: JSON encoding, Info level and above,
// caller information on warnings and errors.
//
// Flush it before the process exits so buffered entries are not lost:
//
//	log := logger.NewLogger()
//	defer log.Sync()
//
// It never fails in practice; NewProduction only errors on an invalid
// configuration, and this one is fixed. Use New if you build your own.
func NewLogger() *Logger {
	log, err := zap.NewProduction()
	if err != nil {
		// Unreachable with a fixed config, but silence is the one outcome a
		// logger must never produce.
		panic(fmt.Errorf("build production logger: %w", err))
	}

	return &Logger{Logger: log}
}

// NewDevelopmentLogger returns a console logger with human-readable output and
// Debug level enabled, for local development.
func NewDevelopmentLogger() (*Logger, error) {
	log, err := zap.NewDevelopment()
	if err != nil {
		return nil, fmt.Errorf("build development logger: %w", err)
	}

	return &Logger{Logger: log}, nil
}

// New builds a logger from an explicit zap configuration, for callers that need
// a different level, encoding or output than the two presets above.
func New(cfg zap.Config) (*Logger, error) {
	log, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}

	return &Logger{Logger: log}, nil
}
