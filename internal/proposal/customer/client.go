package customer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// Retry policy (D-067): at most three attempts, retried only after a transport error or an HTTP
// 502, 503 or 504; backoff doubles from 200 ms with jitter. A GraphQL error, 401 and any other 4xx
// are never retried: the same request would fail the same way.
const (
	maxAttempts = 3
	baseBackoff = 200 * time.Millisecond
	// maxBody caps a response; a page of 1000 recipients is a few hundred KB.
	maxBody = 8 << 20
	// errBodyLen is how much of an unexpected response body goes into a StatusError.
	errBodyLen = 200
)

// Client calls Customer's GraphQL endpoint. It is safe for concurrent use.
type Client struct {
	url     string
	token   string
	timeout time.Duration
	http    *http.Client
	// sleep waits for d or until ctx ends; tests replace it.
	sleep func(ctx context.Context, d time.Duration) error
	// jitter spreads a backoff delay; tests replace it.
	jitter func(d time.Duration) time.Duration
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client. Its own Timeout should stay zero: the per-attempt limit is
// a context deadline (D-067).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// New returns a Client for cfg.
func New(cfg Config, opts ...Option) (*Client, error) {
	var missing []string
	if strings.TrimSpace(cfg.URL) == "" {
		missing = append(missing, KeyURL)
	}
	if strings.TrimSpace(cfg.Token) == "" {
		missing = append(missing, KeyToken)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("customer: missing required config: %s", strings.Join(missing, ", "))
	}
	c := &Client{
		url:     cfg.URL,
		token:   cfg.Token,
		timeout: cfg.Timeout,
		http:    &http.Client{},
		sleep:   sleepCtx,
		jitter:  equalJitter,
	}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// equalJitter returns d/2 plus a random share of the other half: the delay never collapses to
// zero and concurrent callers do not retry in step.
func equalJitter(d time.Duration) time.Duration {
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

// backoff is the delay before the attempt that follows failed attempt n (1-based), before jitter.
func backoff(n int) time.Duration { return baseBackoff << (n - 1) }

type request struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

type gqlError struct {
	Message    string         `json:"message"`
	Path       []any          `json:"path"`
	Extensions map[string]any `json:"extensions"`
}

type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

// execute sends one operation for filter f and decodes data into out. Errors are described on the
// package's error types.
func (c *Client) execute(ctx context.Context, f *compile.Filter, query, operation string, vars map[string]any, out any) error {
	if f == nil {
		return errors.New("customer: nil filter")
	}
	vars["filter"] = f.Variable()
	body, err := json.Marshal(request{Query: query, OperationName: operation, Variables: vars})
	if err != nil {
		return fmt.Errorf("customer: encode request: %w", err)
	}

	var last error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			if err := c.sleep(ctx, c.jitter(backoff(attempt-1))); err != nil {
				return fmt.Errorf("customer: %w", err)
			}
		}
		status, data, err := c.attempt(ctx, body)
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return fmt.Errorf("customer: %w", ctx.Err())
			}
			var tooBig *bodyTooLargeError
			if errors.As(err, &tooBig) {
				return err
			}
			last = fmt.Errorf("customer: request failed: %w", err)
		case status == http.StatusBadGateway, status == http.StatusServiceUnavailable, status == http.StatusGatewayTimeout:
			last = &StatusError{Status: status, Body: snippet(data)}
		default:
			return decode(f, status, data, out)
		}
	}
	return fmt.Errorf("customer: giving up after %d attempts: %w", maxAttempts, last)
}

type bodyTooLargeError struct{}

func (*bodyTooLargeError) Error() string {
	return fmt.Sprintf("customer: response larger than %d bytes", maxBody)
}

// attempt performs one HTTP round trip under the per-attempt deadline. The body is read before the
// deadline context is released.
func (c *Client) attempt(ctx context.Context, body []byte) (int, []byte, error) {
	actx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(actx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return 0, nil, err
	}
	if len(data) > maxBody {
		return 0, nil, &bodyTooLargeError{}
	}
	return res.StatusCode, data, nil
}

func snippet(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > errBodyLen {
		s = s[:errBodyLen] + "…"
	}
	return s
}

// decode turns a final (not retried) response into out or an error. gqlgen answers some GraphQL
// errors with a 4xx status and a JSON body, so a body with errors wins over the status.
func decode(f *compile.Filter, status int, data []byte, out any) error {
	if status == http.StatusUnauthorized {
		return &UnauthorizedError{}
	}
	var resp response
	parsed := json.Unmarshal(data, &resp) == nil
	if parsed && len(resp.Errors) > 0 {
		return mapErrors(f, resp.Errors)
	}
	if status != http.StatusOK {
		return &StatusError{Status: status, Body: snippet(data)}
	}
	if !parsed {
		return fmt.Errorf("customer: response is not JSON: %s", snippet(data))
	}
	if len(resp.Data) == 0 || string(resp.Data) == "null" {
		return errors.New("customer: response has no data")
	}
	if err := json.Unmarshal(resp.Data, out); err != nil {
		return fmt.Errorf("customer: decode data: %w", err)
	}
	return nil
}

// mapErrors converts GraphQL errors (D-067). An UNKNOWN_VALUE or COMPLEXITY_LIMIT error with a
// filter path that maps into the DSL becomes a *dsl.Error; everything else a *Error. One error is
// returned as is, several joined (DSLErrors reads them back).
func mapErrors(f *compile.Filter, errs []gqlError) error {
	out := make([]error, 0, len(errs))
	for _, ge := range errs {
		out = append(out, mapError(f, ge))
	}
	if len(out) == 1 {
		return out[0]
	}
	return errors.Join(out...)
}

func mapError(f *compile.Filter, ge gqlError) error {
	code, _ := ge.Extensions["code"].(string)
	fp, _ := ge.Extensions["filterPath"].([]any)
	var span *dsl.Span
	if len(fp) > 0 {
		if sp, ok := f.Span(fp); ok {
			span = &sp
		}
	}
	if span != nil && (code == CodeUnknownValue || code == CodeComplexityLimit) {
		de := dsl.NewError(f.Source(), dsl.Code(code), ge.Message, span.Start, span.End)
		if raw, ok := ge.Extensions["suggestions"].([]any); ok {
			for _, s := range raw {
				if str, ok := s.(string); ok {
					de.Suggestions = append(de.Suggestions, str)
				}
			}
		}
		return de
	}
	return &Error{
		Code:       code,
		Message:    ge.Message,
		Path:       ge.Path,
		FilterPath: fp,
		Span:       span,
		Extensions: ge.Extensions,
	}
}
