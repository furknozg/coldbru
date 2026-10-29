package mockprovider

import (
	"sync"
	"time"
)

// record is one stored answer.
type record struct {
	hash    [32]byte
	status  int
	body    []byte
	expires time.Time
}

// store keeps the answers by idempotency key, in memory (D-081). An entry lives for ttl; Sweep drops
// the dead ones, and get ignores them in the meantime.
type store struct {
	mu  sync.Mutex
	m   map[string]record
	ttl time.Duration
	now func() time.Time
}

func newStore(ttl time.Duration, now func() time.Time) *store {
	return &store{m: make(map[string]record), ttl: ttl, now: now}
}

// get returns the live record under key.
func (s *store) get(key string) (record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.m[key]
	if !ok || !rec.expires.After(s.now()) {
		return record{}, false
	}
	return rec, true
}

// put stores the answer under key.
func (s *store) put(key string, hash [32]byte, status int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = record{hash: hash, status: status, body: body, expires: s.now().Add(s.ttl)}
}

// Sweep removes the expired records and returns how many it removed.
func (s *store) sweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now, n := s.now(), 0
	for k, rec := range s.m {
		if !rec.expires.After(now) {
			delete(s.m, k)
			n++
		}
	}
	return n
}

// len returns the number of records, expired ones included until the next sweep.
func (s *store) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.m)
}

// flight serializes the requests of one key, so a retry that arrives while the first request is still
// forwarding waits for it and then replays its answer instead of forwarding a second time.
type flight struct {
	mu sync.Mutex
	m  map[string]chan struct{}
}

func newFlight() *flight { return &flight{m: make(map[string]chan struct{})} }

// acquire blocks until key is free or done is closed; ok is false in the second case. release must be
// called once.
func (f *flight) acquire(key string, done <-chan struct{}) (release func(), ok bool) {
	for {
		f.mu.Lock()
		ch, busy := f.m[key]
		if !busy {
			ch = make(chan struct{})
			f.m[key] = ch
			f.mu.Unlock()
			return func() {
				f.mu.Lock()
				delete(f.m, key)
				f.mu.Unlock()
				close(ch)
			}, true
		}
		f.mu.Unlock()
		select {
		case <-ch:
		case <-done:
			return nil, false
		}
	}
}
