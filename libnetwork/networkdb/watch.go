package networkdb

import (
	"net"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/go-events"
)

type Event struct {
	Type      driverapi.EventType
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
//
// Deprecated: use Event, and set the Event.Type to the correct type.
type CreateEvent Event

// UpdateEvent generates a table entry update event to the watchers
//
// Deprecated: use Event, and set the Event.Type to the correct type.
type UpdateEvent Event

// DeleteEvent generates a table entry delete event to the watchers
//
// Deprecated: use Event, and set the Event.Type to the correct type.
type DeleteEvent Event

// Watch creates a watcher with filters for a particular table or
// network or any combination of the tuple. If any of the
// filter is an empty string it acts as a wildcard for that
// field. Watch returns a channel of events, where the events will be
// sent.
func (nDB *NetworkDB) Watch(tname, nid string) (*events.Channel, func()) {
	var matcher events.Matcher

	if tname != "" || nid != "" {
		matcher = events.MatcherFunc(func(ev events.Event) bool {
			evt, ok := ev.(Event)
			if !ok {
				// FIXME(thaJeztah): verify this: the old code would fall-through, and return "true" ("Match")
				return false
			}

			switch evt.Type {
			case driverapi.Create, driverapi.Delete, driverapi.Update:
				// ok
				if tname != "" && evt.Table != tname {
					return false
				}
				if nid != "" && evt.NetworkID != nid {
					return false
				}
				return true
			default:
				// FIXME(thaJeztah): verify this: the old code would fall-through, and return "true" ("Match")
				return false
			}
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
	switch op {
	case driverapi.Create, driverapi.Update, driverapi.Delete:
		return Event{
			Type:      op,
			Table:     tname,
			NetworkID: nid,
			Key:       key,
			Value:     value,
		}
	default:
		return nil
	}
}
