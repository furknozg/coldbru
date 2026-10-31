// Package linktest has a fake links.Creator for tests of the services that depend on the redirect
// service. It hands out codes of the right shape without a network or a database.
package linktest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/notification/links"
	"github.com/furknozg/coldbru/internal/redirect/code"
)

// ErrDown is what a Fake that is down returns, like a redirect service that cannot be reached.
var ErrDown = errors.New("linktest: redirect service down")

// Fake implements links.Creator. The first request for a delivery makes its code; later ones
// return it again, like the real service. It is safe for concurrent use.
type Fake struct {
	mu    sync.Mutex
	codes map[uuid.UUID]string
	reqs  map[uuid.UUID]links.Request
	fail  atomic.Pointer[error]
	calls atomic.Int64
	seq   int
}

// New returns a Fake that is up.
func New() *Fake {
	return &Fake{codes: map[uuid.UUID]string{}, reqs: map[uuid.UUID]links.Request{}}
}

// SetDown makes every Create fail with ErrDown (true) or work again (false).
func (f *Fake) SetDown(down bool) {
	if down {
		f.Fail(ErrDown)
		return
	}
	f.Fail(nil)
}

// Fail makes every Create return err, or work again when err is nil. A *links.RejectedError makes
// the caller treat the request as one that can never succeed.
func (f *Fake) Fail(err error) {
	if err == nil {
		f.fail.Store(nil)
		return
	}
	f.fail.Store(&err)
}

// Calls is the number of Create calls, failed ones included.
func (f *Fake) Calls() int { return int(f.calls.Load()) }

// Create implements links.Creator.
func (f *Fake) Create(_ context.Context, reqs []links.Request) (map[uuid.UUID]string, error) {
	f.calls.Add(1)
	if err := f.fail.Load(); err != nil {
		return nil, *err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[uuid.UUID]string, len(reqs))
	for _, r := range reqs {
		c, ok := f.codes[r.DeliveryID]
		if !ok {
			f.seq++
			c = fmt.Sprintf("T%0*d", code.Length-1, f.seq)
			f.codes[r.DeliveryID] = c
			f.reqs[r.DeliveryID] = r
		}
		out[r.DeliveryID] = c
	}
	return out, nil
}

// Code returns the code of a delivery, or "".
func (f *Fake) Code(delivery uuid.UUID) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.codes[delivery]
}

// Request returns the first request made for a delivery.
func (f *Fake) Request(delivery uuid.UUID) (links.Request, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.reqs[delivery]
	return r, ok
}

// Len is the number of deliveries that have a code.
func (f *Fake) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.codes)
}

// Stub is a Creator made of a function, for tests that need a particular answer.
type Stub func(ctx context.Context, reqs []links.Request) (map[uuid.UUID]string, error)

// Create implements links.Creator.
func (s Stub) Create(ctx context.Context, reqs []links.Request) (map[uuid.UUID]string, error) {
	return s(ctx, reqs)
}

// Placeholder is a code-shaped string for tests that need a delivery link code without a Fake.
func Placeholder(n int) string {
	return "P" + strings.Repeat("0", code.Length-1-len(fmt.Sprint(n))) + fmt.Sprint(n)
}
