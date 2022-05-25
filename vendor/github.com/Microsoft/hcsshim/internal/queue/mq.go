package queue

import (
	"errors"
	"sync"
)

var (
	ErrQueueClosed = errors.New("the queue is closed for reading and writing")
	ErrQueueEmpty  = errors.New("the queue is empty")
)

// MessageQueue represents a threadsafe message queue to be used to retrieve or
// write messages to.
type MessageQueue struct {
	m        *sync.RWMutex
	c        *sync.Cond
	messages []interface{}
	closed   bool
}

// NewMessageQueue returns a new MessageQueue.
func NewMessageQueue() *MessageQueue {
	m := &sync.RWMutex{}
	return &MessageQueue{
		m:        m,
		c:        sync.NewCond(m),
		messages: []interface{}{},
	}
}

// Write writes `msg` to the queue.
func (mq *MessageQueue) Write(msg interface{}) error {
	mq.m.Lock()
	defer mq.m.Unlock()

	if mq.closed {
		return ErrQueueClosed
	}
	mq.messages = append(mq.messages, msg)
	// Signal a waiter that there is now a value available in the queue.
	mq.c.Signal()
	return nil
}

// Read will read a value from the queue if available, otherwise return an error.
func (mq *MessageQueue) Read() (interface{}, error) {
	mq.m.Lock()
	defer mq.m.Unlock()
	if mq.closed {
		return nil, ErrQueueClosed
	}
	if mq.isEmpty() {
		return nil, ErrQueueEmpty
	}
	val := mq.messages[0]
	mq.messages[0] = nil
	mq.messages = mq.messages[1:]
	return val, nil
}

// ReadOrWait will read a value from the queue if available, else it will wait for a
// value to become available. This will block forever if nothing gets written or until
// the queue gets closed.
func (mq *MessageQueue) ReadOrWait() (interface{}, error) {
	mq.m.Lock()
	if mq.closed {
		mq.m.Unlock()
		return nil, ErrQueueClosed
	}
	if mq.isEmpty() {
		for !mq.closed && mq.isEmpty() {
			mq.c.Wait()
		}
		mq.m.Unlock()
		return mq.Read()
	}
	val := mq.messages[0]
	mq.messages[0] = nil
	mq.messages = mq.messages[1:]
	mq.m.Unlock()
	return val, nil
}

// IsEmpty returns if the queue is empty
func (mq *MessageQueue) IsEmpty() bool {
	mq.m.RLock()
	defer mq.m.RUnlock()
	return len(mq.messages) == 0
}

// Nonexported empty check that doesn't lock so we can call this in Read and Write.
func (mq *MessageQueue) isEmpty() bool {
	return len(mq.messages) == 0
}

// Close closes the queue for future writes or reads. Any attempts to read or write from the
// queue after close will return ErrQueueClosed. This is safe to call multiple times.
func (mq *MessageQueue) Close() {
	mq.m.Lock()
	defer mq.m.Unlock()
	// Already closed
	if mq.closed {
		return
	}
	mq.messages = nil
	mq.closed = true
	// If there's anybody currently waiting on a value from ReadOrWait, we need to
	// broadcast so the read(s) can return ErrQueueClosed.
	mq.c.Broadcast()
}
