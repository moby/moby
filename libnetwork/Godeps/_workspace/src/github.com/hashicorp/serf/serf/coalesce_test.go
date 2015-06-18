package serf

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

// Mock EventCounter type
const EventCounter EventType = 9000

type counterEvent struct {
	delta int
}

func (c counterEvent) EventType() EventType {
	return EventCounter
}
func (c counterEvent) String() string {
	return fmt.Sprintf("CounterEvent %d", c.delta)
}

// Mock coalescer
type mockCoalesce struct {
	value int
}

func (c *mockCoalesce) Handle(e Event) bool {
	return e.EventType() == EventCounter
}

func (c *mockCoalesce) Coalesce(e Event) {
	c.value += e.(counterEvent).delta
}

func (c *mockCoalesce) Flush(outChan chan<- Event) {
	outChan <- counterEvent{c.value}
	c.value = 0
}

func testCoalescer(cPeriod, qPeriod time.Duration) (chan<- Event, <-chan Event, chan<- struct{}) {
	in := make(chan Event, 64)
	out := make(chan Event)
	shutdown := make(chan struct{})
	c := &mockCoalesce{}
	go coalesceLoop(in, out, shutdown, cPeriod, qPeriod, c)
	return in, out, shutdown
}

func TestCoalescer_basic(t *testing.T) {
	in, out, shutdown := testCoalescer(5*time.Millisecond, time.Second)
	defer close(shutdown)

	send := []Event{
		counterEvent{1},
		counterEvent{39},
		counterEvent{2},
	}
	for _, e := range send {
		in <- e
	}

	select {
	case e := <-out:
		if e.EventType() != EventCounter {
			t.Fatalf("expected counter, got: %d", e.EventType())
		}

		if e.(counterEvent).delta != 42 {
			t.Fatalf("bad: %#v", e)
		}

	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout")
	}
}

func TestCoalescer_quiescent(t *testing.T) {
	// This tests the quiescence by creating a long coalescence period
	// with a short quiescent period and waiting only a multiple of the
	// quiescent period for results.
	in, out, shutdown := testCoalescer(10*time.Second, 10*time.Millisecond)
	defer close(shutdown)

	send := []Event{
		counterEvent{1},
		counterEvent{39},
		counterEvent{2},
	}
	for _, e := range send {
		in <- e
	}

	select {
	case e := <-out:
		if e.EventType() != EventCounter {
			t.Fatalf("expected counter, got: %d", e.EventType())
		}

		if e.(counterEvent).delta != 42 {
			t.Fatalf("bad: %#v", e)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout")
	}
}

func TestCoalescer_passThrough(t *testing.T) {
	in, out, shutdown := testCoalescer(time.Second, time.Second)
	defer close(shutdown)

	send := []Event{
		UserEvent{
			Name:    "test",
			Payload: []byte("foo"),
		},
	}

	for _, e := range send {
		in <- e
	}

	select {
	case e := <-out:
		if e.EventType() != EventUser {
			t.Fatalf("expected user event, got: %d", e.EventType())
		}

		if e.(UserEvent).Name != "test" {
			t.Fatalf("name should be test. %v", e)
		}

		if !reflect.DeepEqual([]byte("foo"), e.(UserEvent).Payload) {
			t.Fatalf("bad: %#v", e.(UserEvent).Payload)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timeout")
	}
}
