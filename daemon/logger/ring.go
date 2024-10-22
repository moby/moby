package logger // import "github.com/docker/docker/daemon/logger"

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

const (
	defaultRingMaxSize = 1e6 // 1MB
)

// ringLogger is a ring buffer that implements the Logger interface.
// This is used when lossy logging is OK.
type ringLogger struct {
	buffer    *messageRing
	l         Logger
	logInfo   Info
	closeFlag atomic.Bool
	wg        sync.WaitGroup
}

var (
	_ SizedLogger = (*ringLogger)(nil)
	_ LogReader   = (*ringWithReader)(nil)
)

type ringWithReader struct {
	*ringLogger
}

func (r *ringWithReader) ReadLogs(ctx context.Context, cfg ReadConfig) *LogWatcher {
	reader, ok := r.l.(LogReader)
	if !ok {
		// something is wrong if we get here
		panic("expected log reader")
	}
	return reader.ReadLogs(ctx, cfg)
}

func newRingLogger(driver Logger, logInfo Info, maxSize int64) *ringLogger {
	l := &ringLogger{
		buffer:  newRing(maxSize),
		l:       driver,
		logInfo: logInfo,
	}
	l.wg.Add(1)
	go l.run()
	return l
}

// NewRingLogger creates a new Logger that is implemented as a RingBuffer wrapping
// the passed in logger.
func NewRingLogger(driver Logger, logInfo Info, maxSize int64) Logger {
	if maxSize < 0 {
		maxSize = defaultRingMaxSize
	}
	l := newRingLogger(driver, logInfo, maxSize)
	if _, ok := driver.(LogReader); ok {
		return &ringWithReader{l}
	}
	return l
}

// BufSize returns the buffer size of the underlying logger.
// Returns -1 if the logger doesn't match SizedLogger interface.
func (r *ringLogger) BufSize() int {
	if sl, ok := r.l.(SizedLogger); ok {
		return sl.BufSize()
	}
	return -1
}

// Log queues messages into the ring buffer
func (r *ringLogger) Log(msg *Message) error {
	if r.closed() {
		return errClosed
	}
	return r.buffer.Enqueue(msg)
}

// Name returns the name of the underlying logger
func (r *ringLogger) Name() string {
	return r.l.Name()
}

func (r *ringLogger) closed() bool {
	return r.closeFlag.Load()
}

func (r *ringLogger) setClosed() {
	r.closeFlag.Store(true)
}

// Close closes the logger
func (r *ringLogger) Close() error {
	r.setClosed()
	r.buffer.Close()
	r.wg.Wait()
	// empty out the queue
	var logErr bool
	for _, msg := range r.buffer.Drain() {
		if logErr {
			// some error logging a previous message, so re-insert to message pool
			// and assume log driver is hosed
			PutMessage(msg)
			continue
		}

		if err := r.l.Log(msg); err != nil {
			logDriverError(r.l.Name(), string(msg.Line), err)
			logErr = true
		}
	}
	return r.l.Close()
}

// run consumes messages from the ring buffer and forwards them to the underling
// logger.
// This is run in a goroutine when the ringLogger is created
func (r *ringLogger) run() {
	defer r.wg.Done()
	for {
		if r.closed() {
			return
		}
		msg, err := r.buffer.Dequeue()
		if err != nil {
			// buffer is closed
			return
		}
		if err := r.l.Log(msg); err != nil {
			logDriverError(r.l.Name(), string(msg.Line), err)
		}
	}
}

type messageRing struct {
	mu sync.Mutex
	// signals callers of `Dequeue` to wake up either on `Close` or when a new `Message` is added
	wait *sync.Cond

	sizeBytes int64 // current buffer size
	maxBytes  int64 // max buffer size
	queue     []*Message
	closed    bool
}

func newRing(maxBytes int64) *messageRing {
	queueSize := 1000
	if maxBytes == 0 || maxBytes == 1 {
		// With 0 or 1 max byte size, the maximum size of the queue would only ever be 1
		// message long.
		queueSize = 1
	}

	r := &messageRing{queue: make([]*Message, 0, queueSize), maxBytes: maxBytes}
	r.wait = sync.NewCond(&r.mu)
	return r
}

// Enqueue adds a message to the buffer queue
// If the message is too big for the buffer it drops the new message.
// If there are no messages in the queue and the message is still too big, it adds the message anyway.
func (r *messageRing) Enqueue(m *Message) error {
	mSize := int64(len(m.Line))

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return errClosed
	}
	if mSize+r.sizeBytes > r.maxBytes && len(r.queue) > 0 {
		r.wait.Signal()
		r.mu.Unlock()
		return nil
	}

	r.queue = append(r.queue, m)
	r.sizeBytes += mSize
	r.wait.Signal()
	r.mu.Unlock()
	return nil
}

// Dequeue pulls a message off the queue
// If there are no messages, it waits for one.
// If the buffer is closed, it will return immediately.
func (r *messageRing) Dequeue() (*Message, error) {
	r.mu.Lock()
	for len(r.queue) == 0 && !r.closed {
		r.wait.Wait()
	}

	if r.closed {
		r.mu.Unlock()
		return nil, errClosed
	}

	msg := r.queue[0]
	r.queue = r.queue[1:]
	r.sizeBytes -= int64(len(msg.Line))
	r.mu.Unlock()
	return msg, nil
}

var errClosed = errors.New("closed")

// Close closes the buffer ensuring no new messages can be added.
// Any callers waiting to dequeue a message will be woken up.
func (r *messageRing) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}

	r.closed = true
	r.wait.Broadcast()
	r.mu.Unlock()
}

// Drain drains all messages from the queue.
// This can be used after `Close()` to get any remaining messages that were in queue.
func (r *messageRing) Drain() []*Message {
	r.mu.Lock()
	ls := make([]*Message, 0, len(r.queue))
	ls = append(ls, r.queue...)
	r.sizeBytes = 0
	r.queue = r.queue[:0]
	r.mu.Unlock()
	return ls
}
