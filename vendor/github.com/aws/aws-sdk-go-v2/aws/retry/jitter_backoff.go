package retry

import (
	"math"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/internal/rand"
	"github.com/aws/aws-sdk-go-v2/internal/timeconv"
)

// ExponentialJitterBackoff provides backoff delays with jitter based on the
// number of attempts.
type ExponentialJitterBackoff struct {
	maxBackoff time.Duration
	// precomputed number of attempts needed to reach max backoff (legacy mode).
	maxBackoffAttempts float64

	// Base delay for non-throttle errors (x in the formula t_i = b * min(x * r^i, MAX_BACKOFF)).
	baseDelay time.Duration

	// Throttle error checker. When set and the error is a throttle, the base
	// delay is 1s regardless of the configured baseDelay.
	throttle IsErrorThrottle

	// When true, applies MAX_BACKOFF before jitter and uses throttle-aware
	// base delay.
	retries2026 bool

	randFloat64 func() (float64, error)
}

// NewExponentialJitterBackoff returns an ExponentialJitterBackoff configured
// for the max backoff.
func NewExponentialJitterBackoff(maxBackoff time.Duration) *ExponentialJitterBackoff {
	return &ExponentialJitterBackoff{
		maxBackoff: maxBackoff,
		maxBackoffAttempts: math.Log2(
			float64(maxBackoff) / float64(time.Second)),
		baseDelay:   time.Second,
		randFloat64: rand.CryptoRandFloat64,
	}
}

// exponentialJitterBackoffOption is a functional option for ExponentialJitterBackoff.
type exponentialJitterBackoffOption func(*ExponentialJitterBackoff)

// withBaseDelay sets the base delay for non-throttle errors.
func withBaseDelay(d time.Duration) exponentialJitterBackoffOption {
	return func(j *ExponentialJitterBackoff) {
		j.baseDelay = d
	}
}

// withThrottleCheck sets the throttle error checker used to determine if the
// backoff should use the throttle base delay (1s) instead of the configured
// base delay.
func withThrottleCheck(t IsErrorThrottle) exponentialJitterBackoffOption {
	return func(j *ExponentialJitterBackoff) {
		j.throttle = t
	}
}

// newExponentialJitterBackoffWithOptions returns an ExponentialJitterBackoff
// with the given options applied.
func newExponentialJitterBackoffWithOptions(maxBackoff time.Duration, optFns ...exponentialJitterBackoffOption) *ExponentialJitterBackoff {
	j := NewExponentialJitterBackoff(maxBackoff)
	j.retries2026 = true
	for _, fn := range optFns {
		fn(j)
	}
	return j
}

// BackoffDelay returns the duration to wait before the next attempt should be
// made. Returns an error if unable get a duration.
func (j *ExponentialJitterBackoff) BackoffDelay(attempt int, err error) (time.Duration, error) {
	if j.retries2026 {
		return j.backoffDelay2026(attempt, err)
	}
	return j.backoffDelayLegacy(attempt, err)
}

// backoffDelayLegacy preserves the original backoff formula: b * 2^i, capped
// at maxBackoff.
func (j *ExponentialJitterBackoff) backoffDelayLegacy(attempt int, err error) (time.Duration, error) {
	if attempt > int(j.maxBackoffAttempts) {
		return j.maxBackoff, nil
	}

	b, err := j.randFloat64()
	if err != nil {
		return 0, err
	}

	// [0.0, 1.0) * 2 ^ attempts
	ri := int64(1 << uint64(attempt))
	delaySeconds := b * float64(ri)

	return timeconv.FloatSecondsDur(delaySeconds), nil
}

// backoffDelay2026 uses throttle-aware base delay and applies MAX_BACKOFF
// before jitter: t_i = b * min(x * 2^i, MAX_BACKOFF).
func (j *ExponentialJitterBackoff) backoffDelay2026(attempt int, err error) (time.Duration, error) {
	x := j.baseDelay
	if j.throttle != nil && j.throttle.IsErrorThrottle(err) == aws.TrueTernary {
		x = time.Second
	}

	b, randErr := j.randFloat64()
	if randErr != nil {
		return 0, randErr
	}

	ri := math.Pow(2, float64(attempt))
	delaySeconds := float64(x) / float64(time.Second) * ri
	maxBackoffSeconds := float64(j.maxBackoff) / float64(time.Second)
	if delaySeconds > maxBackoffSeconds {
		delaySeconds = maxBackoffSeconds
	}

	return timeconv.FloatSecondsDur(b * delaySeconds), nil
}
