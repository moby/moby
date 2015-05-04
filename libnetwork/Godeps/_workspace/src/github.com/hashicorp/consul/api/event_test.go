package api

import (
	"testing"
)

func TestEvent_FireList(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	event := c.Event()

	params := &UserEvent{Name: "foo"}
	id, meta, err := event.Fire(params, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.RequestTime == 0 {
		t.Fatalf("bad: %v", meta)
	}

	if id == "" {
		t.Fatalf("invalid: %v", id)
	}

	events, qm, err := event.List("", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if qm.LastIndex != event.IDToIndex(id) {
		t.Fatalf("Bad: %#v", qm)
	}

	if events[len(events)-1].ID != id {
		t.Fatalf("bad: %#v", events)
	}
}
