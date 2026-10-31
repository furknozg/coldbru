// Package links is Notification's client of the redirect service's POST /links (D-106). While the
// recipient intake writes the delivery rows of a batch it asks for one short link per delivery;
// the codes are stored with the rows and the sender builds the tracking link from them, so the
// send path makes no network call to the redirect service.
//
// The request is idempotent on the delivery id: asking again for the same deliveries (the batch is
// retried after a failure) returns the same codes. The client does not retry on its own; the
// redelivery of the RECIPIENTS message is the retry, so a redirect service that is down delays the
// batch and never loses it.
package links

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/platform/config"
)

// KeyURL is the base URL of the redirect service, for example http://redirect:8080.
const KeyURL = "REDIRECT_URL"

// Defaults.
const (
	// DefaultTimeout is the time limit of one request.
	DefaultTimeout = 10 * time.Second
	// MaxPerRequest is what POST /links accepts in one request; larger lists are split.
	MaxPerRequest = 1000
	// maxBody caps a response; 1000 links are about 100 KB.
	maxBody = 4 << 20
	// errBodyLen is how much of an unexpected body goes into an error message.
	errBodyLen = 200
)

// Request asks for the link of one delivery. It mirrors internal/redirect/api.LinkRequest (a
// contract test compares the two).
type Request struct {
	DeliveryID uuid.UUID `json:"delivery_id"`
	CampaignID uuid.UUID `json:"campaign_id"`
	Channel    string    `json:"channel"`
	Language   string    `json:"language"`
	Timezone   string    `json:"timezone"`
	TargetURL  string    `json:"target_url"`
}

// Creator creates links. *Client implements it; tests use linktest.Fake.
type Creator interface {
	// Create returns the code of every delivery of reqs. A delivery that has a link keeps its code.
	// An error that satisfies errors.As(err, *RejectedError) means the request can never succeed;
	// any other error is transient and the caller retries later.
	Create(ctx context.Context, reqs []Request) (map[uuid.UUID]string, error)
}

// RejectedError is a 4xx answer of the redirect service other than 429: the request itself is
// wrong, so repeating it does not help.
type RejectedError struct {
	Status int
	Detail string
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("links: redirect service rejected the request (%d): %s", e.Status, e.Detail)
}

// Config configures a Client.
type Config struct {
	// URL is the base URL of the redirect service, without a path.
	URL string
	// Timeout limits one request; zero means DefaultTimeout.
	Timeout time.Duration
	// HTTP overrides the HTTP client; its Timeout should stay zero.
	HTTP *http.Client
}

// LoadConfig reads the Config from v, a Viper built by config.New.
func LoadConfig(v *viper.Viper) (Config, error) {
	if err := config.Require(v, KeyURL); err != nil {
		return Config{}, err
	}
	return Config{URL: v.GetString(KeyURL)}, nil
}

// Client calls the redirect service. It is safe for concurrent use.
type Client struct {
	url     string
	timeout time.Duration
	http    *http.Client
}

var _ Creator = (*Client)(nil)

// New returns a Client for cfg.
func New(cfg Config) (*Client, error) {
	u := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if u == "" {
		return nil, fmt.Errorf("links: %s is required", KeyURL)
	}
	c := &Client{url: u, timeout: cfg.Timeout, http: cfg.HTTP}
	if c.timeout <= 0 {
		c.timeout = DefaultTimeout
	}
	if c.http == nil {
		c.http = &http.Client{}
	}
	return c, nil
}

type response struct {
	Links []struct {
		DeliveryID uuid.UUID `json:"delivery_id"`
		Code       string    `json:"code"`
	} `json:"links"`
}

// Create implements Creator. It splits reqs into requests of at most MaxPerRequest and returns an
// error as soon as one fails; the links the earlier requests created are kept by the redirect
// service and returned again when the caller retries.
func (c *Client) Create(ctx context.Context, reqs []Request) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(reqs))
	for start := 0; start < len(reqs); start += MaxPerRequest {
		chunk := reqs[start:min(start+MaxPerRequest, len(reqs))]
		if err := c.post(ctx, chunk, out); err != nil {
			return nil, err
		}
	}
	for _, r := range reqs {
		if out[r.DeliveryID] == "" {
			return nil, fmt.Errorf("links: the redirect service returned no code for delivery %s", r.DeliveryID)
		}
	}
	return out, nil
}

func (c *Client) post(ctx context.Context, reqs []Request, out map[uuid.UUID]string) error {
	body, err := json.Marshal(struct {
		Links []Request `json:"links"`
	}{reqs})
	if err != nil {
		return fmt.Errorf("links: encode request: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/links", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("links: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("links: redirect service unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fmt.Errorf("links: read response: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusRequestTimeout:
		return &RejectedError{Status: resp.StatusCode, Detail: snippet(raw)}
	default:
		return fmt.Errorf("links: redirect service answered %d: %s", resp.StatusCode, snippet(raw))
	}
	var got response
	if err := json.Unmarshal(raw, &got); err != nil {
		return fmt.Errorf("links: decode response: %w", err)
	}
	for _, l := range got.Links {
		out[l.DeliveryID] = l.Code
	}
	return nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > errBodyLen {
		s = s[:errBodyLen] + "..."
	}
	return s
}

// IsRejected reports whether err is a RejectedError.
func IsRejected(err error) bool {
	var r *RejectedError
	return errors.As(err, &r)
}
