package serf

import (
	"log"
	"os"
	"testing"
	"time"
)

func TestInternalQueryName(t *testing.T) {
	name := internalQueryName(conflictQuery)
	if name != "_serf_conflict" {
		t.Fatalf("bad: %v", name)
	}
}

func TestSerfQueries_Passthrough(t *testing.T) {
	serf := &Serf{}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	outCh := make(chan Event, 4)
	shutdown := make(chan struct{})
	defer close(shutdown)
	eventCh, err := newSerfQueries(serf, logger, outCh, shutdown)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Push a user event
	eventCh <- UserEvent{LTime: 42, Name: "foo"}

	// Push a query
	eventCh <- &Query{LTime: 42, Name: "foo"}

	// Push a query
	eventCh <- MemberEvent{Type: EventMemberJoin}

	// Should get passed through
	for i := 0; i < 3; i++ {
		select {
		case <-outCh:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("time out")
		}
	}
}

func TestSerfQueries_Ping(t *testing.T) {
	serf := &Serf{}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	outCh := make(chan Event, 4)
	shutdown := make(chan struct{})
	defer close(shutdown)
	eventCh, err := newSerfQueries(serf, logger, outCh, shutdown)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Send a ping
	eventCh <- &Query{LTime: 42, Name: "_serf_ping"}

	// Should not get passed through
	select {
	case <-outCh:
		t.Fatalf("Should not passthrough query!")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSerfQueries_Conflict_SameName(t *testing.T) {
	serf := &Serf{config: &Config{NodeName: "foo"}}
	logger := log.New(os.Stderr, "", log.LstdFlags)
	outCh := make(chan Event, 4)
	shutdown := make(chan struct{})
	defer close(shutdown)
	eventCh, err := newSerfQueries(serf, logger, outCh, shutdown)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Query for our own name
	eventCh <- &Query{Name: "_serf_conflict", Payload: []byte("foo")}

	// Should not passthrough OR respond
	select {
	case <-outCh:
		t.Fatalf("Should not passthrough query!")
	case <-time.After(50 * time.Millisecond):
	}
}
