//go:build integration

package mockprovider_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/furknozg/coldbru/internal/mockprovider"
	"github.com/furknozg/coldbru/internal/notification/provider"
)

// Image tags match docker-compose.yml.
const (
	mailpitImage    = "axllent/mailpit:v1.31.5"
	mockserverImage = "mockserver/mockserver:8.0.0"
)

func startContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	t.Helper()
	c, err := testcontainers.GenericContainer(t.Context(), testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Fatalf("start %s: %v", req.Image, err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(c) })
	return c
}

func hostPort(t *testing.T, c testcontainers.Container, port string) string {
	t.Helper()
	host, err := c.Host(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.MappedPort(t.Context(), port)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%s:%s", host, p.Port())
}

// TestInboxesReceiveWhatTheMockAccepts sends one email and one SMS through the mock, with the real
// forwarder, to real Mailpit and MockServer containers, and reads them back through their APIs.
func TestInboxesReceiveWhatTheMockAccepts(t *testing.T) {
	mailpit := startContainer(t, testcontainers.ContainerRequest{
		Image:        mailpitImage,
		ExposedPorts: []string{"1025/tcp", "8025/tcp"},
		WaitingFor:   wait.ForHTTP("/readyz").WithPort("8025/tcp").WithStartupTimeout(90 * time.Second),
	})
	initializer, err := filepath.Abs("../../infra/mockserver/initializer.json")
	if err != nil {
		t.Fatal(err)
	}
	mockserver := startContainer(t, testcontainers.ContainerRequest{
		Image:        mockserverImage,
		ExposedPorts: []string{"1080/tcp"},
		Env:          map[string]string{"MOCKSERVER_INITIALIZATION_JSON_PATH": "/config/initializer.json"},
		Files:        []testcontainers.ContainerFile{{HostFilePath: initializer, ContainerFilePath: "/config/initializer.json", FileMode: 0o644}},
		WaitingFor:   wait.ForLog("started on port: 1080").WithStartupTimeout(90 * time.Second),
	})

	cfg := mockprovider.DefaultConfig()
	cfg.ErrorRate, cfg.Rate429, cfg.TimeoutRate = 0, 0, 0
	cfg.LatencyMin, cfg.LatencyMax = 0, 0
	cfg.SMTPAddr = hostPort(t, mailpit, "1025/tcp")
	cfg.MockServerURL = "http://" + hostPort(t, mockserver, "1080/tcp")
	h, err := mockprovider.NewHandler(mockprovider.HandlerConfig{Config: cfg, Forwarder: mockprovider.NewInboxForwarder(cfg)})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	client, err := provider.New(provider.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	subject := "Çağrı: sepetin seni bekliyor, %30 indirim"
	if _, err := client.SendEmail(t.Context(), "int-email-1", provider.Email{
		To: "ayse@example.com", Subject: subject, HTML: "<p>Merhaba <b>Ayşe</b></p>", Text: "Merhaba Ayşe",
	}); err != nil {
		t.Fatalf("SendEmail: %v", err)
	}
	// A repeat with the same key must not put a second mail in the inbox.
	if _, err := client.SendEmail(t.Context(), "int-email-1", provider.Email{
		To: "ayse@example.com", Subject: subject, HTML: "<p>Merhaba <b>Ayşe</b></p>", Text: "Merhaba Ayşe",
	}); err != nil {
		t.Fatalf("SendEmail repeat: %v", err)
	}
	smsText := "Merhaba Ayşe, sepetinde ürün kaldı: %30 indirim."
	if _, err := client.SendSMS(t.Context(), "int-sms-1", provider.SMS{To: "+905551112233", Text: smsText}); err != nil {
		t.Fatalf("SendSMS: %v", err)
	}
	if _, err := client.SendSMS(t.Context(), "int-sms-1", provider.SMS{To: "+905551112233", Text: smsText}); err != nil {
		t.Fatalf("SendSMS repeat: %v", err)
	}

	// Mailpit: one message, with the subject decoded.
	var list struct {
		Total    int `json:"total"`
		Messages []struct {
			ID      string                     `json:"ID"`
			Subject string                     `json:"Subject"`
			To      []struct{ Address string } `json:"To"`
		} `json:"messages"`
	}
	getJSON(t, "http://"+hostPort(t, mailpit, "8025/tcp")+"/api/v1/messages", &list)
	if list.Total != 1 || len(list.Messages) != 1 {
		t.Fatalf("Mailpit lists %d messages, want 1", list.Total)
	}
	if got := list.Messages[0]; got.Subject != subject || len(got.To) != 1 || got.To[0].Address != "ayse@example.com" {
		t.Errorf("Mailpit message = %+v, want subject %q to ayse@example.com", got, subject)
	}
	var detail struct{ Text, HTML string }
	getJSON(t, "http://"+hostPort(t, mailpit, "8025/tcp")+"/api/v1/message/"+list.Messages[0].ID, &detail)
	if !strings.Contains(detail.Text, "Merhaba Ayşe") || !strings.Contains(detail.HTML, "<b>Ayşe</b>") {
		t.Errorf("Mailpit body = text %q, html %q, want both parts", detail.Text, detail.HTML)
	}

	// MockServer: one SMS request recorded.
	reqBody := bytes.NewReader([]byte(`{"method":"POST","path":"/sms"}`))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, cfg.MockServerURL+"/mockserver/retrieve?type=REQUESTS", reqBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var recorded []json.RawMessage
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatalf("retrieve answered %q: %v", raw, err)
	}
	if len(recorded) != 1 {
		t.Fatalf("MockServer recorded %d SMS requests, want 1: %s", len(recorded), raw)
	}
	for _, want := range []string{"Ayşe", "+905551112233", "int-sms-1"} {
		if !strings.Contains(string(recorded[0]), want) && !strings.Contains(unescape(string(recorded[0])), want) {
			t.Errorf("recorded SMS lacks %q: %s", want, recorded[0])
		}
	}
}

// unescape turns \uXXXX escapes of a JSON text into characters, for a readable contains-check.
func unescape(s string) string {
	var out string
	if err := json.Unmarshal([]byte(`"`+strings.ReplaceAll(s, `"`, `\"`)+`"`), &out); err != nil {
		return s
	}
	return out
}

func getJSON(t *testing.T, url string, into any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("GET %s = %d", url, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
}
