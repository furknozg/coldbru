// Package httpserver wires the shared router, middleware and server lifecycle (D-050).
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/furknozg/coldbru/internal/platform/health"
	"github.com/furknozg/coldbru/internal/platform/metrics"
)

// NewRouter returns a chi router with RequestID, request logging, metrics and
// Recoverer middleware, in that order, and /healthz, /readyz and /metrics
// mounted. Recoverer is innermost so a panic becomes a 500 before the logger
// and metrics middleware finish, and they record it.
// Services mount their own routes on the result.
func NewRouter(logger *slog.Logger, m *metrics.Metrics, reg *health.Registry) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(RequestLogger(logger))
	r.Use(m.Middleware())
	r.Use(middleware.Recoverer)

	r.Method(http.MethodGet, "/healthz", health.LivenessHandler())
	r.Method(http.MethodGet, "/readyz", reg.ReadinessHandler())
	r.Method(http.MethodGet, "/metrics", m.Handler())
	return r
}

// RequestLogger logs one Info line per request with method, path, status,
// duration_ms and request_id. It must run after middleware.RequestID.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
				slog.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}

// NewServer returns an http.Server for addr and h with safe timeouts.
// WriteTimeout is deliberately unset: it would cut off long-lived SSE streams.
func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// Run serves srv until ctx is cancelled or the server fails, then shuts it
// down gracefully within shutdownTimeout. A clean shutdown returns nil.
// Signal handling is the caller's job: pass a ctx from signal.NotifyContext.
func Run(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration, logger *slog.Logger) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	return serve(ctx, srv, ln, shutdownTimeout, logger)
}

func serve(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration, logger *slog.Logger) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	logger.Info("http server listening", "addr", ln.Addr().String())

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	logger.Info("http server shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
