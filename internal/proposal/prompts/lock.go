package prompts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// LockFile is the committed lock file next to this package (D-059). `make generate-prompts`
// rewrites it; TestLockFileIsFresh fails when it is stale.
const LockFile = "prompts.lock.json"

// LockEntry pins one package: its version identity and the sha256 of its request prefix.
type LockEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

// Lock is the content of prompts.lock.json, ordered by name.
type Lock struct {
	Prompts []LockEntry `json:"prompts"`
}

// CurrentLock computes the lock of the shipped packages.
func CurrentLock() Lock { return lockOf(All()) }

func lockOf(pkgs []*Package) Lock {
	l := Lock{Prompts: make([]LockEntry, 0, len(pkgs))}
	for _, p := range pkgs {
		l.Prompts = append(l.Prompts, LockEntry{Name: p.Name, Version: p.Version, SHA256: p.SHA256})
	}
	sort.Slice(l.Prompts, func(i, j int) bool { return l.Prompts[i].Name < l.Prompts[j].Name })
	return l
}

// Marshal encodes the lock as indented JSON with a trailing newline.
func (l Lock) Marshal() []byte {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		panic("prompts: marshal lock: " + err.Error()) // strings only
	}
	return append(b, '\n')
}

// ParseLock decodes a lock file, rejecting unknown fields.
func ParseLock(b []byte) (Lock, error) {
	var l Lock
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&l); err != nil {
		return Lock{}, fmt.Errorf("prompts: parse %s: %w", LockFile, err)
	}
	return l, nil
}

func (l Lock) find(name string) (LockEntry, bool) {
	for _, e := range l.Prompts {
		if e.Name == name {
			return e, true
		}
	}
	return LockEntry{}, false
}

// ErrVersionNotBumped is wrapped by the problems that mean a prompt changed while its version did not.
var ErrVersionNotBumped = errors.New("prompt changed without a version bump")

// bumpProblem describes a package whose hash moved under an unchanged version (D-059).
func bumpProblem(old, cur LockEntry) error {
	return fmt.Errorf("%w: %s@%s hashed %.12s in %s and now hashes %.12s; raise version in %s/manifest.yaml (MAJOR output contract, MINOR behaviour, PATCH anything else), then run `make generate-prompts`",
		ErrVersionNotBumped, cur.Name, cur.Version, old.SHA256, LockFile, cur.SHA256, cur.Name)
}

// Check compares the committed lock with the current one and returns every problem, in package
// order: a hash change under the same version asks for a version bump, any other difference for
// `make generate-prompts` (D-059).
func (l Lock) Check(current Lock) []error {
	var out []error
	for _, cur := range current.Prompts {
		old, ok := l.find(cur.Name)
		switch {
		case !ok:
			out = append(out, fmt.Errorf("prompts: %s is not in %s: run `make generate-prompts`", cur.Name, LockFile))
		case old.Version == cur.Version && old.SHA256 != cur.SHA256:
			out = append(out, bumpProblem(old, cur))
		case old != cur:
			out = append(out, fmt.Errorf("prompts: %s is stale in %s (locked %s, now %s): run `make generate-prompts`", cur.Name, LockFile, old.Version, cur.Version))
		}
	}
	for _, old := range l.Prompts {
		if _, ok := current.find(old.Name); !ok {
			out = append(out, fmt.Errorf("prompts: %s lists %s, which no longer exists: run `make generate-prompts`", LockFile, old.Name))
		}
	}
	return out
}

// Regenerate returns the lock to write for current, given the committed one. It refuses to
// record a changed hash under an unchanged version, so the generator cannot paper over a missing
// version bump; every other difference is simply taken over.
func (l Lock) Regenerate(current Lock) (Lock, error) {
	var errs []error
	for _, cur := range current.Prompts {
		if old, ok := l.find(cur.Name); ok && old.Version == cur.Version && old.SHA256 != cur.SHA256 {
			errs = append(errs, bumpProblem(old, cur))
		}
	}
	if len(errs) > 0 {
		return Lock{}, errors.Join(errs...)
	}
	return current, nil
}
