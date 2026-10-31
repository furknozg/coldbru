package code

import (
	"strings"
	"testing"
)

const testKey = "0123456789abcdef-test-signing-key"

func TestNewCodeVerifies(t *testing.T) {
	s, err := NewSigner(testKey)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for range 500 {
		c, err := s.New()
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != Length || !isBase62(c) {
			t.Fatalf("code %q is not %d base62 characters", c, Length)
		}
		if !s.Verify(c) {
			t.Fatalf("Verify(%q) = false for a code New made", c)
		}
		seen[c] = true
	}
	if len(seen) < 499 {
		t.Errorf("500 codes gave %d distinct ones; the random part looks broken", len(seen))
	}
}

func TestVerifyRejects(t *testing.T) {
	s, _ := NewSigner(testKey)
	other, _ := NewSigner("another-signing-key-of-16+bytes")
	good, _ := s.New()
	foreign, _ := other.New()
	flipped := good[:IDLen] + string(alphabet[(strings.IndexByte(alphabet, good[Length-1])+1)%62])
	idChanged := string(alphabet[(strings.IndexByte(alphabet, good[0])+1)%62]) + good[1:]
	tests := map[string]string{
		"empty":            "",
		"too short":        good[:Length-1],
		"too long":         good + "a",
		"foreign key":      foreign,
		"tag changed":      flipped,
		"identifier moved": idChanged,
		"not base62":       good[:Length-1] + "-",
		"url characters":   "../etc/pas",
		"space":            good[:Length-1] + " ",
	}
	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			if s.Verify(c) {
				t.Errorf("Verify(%q) = true, want false", c)
			}
		})
	}
}

func TestTagIsDeterministicPerKey(t *testing.T) {
	a, _ := NewSigner(testKey)
	b, _ := NewSigner(testKey)
	c, _ := NewSigner("a-different-signing-key-xx")
	if a.tag("abc123") != b.tag("abc123") {
		t.Error("same key and identifier gave different tags")
	}
	if a.tag("abc123") == c.tag("abc123") && a.tag("zzz999") == c.tag("zzz999") {
		t.Error("different keys gave the same tags twice")
	}
	if got := a.tag("abc123"); len(got) != TagLen || !isBase62(got) {
		t.Errorf("tag = %q, want %d base62 characters", got, TagLen)
	}
}

func TestNewSignerRejectsShortKey(t *testing.T) {
	if _, err := NewSigner("short"); err == nil {
		t.Error("NewSigner(short key) error = nil")
	}
	if _, err := NewSigner(""); err == nil {
		t.Error("NewSigner(empty key) error = nil")
	}
}
