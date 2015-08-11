package progressreader

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

// Broadcaster keeps track of one or more observers watching the progress
// of an operation. For example, if multiple clients are trying to pull an
// image, they share a Broadcaster for the download operation.
type Broadcaster struct {
	sync.Mutex
	// c is a channel that observers block on, waiting for the operation
	// to finish.
	c chan struct{}
	// cond is a condition variable used to wake up observers when there's
	// new data available.
	cond *sync.Cond
	// history is a buffer of the progress output so far, so a new observer
	// can catch up.
	history bytes.Buffer
	// wg is a WaitGroup used to wait for all writes to finish on Close
	wg sync.WaitGroup
	// isClosed is set to true when Close is called to avoid closing c
	// multiple times.
	isClosed bool
}

// NewBroadcaster returns a Broadcaster structure
func NewBroadcaster() *Broadcaster {
	b := &Broadcaster{
		c: make(chan struct{}),
	}
	b.cond = sync.NewCond(b)
	return b
}

// closed returns true if and only if the broadcaster has been closed
func (broadcaster *Broadcaster) closed() bool {
	select {
	case <-broadcaster.c:
		return true
	default:
		return false
	}
}

// receiveWrites runs as a goroutine so that writes don't block the Write
// function. It writes the new data in broadcaster.history each time there's
// activity on the broadcaster.cond condition variable.
func (broadcaster *Broadcaster) receiveWrites(observer io.Writer) {
	n := 0

	broadcaster.Lock()

	// The condition variable wait is at the end of this loop, so that the
	// first iteration will write the history so far.
	for {
		newData := broadcaster.history.Bytes()[n:]
		// Make a copy of newData so we can release the lock
		sendData := make([]byte, len(newData), len(newData))
		copy(sendData, newData)
		broadcaster.Unlock()

		if len(sendData) > 0 {
			written, err := observer.Write(sendData)
			if err != nil {
				broadcaster.wg.Done()
				return
			}
			n += written
		}

		broadcaster.Lock()

		// detect closure of the broadcast writer
		if broadcaster.closed() {
			broadcaster.Unlock()
			broadcaster.wg.Done()
			return
		}

		if broadcaster.history.Len() == n {
			broadcaster.cond.Wait()
		}

		// Mutex is still locked as the loop continues
	}
}

// Write adds data to the history buffer, and also writes it to all current
// observers.
func (broadcaster *Broadcaster) Write(p []byte) (n int, err error) {
	broadcaster.Lock()
	defer broadcaster.Unlock()

	// Is the broadcaster closed? If so, the write should fail.
	if broadcaster.closed() {
		return 0, errors.New("attempted write to closed progressreader Broadcaster")
	}

	broadcaster.history.Write(p)
	broadcaster.cond.Broadcast()

	return len(p), nil
}

// Add adds an observer to the Broadcaster. The new observer receives the
// data from the history buffer, and also all subsequent data.
func (broadcaster *Broadcaster) Add(w io.Writer) error {
	// The lock is acquired here so that Add can't race with Close
	broadcaster.Lock()
	defer broadcaster.Unlock()

	if broadcaster.closed() {
		return errors.New("attempted to add observer to closed progressreader Broadcaster")
	}

	broadcaster.wg.Add(1)
	go broadcaster.receiveWrites(w)

	return nil
}

// Close signals to all observers that the operation has finished.
func (broadcaster *Broadcaster) Close() {
	broadcaster.Lock()
	if broadcaster.isClosed {
		broadcaster.Unlock()
		return
	}
	broadcaster.isClosed = true
	close(broadcaster.c)
	broadcaster.cond.Broadcast()
	broadcaster.Unlock()

	// Don't return from Close until all writers have caught up.
	broadcaster.wg.Wait()
}

// Wait blocks until the operation is marked as completed by the Done method.
func (broadcaster *Broadcaster) Wait() {
	<-broadcaster.c
}
