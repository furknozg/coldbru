// Package consent is Notification's client of Customer's consentStatus query (D-011, D-064, D-079).
// Before a batch of messages goes out, the worker asks Customer, the single source of truth, which
// of the batch's customers may still be written to on the channel.
//
// The call is made through a circuit breaker (sony/gobreaker): five consecutive failures open it for
// 30 seconds, then one trial call decides whether it closes. Whatever goes wrong, the caller gets an
// error and no answer, and must not send (fail-closed). The client does not retry on its own; the
// worker's redelivery is the retry.
package consent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sony/gobreaker/v2"
	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/platform/config"
)

// Config keys read from the environment.
const (
	// KeyURL is the full URL of Customer's endpoint, for example http://customer:8080/graphql.
	KeyURL = "CUSTOMER_GRAPHQL_URL"
	// KeyToken is Notification's bearer token, the value of Customer's CUSTOMER_TOKEN_NOTIFICATION
	// (D-063, scope consent:read).
	KeyToken = "CUSTOMER_TOKEN"
)

// Defaults (D-079).
const (
	// DefaultTimeout is the time limit of one request.
	DefaultTimeout = 10 * time.Second
	// DefaultMaxFailures is the number of consecutive failures that opens the breaker.
	DefaultMaxFailures = 5
	// DefaultOpenFor is how long an open breaker rejects calls before it lets a trial through.
	DefaultOpenFor = 30 * time.Second
	// MaxIDs is the largest customerIds list Customer accepts (D-064). Longer lists are split.
	MaxIDs = 1000
	// maxBody caps a response; 1000 rows are well under 200 KB.
	maxBody = 4 << 20
	// errBodyLen is how much of an unexpected body goes into an error message.
	errBodyLen = 200
)

// consentQuery is checked against Customer's schema by a test.
const consentQuery = `query ConsentStatus($ids: [ID!]!, $channel: String!) {
  consentStatus(customerIds: $ids, channel: $channel) { customerId channel optedIn eligible }
}`

const consentOperation = "ConsentStatus"

// ErrUnavailable is returned, wrapped, whenever no answer could be had: the breaker is open or
// rejects a call while it is testing, Customer could not be reached, or it answered with an error.
// The caller must not send.
var ErrUnavailable = errors.New("consent: customer unavailable")

// ErrBreakerOpen is returned, wrapped in ErrUnavailable, when the breaker did not let the call
// through. Customer was not contacted.
var ErrBreakerOpen = errors.New("consent: circuit breaker open")

// Config configures a Client.
type Config struct {
	URL   string
	Token string
	// Timeout limits one request. Zero means DefaultTimeout.
	Timeout time.Duration
	// MaxFailures is the number of consecutive failures that opens the breaker. Zero means
	// DefaultMaxFailures.
	MaxFailures uint32
	// OpenFor is how long the breaker stays open. Zero means DefaultOpenFor.
	OpenFor time.Duration
	// Logger receives the breaker's state changes. Nil discards them.
	Logger *slog.Logger
}

// LoadConfig reads the Config from v, a Viper built by config.New. It fails naming every missing
// key, never the token.
func LoadConfig(v *viper.Viper) (Config, error) {
	if err := config.Require(v, KeyURL, KeyToken); err != nil {
		return Config{}, err
	}
	return Config{URL: v.GetString(KeyURL), Token: v.GetString(KeyToken)}, nil
}

// String omits the token so that a logged Config does not leak it.
func (c Config) String() string {
	return fmt.Sprintf("consent.Config{URL: %q, Timeout: %s}", c.URL, c.Timeout)
}

// Client asks Customer for consent. It is safe for concurrent use.
type Client struct {
	url     string
	token   string
	timeout time.Duration
	http    *http.Client
	breaker *gobreaker.CircuitBreaker[map[string]bool]
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client. Its Timeout should stay zero: the limit is a context deadline.
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
		return nil, fmt.Errorf("consent: missing required config: %s", strings.Join(missing, ", "))
	}
	c := &Client{url: cfg.URL, token: cfg.Token, timeout: cfg.Timeout, http: &http.Client{}}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
	}
	maxFailures, openFor := cfg.MaxFailures, cfg.OpenFor
	if maxFailures == 0 {
		maxFailures = DefaultMaxFailures
	}
	if openFor <= 0 {
		openFor = DefaultOpenFor
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	c.breaker = gobreaker.NewCircuitBreaker[map[string]bool](gobreaker.Settings{
		Name:        "customer-consent",
		MaxRequests: 1, // one trial call while half open
		Timeout:     openFor,
		// Interval stays zero: the closed state never forgets failures on a timer, so "consecutive"
		// means consecutive.
		ReadyToTrip: func(counts gobreaker.Counts) bool { return counts.ConsecutiveFailures >= maxFailures },
		// A cancelled call (shutdown) says nothing about Customer.
		IsExcluded: func(err error) bool { return errors.Is(err, context.Canceled) },
		OnStateChange: func(name string, from, to gobreaker.State) {
			level := slog.LevelWarn
			if to == gobreaker.StateClosed {
				level = slog.LevelInfo
			}
			logger.Log(context.Background(), level, "circuit breaker state changed",
				slog.String("breaker", name), slog.String("from", from.String()), slog.String("to", to.String()))
		},
	})
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// State returns the state of the breaker: "closed", "half-open" or "open".
func (c *Client) State() string { return c.breaker.State().String() }

// Eligible asks Customer which of customerIDs may be written to on channel ("sms" or "email") and
// returns one entry per id: true when the customer is eligible (opted in, active, reachable, D-064).
// Customer answers an unknown id with false. The answer must cover every id; a missing one is an
// error. On any error the result is nil and the error wraps ErrUnavailable.
func (c *Client) Eligible(ctx context.Context, channel string, customerIDs []string) (map[string]bool, error) {
	out := make(map[string]bool, len(customerIDs))
	for start := 0; start < len(customerIDs); start += MaxIDs {
		chunk := customerIDs[start:min(start+MaxIDs, len(customerIDs))]
		part, err := c.breaker.Execute(func() (map[string]bool, error) { return c.query(ctx, channel, chunk) })
		if err != nil {
			if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
				return nil, fmt.Errorf("%w: %w: %w", ErrUnavailable, ErrBreakerOpen, err)
			}
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		for id, ok := range part {
			out[id] = ok
		}
	}
	return out, nil
}

type request struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

type response struct {
	Data struct {
		ConsentStatus []struct {
			CustomerID string `json:"customerId"`
			Channel    string `json:"channel"`
			OptedIn    bool   `json:"optedIn"`
			Eligible   bool   `json:"eligible"`
		} `json:"consentStatus"`
	} `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

// query makes one request for at most MaxIDs ids.
func (c *Client) query(ctx context.Context, channel string, ids []string) (map[string]bool, error) {
	body, err := json.Marshal(request{
		Query: consentQuery, OperationName: consentOperation,
		Variables: map[string]any{"ids": ids, "channel": channel},
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	rctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("response larger than %d bytes", maxBody)
	}
	return decode(res.StatusCode, data, channel, ids)
}

// decode turns a response into the eligibility of every requested id. gqlgen answers some GraphQL
// errors with a 4xx status and a JSON body, so a body with errors wins over the status.
func decode(status int, data []byte, channel string, ids []string) (map[string]bool, error) {
	var resp response
	parsed := json.Unmarshal(data, &resp) == nil
	switch {
	case status == http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized: the service token was rejected (check %s)", KeyToken)
	case parsed && len(resp.Errors) > 0:
		e := resp.Errors[0]
		code, _ := e.Extensions["code"].(string)
		return nil, fmt.Errorf("graphql error %s: %s", code, e.Message)
	case status != http.StatusOK:
		return nil, fmt.Errorf("unexpected HTTP status %d: %s", status, snippet(data))
	case !parsed:
		return nil, fmt.Errorf("response is not JSON: %s", snippet(data))
	}
	got := make(map[string]bool, len(resp.Data.ConsentStatus))
	for _, s := range resp.Data.ConsentStatus {
		if s.Channel != channel {
			return nil, fmt.Errorf("answer for channel %q, asked about %q", s.Channel, channel)
		}
		got[s.CustomerID] = s.Eligible
	}
	for _, id := range ids {
		if _, ok := got[id]; !ok {
			return nil, fmt.Errorf("no consent status for customer %q in the answer", id)
		}
	}
	return got, nil
}

func snippet(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > errBodyLen {
		s = s[:errBodyLen] + "…"
	}
	return s
}
