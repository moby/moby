package logger

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

type mockLogger struct{ c chan *Message }

func (l *mockLogger) Log(msg *Message) error {
	l.c <- msg
	return nil
}

func (l *mockLogger) Name() string {
	return "mock"
}

func (l *mockLogger) Close() error {
	return nil
}

func TestRingLogger(t *testing.T) {
	mockLog := &mockLogger{make(chan *Message)} // no buffer on this channel
	ring := newRingLogger(mockLog, Info{}, 1)
	defer ring.Close()

	// this should never block
	ring.Log(&Message{Line: []byte("1")})
	ring.Log(&Message{Line: []byte("2")})
	ring.Log(&Message{Line: []byte("3")})

	select {
	case msg := <-mockLog.c:
		if string(msg.Line) != "1" {
			t.Fatalf("got unexpected msg: %q", string(msg.Line))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout reading log message")
	}

	select {
	case msg := <-mockLog.c:
		t.Fatalf("expected no more messages in the queue, got: %q", string(msg.Line))
	default:
	}
}

func TestRingCap(t *testing.T) {
	r := newRing(5)
	for i := range 10 {
		// queue messages with "0" to "10"
		// the "5" to "10" messages should be dropped since we only allow 5 bytes in the buffer
		if err := r.Enqueue(&Message{Line: []byte(strconv.Itoa(i))}); err != nil {
			t.Fatal(err)
		}
	}

	// should have messages in the queue for "0" to "4"
	for i := range 5 {
		m, err := r.Dequeue()
		if err != nil {
			t.Fatal(err)
		}
		if string(m.Line) != strconv.Itoa(i) {
			t.Fatalf("got unexpected message for iter %d: %s", i, string(m.Line))
		}
	}

	// queue a message that's bigger than the buffer cap
	if err := r.Enqueue(&Message{Line: []byte("hello world")}); err != nil {
		t.Fatal(err)
	}

	// queue another message that's bigger than the buffer cap
	if err := r.Enqueue(&Message{Line: []byte("eat a banana")}); err != nil {
		t.Fatal(err)
	}

	m, err := r.Dequeue()
	if err != nil {
		t.Fatal(err)
	}
	if string(m.Line) != "hello world" {
		t.Fatalf("got unexpected message: %s", string(m.Line))
	}
	if len(r.queue) != 0 {
		t.Fatalf("expected queue to be empty, got: %d", len(r.queue))
	}
}

func TestRingClose(t *testing.T) {
	r := newRing(1)
	if err := r.Enqueue(&Message{Line: []byte("hello")}); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if err := r.Enqueue(&Message{}); !errors.Is(err, errClosed) {
		t.Fatalf("expected errClosed, got: %v", err)
	}
	if len(r.queue) != 1 {
		t.Fatal("expected empty queue")
	}
	if m, err := r.Dequeue(); err == nil || m != nil {
		t.Fatal("expected err on Dequeue after close")
	}

	ls := r.Drain()
	if len(ls) != 1 {
		t.Fatalf("expected one message: %v", ls)
	}
	if string(ls[0].Line) != "hello" {
		t.Fatalf("got unexpected message: %s", string(ls[0].Line))
	}
}

func TestRingDrain(t *testing.T) {
	r := newRing(5)
	for i := range 5 {
		if err := r.Enqueue(&Message{Line: []byte(strconv.Itoa(i))}); err != nil {
			t.Fatal(err)
		}
	}

	ls := r.Drain()
	if len(ls) != 5 {
		t.Fatal("got unexpected length after drain")
	}

	for i := range 5 {
		if string(ls[i].Line) != strconv.Itoa(i) {
			t.Fatalf("got unexpected message at position %d: %s", i, string(ls[i].Line))
		}
	}
	if r.sizeBytes != 0 {
		t.Fatalf("expected buffer size to be 0 after drain, got: %d", r.sizeBytes)
	}

	ls = r.Drain()
	if len(ls) != 0 {
		t.Fatalf("expected 0 messages on 2nd drain: %v", ls)
	}
}

type nopLogger struct{}

func (nopLogger) Name() string       { return "nopLogger" }
func (nopLogger) Close() error       { return nil }
func (nopLogger) Log(*Message) error { return nil }

// flakyLogger fails the first N calls to Log, then succeeds.
type flakyLogger struct {
	mu        sync.Mutex
	failCount int
	logs      []*Message
}

func (l *flakyLogger) Name() string { return "flaky" }
func (l *flakyLogger) Close() error { return nil }
func (l *flakyLogger) Log(msg *Message) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failCount > 0 {
		l.failCount--
		return errors.New("simulated log failure")
	}
	l.logs = append(l.logs, msg)
	return nil
}

// TestRingLoggerRetryOnError verifies that messages are not dropped when the
// underlying logger returns transient errors. They should be requeued and
// delivered once the logger recovers.
func TestRingLoggerRetryOnError(t *testing.T) {
	flaky := &flakyLogger{failCount: 3}
	ring := newRingLogger(flaky, Info{}, 100)
	defer ring.Close()

	msg := &Message{Line: []byte("hello")}
	if err := ring.Log(msg); err != nil {
		t.Fatal(err)
	}

	// Wait for the consumer to retry and eventually succeed.
	time.Sleep(500 * time.Millisecond)

	flaky.mu.Lock()
	if len(flaky.logs) != 1 {
		t.Fatalf("expected 1 log message, got %d", len(flaky.logs))
	}
	if string(flaky.logs[0].Line) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(flaky.logs[0].Line))
	}
	flaky.mu.Unlock()
}

// TestRingLoggerDoesNotDropOnError verifies that multiple messages are buffered
// while the logger is failing and all are eventually delivered.
func TestRingLoggerDoesNotDropOnError(t *testing.T) {
	flaky := &flakyLogger{failCount: 5}
	ring := newRingLogger(flaky, Info{}, 1000)
	defer ring.Close()

	for i := range 3 {
		if err := ring.Log(&Message{Line: []byte(strconv.Itoa(i))}); err != nil {
			t.Fatal(err)
		}
	}

	// Wait for the consumer to work through the retries.
	time.Sleep(800 * time.Millisecond)

	flaky.mu.Lock()
	if len(flaky.logs) != 3 {
		t.Fatalf("expected 3 log messages, got %d", len(flaky.logs))
	}
	for i := range 3 {
		if string(flaky.logs[i].Line) != strconv.Itoa(i) {
			t.Fatalf("expected message %d to be %q, got %q", i, strconv.Itoa(i), string(flaky.logs[i].Line))
		}
	}
	flaky.mu.Unlock()
}

// TestRingLoggerRequeuePreservesOrder verifies that message order is preserved
// when the logger fails intermittently.
func TestRingLoggerRequeuePreservesOrder(t *testing.T) {
	flaky := &flakyLogger{failCount: 2}
	ring := newRingLogger(flaky, Info{}, 1000)
	defer ring.Close()

	// Queue three messages. The first will fail twice, then succeed.
	for i := range 3 {
		if err := ring.Log(&Message{Line: []byte(strconv.Itoa(i))}); err != nil {
			t.Fatal(err)
		}
	}

	// Wait for retries to complete.
	time.Sleep(500 * time.Millisecond)

	flaky.mu.Lock()
	if len(flaky.logs) != 3 {
		t.Fatalf("expected 3 log messages, got %d", len(flaky.logs))
	}
	for i := range 3 {
		if string(flaky.logs[i].Line) != strconv.Itoa(i) {
			t.Fatalf("expected message %d to be %q, got %q", i, strconv.Itoa(i), string(flaky.logs[i].Line))
		}
	}
	flaky.mu.Unlock()
}

// TestRingLoggerRequeueClosed verifies that requeue fails gracefully when the
// ring is closed.
func TestRingLoggerRequeueClosed(t *testing.T) {
	r := newRing(100)
	r.Close()
	msg := &Message{Line: []byte("test")}
	if r.requeue(msg) {
		t.Fatal("expected requeue to fail on closed ring")
	}
}

func BenchmarkRingLoggerThroughputNoReceiver(b *testing.B) {
	mockLog := &mockLogger{make(chan *Message)}
	defer mockLog.Close()
	l := NewRingLogger(mockLog, Info{}, -1)
	msg := &Message{Line: []byte("hello humans and everyone else!")}
	b.SetBytes(int64(len(msg.Line)))

	for b.Loop() {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRingLoggerThroughputWithReceiverDelay0(b *testing.B) {
	l := NewRingLogger(nopLogger{}, Info{}, -1)
	msg := &Message{Line: []byte("hello humans and everyone else!")}
	b.SetBytes(int64(len(msg.Line)))

	for b.Loop() {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}

func consumeWithDelay(delay time.Duration, c <-chan *Message) (cancel func()) {
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		close(started)
		ticker := time.NewTicker(delay)
		for range ticker.C {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-c:
			}
		}
	}()
	<-started
	return cancel
}

func BenchmarkRingLoggerThroughputConsumeDelay1(b *testing.B) {
	mockLog := &mockLogger{make(chan *Message)}
	defer mockLog.Close()
	l := NewRingLogger(mockLog, Info{}, -1)
	msg := &Message{Line: []byte("hello humans and everyone else!")}
	b.SetBytes(int64(len(msg.Line)))

	cancel := consumeWithDelay(1*time.Millisecond, mockLog.c)
	defer cancel()

	for b.Loop() {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRingLoggerThroughputConsumeDelay10(b *testing.B) {
	mockLog := &mockLogger{make(chan *Message)}
	defer mockLog.Close()
	l := NewRingLogger(mockLog, Info{}, -1)
	msg := &Message{Line: []byte("hello humans and everyone else!")}
	b.SetBytes(int64(len(msg.Line)))

	cancel := consumeWithDelay(10*time.Millisecond, mockLog.c)
	defer cancel()

	for b.Loop() {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRingLoggerThroughputConsumeDelay50(b *testing.B) {
	mockLog := &mockLogger{make(chan *Message)}
	defer mockLog.Close()
	l := NewRingLogger(mockLog, Info{}, -1)
	msg := &Message{Line: []byte("hello humans and everyone else!")}
	b.SetBytes(int64(len(msg.Line)))

	cancel := consumeWithDelay(50*time.Millisecond, mockLog.c)
	defer cancel()

	for b.Loop() {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRingLoggerThroughputConsumeDelay100(b *testing.B) {
	mockLog := &mockLogger{make(chan *Message)}
	defer mockLog.Close()
	l := NewRingLogger(mockLog, Info{}, -1)
	msg := &Message{Line: []byte("hello humans and everyone else!")}
	b.SetBytes(int64(len(msg.Line)))

	cancel := consumeWithDelay(100*time.Millisecond, mockLog.c)
	defer cancel()

	for b.Loop() {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRingLoggerThroughputConsumeDelay300(b *testing.B) {
	mockLog := &mockLogger{make(chan *Message)}
	defer mockLog.Close()
	l := NewRingLogger(mockLog, Info{}, -1)
	msg := &Message{Line: []byte("hello humans and everyone else!")}
	b.SetBytes(int64(len(msg.Line)))

	cancel := consumeWithDelay(300*time.Millisecond, mockLog.c)
	defer cancel()

	for b.Loop() {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRingLoggerThroughputConsumeDelay500(b *testing.B) {
	mockLog := &mockLogger{make(chan *Message)}
	defer mockLog.Close()
	l := NewRingLogger(mockLog, Info{}, -1)
	msg := &Message{Line: []byte("hello humans and everyone else!")}
	b.SetBytes(int64(len(msg.Line)))

	cancel := consumeWithDelay(500*time.Millisecond, mockLog.c)
	defer cancel()

	for b.Loop() {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}
