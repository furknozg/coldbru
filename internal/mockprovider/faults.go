package mockprovider

import (
	"math/rand/v2"
	"net/http"
	"sync"
	"time"
)

// Fault is what the injector decided for one request.
type Fault int

// Faults.
const (
	FaultNone Fault = iota
	// Fault5xx answers 502 or 503; nothing is forwarded or stored.
	Fault5xx
	// Fault429 answers a spontaneous 429 with a Retry-After of 1 to 3 s; nothing is forwarded or stored.
	Fault429
	// FaultTimeout accepts and stores the message, then holds the answer back.
	FaultTimeout
)

// injector draws the random choices of the mock. It is safe for concurrent use; with a seed, the
// sequence of draws is deterministic (the order in which concurrent requests draw is not).
type injector struct {
	mu  sync.Mutex
	rng *rand.Rand
	cfg Config
}

func newInjector(cfg Config) *injector {
	seed1, seed2 := rand.Uint64(), rand.Uint64()
	if cfg.HasSeed {
		seed1, seed2 = cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15
	}
	return &injector{rng: rand.New(rand.NewPCG(seed1, seed2)), cfg: cfg}
}

// next decides the fault of one request with a single draw: the three rates are consecutive bands of
// [0, 1), so they are exclusive.
func (i *injector) next() Fault {
	i.mu.Lock()
	r := i.rng.Float64()
	i.mu.Unlock()
	switch {
	case r < i.cfg.ErrorRate:
		return Fault5xx
	case r < i.cfg.ErrorRate+i.cfg.Rate429:
		return Fault429
	case r < i.cfg.ErrorRate+i.cfg.Rate429+i.cfg.TimeoutRate:
		return FaultTimeout
	}
	return FaultNone
}

// status5xx is 502 or 503.
func (i *injector) status5xx() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.rng.IntN(2) == 0 {
		return http.StatusBadGateway
	}
	return http.StatusServiceUnavailable
}

// retryAfter is 1, 2 or 3 seconds.
func (i *injector) retryAfter() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return 1 + i.rng.IntN(3)
}

// latency is uniform in [LatencyMin, LatencyMax].
func (i *injector) latency() time.Duration {
	span := i.cfg.LatencyMax - i.cfg.LatencyMin
	if span <= 0 {
		return i.cfg.LatencyMin
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.cfg.LatencyMin + time.Duration(i.rng.Int64N(int64(span)+1))
}
