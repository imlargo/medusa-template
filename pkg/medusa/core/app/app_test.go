package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/imlargo/medusa/pkg/medusa/core/logger"
)

// fakeServer is a server whose Start blocks until stopped or ctx is done.
type fakeServer struct {
	startErr error
	stopErr  error
	started  atomic.Bool
	stopped  atomic.Bool
	release  chan struct{}
}

func newFakeServer() *fakeServer {
	return &fakeServer{release: make(chan struct{})}
}

func (s *fakeServer) Start(ctx context.Context) error {
	s.started.Store(true)
	if s.startErr != nil {
		return s.startErr
	}

	select {
	case <-ctx.Done():
	case <-s.release:
	}
	return nil
}

func (s *fakeServer) Stop(context.Context) error {
	s.stopped.Store(true)
	close(s.release)
	return s.stopErr
}

// quietLogger keeps test output readable while still exercising the log calls.
func quietLogger(t *testing.T) *logger.Logger {
	t.Helper()

	core, _ := observer.New(zap.InfoLevel)
	return &logger.Logger{Logger: zap.New(core)}
}

// Run used to return nil unconditionally, so a crashed server was
// indistinguishable from a clean shutdown.
func TestRunReturnsAServerFailure(t *testing.T) {
	boom := errors.New("listen: address already in use")
	srv := newFakeServer()
	srv.startErr = boom

	app := NewApp(WithName("test"), WithServer(srv), WithLogger(quietLogger(t)))

	err := app.Run(context.Background())

	if !errors.Is(err, boom) {
		t.Fatalf("Run() = %v, want it to wrap %v", err, boom)
	}
	if !srv.stopped.Load() {
		t.Error("server was not stopped after the failure")
	}
}

func TestRunReturnsNilOnAContextCancellation(t *testing.T) {
	srv := newFakeServer()
	app := NewApp(WithServer(srv), WithLogger(quietLogger(t)))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	if err := app.Run(ctx); err != nil {
		t.Errorf("Run() = %v, want nil on a deliberate shutdown", err)
	}
	if !srv.started.Load() {
		t.Error("server never started")
	}
	if !srv.stopped.Load() {
		t.Error("server was not stopped")
	}
}

func TestRunAbortsWhenAnOnStartHookFails(t *testing.T) {
	boom := errors.New("migrations failed")
	srv := newFakeServer()

	app := NewApp(
		WithServer(srv),
		WithLogger(quietLogger(t)),
		WithOnStart(func(context.Context) error { return boom }),
	)

	err := app.Run(context.Background())

	if !errors.Is(err, boom) {
		t.Fatalf("Run() = %v, want it to wrap %v", err, boom)
	}
	if srv.started.Load() {
		t.Error("server started even though an onStart hook failed")
	}
}

func TestRunRunsEveryOnStopHookAndCollectsTheirErrors(t *testing.T) {
	first := errors.New("first hook")
	second := errors.New("second hook")

	srv := newFakeServer()
	var secondRan atomic.Bool

	app := NewApp(
		WithServer(srv),
		WithLogger(quietLogger(t)),
		WithOnStop(func(context.Context) error { return first }),
		WithOnStop(func(context.Context) error {
			secondRan.Store(true)
			return second
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx)

	if !secondRan.Load() {
		t.Error("the second onStop hook was skipped after the first failed")
	}
	for _, want := range []error{first, second} {
		if !errors.Is(err, want) {
			t.Errorf("Run() = %v, want it to wrap %v", err, want)
		}
	}
}

// Shutdown hooks must not inherit the cancellation that triggered the shutdown,
// or they are dead on arrival.
func TestOnStopHooksGetALiveContext(t *testing.T) {
	srv := newFakeServer()
	var hookCtxErr error

	app := NewApp(
		WithServer(srv),
		WithLogger(quietLogger(t)),
		WithOnStop(func(ctx context.Context) error {
			hookCtxErr = ctx.Err()
			return nil
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if hookCtxErr != nil {
		t.Errorf("onStop hook received a canceled context (%v), want a live one", hookCtxErr)
	}
}

func TestRunCollectsStopErrors(t *testing.T) {
	boom := errors.New("shutdown timed out")
	srv := newFakeServer()
	srv.stopErr = boom

	app := NewApp(WithServer(srv), WithLogger(quietLogger(t)))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := app.Run(ctx); !errors.Is(err, boom) {
		t.Errorf("Run() = %v, want it to wrap %v", err, boom)
	}
}
