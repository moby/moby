package xfer // import "github.com/docker/docker/distribution/xfer"

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/pkg/progress"
)

func TestTransfer(t *testing.T) {
	makeXferFunc := func(id string) doFunc {
		return func(progressChan chan<- progress.Progress, start <-chan struct{}, _ chan<- struct{}) transfer {
			select {
			case <-start:
			default:
				t.Errorf("%s: transfer function not started even though concurrency limit not reached", id)
			}

			xfer := newTransfer()
			go func() {
				for i := 0; i <= 10; i++ {
					progressChan <- progress.Progress{ID: id, Action: "testing", Current: int64(i), Total: 10}
					time.Sleep(10 * time.Millisecond)
				}
				close(progressChan)
			}()
			return xfer
		}
	}

	tm := newTransferManager(5)
	progressChan := make(chan progress.Progress)
	progressDone := make(chan struct{})
	receivedProgress := make(map[string]int64)

	go func() {
		for p := range progressChan {
			val, present := receivedProgress[p.ID]
			if present && p.Current <= val {
				t.Errorf("%s: got unexpected progress value: %d (expected <= %d)", p.ID, p.Current, val)
			}
			receivedProgress[p.ID] = p.Current
		}
		close(progressDone)
	}()

	// Start a few transfers
	ids := []string{"id1", "id2", "id3"}
	xfers := make([]transfer, len(ids))
	watchers := make([]*watcher, len(ids))
	for i, id := range ids {
		xfers[i], watchers[i] = tm.transfer(id, makeXferFunc(id), progress.ChanOutput(progressChan))
	}

	for i, xfer := range xfers {
		<-xfer.done()
		xfer.release(watchers[i])
	}
	close(progressChan)
	<-progressDone

	for _, id := range ids {
		if receivedProgress[id] != 10 {
			t.Fatalf("final progress value %d instead of 10", receivedProgress[id])
		}
	}
}

func TestConcurrencyLimit(t *testing.T) {
	const concurrencyLimit = 3
	var runningJobs int32

	makeXferFunc := func(id string) doFunc {
		return func(progressChan chan<- progress.Progress, start <-chan struct{}, _ chan<- struct{}) transfer {
			xfer := newTransfer()
			go func() {
				<-start
				totalJobs := atomic.AddInt32(&runningJobs, 1)
				if int(totalJobs) > concurrencyLimit {
					t.Errorf("%s: too many jobs running (%d > %d)", id, totalJobs, concurrencyLimit)
				}
				for i := 0; i <= 10; i++ {
					progressChan <- progress.Progress{ID: id, Action: "testing", Current: int64(i), Total: 10}
					time.Sleep(10 * time.Millisecond)
				}
				atomic.AddInt32(&runningJobs, -1)
				close(progressChan)
			}()
			return xfer
		}
	}

	tm := newTransferManager(concurrencyLimit)
	progressChan := make(chan progress.Progress)
	progressDone := make(chan struct{})
	receivedProgress := make(map[string]int64)

	go func() {
		for p := range progressChan {
			receivedProgress[p.ID] = p.Current
		}
		close(progressDone)
	}()

	// Start more transfers than the concurrency limit
	ids := []string{"id1", "id2", "id3", "id4", "id5", "id6", "id7", "id8"}
	xfers := make([]transfer, len(ids))
	watchers := make([]*watcher, len(ids))
	for i, id := range ids {
		xfers[i], watchers[i] = tm.transfer(id, makeXferFunc(id), progress.ChanOutput(progressChan))
	}

	for i, xfer := range xfers {
		<-xfer.done()
		xfer.release(watchers[i])
	}
	close(progressChan)
	<-progressDone

	for _, id := range ids {
		if receivedProgress[id] != 10 {
			t.Fatalf("final progress value %d instead of 10", receivedProgress[id])
		}
	}
}

func TestInactiveJobs(t *testing.T) {
	const concurrencyLimit = 3
	var runningJobs int32
	testDone := make(chan struct{})

	makeXferFunc := func(id string) doFunc {
		return func(progressChan chan<- progress.Progress, start <-chan struct{}, inactive chan<- struct{}) transfer {
			xfer := newTransfer()
			go func() {
				<-start
				totalJobs := atomic.AddInt32(&runningJobs, 1)
				if int(totalJobs) > concurrencyLimit {
					t.Errorf("%s: too many jobs running (%d > %d)", id, totalJobs, concurrencyLimit)
				}
				for i := 0; i <= 10; i++ {
					progressChan <- progress.Progress{ID: id, Action: "testing", Current: int64(i), Total: 10}
					time.Sleep(10 * time.Millisecond)
				}
				atomic.AddInt32(&runningJobs, -1)
				close(inactive)
				<-testDone
				close(progressChan)
			}()
			return xfer
		}
	}

	tm := newTransferManager(concurrencyLimit)
	progressChan := make(chan progress.Progress)
	progressDone := make(chan struct{})
	receivedProgress := make(map[string]int64)

	go func() {
		for p := range progressChan {
			receivedProgress[p.ID] = p.Current
		}
		close(progressDone)
	}()

	// Start more transfers than the concurrency limit
	ids := []string{"id1", "id2", "id3", "id4", "id5", "id6", "id7", "id8"}
	xfers := make([]transfer, len(ids))
	watchers := make([]*watcher, len(ids))
	for i, id := range ids {
		xfers[i], watchers[i] = tm.transfer(id, makeXferFunc(id), progress.ChanOutput(progressChan))
	}

	close(testDone)
	for i, xfer := range xfers {
		<-xfer.done()
		xfer.release(watchers[i])
	}
	close(progressChan)
	<-progressDone

	for _, id := range ids {
		if receivedProgress[id] != 10 {
			t.Fatalf("final progress value %d instead of 10", receivedProgress[id])
		}
	}
}

func TestWatchRelease(t *testing.T) {
	ready := make(chan struct{})

	makeXferFunc := func(id string) doFunc {
		return func(progressChan chan<- progress.Progress, start <-chan struct{}, _ chan<- struct{}) transfer {
			xfer := newTransfer()
			go func() {
				defer func() {
					close(progressChan)
				}()
				<-ready
				for i := int64(0); ; i++ {
					select {
					case <-time.After(10 * time.Millisecond):
					case <-xfer.context().Done():
						return
					}
					progressChan <- progress.Progress{ID: id, Action: "testing", Current: i, Total: 10}
				}
			}()
			return xfer
		}
	}

	tm := newTransferManager(5)

	type watcherInfo struct {
		watcher               *watcher
		progressChan          chan progress.Progress
		progressDone          chan struct{}
		receivedFirstProgress chan struct{}
	}

	progressConsumer := func(w watcherInfo) {
		first := true
		for range w.progressChan {
			if first {
				close(w.receivedFirstProgress)
			}
			first = false
		}
		close(w.progressDone)
	}

	// Start a transfer
	watchers := make([]watcherInfo, 5)
	var xfer transfer
	watchers[0].progressChan = make(chan progress.Progress)
	watchers[0].progressDone = make(chan struct{})
	watchers[0].receivedFirstProgress = make(chan struct{})
	xfer, watchers[0].watcher = tm.transfer("id1", makeXferFunc("id1"), progress.ChanOutput(watchers[0].progressChan))
	go progressConsumer(watchers[0])

	// Give it multiple watchers
	for i := 1; i != len(watchers); i++ {
		watchers[i].progressChan = make(chan progress.Progress)
		watchers[i].progressDone = make(chan struct{})
		watchers[i].receivedFirstProgress = make(chan struct{})
		watchers[i].watcher = xfer.watch(progress.ChanOutput(watchers[i].progressChan))
		go progressConsumer(watchers[i])
	}

	// Now that the watchers are set up, allow the transfer goroutine to
	// proceed.
	close(ready)

	// Confirm that each watcher gets progress output.
	for _, w := range watchers {
		<-w.receivedFirstProgress
	}

	// Release one watcher every 5ms
	for _, w := range watchers {
		xfer.release(w.watcher)
		<-time.After(5 * time.Millisecond)
	}

	// Now that all watchers have been released, Released() should
	// return a closed channel.
	<-xfer.released()

	// Done() should return a closed channel because the xfer func returned
	// due to cancellation.
	<-xfer.done()

	for _, w := range watchers {
		close(w.progressChan)
		<-w.progressDone
	}
}

func TestWatchFinishedTransfer(t *testing.T) {
	makeXferFunc := func(id string) doFunc {
		return func(progressChan chan<- progress.Progress, _ <-chan struct{}, _ chan<- struct{}) transfer {
			xfer := newTransfer()
			go func() {
				// Finish immediately
				close(progressChan)
			}()
			return xfer
		}
	}

	tm := newTransferManager(5)

	// Start a transfer
	watchers := make([]*watcher, 3)
	var xfer transfer
	xfer, watchers[0] = tm.transfer("id1", makeXferFunc("id1"), progress.ChanOutput(make(chan progress.Progress)))

	// Give it a watcher immediately
	watchers[1] = xfer.watch(progress.ChanOutput(make(chan progress.Progress)))

	// Wait for the transfer to complete
	<-xfer.done()

	// Set up another watcher
	watchers[2] = xfer.watch(progress.ChanOutput(make(chan progress.Progress)))

	// Release the watchers
	for _, w := range watchers {
		xfer.release(w)
	}

	// Now that all watchers have been released, Released() should
	// return a closed channel.
	<-xfer.released()
}

func TestDuplicateTransfer(t *testing.T) {
	ready := make(chan struct{})

	var xferFuncCalls int32

	makeXferFunc := func(id string) doFunc {
		return func(progressChan chan<- progress.Progress, _ <-chan struct{}, _ chan<- struct{}) transfer {
			atomic.AddInt32(&xferFuncCalls, 1)
			xfer := newTransfer()
			go func() {
				defer func() {
					close(progressChan)
				}()
				<-ready
				for i := int64(0); ; i++ {
					select {
					case <-time.After(10 * time.Millisecond):
					case <-xfer.context().Done():
						return
					}
					progressChan <- progress.Progress{ID: id, Action: "testing", Current: i, Total: 10}
				}
			}()
			return xfer
		}
	}

	tm := newTransferManager(5)

	type transferInfo struct {
		xfer                  transfer
		watcher               *watcher
		progressChan          chan progress.Progress
		progressDone          chan struct{}
		receivedFirstProgress chan struct{}
	}

	progressConsumer := func(t transferInfo) {
		first := true
		for range t.progressChan {
			if first {
				close(t.receivedFirstProgress)
			}
			first = false
		}
		close(t.progressDone)
	}

	// Try to start multiple transfers with the same ID
	transfers := make([]transferInfo, 5)
	for i := range transfers {
		t := &transfers[i]
		t.progressChan = make(chan progress.Progress)
		t.progressDone = make(chan struct{})
		t.receivedFirstProgress = make(chan struct{})
		t.xfer, t.watcher = tm.transfer("id1", makeXferFunc("id1"), progress.ChanOutput(t.progressChan))
		go progressConsumer(*t)
	}

	// Allow the transfer goroutine to proceed.
	close(ready)

	// Confirm that each watcher gets progress output.
	for _, t := range transfers {
		<-t.receivedFirstProgress
	}

	// Confirm that the transfer function was called exactly once.
	if xferFuncCalls != 1 {
		t.Fatal("transfer function wasn't called exactly once")
	}

	// Release one watcher every 5ms
	for _, t := range transfers {
		t.xfer.release(t.watcher)
		<-time.After(5 * time.Millisecond)
	}

	for _, t := range transfers {
		// Now that all watchers have been released, Released() should
		// return a closed channel.
		<-t.xfer.released()
		// Done() should return a closed channel because the xfer func returned
		// due to cancellation.
		<-t.xfer.done()
	}

	for _, t := range transfers {
		close(t.progressChan)
		<-t.progressDone
	}
}
