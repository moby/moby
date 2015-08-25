package events

import (
	"testing"
	"time"

	"github.com/docker/docker/pkg/jsonmessage"
)

func TestEventsLog(t *testing.T) {
	e := New()
	_, l1 := e.Subscribe()
	_, l2 := e.Subscribe()
	defer e.Evict(l1)
	defer e.Evict(l2)
	count := e.SubscribersCount()
	if count != 2 {
		t.Fatalf("Must be 2 subscribers, got %d", count)
	}
	e.Log("test", "cont", "image")
	select {
	case msg := <-l1:
		jmsg, ok := msg.(*jsonmessage.JSONMessage)
		if !ok {
			t.Fatalf("Unexpected type %T", msg)
		}
		if len(e.events) != 1 {
			t.Fatalf("Must be only one event, got %d", len(e.events))
		}
		if jmsg.Status != "test" {
			t.Fatalf("Status should be test, got %s", jmsg.Status)
		}
		if jmsg.ID != "cont" {
			t.Fatalf("ID should be cont, got %s", jmsg.ID)
		}
		if jmsg.From != "image" {
			t.Fatalf("From should be image, got %s", jmsg.From)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for broadcasted message")
	}
	select {
	case msg := <-l2:
		jmsg, ok := msg.(*jsonmessage.JSONMessage)
		if !ok {
			t.Fatalf("Unexpected type %T", msg)
		}
		if len(e.events) != 1 {
			t.Fatalf("Must be only one event, got %d", len(e.events))
		}
		if jmsg.Status != "test" {
			t.Fatalf("Status should be test, got %s", jmsg.Status)
		}
		if jmsg.ID != "cont" {
			t.Fatalf("ID should be cont, got %s", jmsg.ID)
		}
		if jmsg.From != "image" {
			t.Fatalf("From should be image, got %s", jmsg.From)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for broadcasted message")
	}
}

func TestEventsLogTimeout(t *testing.T) {
	e := New()
	_, l := e.Subscribe()
	defer e.Evict(l)

	c := make(chan struct{})
	go func() {
		e.Log("test", "cont", "image")
		close(c)
	}()

	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatal("Timeout publishing message")
	}
}

func TestEventsCap(t *testing.T) {
	e := New()
	for i := 0; i < eventsLimit+1; i++ {
		e.Log("action", "id", "from")
	}
	// let all events go through
	time.Sleep(1 * time.Second)

	current, l := e.Subscribe()
	if len(current) != eventsLimit {
		t.Fatalf("Must be %d events, got %d", eventsLimit, len(current))
	}
	if len(e.events) != eventsLimit {
		t.Fatalf("Must be %d events, got %d", eventsLimit, len(e.events))
	}

	for i := 0; i < 10; i++ {
		e.Log("action", "id", "from")
	}
	// let all events go through
	time.Sleep(1 * time.Second)

	var msgs []*jsonmessage.JSONMessage
	for len(msgs) < 10 {
		select {
		case m := <-l:
			jm, ok := (m).(*jsonmessage.JSONMessage)
			if !ok {
				t.Fatalf("Unexpected type %T", m)
			}
			msgs = append(msgs, jm)
		default:
			t.Fatalf("There is no enough events in channel")
		}
	}
}
