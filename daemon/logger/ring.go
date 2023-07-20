package logger // import "github.com/docker/docker/daemon/logger"

import (
	"errors"
	"sync"
	"sync/atomic"
)

const (
	defaultRingMaxSize = 1e6 // 1MB
	defaultQueueSize   = 1000
)

// RingLogger is a ring buffer that implements the Logger interface.
// This is used when lossy logging is OK.
type RingLogger struct {
	buffer    *messageRing
	l         Logger
	logInfo   Info
	closeFlag int32
	wg        sync.WaitGroup
}

var _ SizedLogger = &RingLogger{}

type ringWithReader struct {
	*RingLogger
}

func (r *ringWithReader) ReadLogs(cfg ReadConfig) *LogWatcher {
	reader, ok := r.l.(LogReader)
	if !ok {
		// something is wrong if we get here
		panic("expected log reader")
	}
	return reader.ReadLogs(cfg)
}

func newRingLogger(driver Logger, logInfo Info, maxSize int64) *RingLogger {
	if maxSize < 0 {
		maxSize = defaultRingMaxSize
	}
	l := &RingLogger{
		buffer:  newRing(maxSize, defaultQueueSize),
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
	l := newRingLogger(driver, logInfo, maxSize)
	if _, ok := driver.(LogReader); ok {
		return &ringWithReader{l}
	}
	return l
}

// BufSize returns the buffer size of the underlying logger.
// Returns -1 if the logger doesn't match SizedLogger interface.
func (r *RingLogger) BufSize() int {
	if sl, ok := r.l.(SizedLogger); ok {
		return sl.BufSize()
	}
	return -1
}

// Log queues messages into the ring buffer
func (r *RingLogger) Log(msg *Message) error {
	if r.closed() {
		return errClosed
	}
	return r.buffer.Enqueue(msg)
}

// Name returns the name of the underlying logger
func (r *RingLogger) Name() string {
	return r.l.Name()
}

func (r *RingLogger) closed() bool {
	return atomic.LoadInt32(&r.closeFlag) == 1
}

func (r *RingLogger) setClosed() {
	atomic.StoreInt32(&r.closeFlag, 1)
}

// Close closes the logger
func (r *RingLogger) Close() error {
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
// This is run in a goroutine when the RingLogger is created
func (r *RingLogger) run() {
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
	maxBytes  int64 // max buffer size size
	queue     []*Message
	closed    bool

	// head: index of the next message to dequeue from
	// tail: index of the next message to enqueue to
	// count: number of messages in the queue
	head, tail, count int

	// tracks the number of times the queue has been grown
	// Used to determine if we can shrink the queue back down on dequeue
	growCount int

	dropped int64
}

func newRing(maxBytes int64, queueSize int) *messageRing {
	if queueSize == 0 {
		queueSize = defaultQueueSize
	}

	r := &messageRing{queue: make([]*Message, queueSize), maxBytes: maxBytes}
	r.wait = sync.NewCond(&r.mu)
	return r
}

func (r *messageRing) isEmpty() bool {
	return r.head == r.tail && !r.isFull()
}

func (r *messageRing) isFull() bool {
	return r.count == cap(r.queue)
}

func (r *messageRing) next(i int) int {
	return (i + 1) % cap(r.queue)
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

	// drop old messages until there is enough space for the new message
	// This is not accounting for slots in the queue, just the size of the messages
	for mSize+r.sizeBytes > r.maxBytes && !r.isEmpty() {
		r.dequeue()
		r.dropped++
	}

	if r.isFull() {
		// We've run out of space to store messages in the circular buffer even though we have not reached our byte limit.
		// This means we need to resize the queue to make room for more messages.
		//
		r.growCount++
		buf := make([]*Message, len(r.queue)*2)
		r.copyTo(buf)
		r.queue = buf
		r.head = 0
		r.tail = r.count
	}

	r.queue[r.tail] = m
	r.tail = r.next(r.tail)
	r.sizeBytes += mSize
	r.count++

	r.wait.Signal()
	r.mu.Unlock()
	return nil
}

// Dequeue pulls a message off the queue
// If there are no messages, it waits for one.
// If the buffer is closed, it will return immediately.
func (r *messageRing) Dequeue() (*Message, error) {
	r.mu.Lock()
	for r.isEmpty() && !r.closed {
		r.wait.Wait()
	}

	if r.closed {
		r.mu.Unlock()
		return nil, errClosed
	}

	msg := r.dequeue()
	r.mu.Unlock()
	return msg, nil
}

// callers must validate that there is at least one message in the queue
// callers must hold the lock
func (r *messageRing) dequeue() *Message {
	msg := r.queue[r.head]
	r.queue[r.head] = nil
	r.head = r.next(r.head)
	r.sizeBytes -= int64(len(msg.Line))
	r.count--

	if r.growCount > 0 && r.count <= cap(r.queue)/4 {
		// We've shrunk the queue enough that we can resize it back down.
		// This is to prevent the queue from growing too large and never shrinking back down.
		buf := make([]*Message, cap(r.queue)/2)
		r.copyTo(buf)
		r.queue = buf
		r.head = 0
		r.tail = r.count
		r.growCount--
	}
	return msg
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

func (r *messageRing) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func (r *messageRing) copyTo(buf []*Message) {
	if r.isEmpty() {
		return
	}

	// Here we have a circular buffer that we need to copy to the passed in slice.
	// The assumption here is that `ls` is at least as large enough to hold all the messages in the queue.
	// It maybe larger than the queue, but it must not be smaller.

	// +---+---+---+---+---+---+---+---+---+---+
	// | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 |
	// +---+---+---+---+---+---+---+---+---+---+
	//   ^                                   ^
	//   |                                   |
	// head                                tail

	// The above example is a simplified view of the queue. The head is at index 0 (beginning) and the tail is at index 9 (end).
	// In reality this could happen at any point in the queue. The head could be at index 5 and the tail at index 4.

	// Copy from head marker of the slice
	// Since this is a circular buffer we need to account for the wrap around.
	if r.head < r.tail {
		copy(buf, r.queue[r.head:r.tail])
	} else {
		n := copy(buf, r.queue[r.head:])
		copy(buf[n:], r.queue[:r.tail])
	}

}

// Drain drains all messages from the queue.
// This can be used after `Close()` to get any remaining messages that were in queue.
func (r *messageRing) Drain() []*Message {
	r.mu.Lock()
	ls := make([]*Message, r.count)

	r.copyTo(ls)

	r.sizeBytes = 0
	r.head = 0
	r.tail = 0
	r.count = 0

	r.mu.Unlock()
	return ls
}
