package ui

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/remote"
)

func TestActiveConnectionTimeoutDisconnects(t *testing.T) {
	if activeConnectionTimeout != 10*time.Second {
		t.Fatalf("active connection timeout = %s", activeConnectionTimeout)
	}
	var disconnects atomic.Int32
	start := time.Now()
	_, err := connectTerminalWithin(context.Background(), 40*time.Millisecond, func(ctx context.Context) (*remote.TerminalSession, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}, func() { disconnects.Add(1) })
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("timeout error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("timeout took %s", elapsed)
	}
	if got := disconnects.Load(); got != 1 {
		t.Fatalf("disconnect called %d times", got)
	}
}

func TestActiveConnectionKeepsImmediateFailure(t *testing.T) {
	want := errors.New("authentication rejected")
	var disconnects atomic.Int32
	_, err := connectTerminalWithin(context.Background(), time.Second, func(context.Context) (*remote.TerminalSession, error) {
		return nil, want
	}, func() { disconnects.Add(1) })
	if !errors.Is(err, want) || disconnects.Load() != 0 {
		t.Fatalf("error = %v, disconnects = %d", err, disconnects.Load())
	}
}

func TestActiveConnectionTimeoutDoesNotWaitForStuckStep(t *testing.T) {
	var disconnects atomic.Int32
	start := time.Now()
	_, err := connectTerminalWithin(context.Background(), 30*time.Millisecond, func(context.Context) (*remote.TerminalSession, error) {
		time.Sleep(150 * time.Millisecond)
		return nil, nil
	}, func() { disconnects.Add(1) })
	if !errors.Is(err, context.DeadlineExceeded) || disconnects.Load() != 1 {
		t.Fatalf("error = %v, disconnects = %d", err, disconnects.Load())
	}
	if elapsed := time.Since(start); elapsed >= 120*time.Millisecond {
		t.Fatalf("stuck step delayed timeout to %s", elapsed)
	}
}
