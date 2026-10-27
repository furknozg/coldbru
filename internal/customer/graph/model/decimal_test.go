package model_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

func TestDecimalUnmarshalGQL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      any
		want    model.Decimal
		wantErr bool
	}{
		{name: "integer", in: "100", want: "100"},
		{name: "one fraction digit", in: "12.5", want: "12.5"},
		{name: "two fraction digits", in: "1234.50", want: "1234.50"},
		{name: "negative", in: "-0.99", want: "-0.99"},
		{name: "zero", in: "0", want: "0"},
		{name: "three fraction digits", in: "1.234", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "plus sign", in: "+1", wantErr: true},
		{name: "trailing dot", in: "1.", wantErr: true},
		{name: "leading dot", in: ".5", wantErr: true},
		{name: "exponent", in: "1e3", wantErr: true},
		{name: "spaces", in: " 1.00", wantErr: true},
		{name: "comma", in: "1,00", wantErr: true},
		{name: "text", in: "abc", wantErr: true},
		{name: "float rejected", in: 12.5, wantErr: true},
		{name: "int rejected", in: 12, wantErr: true},
		{name: "json number rejected", in: json.Number("12"), wantErr: true},
		{name: "bool rejected", in: true, wantErr: true},
		{name: "nil rejected", in: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got model.Decimal
			err := got.UnmarshalGQL(tt.in)
			if tt.wantErr {
				if !errors.Is(err, model.ErrInvalidDecimal) {
					t.Fatalf("UnmarshalGQL(%#v) error = %v, want ErrInvalidDecimal", tt.in, err)
				}
				if got != "" {
					t.Errorf("value changed on error: %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalGQL(%#v) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("UnmarshalGQL(%#v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecimalMarshalGQL(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	model.Decimal("1234.50").MarshalGQL(&buf)
	if got, want := buf.String(), `"1234.50"`; got != want {
		t.Errorf("MarshalGQL = %s, want %s", got, want)
	}
}

func TestParseDecimal(t *testing.T) {
	t.Parallel()
	if _, err := model.ParseDecimal("1.001"); !errors.Is(err, model.ErrInvalidDecimal) {
		t.Errorf("ParseDecimal(1.001) error = %v, want ErrInvalidDecimal", err)
	}
	d, err := model.ParseDecimal("7.10")
	if err != nil || d.String() != "7.10" {
		t.Errorf("ParseDecimal(7.10) = %q, %v", d, err)
	}
}
