package consumer

import (
	"math"
	"math/rand/v2"
	"time"
)

// MaxDeliver is the delivery limit of a consumer: after this many attempts the
// event is logged and terminated (D-024, D-054, D-055).
const MaxDeliver = 20

// Backoff is the redelivery delay policy: exponential, capped, with jitter.
type Backoff struct {
	Base   time.Duration // delay of the first retry
	Max    time.Duration // cap before jitter
	Factor float64       // growth per attempt
	Jitter float64       // fraction, 0.2 means +-20%
}

// DefaultBackoff is the production policy: 1s, 2s, 4s, 8s ... up to 15 minutes, +-20% (D-054, D-055).
var DefaultBackoff = Backoff{Base: time.Second, Factor: 2, Max: 15 * time.Minute, Jitter: 0.2}

// Delay returns how long to wait before redelivery after the given failed
// attempt (1 for the first delivery). The result is Base*Factor^(attempt-1),
// capped at Max, then moved by a random amount within +-Jitter. It is never
// negative.
func (b Backoff) Delay(attempt int) time.Duration {
	attempt = max(attempt, 1)
	d := float64(b.Base) * math.Pow(b.Factor, float64(attempt-1))
	if limit := float64(b.Max); b.Max > 0 && d > limit {
		d = limit
	}
	if b.Jitter > 0 {
		d *= 1 + b.Jitter*(2*rand.Float64()-1)
	}
	return time.Duration(max(d, 0))
}
