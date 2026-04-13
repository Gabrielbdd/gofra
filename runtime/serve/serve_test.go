package runtimeserve_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	runtimeserve "databit.com.br/gofra/runtime/serve"
)

func TestStartupMarksHealthAfterBind(t *testing.T) {
	t.Parallel()
	h := &spyHealth{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runtimeserve.Serve(ctx, runtimeserve.Config{
			Handler:             http.NotFoundHandler(),
			Addr:                freeAddr(t),
			Health:              h,
			ReadinessDrainDelay: 1 * time.Millisecond,
		})
	}()

	// Wait for startup.
	waitForHealth(t, h, 2*time.Second)

	if !h.started.Load() {
		t.Fatal("MarkStarted() was not called")
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestContextCancellationTriggersShutdown(t *testing.T) {
	t.Parallel()
	h := &spyHealth{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runtimeserve.Serve(ctx, runtimeserve.Config{
			Handler:             http.NotFoundHandler(),
			Addr:                freeAddr(t),
			Health:              h,
			ReadinessDrainDelay: 1 * time.Millisecond,
			ShutdownTimeout:     1 * time.Second,
		})
	}()

	waitForHealth(t, h, 2*time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve() did not return after context cancellation")
	}

	if !h.notReady.Load() {
		t.Error("SetNotReady() was not called during shutdown")
	}
}

func TestOnShutdownCalled(t *testing.T) {
	t.Parallel()
	h := &spyHealth{}
	ctx, cancel := context.WithCancel(context.Background())
	var shutdownCalled atomic.Bool

	done := make(chan error, 1)
	go func() {
		done <- runtimeserve.Serve(ctx, runtimeserve.Config{
			Handler:                 http.NotFoundHandler(),
			Addr:                    freeAddr(t),
			Health:                  h,
			ReadinessDrainDelay:     1 * time.Millisecond,
			ResourceShutdownTimeout: 1 * time.Second,
			OnShutdown: func(context.Context) error {
				shutdownCalled.Store(true)
				return nil
			},
		})
	}()

	waitForHealth(t, h, 2*time.Second)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
	if !shutdownCalled.Load() {
		t.Error("OnShutdown was not called")
	}
}

func TestInFlightRequestCompletes(t *testing.T) {
	t.Parallel()
	h := &spyHealth{}
	ctx, cancel := context.WithCancel(context.Background())

	requestStarted := make(chan struct{})
	requestDone := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-requestDone
		w.WriteHeader(http.StatusOK)
	})

	addr := freeAddr(t)
	done := make(chan error, 1)
	go func() {
		done <- runtimeserve.Serve(ctx, runtimeserve.Config{
			Handler:             handler,
			Addr:                addr,
			Health:              h,
			ReadinessDrainDelay: 1 * time.Millisecond,
			ShutdownTimeout:     5 * time.Second,
		})
	}()

	waitForHealth(t, h, 2*time.Second)

	// Start an in-flight request.
	respCh := make(chan *http.Response, 1)
	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/slow", addr))
		if err == nil {
			respCh <- resp
		}
	}()

	// Wait for handler to start processing.
	<-requestStarted

	// Trigger shutdown while request is in-flight.
	cancel()

	// Let the request complete.
	close(requestDone)

	// The request should get a response.
	select {
	case resp := <-respCh:
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("response status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	case <-time.After(5 * time.Second):
		t.Error("in-flight request did not complete")
	}

	if err := <-done; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestBindFailureReturnsError(t *testing.T) {
	t.Parallel()

	// Bind a port first to cause a conflict.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	err = runtimeserve.Serve(context.Background(), runtimeserve.Config{
		Handler: http.NotFoundHandler(),
		Addr:    ln.Addr().String(),
	})
	if err == nil {
		t.Fatal("Serve() returned nil, want bind error")
	}
}

func TestShutdownWithNoInflightRequests(t *testing.T) {
	t.Parallel()
	h := &spyHealth{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runtimeserve.Serve(ctx, runtimeserve.Config{
			Handler:             http.NotFoundHandler(),
			Addr:                freeAddr(t),
			Health:              h,
			ReadinessDrainDelay: 1 * time.Millisecond,
			ShutdownTimeout:     1 * time.Second,
		})
	}()

	waitForHealth(t, h, 2*time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve() hung with no in-flight requests")
	}
}

func TestNilHealthIsSafe(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	addr := freeAddr(t)

	done := make(chan error, 1)
	go func() {
		done <- runtimeserve.Serve(ctx, runtimeserve.Config{
			Handler:             http.NotFoundHandler(),
			Addr:                addr,
			ReadinessDrainDelay: 1 * time.Millisecond,
		})
	}()

	// Wait for the server to accept connections.
	waitForListening(t, addr, 2*time.Second)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestNilOnShutdownIsSafe(t *testing.T) {
	t.Parallel()
	h := &spyHealth{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runtimeserve.Serve(ctx, runtimeserve.Config{
			Handler:             http.NotFoundHandler(),
			Addr:                freeAddr(t),
			Health:              h,
			ReadinessDrainDelay: 1 * time.Millisecond,
			OnShutdown:          nil,
		})
	}()

	waitForHealth(t, h, 2*time.Second)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

func TestDefaultsApplied(t *testing.T) {
	t.Parallel()

	// Verify Serve works with zero-value timeouts (defaults applied).
	h := &spyHealth{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runtimeserve.Serve(ctx, runtimeserve.Config{
			Handler: http.NotFoundHandler(),
			Addr:    freeAddr(t),
			Health:  h,
			// All timeout fields intentionally zero — defaults should kick in.
		})
	}()

	waitForHealth(t, h, 2*time.Second)

	// Cancel immediately — if defaults were not applied this would hang
	// or panic.
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve() error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve() hung — defaults may not have been applied")
	}
}

// helpers

type spyHealth struct {
	started  atomic.Bool
	notReady atomic.Bool
}

func (s *spyHealth) MarkStarted()  { s.started.Store(true) }
func (s *spyHealth) SetNotReady()  { s.notReady.Store(true) }

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("get free addr: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func waitForHealth(t *testing.T, h *spyHealth, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for !h.started.Load() {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for MarkStarted()")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func waitForListening(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s to accept connections", addr)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}
