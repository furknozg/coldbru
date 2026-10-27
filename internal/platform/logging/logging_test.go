package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/furknozg/coldbru/internal/platform/logging"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		want    slog.Level
		wantErr bool
	}{
		{name: "debug", in: "debug", want: slog.LevelDebug},
		{name: "info", in: "info", want: slog.LevelInfo},
		{name: "warn", in: "warn", want: slog.LevelWarn},
		{name: "error", in: "error", want: slog.LevelError},
		{name: "case insensitive", in: "WaRn", want: slog.LevelWarn},
		{name: "unknown", in: "verbose", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := logging.ParseLevel(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("level = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewUnknownLevel(t *testing.T) {
	t.Parallel()
	if _, err := logging.New(&bytes.Buffer{}, "loud"); err == nil {
		t.Fatal("want error for unknown level")
	}
}

func TestNewWritesJSONAndFiltersByLevel(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l, err := logging.New(&buf, "WARN")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	l.Info("hidden")
	l.Warn("shown", "k", "v")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("output is not one JSON object: %v (%q)", err, buf.String())
	}
	if rec["msg"] != "shown" || rec["level"] != "WARN" || rec["k"] != "v" {
		t.Errorf("unexpected record: %v", rec)
	}
}
