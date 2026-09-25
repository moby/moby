package actionscache

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const maxBackoff = time.Second * 90
const minBackoff = time.Second * 1

var defaultBackoffPool = &BackoffPool{}

type BackoffPool struct {
	mu      sync.Mutex
	queue   []chan struct{}
	timer   *time.Timer
	backoff time.Duration
	target  time.Time
}

func (b *BackoffPool) Wait(ctx context.Context, timeout time.Duration) error {
	b.mu.Lock()
	if b.timer == nil {
		b.mu.Unlock()
		return nil
	}

	done := make(chan struct{})
	b.queue = append(b.queue, done)

	b.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	case <-time.After(timeout):
		return errors.Errorf("maximum timeout reached")
	}
}

func (b *BackoffPool) Reset() {
	b.mu.Lock()
	b.reset()
	b.backoff = 0
	b.mu.Unlock()
}
func (b *BackoffPool) reset() {
	for _, done := range b.queue {
		close(done)
	}
	b.queue = nil
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
}

func (b *BackoffPool) trigger(t *time.Timer) {
	b.mu.Lock()
	if b.timer != t {
		// this timer is not the current one
		b.mu.Unlock()
		return
	}

	b.reset()
	b.backoff = b.backoff * 2
	if b.backoff > maxBackoff {
		b.backoff = maxBackoff
	}
	b.mu.Unlock()
}

func (b *BackoffPool) Delay() {
	b.mu.Lock()
	if b.timer != nil {
		minTime := time.Now().Add(minBackoff)
		if b.target.Before(minTime) {
			b.target = minTime
			b.timer.Stop()
			b.setupTimer()
		}
		b.mu.Unlock()
		return
	}

	if b.backoff == 0 {
		b.backoff = minBackoff
	}

	b.target = time.Now().Add(b.backoff)
	b.setupTimer()

	b.mu.Unlock()
}

func (b *BackoffPool) setupTimer() {
	var t *time.Timer
	b.timer = time.AfterFunc(time.Until(b.target), func() {
		b.trigger(t)
	})
	t = b.timer
}
