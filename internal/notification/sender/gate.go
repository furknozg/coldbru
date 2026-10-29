package sender

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// gate is what every sender goroutine of one channel passes before it calls the provider (D-024,
// D-079): the channel's token bucket, shared by all of them, and a pause that a 429 sets for all
// of them at once.
type gate struct {
	limiter *rate.Limiter
	now     func() time.Time

	mu    sync.Mutex
	until time.Time
}

func newGate(limit rate.Limit, burst int) *gate {
	return &gate{limiter: rate.NewLimiter(limit, burst), now: time.Now}
}

// PauseFor stops the channel for d from now. A pause never shortens a longer one that is running.
func (g *gate) PauseFor(d time.Duration) {
	if d <= 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if until := g.now().Add(d); until.After(g.until) {
		g.until = until
	}
}

// Paused returns how long the channel is still paused; zero when it is not.
func (g *gate) Paused() time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	return max(g.until.Sub(g.now()), 0)
}

// Wait returns when the caller may make one provider call: no pause is running and the limiter
// has a token. It returns ctx's error if ctx ends first.
func (g *gate) Wait(ctx context.Context) error {
	for {
		if d := g.Paused(); d > 0 {
			if err := sleep(ctx, d); err != nil {
				return err
			}
			continue
		}
		if err := g.limiter.Wait(ctx); err != nil {
			return err
		}
		// A 429 may have come in while this caller was waiting for its token.
		if g.Paused() == 0 {
			return nil
		}
	}
}

// sleep waits for d or until ctx ends.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
