package scheduler

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type countingUseCase struct{ n atomic.Int32 }

func (c *countingUseCase) RunOnce(context.Context) { c.n.Add(1) }

func TestWorker_RunsUntilCancelled(t *testing.T) {
	uc := &countingUseCase{}
	w := NewWorker(uc, 5*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	deadline := time.After(time.Second)
	for uc.n.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("worker ran %d times", uc.n.Load())
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}
