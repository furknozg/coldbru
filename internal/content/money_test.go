package content

import "testing"

func TestParseMoneyAndString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    Money
		str     string
		wantErr bool
	}{
		{"0.04", 40_000, "0.0400", false},
		{"0.001", 1_000, "0.0010", false},
		{"1", 1_000_000, "1.0000", false},
		{" 0.0330 ", 33_000, "0.0330", false},
		{"0.000001", 1, "0.0000", false},
		{"0.00005", 50, "0.0001", false}, // half up
		{"0.000049", 49, "0.0000", false},
		{"0.123456", 123_456, "0.1235", false},
		{"12345.6789", 12_345_678_900, "12345.6789", false},
		{"0", 0, "0.0000", false},
		{"-1", 0, "", true},
		{"1.2345678", 0, "", true},
		{"abc", 0, "", true},
		{"", 0, "", true},
		{"1e3", 0, "", true},
		{".5", 0, "", true},
		{"99999999999999999999", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := ParseMoney(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want || got.String() != tt.str {
				t.Errorf("ParseMoney(%q) = %d (%s), want %d (%s)", tt.in, got, got, tt.want, tt.str)
			}
		})
	}
}

func TestEstimatesAreExactWhereFloatsDrift(t *testing.T) {
	t.Parallel()
	p := Prices{SMSSegment: 40_000, Email: 1_000}
	tests := []struct {
		name string
		got  Money
		want string
	}{
		{"sms 3 segments x 33", p.EstimateSMS(3, 33), "3.9600"},
		{"sms 1 segment x 0", p.EstimateSMS(1, 0), "0.0000"},
		{"sms negative counts are zero", p.EstimateSMS(-1, 10), "0.0000"},
		{"email 33", p.EstimateEmail(33), "0.0330"},
		{"email 3 (0.003, float64 gives 0.003000000000000000444)", p.EstimateEmail(3), "0.0030"},
		{"email a million", p.EstimateEmail(1_000_000), "1000.0000"},
		{"sms 0.1+0.2 style: 3 x 0.1", Prices{SMSSegment: 100_000}.EstimateSMS(1, 3), "0.3000"},
	}
	for _, tt := range tests {
		if got := tt.got.String(); got != tt.want {
			t.Errorf("%s = %s, want %s", tt.name, got, tt.want)
		}
	}
	// A price finer than the output: 0.00033 x 3 = 0.00099 rounds to 0.0010, never 0.000999999.
	if got := Money(330).Times(3).String(); got != "0.0010" {
		t.Errorf("0.00033 x 3 = %s", got)
	}
}

func TestSumMoney(t *testing.T) {
	t.Parallel()
	got, err := SumMoney("0.0330", "1.3200", "", "0.0001")
	if err != nil || got.String() != "1.3531" {
		t.Errorf("SumMoney = %s, %v", got, err)
	}
	if _, err := SumMoney("0.1", "x"); err == nil {
		t.Error("a malformed amount was summed")
	}
	if got, _ := SumMoney(); got != 0 {
		t.Errorf("empty sum = %d", got)
	}
}
