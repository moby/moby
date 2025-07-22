package networkdb

import (
	"net"
	"strings"

	"github.com/docker/go-events"
)

type opType uint8

const (
	opCreate opType = 1 + iota
	opUpdate
	opDelete
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
// sent. The watch channel is initialized with synthetic create events for all
// the existing table entries not owned by this node which match the filters.
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

	// Synthesize events for all the existing table entries not owned by
	// this node so that the watcher receives all state without racing with
	// any concurrent mutations to the table.
	nDB.RLock()
	defer nDB.RUnlock()
	if tname == "" {
		var prefix []byte
		if nid != "" {
			prefix = []byte("/" + nid + "/")
		} else {
			prefix = []byte("/")
		}
		nDB.indexes[byNetwork].Root().WalkPrefix(prefix, func(path []byte, v *entry) bool {
			if !v.deleting && v.node != nDB.config.NodeID {
				tuple := strings.SplitN(string(path[1:]), "/", 3)
				if len(tuple) == 3 {
					entryNid, entryTname, key := tuple[0], tuple[1], tuple[2]
					sink.Write(makeEvent(opCreate, entryTname, entryNid, key, v.value))
				}
			}
			return false
		})
	} else {
		prefix := []byte("/" + tname + "/")
		if nid != "" {
			prefix = append(prefix, []byte(nid+"/")...)
		}
		nDB.indexes[byTable].Root().WalkPrefix(prefix, func(path []byte, v *entry) bool {
			if !v.deleting && v.node != nDB.config.NodeID {
				tuple := strings.SplitN(string(path[1:]), "/", 3)
				if len(tuple) == 3 {
					entryTname, entryNid, key := tuple[0], tuple[1], tuple[2]
					sink.Write(makeEvent(opCreate, entryTname, entryNid, key, v.value))
				}
			}
			return false
		})
	}

	nDB.broadcaster.Add(sink)
	return ch, func() {
		nDB.broadcaster.Remove(sink)
		ch.Close()
		sink.Close()
	}
}

func makeEvent(op opType, tname, nid, key string, value []byte) events.Event {
	ev := event{
		Table:     tname,
		NetworkID: nid,
		Key:       key,
		Value:     value,
	}

	switch op {
	case opCreate:
		return CreateEvent(ev)
	case opUpdate:
		return UpdateEvent(ev)
	case opDelete:
		return DeleteEvent(ev)
	}

	return nil
}
