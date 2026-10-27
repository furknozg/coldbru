package audience

import (
	"slices"
	"testing"
)

func TestKey(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"İstanbul", "istanbul"},
		{"Istanbul", "istanbul"},
		{"istanbul", "istanbul"},
		{"ISTANBUL", "istanbul"},
		{"ıstanbul", "istanbul"},
		{"İstanbul", "istanbul"}, // I + combining dot above
		{"ESKISEHIR", "eskisehir"},
		{"Eskişehir", "eskisehir"},
		{"DIYARBAKIR", "diyarbakir"},
		{"Diyarbakır", "diyarbakir"},
		{"Muğla", "mugla"},
		{"MUGLA", "mugla"},
		{"London", "london"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := Key(tt.in); got != tt.want {
				t.Errorf("Key(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCitiesLookup(t *testing.T) {
	t.Parallel()
	c, err := NewCities([]string{"İstanbul", "Eskişehir", "Diyarbakır", "London", "London"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"istanbul", "İstanbul", true},
		{"ISTANBUL", "İstanbul", true},
		{"ESKISEHIR", "Eskişehir", true},
		{"eskişehir", "Eskişehir", true},
		{"DIYARBAKIR", "Diyarbakır", true},
		{"diyarbakir", "Diyarbakır", true},
		{"LONDON", "London", true},
		{"Atlantis", "", false},
	}
	for _, tt := range tests {
		got, ok := c.Lookup(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Lookup(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
	if got, want := c.Names(), []string{"Diyarbakır", "Eskişehir", "London", "İstanbul"}; !slices.Equal(got, want) {
		t.Errorf("Names() = %v, want %v (duplicates removed, sorted)", got, want)
	}
}

func TestNewCitiesRejectsAmbiguousNames(t *testing.T) {
	t.Parallel()
	if _, err := NewCities([]string{"İstanbul", "Istanbul"}); err == nil {
		t.Fatal("NewCities with two spellings of one key: error = nil, want error")
	}
}

func TestSuggest(t *testing.T) {
	t.Parallel()
	cities := []string{"Ankara", "Antalya", "Adana", "Bursa", "İstanbul", "İzmir", "Izmit", "Kayseri"}
	tests := []struct {
		name, in string
		want     []string
	}{
		{"typo", "Istnbul", []string{"İstanbul"}},
		{"case and dot", "istanbul", []string{"İstanbul"}},
		{"prefix", "ist", []string{"İstanbul"}},
		{"prefix with several", "an", []string{"Ankara", "Antalya"}},
		{"distance 2", "Ankra", []string{"Ankara"}},
		{"nothing close", "Atlantis", []string{}},
		{"empty", "", []string{}},
		{"closest first, capped at three", "izm", []string{"İzmir", "Izmit"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Suggest(tt.in, cities)
			if got == nil {
				t.Fatal("Suggest returned nil, want a non-nil slice")
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Suggest(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}

	t.Run("at most three", func(t *testing.T) {
		t.Parallel()
		got := Suggest("aaa", []string{"aab", "aac", "aad", "aae", "aaf"})
		if len(got) != 3 {
			t.Errorf("len = %d (%v), want 3", len(got), got)
		}
	})
	t.Run("closed list", func(t *testing.T) {
		t.Parallel()
		if got := Suggest("gold", loyaltyValues); !slices.Equal(got, []string{"Gold"}) {
			t.Errorf("Suggest(gold) = %v, want [Gold]", got)
		}
	})
}
