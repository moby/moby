package networkdb

import (
	"net"
	"strings"

	"github.com/docker/go-events"
)

type WatchEvent struct {
	Table     string
	NetworkID string
	Key       string
	Value     []byte // Current value of the entry, or nil if deleted
	Prev      []byte // Previous value of the entry, or nil if created
}

func (e WatchEvent) IsCreate() bool {
	return e.Prev == nil && e.Value != nil
}

func (e WatchEvent) IsUpdate() bool {
	return e.Prev != nil && e.Value != nil
}

func (e WatchEvent) IsDelete() bool {
	return e.Prev != nil && e.Value == nil
}

func (e WatchEvent) String() string {
	kind := "Unknown"
	switch {
	case e.IsCreate():
		kind = "Create"
	case e.IsUpdate():
		kind = "Update"
	case e.IsDelete():
		kind = "Delete"
	}
	return kind + "(" + e.Table + "/" + e.NetworkID + "/" + e.Key + ")"
}

// NodeTable represents table event for node join and leave
const NodeTable = "NodeTable"

// NodeAddr represents the value carried for node event in NodeTable
type NodeAddr struct {
	Addr net.IP
}

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
			evt := ev.(WatchEvent)

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
					sink.Write(WatchEvent{
						Table:     entryTname,
						NetworkID: entryNid,
						Key:       key,
						Value:     v.value,
					})
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
					sink.Write(WatchEvent{
						Table:     entryTname,
						NetworkID: entryNid,
						Key:       key,
						Value:     v.value,
					})
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
