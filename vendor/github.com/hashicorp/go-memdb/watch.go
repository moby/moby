package memdb

import (
	"context"
	"time"
)

// WatchSet is a collection of watch channels.
type WatchSet map[<-chan struct{}]struct{}

// NewWatchSet constructs a new watch set.
func NewWatchSet() WatchSet {
	return make(map[<-chan struct{}]struct{})
}

// Add appends a watchCh to the WatchSet if non-nil.
func (w WatchSet) Add(watchCh <-chan struct{}) {
	if w == nil {
		return
	}

	if _, ok := w[watchCh]; !ok {
		w[watchCh] = struct{}{}
	}
}

// AddWithLimit appends a watchCh to the WatchSet if non-nil, and if the given
// softLimit hasn't been exceeded. Otherwise, it will watch the given alternate
// channel. It's expected that the altCh will be the same on many calls to this
// function, so you will exceed the soft limit a little bit if you hit this, but
// not by much.
//
// This is useful if you want to track individual items up to some limit, after
// which you watch a higher-level channel (usually a channel from start start of
// an iterator higher up in the radix tree) that will watch a superset of items.
func (w WatchSet) AddWithLimit(softLimit int, watchCh <-chan struct{}, altCh <-chan struct{}) {
	// This is safe for a nil WatchSet so we don't need to check that here.
	if len(w) < softLimit {
		w.Add(watchCh)
	} else {
		w.Add(altCh)
	}
}

// Watch is used to wait for either the watch set to trigger or a timeout.
// Returns true on timeout.
func (w WatchSet) Watch(timeoutCh <-chan time.Time) bool {
	if w == nil {
		return false
	}

	// Create a context that gets cancelled when the timeout is triggered
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-timeoutCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	return w.WatchCtx(ctx) == context.Canceled
}

// WatchCtx is used to wait for either the watch set to trigger or for the
// context to be cancelled. Watch with a timeout channel can be mimicked by
// creating a context with a deadline. WatchCtx should be preferred over Watch.
func (w WatchSet) WatchCtx(ctx context.Context) error {
	if w == nil {
		return nil
	}

	if n := len(w); n <= aFew {
		idx := 0
		chunk := make([]<-chan struct{}, aFew)
		for watchCh := range w {
			chunk[idx] = watchCh
			idx++
		}
		return watchFew(ctx, chunk)
	}

	return w.watchMany(ctx)
}

// watchMany is used if there are many watchers.
func (w WatchSet) watchMany(ctx context.Context) error {
	// Set up a goroutine for each watcher.
	triggerCh := make(chan struct{}, 1)
	watcher := func(chunk []<-chan struct{}) {
		if err := watchFew(ctx, chunk); err == nil {
			select {
			case triggerCh <- struct{}{}:
			default:
			}
		}
	}

	// Apportion the watch channels into chunks we can feed into the
	// watchFew helper.
	idx := 0
	chunk := make([]<-chan struct{}, aFew)
	for watchCh := range w {
		subIdx := idx % aFew
		chunk[subIdx] = watchCh
		idx++

		// Fire off this chunk and start a fresh one.
		if idx%aFew == 0 {
			go watcher(chunk)
			chunk = make([]<-chan struct{}, aFew)
		}
	}

	// Make sure to watch any residual channels in the last chunk.
	if idx%aFew != 0 {
		go watcher(chunk)
	}

	// Wait for a channel to trigger or timeout.
	select {
	case <-triggerCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WatchCh returns a channel that is used to wait for either the watch set to trigger
// or for the context to be cancelled. WatchCh creates a new goroutine each call, so
// callers may need to cache the returned channel to avoid creating extra goroutines.
func (w WatchSet) WatchCh(ctx context.Context) <-chan error {
	// Create the outgoing channel
	triggerCh := make(chan error, 1)

	// Create a goroutine to collect the error from WatchCtx
	go func() {
		triggerCh <- w.WatchCtx(ctx)
	}()

	return triggerCh
}
