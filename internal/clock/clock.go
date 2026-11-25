// Package clock provides an injectable clock and a sortable ID generator so
// tests are deterministic.
package clock

import (
	"crypto/rand"
	"strings"
	"sync"
	"time"
)

// Clock returns the current time.
type Clock interface{ Now() time.Time }

// Real is the system clock, always in UTC.
type Real struct{}

// Now returns the current UTC time.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fake is a manually advanced clock for tests.
type Fake struct {
	mu sync.Mutex
	t  time.Time
}

// NewFake returns a fake clock set to t.
func NewFake(t time.Time) *Fake { return &Fake{t: t.UTC()} }

// Now returns the fake time.
func (f *Fake) Now() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.t }

// Advance moves the fake clock forward.
func (f *Fake) Advance(d time.Duration) { f.mu.Lock(); f.t = f.t.Add(d); f.mu.Unlock() }

// IDs generates sortable unique identifiers.
type IDs interface{ New() string }

const crockford = "0123456789abcdefghjkmnpqrstvwxyz"

// ULIDs generates lowercase ULIDs from a clock and crypto randomness.
type ULIDs struct{ Clock Clock }

// New returns a 26-character ULID.
func (u ULIDs) New() string {
	ms := uint64(u.Clock.Now().UnixMilli())
	var b strings.Builder
	b.Grow(26)
	for i := 9; i >= 0; i-- {
		b.WriteByte(crockford[(ms>>(uint(i)*5))&31])
	}
	r := make([]byte, 16)
	_, _ = rand.Read(r)
	for i := 0; i < 16; i++ {
		b.WriteByte(crockford[r[i]&31])
	}
	return b.String()
}

// Seq is a deterministic ID generator for tests.
type Seq struct {
	mu     sync.Mutex
	n      int
	Prefix string
}

// New returns the next ID in sequence.
func (s *Seq) New() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return s.Prefix + strings.Repeat("0", 6-len(itoa(s.n))) + itoa(s.n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
