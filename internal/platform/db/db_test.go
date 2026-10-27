package db_test

import (
	"context"
	"testing"

	"github.com/furknozg/coldbru/internal/platform/db"
)

func TestOpen_InvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "empty scheme garbage", url: "://not a url"},
		{name: "bad pool setting", url: "postgres://u:p@localhost:5432/d?pool_max_conns=abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := db.Open(context.Background(), tt.url)
			if err == nil {
				pool.Close()
				t.Fatal("Open() error = nil, want error")
			}
			if pool != nil {
				t.Errorf("Open() pool = %v, want nil", pool)
			}
		})
	}
}
