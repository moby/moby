package serf

import (
	"reflect"
	"testing"
	"time"
)

func TestUserEventCoalesce_Basic(t *testing.T) {
	outCh := make(chan Event, 64)
	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	c := &userEventCoalescer{
		events: make(map[string]*latestUserEvents),
	}

	inCh := coalescedEventCh(outCh, shutdownCh,
		5*time.Millisecond, 5*time.Millisecond, c)

	send := []Event{
		UserEvent{
			LTime:    1,
			Name:     "foo",
			Coalesce: true,
		},
		UserEvent{
			LTime:    2,
			Name:     "foo",
			Coalesce: true,
		},
		UserEvent{
			LTime:    2,
			Name:     "bar",
			Payload:  []byte("test1"),
			Coalesce: true,
		},
		UserEvent{
			LTime:    2,
			Name:     "bar",
			Payload:  []byte("test2"),
			Coalesce: true,
		},
	}

	for _, e := range send {
		inCh <- e
	}

	var gotFoo, gotBar1, gotBar2 bool
	timeout := time.After(10 * time.Millisecond)
USEREVENTFORLOOP:
	for {
		select {
		case e := <-outCh:
			ue := e.(UserEvent)
			switch ue.Name {
			case "foo":
				if ue.LTime != 2 {
					t.Fatalf("bad ltime for foo %#v", ue)
				}
				gotFoo = true
			case "bar":
				if ue.LTime != 2 {
					t.Fatalf("bad ltime for bar %#v", ue)
				}
				if reflect.DeepEqual(ue.Payload, []byte("test1")) {
					gotBar1 = true
				}
				if reflect.DeepEqual(ue.Payload, []byte("test2")) {
					gotBar2 = true
				}
			}
		case <-timeout:
			break USEREVENTFORLOOP
		}
	}

	if !gotFoo || !gotBar1 || !gotBar2 {
		t.Fatalf("missing messages %v %v %v", gotFoo, gotBar1, gotBar2)
	}
}

func TestUserEventCoalesce_passThrough(t *testing.T) {
	cases := []struct {
		e      Event
		handle bool
	}{
		{UserEvent{Coalesce: false}, false},
		{UserEvent{Coalesce: true}, true},
		{MemberEvent{Type: EventMemberJoin}, false},
		{MemberEvent{Type: EventMemberLeave}, false},
		{MemberEvent{Type: EventMemberFailed}, false},
	}

	for _, tc := range cases {
		c := &userEventCoalescer{}
		if tc.handle != c.Handle(tc.e) {
			t.Fatalf("bad: %#v", tc.e)
		}
	}
}
