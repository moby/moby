package events

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// RetryingSink retries the write until success or an ErrSinkClosed is
// returned. Underlying sink must have p > 0 of succeeding or the sink will
// block. Retry is configured with a RetryStrategy.  Concurrent calls to a
// retrying sink are serialized through the sink, meaning that if one is
// in-flight, another will not proceed.
type RetryingSink struct {
	sink     Sink
	strategy RetryStrategy
	closed   chan struct{}
	once     sync.Once
}

// NewRetryingSink returns a sink that will retry writes to a sink, backing
// off on failure. Parameters threshold and backoff adjust the behavior of the
// circuit breaker.
func NewRetryingSink(sink Sink, strategy RetryStrategy) *RetryingSink {
	rs := &RetryingSink{
		sink:     sink,
		strategy: strategy,
		closed:   make(chan struct{}),
	}

	return rs
}

// Write attempts to flush the events to the downstream sink until it succeeds
// or the sink is closed.
func (rs *RetryingSink) Write(event Event) error {
	logger := logrus.WithField("event", event)

retry:
	select {
	case <-rs.closed:
		return ErrSinkClosed
	default:
	}

	if backoff := rs.strategy.Proceed(event); backoff > 0 {
		select {
		case <-time.After(backoff):
			// TODO(stevvooe): This branch holds up the next try. Before, we
			// would simply break to the "retry" label and then possibly wait
			// again. However, this requires all retry strategies to have a
			// large probability of probing the sync for success, rather than
			// just backing off and sending the request.
		case <-rs.closed:
			return ErrSinkClosed
		}
	}

	if err := rs.sink.Write(event); err != nil {
		if err == ErrSinkClosed {
			// terminal!
			return err
		}

		logger := logger.WithError(err) // shadow!!

		if rs.strategy.Failure(event, err) {
			logger.Errorf("retryingsink: dropped event")
			return nil
		}

		logger.Errorf("retryingsink: error writing event, retrying")
		goto retry
	}

	rs.strategy.Success(event)
	return nil
}

// Close closes the sink and the underlying sink.
func (rs *RetryingSink) Close() error {
	rs.once.Do(func() {
		close(rs.closed)
	})

	return nil
}

func (rs *RetryingSink) String() string {
	// Serialize a copy of the RetryingSink without the sync.Once, to avoid
	// a data race.
	rs2 := map[string]interface{}{
		"sink":     rs.sink,
		"strategy": rs.strategy,
		"closed":   rs.closed,
	}
	return fmt.Sprint(rs2)
}

// RetryStrategy defines a strategy for retrying event sink writes.
//
// All methods should be goroutine safe.
type RetryStrategy interface {
	// Proceed is called before every event send. If proceed returns a
	// positive, non-zero integer, the retryer will back off by the provided
	// duration.
	//
	// An event is provided, by may be ignored.
	Proceed(event Event) time.Duration

	// Failure reports a failure to the strategy. If this method returns true,
	// the event should be dropped.
	Failure(event Event, err error) bool

	// Success should be called when an event is sent successfully.
	Success(event Event)
}

// Breaker implements a circuit breaker retry strategy.
//
// The current implementation never drops events.
type Breaker struct {
	threshold int
	recent    int
	last      time.Time
	backoff   time.Duration // time after which we retry after failure.
	mu        sync.Mutex
}

var _ RetryStrategy = &Breaker{}

// NewBreaker returns a breaker that will backoff after the threshold has been
// tripped. A Breaker is thread safe and may be shared by many goroutines.
func NewBreaker(threshold int, backoff time.Duration) *Breaker {
	return &Breaker{
		threshold: threshold,
		backoff:   backoff,
	}
}

// Proceed checks the failures against the threshold.
func (b *Breaker) Proceed(event Event) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.recent < b.threshold {
		return 0
	}

	return b.last.Add(b.backoff).Sub(time.Now())
}

// Success resets the breaker.
func (b *Breaker) Success(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.recent = 0
	b.last = time.Time{}
}

// Failure records the failure and latest failure time.
func (b *Breaker) Failure(event Event, err error) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.recent++
	b.last = time.Now().UTC()
	return false // never drop events.
}

var (
	// DefaultExponentialBackoffConfig provides a default configuration for
	// exponential backoff.
	DefaultExponentialBackoffConfig = ExponentialBackoffConfig{
		Base:   time.Second,
		Factor: time.Second,
		Max:    20 * time.Second,
	}
)

// ExponentialBackoffConfig configures backoff parameters.
//
// Note that these parameters operate on the upper bound for choosing a random
// value. For example, at Base=1s, a random value in [0,1s) will be chosen for
// the backoff value.
type ExponentialBackoffConfig struct {
	// Base is the minimum bound for backing off after failure.
	Base time.Duration

	// Factor sets the amount of time by which the backoff grows with each
	// failure.
	Factor time.Duration

	// Max is the absolute maxiumum bound for a single backoff.
	Max time.Duration
}

// ExponentialBackoff implements random backoff with exponentially increasing
// bounds as the number consecutive failures increase.
type ExponentialBackoff struct {
	failures uint64 // consecutive failure counter (needs to be 64-bit aligned)
	config   ExponentialBackoffConfig
}

// NewExponentialBackoff returns an exponential backoff strategy with the
// desired config. If config is nil, the default is returned.
func NewExponentialBackoff(config ExponentialBackoffConfig) *ExponentialBackoff {
	return &ExponentialBackoff{
		config: config,
	}
}

// Proceed returns the next randomly bound exponential backoff time.
func (b *ExponentialBackoff) Proceed(event Event) time.Duration {
	return b.backoff(atomic.LoadUint64(&b.failures))
}

// Success resets the failures counter.
func (b *ExponentialBackoff) Success(event Event) {
	atomic.StoreUint64(&b.failures, 0)
}

// Failure increments the failure counter.
func (b *ExponentialBackoff) Failure(event Event, err error) bool {
	atomic.AddUint64(&b.failures, 1)
	return false
}

// backoff calculates the amount of time to wait based on the number of
// consecutive failures.
func (b *ExponentialBackoff) backoff(failures uint64) time.Duration {
	if failures <= 0 {
		// proceed normally when there are no failures.
		return 0
	}

	factor := b.config.Factor
	if factor <= 0 {
		factor = DefaultExponentialBackoffConfig.Factor
	}

	backoff := b.config.Base + factor*time.Duration(1<<(failures-1))

	max := b.config.Max
	if max <= 0 {
		max = DefaultExponentialBackoffConfig.Max
	}

	if backoff > max || backoff < 0 {
		backoff = max
	}

	// Choose a uniformly distributed value from [0, backoff).
	return time.Duration(rand.Int63n(int64(backoff)))
}
