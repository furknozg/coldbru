// Package provider is Notification's client of the SMS and email provider (D-079). The contract,
// which the mock provider implements (T-031):
//
//	POST {base}/v1/sms    {"to", "text"}
//	POST {base}/v1/email  {"to", "subject", "html", "text"}
//	header Idempotency-Key: <delivery id>
//	202 {"message_id"}    accepted
//	429 + Retry-After     rate limited (seconds or an HTTP date)
//	5xx, 408, network error, timeout   transient
//	other 4xx             permanent
//
// Every outcome that is not a success is one of three error types, so the worker decides on the
// type alone: *RateLimitedError, *TransientError, *PermanentError. The client itself never retries:
// retrying is the worker's job, through NATS redelivery with the same Idempotency-Key.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/platform/config"
)

// KeyBaseURL is the config key of the provider's base URL, for example http://mockprovider:8080.
const KeyBaseURL = "PROVIDER_BASE_URL"

// DefaultTimeout is the time limit of one request (D-079 brief: 10 s).
const DefaultTimeout = 10 * time.Second

const (
	// maxBody caps what is read of a response.
	maxBody = 1 << 20
	// errBodyLen is how much of an error body goes into an error message.
	errBodyLen = 200
	// MaxRetryAfter caps a Retry-After value: a provider that asks for more is treated as asking for
	// this much, so a bad header cannot park a channel for hours.
	MaxRetryAfter = 30 * time.Second
)

// Config configures a Client.
type Config struct {
	// BaseURL is the provider's root, without a trailing path such as /v1.
	BaseURL string
	// Timeout limits one request. Zero means DefaultTimeout.
	Timeout time.Duration
}

// LoadConfig reads the Config from v, a Viper built by config.New.
func LoadConfig(v *viper.Viper) (Config, error) {
	if err := config.Require(v, KeyBaseURL); err != nil {
		return Config{}, err
	}
	return Config{BaseURL: v.GetString(KeyBaseURL)}, nil
}

// SMS is one text message.
type SMS struct {
	To   string `json:"to"`
	Text string `json:"text"`
}

// Email is one email.
type Email struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}

// Result is what the provider answered to an accepted message.
type Result struct {
	// MessageID is the provider's id of the message.
	MessageID string
	// Status is the HTTP status of the answer.
	Status int
}

// Client calls the provider. It is safe for concurrent use.
type Client struct {
	base    string
	timeout time.Duration
	http    *http.Client
	now     func() time.Time
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client. Its Timeout should stay zero: the limit is a context deadline.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// New returns a Client for cfg.
func New(cfg Config, opts ...Option) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("provider: %s %q must be an absolute http or https URL", KeyBaseURL, cfg.BaseURL)
	}
	c := &Client{
		base:    base,
		timeout: cfg.Timeout,
		// A redirect would turn the POST into something else; any 3xx is an error instead.
		http: &http.Client{Transport: transport(), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		now:  time.Now,
	}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// transport keeps enough idle connections for the sender pool; the default of 2 per host would open a
// new connection for most of the requests.
func transport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = 64
	return t
}

// SendSMS sends m with idempotencyKey as the Idempotency-Key header.
func (c *Client) SendSMS(ctx context.Context, idempotencyKey string, m SMS) (Result, error) {
	return c.post(ctx, "/v1/sms", idempotencyKey, m)
}

// SendEmail sends m with idempotencyKey as the Idempotency-Key header.
func (c *Client) SendEmail(ctx context.Context, idempotencyKey string, m Email) (Result, error) {
	return c.post(ctx, "/v1/email", idempotencyKey, m)
}

func (c *Client) post(ctx context.Context, path, key string, payload any) (Result, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, &PermanentError{Err: fmt.Errorf("encode request: %w", err)}
	}
	rctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, c.base+path, bytes.NewReader(body))
	if err != nil {
		return Result{}, &PermanentError{Err: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Idempotency-Key", key)

	res, err := c.http.Do(req)
	if err != nil {
		// Timeout, refused or reset connection: the message may or may not have arrived, and the
		// same Idempotency-Key makes the repeat safe.
		return Result{}, &TransientError{Err: fmt.Errorf("provider request: %w", err)}
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return Result{}, &TransientError{Status: res.StatusCode, Err: fmt.Errorf("read response: %w", err)}
	}
	return c.result(res, data)
}

// result classifies a response.
func (c *Client) result(res *http.Response, data []byte) (Result, error) {
	status := res.StatusCode
	switch {
	case status >= 200 && status < 300:
		var ok struct {
			MessageID string `json:"message_id"`
		}
		if err := json.Unmarshal(data, &ok); err != nil || ok.MessageID == "" {
			// Accepted, but the id is unreadable. Asking again with the same key returns it.
			return Result{}, &TransientError{Status: status, Err: fmt.Errorf("status %d without a message_id: %s", status, snippet(data))}
		}
		return Result{MessageID: ok.MessageID, Status: status}, nil
	case status == http.StatusTooManyRequests:
		return Result{}, &RateLimitedError{RetryAfter: ParseRetryAfter(res.Header.Get("Retry-After"), c.now()), Body: snippet(data)}
	case status == http.StatusRequestTimeout, status >= 500:
		return Result{}, &TransientError{Status: status, Err: fmt.Errorf("status %d: %s", status, snippet(data))}
	default:
		// Other 4xx, and 1xx and 3xx, which a correct provider does not answer with.
		return Result{}, &PermanentError{Status: status, Err: fmt.Errorf("status %d: %s", status, snippet(data))}
	}
}

// ParseRetryAfter reads a Retry-After header value: a number of seconds or an HTTP date, relative
// to now. A missing, malformed or past value is zero; a value above MaxRetryAfter is MaxRetryAfter.
func ParseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		if secs <= 0 {
			return 0
		}
		if secs >= int64(MaxRetryAfter/time.Second) {
			return MaxRetryAfter
		}
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		d = t.Sub(now)
	} else {
		return 0
	}
	return min(max(d, 0), MaxRetryAfter)
}

func snippet(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > errBodyLen {
		s = s[:errBodyLen] + "…"
	}
	return s
}

// RateLimitedError is HTTP 429. RetryAfter is how long the provider asked to wait, zero if it did
// not say.
type RateLimitedError struct {
	RetryAfter time.Duration
	Body       string
}

// Error implements error.
func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("provider: rate limited (429), retry after %s", e.RetryAfter)
}

// TransientError is a failure worth repeating: a network error, a timeout, 408, 5xx, or a success
// answer that cannot be read. Status is 0 when there was no response.
type TransientError struct {
	Status int
	Err    error
}

// Error implements error.
func (e *TransientError) Error() string { return "provider: transient failure: " + e.Err.Error() }

// Unwrap returns the cause.
func (e *TransientError) Unwrap() error { return e.Err }

// PermanentError is a failure that repeating cannot fix: a 4xx other than 408 and 429.
type PermanentError struct {
	Status int
	Err    error
}

// Error implements error.
func (e *PermanentError) Error() string { return "provider: permanent failure: " + e.Err.Error() }

// Unwrap returns the cause.
func (e *PermanentError) Unwrap() error { return e.Err }

// Class is how the worker treats a result.
type Class int

// Classes.
const (
	ClassOK Class = iota
	ClassRateLimited
	ClassTransient
	ClassPermanent
)

// Classify returns the class of err, nil being ClassOK. An error of an unknown type is transient:
// the Idempotency-Key makes a repeat safe, which is not true of giving up.
func Classify(err error) Class {
	var rl *RateLimitedError
	var perm *PermanentError
	switch {
	case err == nil:
		return ClassOK
	case errors.As(err, &rl):
		return ClassRateLimited
	case errors.As(err, &perm):
		return ClassPermanent
	default:
		return ClassTransient
	}
}

// StatusOf returns the HTTP status err carries, 429 for a rate limit, or 0 when there was no
// response (network error, timeout).
func StatusOf(err error) int {
	var rl *RateLimitedError
	var tr *TransientError
	var perm *PermanentError
	switch {
	case errors.As(err, &rl):
		return http.StatusTooManyRequests
	case errors.As(err, &tr):
		return tr.Status
	case errors.As(err, &perm):
		return perm.Status
	}
	return 0
}

// CodeClass is the label of a provider answer for metrics: "2xx", "4xx", "5xx", or "error" when
// there was no response. 429 and 408 are "4xx".
func CodeClass(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	case status > 0:
		return "other"
	}
	return "error"
}
