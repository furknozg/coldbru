// Package code makes and checks the short link codes of the redirect service (D-106). A code is ten
// base62 characters: six random ones that identify the link and a four character HMAC tag over
// them. A code whose tag does not match is rejected before any database lookup, so guessing
// codes costs the redirect service nothing.
package code

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
)

// Code layout: IDLen random characters followed by TagLen tag characters.
const (
	IDLen  = 6
	TagLen = 4
	// Length is the length of a whole code.
	Length = IDLen + TagLen
	// MinKeyLen is the shortest signing key accepted (bytes).
	MinKeyLen = 16
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// tagSpace is 62^TagLen, the number of distinct tags.
const tagSpace = 62 * 62 * 62 * 62

// Signer makes and verifies codes with one HMAC key (REDIRECT_SIGNING_KEY). It is safe for
// concurrent use.
type Signer struct {
	key []byte
}

// NewSigner returns a Signer for key. The key must be at least MinKeyLen bytes.
func NewSigner(key string) (*Signer, error) {
	if len(key) < MinKeyLen {
		return nil, fmt.Errorf("code: signing key has %d bytes, want at least %d", len(key), MinKeyLen)
	}
	return &Signer{key: []byte(key)}, nil
}

// New returns a fresh code: random identifier plus tag.
func (s *Signer) New() (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	return id + s.tag(id), nil
}

// Verify reports whether c has the right length, only base62 characters and the tag its identifier
// gets under the key. It compares in constant time.
func (s *Signer) Verify(c string) bool {
	if len(c) != Length || !isBase62(c) {
		return false
	}
	want := s.tag(c[:IDLen])
	return subtle.ConstantTimeCompare([]byte(c[IDLen:]), []byte(want)) == 1
}

// tag is the TagLen characters of HMAC-SHA256(key, id), read as a number in base 62.
func (s *Signer) tag(id string) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(id))
	n := binary.BigEndian.Uint64(m.Sum(nil)[:8]) % tagSpace
	out := make([]byte, TagLen)
	for i := TagLen - 1; i >= 0; i-- {
		out[i] = alphabet[n%62]
		n /= 62
	}
	return string(out)
}

// randomID returns IDLen uniformly random base62 characters. Bytes of 248 and above are discarded
// so that 256 mod 62 does not favour the first characters.
func randomID() (string, error) {
	out := make([]byte, 0, IDLen)
	buf := make([]byte, 16)
	for len(out) < IDLen {
		if _, err := rand.Read(buf); err != nil {
			return "", errors.Join(errors.New("code: read random bytes"), err)
		}
		for _, b := range buf {
			if b >= 248 {
				continue
			}
			out = append(out, alphabet[int(b)%62])
			if len(out) == IDLen {
				break
			}
		}
	}
	return string(out), nil
}

func isBase62(s string) bool {
	for i := range len(s) {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}
