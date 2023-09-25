package networkdb

import (
	"net"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/go-events"
)

type event struct {
	Table     string
	NetworkID string
	Key       string
	Value     []byte
}

// NodeTable represents table event for node join and leave
const NodeTable = "NodeTable"

// NodeAddr represents the value carried for node event in NodeTable
type NodeAddr struct {
	Addr net.IP
}

// CreateEvent generates a table entry create event to the watchers
type CreateEvent event

// UpdateEvent generates a table entry update event to the watchers
type UpdateEvent event

// DeleteEvent generates a table entry delete event to the watchers
type DeleteEvent event

// Watch creates a watcher with filters for a particular table or
// network or any combination of the tuple. If any of the
// filter is an empty string it acts as a wildcard for that
// field. Watch returns a channel of events, where the events will be
// sent.
func (nDB *NetworkDB) Watch(tname, nid string) (*events.Channel, func()) {
	var matcher events.Matcher

	if tname != "" || nid != "" {
		matcher = events.MatcherFunc(func(ev events.Event) bool {
			var evt event
			switch ev := ev.(type) {
			case CreateEvent:
				evt = event(ev)
			case UpdateEvent:
				evt = event(ev)
			case DeleteEvent:
				evt = event(ev)
			}

			if tname != "" && evt.Table != tname {
				return false
			}

			if nid != "" && evt.NetworkID != nid {
				return false
			}

			return true
		})
	}

	ch := events.NewChannel(0)
	sink := events.Sink(events.NewQueue(ch))

	if matcher != nil {
		sink = events.NewFilter(sink, matcher)
	}

	nDB.broadcaster.Add(sink)
	return ch, func() {
		nDB.broadcaster.Remove(sink)
		ch.Close()
		sink.Close()
	}
}

func makeEvent(op driverapi.EventType, tname, nid, key string, value []byte) events.Event {
	ev := event{
		Table:     tname,
		NetworkID: nid,
		Key:       key,
		Value:     value,
	}

	switch op {
	case driverapi.Create:
		return CreateEvent(ev)
	case driverapi.Update:
		return UpdateEvent(ev)
	case driverapi.Delete:
		return DeleteEvent(ev)
	}

	return nil
}
