package logger // import "github.com/docker/docker/daemon/logger"

import (
	"context"
	"strconv"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
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
	defer ring.setClosed()

	// this should never block
	ring.Log(&Message{Line: []byte("1")})
	ring.Log(&Message{Line: []byte("2")})
	ring.Log(&Message{Line: []byte("3")})

	select {
	case msg := <-mockLog.c:
		assert.Equal(t, string(msg.Line), "3")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout reading log message")
	}

	select {
	case msg := <-mockLog.c:
		t.Fatalf("expected no more messages in the queue, got: %q", string(msg.Line))
	default:
	}

	assert.Equal(t, ring.buffer.dropped, int64(2))
}

func TestRingCap(t *testing.T) {
	r := newRing(5, 0)
	for i := 0; i < 10; i++ {
		// queue messages with "0" to "10"
		// the "0" to "4" messages should be dropped since we only allow 5 bytes in the buffer
		err := r.Enqueue(&Message{Line: []byte(strconv.Itoa(i))})
		assert.NilError(t, err)
	}

	// should have messages in the queue for "0" to "4"
	for i := 5; i < 10; i++ {
		m, err := r.Dequeue()
		assert.NilError(t, err)
		assert.Equal(t, string(m.Line), strconv.Itoa(i))
	}

	// queue a message that's bigger than the buffer cap

	err := r.Enqueue(&Message{Line: []byte("hello world")})
	assert.NilError(t, err)

	// queue another message that's bigger than the buffer cap
	err = r.Enqueue(&Message{Line: []byte("eat a banana")})
	assert.NilError(t, err)

	m, err := r.Dequeue()
	assert.NilError(t, err)
	assert.Equal(t, string(m.Line), "eat a banana")
	assert.Equal(t, r.count, 0)
}

func TestRingClose(t *testing.T) {
	r := newRing(1, 0)
	err := r.Enqueue(&Message{Line: []byte("hello")})
	assert.NilError(t, err)

	r.Close()

	assert.ErrorIs(t, r.Enqueue(&Message{}), errClosed)
	assert.Equal(t, r.count, 1)

	m, err := r.Dequeue()
	assert.ErrorIs(t, err, errClosed)
	assert.Assert(t, is.Nil(m))

	ls := r.Drain()
	assert.Assert(t, is.Len(ls, 1))
	assert.Equal(t, string(ls[0].Line), "hello")
}

func TestRingDrain(t *testing.T) {
	r := newRing(5, 0)
	for i := 0; i < 5; i++ {
		err := r.Enqueue(&Message{Line: []byte(strconv.Itoa(i))})
		assert.NilError(t, err)
	}

	ls := r.Drain()
	assert.Assert(t, is.Len(ls, 5))

	for i := 0; i < 5; i++ {
		assert.Check(t, is.Equal(string(ls[i].Line), strconv.Itoa(i)))
	}
	assert.Check(t, is.Equal(r.count, 0))

	ls = r.Drain()
	assert.Assert(t, is.Len(ls, 0))
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

	for i := 0; i < b.N; i++ {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRingLoggerThroughputWithReceiverDelay0(b *testing.B) {
	l := NewRingLogger(nopLogger{}, Info{}, -1)
	msg := &Message{Line: []byte("hello humans and everyone else!")}
	b.SetBytes(int64(len(msg.Line)))

	for i := 0; i < b.N; i++ {
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

	for i := 0; i < b.N; i++ {
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

	for i := 0; i < b.N; i++ {
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

	for i := 0; i < b.N; i++ {
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

	for i := 0; i < b.N; i++ {
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

	for i := 0; i < b.N; i++ {
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

	for i := 0; i < b.N; i++ {
		if err := l.Log(msg); err != nil {
			b.Fatal(err)
		}
	}
}
