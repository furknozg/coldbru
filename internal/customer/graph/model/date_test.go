package model_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

func TestDateUnmarshalGQL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      any
		want    time.Time
		wantErr bool
	}{
		{name: "plain date", in: "2026-01-31", want: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)},
		{name: "leap day", in: "2024-02-29", want: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)},
		{name: "not a leap year", in: "2026-02-29", wantErr: true},
		{name: "month 13", in: "2026-13-01", wantErr: true},
		{name: "day 32", in: "2026-01-32", wantErr: true},
		{name: "datetime", in: "2026-01-31T10:00:00Z", wantErr: true},
		{name: "slashes", in: "2026/01/31", wantErr: true},
		{name: "no zero padding", in: "2026-1-3", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "number rejected", in: 20260131, wantErr: true},
		{name: "nil rejected", in: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got model.Date
			err := got.UnmarshalGQL(tt.in)
			if tt.wantErr {
				if !errors.Is(err, model.ErrInvalidDate) {
					t.Fatalf("UnmarshalGQL(%#v) error = %v, want ErrInvalidDate", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalGQL(%#v) error = %v", tt.in, err)
			}
			if !got.Time.Equal(tt.want) {
				t.Errorf("UnmarshalGQL(%#v) = %v, want %v", tt.in, got.Time, tt.want)
			}
		})
	}
}

func TestDateMarshalGQL(t *testing.T) {
	t.Parallel()
	d, err := model.ParseDate("2026-10-27")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	d.MarshalGQL(&buf)
	if got, want := buf.String(), `"2026-10-27"`; got != want {
		t.Errorf("MarshalGQL = %s, want %s", got, want)
	}
}
