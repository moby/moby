package events

import (
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
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
	var timer *time.Timer

retry:
	select {
	case <-rs.closed:
		return ErrSinkClosed
	default:
	}

	if backoff := rs.strategy.Proceed(event); backoff > 0 {
		if timer == nil {
			timer = time.NewTimer(backoff)
			defer timer.Stop()
		} else {
			timer.Reset(backoff)
		}

		select {
		case <-timer.C:
			goto retry
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
	select {
	case <-rs.closed:
		return ErrSinkClosed
	default:
		close(rs.closed)
		return rs.sink.Close()
	}
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

// TODO(stevvooe): We are using circuit breaker here. May want to provide
// bounded exponential backoff, as well.

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
