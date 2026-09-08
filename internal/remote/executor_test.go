package remote

import (
	"context"
	"errors"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"testing"
	"time"
)

func TestQueuedMutationCancellation(t *testing.T) {
	e := &Executor{}
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	e.locks.Store("host", gate)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	_, err := e.Execute(ctx, domain.Request{HostID: "host", TaskID: "task", CallID: "call", Operation: "shell", Command: "never dispatched"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
