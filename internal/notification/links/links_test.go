package links

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"

	rapi "github.com/furknozg/coldbru/internal/redirect/api"
)

func reqs(n int) []Request {
	out := make([]Request, n)
	for i := range out {
		out[i] = Request{
			DeliveryID: uuid.New(), CampaignID: uuid.New(), Channel: "sms", Language: "tr", Timezone: "Europe/Istanbul", TargetURL: "https://shop.example.com",
		}
	}
	return out
}

// echo answers every request with a code made from the position of the delivery.
func echo(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/links" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request = %s %s (%s)", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
		}
		var body struct{ Links []Request }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		out := rapi.CreateLinksResponse{}
		for _, l := range body.Links {
			out.Links = append(out.Links, rapi.LinkResult{DeliveryID: l.DeliveryID, Code: "C" + l.DeliveryID.String()[:9]})
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCreateReturnsTheCodePerDelivery(t *testing.T) {
	var calls atomic.Int32
	srv := echo(t, &calls)
	c, err := New(Config{URL: srv.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	in := reqs(3)
	got, err := c.Create(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range in {
		if got[r.DeliveryID] != "C"+r.DeliveryID.String()[:9] {
			t.Errorf("code of %s = %q", r.DeliveryID, got[r.DeliveryID])
		}
	}
	if calls.Load() != 1 {
		t.Errorf("%d requests, want 1", calls.Load())
	}
}

func TestCreateSplitsLargeBatches(t *testing.T) {
	var calls atomic.Int32
	srv := echo(t, &calls)
	c, _ := New(Config{URL: srv.URL})
	got, err := c.Create(t.Context(), reqs(MaxPerRequest*2+5))
	if err != nil || len(got) != MaxPerRequest*2+5 || calls.Load() != 3 {
		t.Errorf("%d codes, %d requests, err = %v; want %d codes in 3 requests", len(got), calls.Load(), err, MaxPerRequest*2+5)
	}
}

func TestCreateClassifiesFailures(t *testing.T) {
	tests := map[string]struct {
		status   int
		rejected bool
	}{
		"422 is rejected":              {422, true},
		"400 is rejected":              {400, true},
		"429 is transient":             {429, false},
		"408 is transient":             {408, false},
		"500 is transient":             {500, false},
		"502 is transient":             {502, false},
		"503 is transient":             {503, false},
		"a 3xx is transient and wrong": {302, false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"detail":"x"}`))
			}))
			defer srv.Close()
			c, _ := New(Config{URL: srv.URL, HTTP: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}})
			_, err := c.Create(t.Context(), reqs(1))
			if err == nil || IsRejected(err) != tt.rejected {
				t.Errorf("err = %v, rejected = %v; want an error, rejected = %v", err, IsRejected(err), tt.rejected)
			}
		})
	}
}

func TestCreateFailsWhenTheServiceIsUnreachableOrSlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	c, _ := New(Config{URL: url})
	if _, err := c.Create(t.Context(), reqs(1)); err == nil || IsRejected(err) {
		t.Errorf("unreachable: err = %v, want a transient error", err)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer slow.Close()
	c, _ = New(Config{URL: slow.URL, Timeout: 100 * time.Millisecond})
	start := time.Now()
	if _, err := c.Create(t.Context(), reqs(1)); err == nil || IsRejected(err) || time.Since(start) > 2*time.Second {
		t.Errorf("slow: err = %v after %s, want a transient error after the timeout", err, time.Since(start))
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.Create(ctx, reqs(1)); err == nil {
		t.Error("a cancelled context: err = nil")
	}
}

func TestCreateFailsWhenACodeIsMissingOrTheBodyIsBroken(t *testing.T) {
	for name, body := range map[string]string{"no links": `{"links":[]}`, "not json": `<html>`, "empty code": ""} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if body == "" {
					var in struct{ Links []Request }
					_ = json.NewDecoder(r.Body).Decode(&in)
					_ = json.NewEncoder(w).Encode(map[string]any{"links": []map[string]string{{"delivery_id": in.Links[0].DeliveryID.String(), "code": ""}}})
					return
				}
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			c, _ := New(Config{URL: srv.URL})
			if _, err := c.Create(t.Context(), reqs(1)); err == nil || IsRejected(err) {
				t.Errorf("err = %v, want a transient error", err)
			}
		})
	}
}

func TestLoadConfigAndNew(t *testing.T) {
	if _, err := LoadConfig(viper.New()); err == nil || !strings.Contains(err.Error(), KeyURL) {
		t.Errorf("LoadConfig without %s: err = %v", KeyURL, err)
	}
	v := viper.New()
	v.Set(KeyURL, "http://redirect:8080")
	cfg, err := LoadConfig(v)
	if err != nil || cfg.URL != "http://redirect:8080" {
		t.Errorf("LoadConfig = %+v, %v", cfg, err)
	}
	if _, err := New(Config{URL: "  "}); err == nil {
		t.Error("New with a blank URL: err = nil")
	}
}

// The request and response shapes are the redirect service's (a copy, no import in production code).
func TestRequestMirrorsTheRedirectAPI(t *testing.T) {
	names := func(v any) []string {
		var out []string
		rt := reflect.TypeOf(v)
		for i := range rt.NumField() {
			name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
			out = append(out, name)
		}
		slices.Sort(out)
		return out
	}
	if got, want := names(Request{}), names(rapi.LinkRequest{}); !slices.Equal(got, want) {
		t.Errorf("Request fields %v, redirect's LinkRequest %v", got, want)
	}
	id := uuid.New()
	raw, err := json.Marshal(Request{DeliveryID: id, CampaignID: id, Channel: "email", Language: "en", Timezone: "Europe/London", TargetURL: "https://a.co"})
	if err != nil {
		t.Fatal(err)
	}
	var back rapi.LinkRequest
	if err := json.Unmarshal(raw, &back); err != nil || back.DeliveryID != id || back.Channel != "email" || back.Timezone != "Europe/London" || back.TargetURL != "https://a.co" {
		t.Errorf("redirect decoded %+v (%v)", back, err)
	}
	if rapi.MaxLinksPerRequest != MaxPerRequest {
		t.Errorf("MaxPerRequest = %d, redirect accepts %d", MaxPerRequest, rapi.MaxLinksPerRequest)
	}
}
