package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

func TestRunWritesTheLock(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), prompts.LockFile)
	if err := run(path); err != nil {
		t.Fatalf("first run: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(prompts.CurrentLock().Marshal()) {
		t.Error("the written lock is not the current lock")
	}
	if err := run(path); err != nil {
		t.Fatalf("second run: %v", err)
	}
	again, _ := os.ReadFile(path)
	if string(again) != string(got) {
		t.Error("running twice changed the file")
	}
}

func TestRunRefusesAChangedHashUnderTheSameVersion(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), prompts.LockFile)
	cur := prompts.CurrentLock()
	cur.Prompts[0].SHA256 = strings.Repeat("0", 64)
	if err := os.WriteFile(path, cur.Marshal(), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run(path)
	if !errors.Is(err, prompts.ErrVersionNotBumped) {
		t.Fatalf("err = %v, want ErrVersionNotBumped", err)
	}
	if b, _ := os.ReadFile(path); string(b) != string(cur.Marshal()) {
		t.Error("a refused run still rewrote the lock")
	}
}

func TestRunReportsAnUnreadableLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(bad); err == nil {
		t.Error("garbage lock accepted")
	}
	if err := run(dir); err == nil { // a directory is neither missing nor readable
		t.Error("directory accepted as lock file")
	}
}
