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

// blockingLogger simulates a logger whose Log method blocks until
// its channel is closed, reproducing the scenario in issue #51301.
type blockingLogger struct {
	once    sync.Once
	blocked chan struct{} // closed when Log is blocking
	unblock chan struct{} // close to unblock Log
}

func (l *blockingLogger) Log(*Message) error {
	l.once.Do(func() { close(l.blocked) })
	<-l.unblock
	return nil
}

func (l *blockingLogger) Name() string { return "blocking" }
func (l *blockingLogger) Close() error { return nil }

// TestRingLoggerCloseWithBlockingLog verifies that Close does not hang
// when the underlying logger's Log method is blocked (issue #51301).
// It also verifies that Close properly waits for the in-flight Log()
// goroutine before proceeding (preventing concurrent Log/Close on the
// underlying driver).
func TestRingLoggerCloseWithBlockingLog(t *testing.T) {
	bl := &blockingLogger{
		blocked: make(chan struct{}),
		unblock: make(chan struct{}),
	}
	ring := newRingLogger(bl, Info{}, defaultRingMaxSize)

	// Enqueue a message so that run() will call bl.Log().
	ring.Log(&Message{Line: []byte("hello")})

	// Wait until the logger is actually blocked inside Log().
	select {
	case <-bl.blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Log to be called")
	}

	// Close must return promptly even though Log() is still blocking.
	// Before the fix, Close() would block forever on wg.Wait() because
	// run() was stuck in the blocking Log() call.
	closed := make(chan error, 1)
	go func() {
		closed <- ring.Close()
	}()

	// Unblock the logger after a brief delay so Close() can proceed
	// without waiting the full orphan timeout. This exercises the
	// code path where the orphaned Log() goroutine completes during
	// the wait in Close().
	time.Sleep(50 * time.Millisecond)
	close(bl.unblock)

	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("unexpected Close error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close blocked; goroutine leak detected (issue #51301)")
	}
}

type nopLogger struct{}

func (nopLogger) Name() string       { return "nopLogger" }
func (nopLogger) Close() error       { return nil }
func (nopLogger) Log(*Message) error { return nil }

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
