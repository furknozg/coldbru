// Package health provides liveness and readiness HTTP handlers (D-050).
package health

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"sync"
	"time"
)

// DefaultTimeout is the per-check deadline used by readiness probes.
const DefaultTimeout = 2 * time.Second

const (
	statusOK   = "ok"
	statusFail = "fail"
)

// Check reports whether one dependency is ready. A nil error means ready.
type Check func(ctx context.Context) error

// Registry holds named readiness checks. Create it with NewRegistry.
type Registry struct {
	// Timeout bounds every check; NewRegistry sets it to DefaultTimeout.
	Timeout time.Duration

	mu     sync.RWMutex
	checks map[string]Check
}

// NewRegistry returns an empty registry with the default check timeout.
func NewRegistry() *Registry {
	return &Registry{Timeout: DefaultTimeout, checks: make(map[string]Check)}
}

// Add registers a check under name, replacing any earlier check with that name.
func (r *Registry) Add(name string, c Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[name] = c
}

type checkResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type report struct {
	Status string                 `json:"status"`
	Checks map[string]checkResult `json:"checks,omitempty"`
}

// LivenessHandler always answers 200 {"status":"ok"} and checks no dependency.
func LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, report{Status: statusOK})
	})
}

// ReadinessHandler runs every registered check concurrently, each bounded by
// the registry timeout. It answers 200 when all pass (or none are registered)
// and 503 otherwise.
func (r *Registry) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rep := r.run(req.Context())
		code := http.StatusOK
		if rep.Status != statusOK {
			code = http.StatusServiceUnavailable
		}
		writeJSON(w, code, rep)
	})
}

func (r *Registry) run(ctx context.Context) report {
	r.mu.RLock()
	checks := make(map[string]Check, len(r.checks))
	maps.Copy(checks, r.checks)
	r.mu.RUnlock()

	rep := report{Status: statusOK, Checks: make(map[string]checkResult, len(checks))}
	if len(checks) == 0 {
		rep.Checks = nil
		return rep
	}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for name, c := range checks {
		wg.Go(func() {
			res := runCheck(ctx, r.Timeout, c)
			mu.Lock()
			rep.Checks[name] = res
			mu.Unlock()
		})
	}
	wg.Wait()

	for _, res := range rep.Checks {
		if res.Status != statusOK {
			rep.Status = statusFail
			break
		}
	}
	return rep
}

// runCheck runs c in its own goroutine so a check that ignores its context
// cannot hold the probe past the timeout.
func runCheck(ctx context.Context, timeout time.Duration, c Check) checkResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- c(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			return checkResult{Status: statusFail, Error: err.Error()}
		}
		return checkResult{Status: statusOK}
	case <-ctx.Done():
		return checkResult{Status: statusFail, Error: ctx.Err().Error()}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	// The status line is already sent; a failed write only means the client left.
	_ = json.NewEncoder(w).Encode(v)
}
