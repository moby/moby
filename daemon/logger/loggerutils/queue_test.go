package loggerutils

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/daemon/logger"
	"gotest.tools/v3/assert"
)

func TestQueue(t *testing.T) {
	q := NewMessageQueue(2)
	msg := &logger.Message{Line: []byte("hello")}

	ctx := context.Background()
	err := q.Enqueue(ctx, msg)
	assert.Check(t, err)

	recv := q.Receiver()
	// These pointer values should be the same
	assert.Equal(t, msg, <-recv)

	err = q.Enqueue(ctx, msg)
	assert.Check(t, err)

	err = q.Enqueue(ctx, msg)
	assert.Check(t, err)

	q.Close()

	// We have 2 messages in the queue
	// Even though this is closed, we should get a true value from dequeue twice.
	assert.Equal(t, msg, <-recv)
	assert.Equal(t, msg, <-recv)

	// This should not block and should return false
	_, more := <-recv
	assert.Check(t, !more, "expected no more messages in the queue")

	// Test with unbuffered
	q = &MessageQueue{}
	recv = q.Receiver()

	chAdd := make(chan error, 1)
	go func() {
		chAdd <- q.Enqueue(ctx, msg)
	}()

	assert.Equal(t, msg, <-recv)
	assert.Assert(t, <-chAdd)

	ctxC, cancel := context.WithCancel(ctx)
	cancel()

	err = q.Enqueue(ctxC, msg)
	assert.ErrorIs(t, err, context.Canceled)

	// Test that blocked senders do not cause a panic on close.
	// This check is useful because the underlying implementation uses channels
	// with the send channel eventually getting closed when q.Close is called.
	go func() {
		chAdd <- q.Enqueue(ctx, msg)
	}()

	// Wait for enqueue to be ready (or as close to ready as it can be)
	for {
		q.mu.Lock()
		if q.sendWaiters == 1 {
			q.mu.Unlock()
			break
		}
		q.mu.Unlock()
		time.Sleep(time.Millisecond)
	}

	q.Close()

	select {
	case <-time.After(5 * time.Second):
	case err := <-chAdd:
		assert.ErrorIs(t, err, ErrQueueClosed)
	}

	// Double-close should not cause any issues
	q.Close()
}
