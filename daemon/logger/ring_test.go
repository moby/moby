package logger

import (
	"context"
	"errors"
	"fmt"
	"slices"
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

type fallibleLogger struct {
	mu        sync.Mutex
	errs      []error
	alwaysErr error
	attempts  chan struct{}
	received  chan string
}

func (l *fallibleLogger) Log(msg *Message) error {
	l.mu.Lock()
	err := l.alwaysErr
	if err == nil && len(l.errs) > 0 {
		err, l.errs = l.errs[0], l.errs[1:]
	}
	l.mu.Unlock()

	if l.attempts != nil {
		select {
		case l.attempts <- struct{}{}:
		default:
		}
	}
	if err != nil {
		return err
	}
	line := string(msg.Line)
	PutMessage(msg)
	l.received <- line
	return nil
}

func (l *fallibleLogger) Name() string { return "fallible" }

func (l *fallibleLogger) Close() error {
	if l.received != nil {
		close(l.received)
	}
	return nil
}

func TestRingLoggerRetries(t *testing.T) {
	t.Run("default retries recover transient failure in order", func(t *testing.T) {
		drv := &fallibleLogger{
			errs:     []error{errors.New("transient 1"), errors.New("transient 2")},
			received: make(chan string, 2),
		}
		ring := NewRingLogger(drv, Info{}, -1)

		if err := ring.Log(&Message{Source: "stdout", Line: []byte("first")}); err != nil {
			t.Fatal(err)
		}
		if err := ring.Log(&Message{Source: "stdout", Line: []byte("second")}); err != nil {
			t.Fatal(err)
		}

		var got []string
		for range 2 {
			select {
			case line := <-drv.received:
				got = append(got, line)
			case <-time.After(3 * time.Second):
				t.Fatalf("timed out waiting for retried messages; got %v", got)
			}
		}
		if err := ring.Close(); err != nil {
			t.Fatal(err)
		}
		want := []string{"first", "second"}
		if !slices.Equal(got, want) {
			t.Fatalf("got messages %v, want %v", got, want)
		}
	})

	t.Run("bounded retries drop exhausted message and return to pool", func(t *testing.T) {
		drv := &fallibleLogger{
			errs: []error{
				nil,                        // "a" succeeds
				errors.New("lost try 1"),   // "lost" initial attempt fails
				errors.New("lost retry 1"), // "lost" retry #1 fails -> exhausted
				errors.New("b try 1"),      // "b" initial attempt fails, retry #1 succeeds
			},
			received: make(chan string, 3),
		}
		ring := NewRingLogger(drv, Info{
			Config: map[string]string{"max-non-blocking-retries": "1"},
		}, -1)

		lostMsg := &Message{Source: "stdout", Line: []byte("lost")}
		for _, m := range []*Message{
			{Source: "stdout", Line: []byte("a")},
			lostMsg,
			{Source: "stdout", Line: []byte("b")},
		} {
			if err := ring.Log(m); err != nil {
				t.Fatal(err)
			}
		}

		var got []string
		for range 2 {
			select {
			case line := <-drv.received:
				got = append(got, line)
			case <-time.After(3 * time.Second):
				t.Fatalf("timed out waiting for messages; got %v", got)
			}
		}
		if err := ring.Close(); err != nil {
			t.Fatal(err)
		}
		want := []string{"a", "b"}
		if !slices.Equal(got, want) {
			t.Fatalf("got messages %v, want %v", got, want)
		}
		if len(lostMsg.Line) != 0 || lostMsg.Source != "" {
			t.Fatalf("exhausted message was not reset via PutMessage: %+v", lostMsg)
		}
	})

	t.Run("zero retries drops on first failure", func(t *testing.T) {
		drv := &fallibleLogger{
			errs:     []error{errors.New("fail first")},
			received: make(chan string, 2),
		}
		ring := NewRingLogger(drv, Info{
			Config: map[string]string{"max-non-blocking-retries": "0"},
		}, -1)

		dropped := &Message{Source: "stdout", Line: []byte("drop")}
		if err := ring.Log(dropped); err != nil {
			t.Fatal(err)
		}
		if err := ring.Log(&Message{Source: "stdout", Line: []byte("keep")}); err != nil {
			t.Fatal(err)
		}

		select {
		case got := <-drv.received:
			if got != "keep" {
				t.Fatalf("got message %q, want %q", got, "keep")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for message")
		}
		if err := ring.Close(); err != nil {
			t.Fatal(err)
		}
		if len(dropped.Line) != 0 || dropped.Source != "" {
			t.Fatalf("dropped message was not reset via PutMessage: %+v", dropped)
		}
	})

	t.Run("close stops infinite retries promptly", func(t *testing.T) {
		drv := &fallibleLogger{
			alwaysErr: errors.New("persistent failure"),
			attempts:  make(chan struct{}, 16),
		}
		ring := NewRingLogger(drv, Info{
			Config: map[string]string{"max-non-blocking-retries": "-1"},
		}, -1)

		inFlight := &Message{Source: "stdout", Line: []byte("buffered")}
		if err := ring.Log(inFlight); err != nil {
			t.Fatal(err)
		}

		// Wait for initial attempt and at least one retry.
		for range 2 {
			select {
			case <-drv.attempts:
			case <-time.After(3 * time.Second):
				t.Fatal("timed out waiting for retry attempts")
			}
		}

		closed := make(chan error, 1)
		go func() {
			closed <- ring.Close()
		}()
		select {
		case err := <-closed:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Close blocked while infinite retries were active")
		}
		if len(inFlight.Line) != 0 || inFlight.Source != "" {
			t.Fatalf("in-flight message was not reset via PutMessage on Close: %+v", inFlight)
		}
	})
}

func TestValidateLogOptsMaxNonBlockingRetries(t *testing.T) {
	const drvName = "test-ring-retry-opts"
	t.Cleanup(func() {
		factory.m.Lock()
		delete(factory.registry, drvName)
		delete(factory.optValidator, drvName)
		factory.m.Unlock()
	})
	if err := RegisterLogDriver(drvName, func(Info) (Logger, error) { return nopLogger{}, nil }); err != nil {
		t.Fatal(err)
	}
	if err := RegisterLogOptValidator(drvName, func(cfg map[string]string) error {
		for k := range cfg {
			return fmt.Errorf("unexpected driver opt %q", k)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, retries := range []string{"-1", "0", "3"} {
		cfg := map[string]string{
			"mode":                     "non-blocking",
			"max-non-blocking-retries": retries,
		}
		if err := ValidateLogOpts(drvName, cfg); err != nil {
			t.Fatalf("ValidateLogOpts(%q) returned unexpected error: %v", retries, err)
		}
	}

	if err := ValidateLogOpts(drvName, map[string]string{"max-non-blocking-retries": "3"}); err == nil {
		t.Fatal("expected error when max-non-blocking-retries is set without mode=non-blocking")
	}
	if err := ValidateLogOpts(drvName, map[string]string{
		"mode":                     "non-blocking",
		"max-non-blocking-retries": "not-an-int",
	}); err == nil {
		t.Fatal("expected error for non-integer max-non-blocking-retries")
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
