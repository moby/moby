package events

import (
	"strconv"
	"testing"
	"time"

	"github.com/moby/moby/api/types/events"
	eventstestutils "github.com/moby/moby/v2/daemon/events/testutils"
	"github.com/moby/moby/v2/daemon/internal/timestamp"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestEventsLog(t *testing.T) {
	e := New()
	_, l1, _ := e.Subscribe()
	_, l2, _ := e.Subscribe()
	defer e.Evict(l1)
	defer e.Evict(l2)
	subscriberCount := e.SubscribersCount()
	assert.Check(t, is.Equal(subscriberCount, 2))

	e.Log("test", events.ContainerEventType, events.Actor{
		ID:         "cont",
		Attributes: map[string]string{"image": "image"},
	})
	select {
	case msg := <-l1:
		assert.Check(t, is.Len(e.events, 1))

		jmsg, ok := msg.(events.Message)
		assert.Assert(t, ok, "unexpected type: %T", msg)
		assert.Check(t, is.Equal(jmsg.Action, events.Action("test")))
		assert.Check(t, is.Equal(jmsg.Actor.ID, "cont"))
		assert.Check(t, is.Equal(jmsg.Actor.Attributes["image"], "image"))
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for broadcasted message")
	}
	select {
	case msg := <-l2:
		assert.Check(t, is.Len(e.events, 1))

		jmsg, ok := msg.(events.Message)
		assert.Assert(t, ok, "unexpected type: %T", msg)
		assert.Check(t, is.Equal(jmsg.Action, events.Action("test")))
		assert.Check(t, is.Equal(jmsg.Actor.ID, "cont"))
		assert.Check(t, is.Equal(jmsg.Actor.Attributes["image"], "image"))
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for broadcasted message")
	}
}

func TestEventsLogTimeout(t *testing.T) {
	e := New()
	_, l, _ := e.Subscribe()
	defer e.Evict(l)

	c := make(chan struct{})
	go func() {
		e.Log("test", events.ImageEventType, events.Actor{
			ID: "image",
		})
		close(c)
	}()

	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatal("Timeout publishing message")
	}
}

func TestLogEvents(t *testing.T) {
	e := New()

	for i := range eventsLimit + 16 {
		num := strconv.Itoa(i)
		e.Log(events.Action("action_"+num), events.ContainerEventType, events.Actor{
			ID:         "cont_" + num,
			Attributes: map[string]string{"image": "image_" + num},
		})
	}
	time.Sleep(50 * time.Millisecond)
	current, l, _ := e.Subscribe()
	for i := range 10 {
		num := strconv.Itoa(i + eventsLimit + 16)
		e.Log(events.Action("action_"+num), events.ContainerEventType, events.Actor{
			ID:         "cont_" + num,
			Attributes: map[string]string{"image": "image_" + num},
		})
	}
	assert.Assert(t, is.Len(e.events, eventsLimit))

	var msgs []events.Message
	for len(msgs) < 10 {
		m := <-l
		jm, ok := (m).(events.Message)
		if !ok {
			t.Fatalf("Unexpected type %T", m)
		}
		msgs = append(msgs, jm)
	}

	assert.Assert(t, is.Len(current, eventsLimit))

	first := current[0]
	assert.Check(t, is.Equal(first.Action, events.Action("action_16")))

	last := current[len(current)-1]
	assert.Check(t, is.Equal(last.Action, events.Action("action_271")))

	firstC := msgs[0]
	assert.Check(t, is.Equal(firstC.Action, events.Action("action_272")))

	lastC := msgs[len(msgs)-1]
	assert.Check(t, is.Equal(lastC.Action, events.Action("action_281")))
}

// Regression-test for https://github.com/moby/moby/issues/20999
//
// Fixtures:
//
//	2016-03-07T17:28:03.022433271+02:00 container die 0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079 (image=ubuntu, name=small_hoover)
//	2016-03-07T17:28:03.091719377+02:00 network disconnect 19c5ed41acb798f26b751e0035cd7821741ab79e2bbd59a66b5fd8abf954eaa0 (type=bridge, container=0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079, name=bridge)
//	2016-03-07T17:28:03.129014751+02:00 container destroy 0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079 (image=ubuntu, name=small_hoover)
func TestLoadBufferedEvents(t *testing.T) {
	now := time.Now()
	f, err := timestamp.GetTimestamp("2016-03-07T17:28:03.100000000+02:00", now)
	assert.NilError(t, err)

	s, sNano, err := timestamp.ParseTimestamps(f, -1)
	assert.NilError(t, err)

	m1, err := eventstestutils.Scan("2016-03-07T17:28:03.022433271+02:00 container die 0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079 (image=ubuntu, name=small_hoover)")
	assert.NilError(t, err)

	m2, err := eventstestutils.Scan("2016-03-07T17:28:03.091719377+02:00 network disconnect 19c5ed41acb798f26b751e0035cd7821741ab79e2bbd59a66b5fd8abf954eaa0 (type=bridge, container=0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079, name=bridge)")
	assert.NilError(t, err)

	m3, err := eventstestutils.Scan("2016-03-07T17:28:03.129014751+02:00 container destroy 0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079 (image=ubuntu, name=small_hoover)")
	assert.NilError(t, err)

	evts := &Events{
		events: []events.Message{*m1, *m2, *m3},
	}

	since := time.Unix(s, sNano)
	until := time.Time{}

	messages := evts.loadBufferedEvents(since, until, nil)
	assert.Assert(t, is.Len(messages, 1))
}

func TestLoadBufferedEventsOnlyFromPast(t *testing.T) {
	now := time.Now()
	f, err := timestamp.GetTimestamp("2016-03-07T17:28:03.090000000+02:00", now)
	assert.NilError(t, err)

	s, sNano, err := timestamp.ParseTimestamps(f, 0)
	assert.NilError(t, err)

	f, err = timestamp.GetTimestamp("2016-03-07T17:28:03.100000000+02:00", now)
	assert.NilError(t, err)

	u, uNano, err := timestamp.ParseTimestamps(f, 0)
	assert.NilError(t, err)

	m1, err := eventstestutils.Scan("2016-03-07T17:28:03.022433271+02:00 container die 0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079 (image=ubuntu, name=small_hoover)")
	assert.NilError(t, err)

	m2, err := eventstestutils.Scan("2016-03-07T17:28:03.091719377+02:00 network disconnect 19c5ed41acb798f26b751e0035cd7821741ab79e2bbd59a66b5fd8abf954eaa0 (type=bridge, container=0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079, name=bridge)")
	assert.NilError(t, err)

	m3, err := eventstestutils.Scan("2016-03-07T17:28:03.129014751+02:00 container destroy 0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079 (image=ubuntu, name=small_hoover)")
	assert.NilError(t, err)

	evts := &Events{
		events: []events.Message{*m1, *m2, *m3},
	}

	since := time.Unix(s, sNano)
	until := time.Unix(u, uNano)

	messages := evts.loadBufferedEvents(since, until, nil)
	assert.Assert(t, is.Len(messages, 1))
	assert.Check(t, is.Equal(messages[0].Type, events.NetworkEventType))
}

// Regression-test for https://github.com/moby/moby/issues/13753
func TestIgnoreBufferedWhenNoTimes(t *testing.T) {
	m1, err := eventstestutils.Scan("2016-03-07T17:28:03.022433271+02:00 container die 0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079 (image=ubuntu, name=small_hoover)")
	assert.NilError(t, err)

	m2, err := eventstestutils.Scan("2016-03-07T17:28:03.091719377+02:00 network disconnect 19c5ed41acb798f26b751e0035cd7821741ab79e2bbd59a66b5fd8abf954eaa0 (type=bridge, container=0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079, name=bridge)")
	assert.NilError(t, err)

	m3, err := eventstestutils.Scan("2016-03-07T17:28:03.129014751+02:00 container destroy 0b863f2a26c18557fc6cdadda007c459f9ec81b874780808138aea78a3595079 (image=ubuntu, name=small_hoover)")
	assert.NilError(t, err)

	evts := &Events{
		events: []events.Message{*m1, *m2, *m3},
	}

	since := time.Time{}
	until := time.Time{}

	messages := evts.loadBufferedEvents(since, until, nil)
	assert.Assert(t, is.Len(messages, 0))
}
