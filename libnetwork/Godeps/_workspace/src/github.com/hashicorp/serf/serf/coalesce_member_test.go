package serf

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestMemberEventCoalesce_Basic(t *testing.T) {
	outCh := make(chan Event, 64)
	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	c := &memberEventCoalescer{
		lastEvents:   make(map[string]EventType),
		latestEvents: make(map[string]coalesceEvent),
	}

	inCh := coalescedEventCh(outCh, shutdownCh,
		5*time.Millisecond, 5*time.Millisecond, c)

	send := []Event{
		MemberEvent{
			Type:    EventMemberJoin,
			Members: []Member{Member{Name: "foo"}},
		},
		MemberEvent{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "foo"}},
		},
		MemberEvent{
			Type:    EventMemberLeave,
			Members: []Member{Member{Name: "bar"}},
		},
		MemberEvent{
			Type:    EventMemberUpdate,
			Members: []Member{Member{Name: "zip", Tags: map[string]string{"role": "foo"}}},
		},
		MemberEvent{
			Type:    EventMemberUpdate,
			Members: []Member{Member{Name: "zip", Tags: map[string]string{"role": "bar"}}},
		},
		MemberEvent{
			Type:    EventMemberReap,
			Members: []Member{Member{Name: "dead"}},
		},
	}

	for _, e := range send {
		inCh <- e
	}

	events := make(map[EventType]Event)
	timeout := time.After(10 * time.Millisecond)

MEMBEREVENTFORLOOP:
	for {
		select {
		case e := <-outCh:
			events[e.EventType()] = e
		case <-timeout:
			break MEMBEREVENTFORLOOP
		}
	}

	if len(events) != 3 {
		t.Fatalf("bad: %#v", events)
	}

	if e, ok := events[EventMemberLeave]; !ok {
		t.Fatalf("bad: %#v", events)
	} else {
		me := e.(MemberEvent)

		if len(me.Members) != 2 {
			t.Fatalf("bad: %#v", me)
		}

		expected := []string{"bar", "foo"}
		names := []string{me.Members[0].Name, me.Members[1].Name}
		sort.Strings(names)

		if !reflect.DeepEqual(expected, names) {
			t.Fatalf("bad: %#v", names)
		}
	}

	if e, ok := events[EventMemberUpdate]; !ok {
		t.Fatalf("bad: %#v", events)
	} else {
		me := e.(MemberEvent)
		if len(me.Members) != 1 {
			t.Fatalf("bad: %#v", me)
		}

		if me.Members[0].Name != "zip" {
			t.Fatalf("bad: %#v", me.Members)
		}
		if me.Members[0].Tags["role"] != "bar" {
			t.Fatalf("bad: %#v", me.Members[0].Tags)
		}
	}

	if e, ok := events[EventMemberReap]; !ok {
		t.Fatalf("bad: %#v", events)
	} else {
		me := e.(MemberEvent)
		if len(me.Members) != 1 {
			t.Fatalf("bad: %#v", me)
		}

		if me.Members[0].Name != "dead" {
			t.Fatalf("bad: %#v", me.Members)
		}
	}
}

func TestMemberEventCoalesce_TagUpdate(t *testing.T) {
	outCh := make(chan Event, 64)
	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	c := &memberEventCoalescer{
		lastEvents:   make(map[string]EventType),
		latestEvents: make(map[string]coalesceEvent),
	}

	inCh := coalescedEventCh(outCh, shutdownCh,
		5*time.Millisecond, 5*time.Millisecond, c)

	inCh <- MemberEvent{
		Type:    EventMemberUpdate,
		Members: []Member{Member{Name: "foo", Tags: map[string]string{"role": "foo"}}},
	}

	time.Sleep(10 * time.Millisecond)

	select {
	case e := <-outCh:
		if e.EventType() != EventMemberUpdate {
			t.Fatalf("expected update")
		}
	default:
		t.Fatalf("expected update")
	}

	// Second update should not be suppressed even though
	// last event was an update
	inCh <- MemberEvent{
		Type:    EventMemberUpdate,
		Members: []Member{Member{Name: "foo", Tags: map[string]string{"role": "bar"}}},
	}
	time.Sleep(10 * time.Millisecond)

	select {
	case e := <-outCh:
		if e.EventType() != EventMemberUpdate {
			t.Fatalf("expected update")
		}
	default:
		t.Fatalf("expected update")
	}
}

func TestMemberEventCoalesce_passThrough(t *testing.T) {
	cases := []struct {
		e      Event
		handle bool
	}{
		{UserEvent{}, false},
		{MemberEvent{Type: EventMemberJoin}, true},
		{MemberEvent{Type: EventMemberLeave}, true},
		{MemberEvent{Type: EventMemberFailed}, true},
		{MemberEvent{Type: EventMemberUpdate}, true},
		{MemberEvent{Type: EventMemberReap}, true},
	}

	for _, tc := range cases {
		c := &memberEventCoalescer{}
		if tc.handle != c.Handle(tc.e) {
			t.Fatalf("bad: %#v", tc.e)
		}
	}
}
