package xfer // import "github.com/docker/docker/distribution/xfer"

import (
	"context"
	"runtime"
	"sync"

	"github.com/docker/docker/pkg/progress"
	"github.com/pkg/errors"
)

// DoNotRetry is an error wrapper indicating that the error cannot be resolved
// with a retry.
type DoNotRetry struct {
	Err error
}

// Error returns the stringified representation of the encapsulated error.
func (e DoNotRetry) Error() string {
	return e.Err.Error()
}

// IsDoNotRetryError returns true if the error is caused by DoNotRetry error,
// and the transfer should not be retried.
func IsDoNotRetryError(err error) bool {
	var dnr DoNotRetry
	return errors.As(err, &dnr)
}

// watcher is returned by Watch and can be passed to Release to stop watching.
type watcher struct {
	// signalChan is used to signal to the watcher goroutine that
	// new progress information is available, or that the transfer
	// has finished.
	signalChan chan struct{}
	// releaseChan signals to the watcher goroutine that the watcher
	// should be detached.
	releaseChan chan struct{}
	// running remains open as long as the watcher is watching the
	// transfer. It gets closed if the transfer finishes or the
	// watcher is detached.
	running chan struct{}
}

// transfer represents an in-progress transfer.
type transfer interface {
	watch(progressOutput progress.Output) *watcher
	release(*watcher)
	context() context.Context
	close()
	done() <-chan struct{}
	released() <-chan struct{}
	broadcast(mainProgressChan <-chan progress.Progress)
}

type xfer struct {
	mu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	// watchers keeps track of the goroutines monitoring progress output,
	// indexed by the channels that release them.
	watchers map[chan struct{}]*watcher

	// lastProgress is the most recently received progress event.
	lastProgress progress.Progress
	// hasLastProgress is true when lastProgress has been set.
	hasLastProgress bool

	// running remains open as long as the transfer is in progress.
	running chan struct{}
	// releasedChan stays open until all watchers release the transfer and
	// the transfer is no longer tracked by the transferManager.
	releasedChan chan struct{}

	// broadcastDone is true if the main progress channel has closed.
	broadcastDone bool
	// closed is true if Close has been called
	closed bool
	// broadcastSyncChan allows watchers to "ping" the broadcasting
	// goroutine to wait for it for deplete its input channel. This ensures
	// a detaching watcher won't miss an event that was sent before it
	// started detaching.
	broadcastSyncChan chan struct{}
}

// newTransfer creates a new transfer.
func newTransfer() transfer {
	t := &xfer{
		watchers:          make(map[chan struct{}]*watcher),
		running:           make(chan struct{}),
		releasedChan:      make(chan struct{}),
		broadcastSyncChan: make(chan struct{}),
	}

	// This uses context.Background instead of a caller-supplied context
	// so that a transfer won't be cancelled automatically if the client
	// which requested it is ^C'd (there could be other viewers).
	t.ctx, t.cancel = context.WithCancel(context.Background())

	return t
}

// Broadcast copies the progress and error output to all viewers.
func (t *xfer) broadcast(mainProgressChan <-chan progress.Progress) {
	for {
		var (
			p  progress.Progress
			ok bool
		)
		select {
		case p, ok = <-mainProgressChan:
		default:
			// We've depleted the channel, so now we can handle
			// reads on broadcastSyncChan to let detaching watchers
			// know we're caught up.
			select {
			case <-t.broadcastSyncChan:
				continue
			case p, ok = <-mainProgressChan:
			}
		}

		t.mu.Lock()
		if ok {
			t.lastProgress = p
			t.hasLastProgress = true
			for _, w := range t.watchers {
				select {
				case w.signalChan <- struct{}{}:
				default:
				}
			}
		} else {
			t.broadcastDone = true
		}
		t.mu.Unlock()
		if !ok {
			close(t.running)
			return
		}
	}
}

// Watch adds a watcher to the transfer. The supplied channel gets progress
// updates and is closed when the transfer finishes.
func (t *xfer) watch(progressOutput progress.Output) *watcher {
	t.mu.Lock()
	defer t.mu.Unlock()

	w := &watcher{
		releaseChan: make(chan struct{}),
		signalChan:  make(chan struct{}),
		running:     make(chan struct{}),
	}

	t.watchers[w.releaseChan] = w

	if t.broadcastDone {
		close(w.running)
		return w
	}

	go func() {
		defer func() {
			close(w.running)
		}()
		var (
			done           bool
			lastWritten    progress.Progress
			hasLastWritten bool
		)
		for {
			t.mu.Lock()
			hasLastProgress := t.hasLastProgress
			lastProgress := t.lastProgress
			t.mu.Unlock()

			// Make sure we don't write the last progress item
			// twice.
			if hasLastProgress && (!done || !hasLastWritten || lastProgress != lastWritten) {
				progressOutput.WriteProgress(lastProgress)
				lastWritten = lastProgress
				hasLastWritten = true
			}

			if done {
				return
			}

			select {
			case <-w.signalChan:
			case <-w.releaseChan:
				done = true
				// Since the watcher is going to detach, make
				// sure the broadcaster is caught up so we
				// don't miss anything.
				select {
				case t.broadcastSyncChan <- struct{}{}:
				case <-t.running:
				}
			case <-t.running:
				done = true
			}
		}
	}()

	return w
}

// Release is the inverse of Watch; indicating that the watcher no longer wants
// to be notified about the progress of the transfer. All calls to Watch must
// be paired with later calls to Release so that the lifecycle of the transfer
// is properly managed.
func (t *xfer) release(watcher *watcher) {
	t.mu.Lock()
	delete(t.watchers, watcher.releaseChan)

	if len(t.watchers) == 0 {
		if t.closed {
			// released may have been closed already if all
			// watchers were released, then another one was added
			// while waiting for a previous watcher goroutine to
			// finish.
			select {
			case <-t.releasedChan:
			default:
				close(t.releasedChan)
			}
		} else {
			t.cancel()
		}
	}
	t.mu.Unlock()

	close(watcher.releaseChan)
	// Block until the watcher goroutine completes
	<-watcher.running
}

// Done returns a channel which is closed if the transfer completes or is
// cancelled. Note that having 0 watchers causes a transfer to be cancelled.
func (t *xfer) done() <-chan struct{} {
	// Note that this doesn't return t.ctx.Done() because that channel will
	// be closed the moment Cancel is called, and we need to return a
	// channel that blocks until a cancellation is actually acknowledged by
	// the transfer function.
	return t.running
}

// Released returns a channel which is closed once all watchers release the
// transfer AND the transfer is no longer tracked by the transferManager.
func (t *xfer) released() <-chan struct{} {
	return t.releasedChan
}

// Context returns the context associated with the transfer.
func (t *xfer) context() context.Context {
	return t.ctx
}

// Close is called by the transferManager when the transfer is no longer
// being tracked.
func (t *xfer) close() {
	t.mu.Lock()
	t.closed = true
	if len(t.watchers) == 0 {
		close(t.releasedChan)
	}
	t.mu.Unlock()
}

// doFunc is a function called by the transferManager to actually perform
// a transfer. It should be non-blocking. It should wait until the start channel
// is closed before transferring any data. If the function closes inactive, that
// signals to the transferManager that the job is no longer actively moving
// data - for example, it may be waiting for a dependent transfer to finish.
// This prevents it from taking up a slot.
type doFunc func(progressChan chan<- progress.Progress, start <-chan struct{}, inactive chan<- struct{}) transfer

// transferManager is used by LayerDownloadManager and LayerUploadManager to
// schedule and deduplicate transfers. It is up to the transferManager
// to make the scheduling and concurrency decisions.
type transferManager struct {
	mu sync.Mutex

	concurrencyLimit int
	activeTransfers  int
	transfers        map[string]transfer
	waitingTransfers []chan struct{}
}

// newTransferManager returns a new transferManager.
func newTransferManager(concurrencyLimit int) *transferManager {
	return &transferManager{
		concurrencyLimit: concurrencyLimit,
		transfers:        make(map[string]transfer),
	}
}

// setConcurrency sets the concurrencyLimit
func (tm *transferManager) setConcurrency(concurrency int) {
	tm.mu.Lock()
	tm.concurrencyLimit = concurrency
	tm.mu.Unlock()
}

// transfer checks if a transfer matching the given key is in progress. If not,
// it starts one by calling xferFunc. The caller supplies a channel which
// receives progress output from the transfer.
func (tm *transferManager) transfer(key string, xferFunc doFunc, progressOutput progress.Output) (transfer, *watcher) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for {
		xfer, present := tm.transfers[key]
		if !present {
			break
		}
		// transfer is already in progress.
		watcher := xfer.watch(progressOutput)

		select {
		case <-xfer.context().Done():
			// We don't want to watch a transfer that has been cancelled.
			// Wait for it to be removed from the map and try again.
			xfer.release(watcher)
			tm.mu.Unlock()
			// The goroutine that removes this transfer from the
			// map is also waiting for xfer.Done(), so yield to it.
			// This could be avoided by adding a Closed method
			// to transfer to allow explicitly waiting for it to be
			// removed the map, but forcing a scheduling round in
			// this very rare case seems better than bloating the
			// interface definition.
			runtime.Gosched()
			<-xfer.done()
			tm.mu.Lock()
		default:
			return xfer, watcher
		}
	}

	start := make(chan struct{})
	inactive := make(chan struct{})

	if tm.concurrencyLimit == 0 || tm.activeTransfers < tm.concurrencyLimit {
		close(start)
		tm.activeTransfers++
	} else {
		tm.waitingTransfers = append(tm.waitingTransfers, start)
	}

	mainProgressChan := make(chan progress.Progress)
	xfer := xferFunc(mainProgressChan, start, inactive)
	watcher := xfer.watch(progressOutput)
	go xfer.broadcast(mainProgressChan)
	tm.transfers[key] = xfer

	// When the transfer is finished, remove from the map.
	go func() {
		for {
			select {
			case <-inactive:
				tm.mu.Lock()
				tm.inactivate(start)
				tm.mu.Unlock()
				inactive = nil
			case <-xfer.done():
				tm.mu.Lock()
				if inactive != nil {
					tm.inactivate(start)
				}
				delete(tm.transfers, key)
				tm.mu.Unlock()
				xfer.close()
				return
			}
		}
	}()

	return xfer, watcher
}

func (tm *transferManager) inactivate(start chan struct{}) {
	// If the transfer was started, remove it from the activeTransfers
	// count.
	select {
	case <-start:
		// Start next transfer if any are waiting
		if len(tm.waitingTransfers) != 0 {
			close(tm.waitingTransfers[0])
			tm.waitingTransfers = tm.waitingTransfers[1:]
		} else {
			tm.activeTransfers--
		}
	default:
	}
}
