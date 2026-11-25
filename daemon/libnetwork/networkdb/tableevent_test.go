package networkdb

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/docker/go-events"
	"github.com/hashicorp/memberlist"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/lamport"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWatch_out_of_order(t *testing.T) {
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.Assert(t, nDB.JoinNetwork("network1"))

	(&eventDelegate{nDB}).NotifyJoin(&memberlist.Node{
		Name: "node1",
		Addr: net.IPv4(1, 2, 3, 4),
	})

	d := &delegate{nDB}

	msgs := messageBuffer{t: t}
	appendTableEvent := tableEventHelper(&msgs, "node1", "network1", "table1")
	msgs.Append(MessageType_NETWORK_EVENT, &NetworkEvent{
		Type:      NetworkEvent_JOIN,
		LTime:     1,
		NodeName:  "node1",
		NetworkID: "network1",
	})
	appendTableEvent(1, TableEvent_CREATE, "tombstone1", []byte("a"))
	appendTableEvent(2, TableEvent_DELETE, "tombstone1", []byte("b"))
	appendTableEvent(3, TableEvent_CREATE, "key1", []byte("value1"))
	d.NotifyMsg(msgs.Compound())
	msgs.Reset()

	nDB.CreateEntry("table1", "network1", "local1", []byte("should not see me in watch events"))
	watch, cancel := nDB.Watch("table1", "network1")
	defer cancel()

	got := drainChannel(watch.C)
	assert.Check(t, is.DeepEqual(got, []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Value: []byte("value1")},
	}))

	// Receive events from node1, with events not received or received out of order
	// Create, (hidden update), delete
	appendTableEvent(4, TableEvent_CREATE, "key2", []byte("a"))
	appendTableEvent(6, TableEvent_DELETE, "key2", []byte("b"))
	// (Hidden recreate), delete
	appendTableEvent(8, TableEvent_DELETE, "key2", []byte("c"))
	// (Hidden recreate), update
	appendTableEvent(10, TableEvent_UPDATE, "key2", []byte("d"))

	// Update, create
	appendTableEvent(11, TableEvent_UPDATE, "key3", []byte("b"))
	appendTableEvent(10, TableEvent_CREATE, "key3", []byte("a"))

	// (Hidden create), update, update
	appendTableEvent(13, TableEvent_UPDATE, "key4", []byte("b"))
	appendTableEvent(14, TableEvent_UPDATE, "key4", []byte("c"))

	// Delete, create
	appendTableEvent(16, TableEvent_DELETE, "key5", []byte("a"))
	appendTableEvent(15, TableEvent_CREATE, "key5", []byte("a"))
	// (Hidden recreate), delete
	appendTableEvent(18, TableEvent_DELETE, "key5", []byte("b"))

	d.NotifyMsg(msgs.Compound())
	msgs.Reset()

	got = drainChannel(watch.C)
	assert.Check(t, is.DeepEqual(got, []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key2", Value: []byte("a")},
		// Delete value should match last observed value,
		// irrespective of the content of the delete event over the wire.
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key2", Prev: []byte("a")},
		// Updates to previously-deleted keys should be observed as creates.
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key2", Value: []byte("d")},

		// Out-of-order update events should be observed as creates.
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key3", Value: []byte("b")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key4", Value: []byte("b")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key4", Prev: []byte("b"), Value: []byte("c")},

		// key5 should not appear in the events.
	}))
}

func TestWatch_filters(t *testing.T) {
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.Assert(t, nDB.JoinNetwork("network1"))
	assert.Assert(t, nDB.JoinNetwork("network2"))

	(&eventDelegate{nDB}).NotifyJoin(&memberlist.Node{
		Name: "node1",
		Addr: net.IPv4(1, 2, 3, 4),
	})

	var ltime lamport.Clock
	msgs := messageBuffer{t: t}
	msgs.Append(MessageType_NETWORK_EVENT, &NetworkEvent{
		Type:      NetworkEvent_JOIN,
		LTime:     uint64(ltime.Increment()),
		NodeName:  "node1",
		NetworkID: "network1",
	})
	msgs.Append(MessageType_NETWORK_EVENT, &NetworkEvent{
		Type:      NetworkEvent_JOIN,
		LTime:     uint64(ltime.Increment()),
		NodeName:  "node1",
		NetworkID: "network2",
	})
	for _, nid := range []string{"network1", "network2"} {
		for _, tname := range []string{"table1", "table2"} {
			msgs.Append(MessageType_TABLE_EVENT, &TableEvent{
				Type:      TableEvent_CREATE,
				LTime:     uint64(ltime.Increment()),
				NodeName:  "node1",
				NetworkID: nid,
				TableName: tname,
				Key:       nid + "." + tname + ".dead",
				Value:     []byte("deaddead"),
			})
			msgs.Append(MessageType_TABLE_EVENT, &TableEvent{
				Type:      TableEvent_DELETE,
				LTime:     uint64(ltime.Increment()),
				NodeName:  "node1",
				NetworkID: nid,
				TableName: tname,
				Key:       nid + "." + tname + ".dead",
				Value:     []byte("deaddead"),
			})
			msgs.Append(MessageType_TABLE_EVENT, &TableEvent{
				Type:      TableEvent_CREATE,
				LTime:     uint64(ltime.Increment()),
				NodeName:  "node1",
				NetworkID: nid,
				TableName: tname,
				Key:       nid + "." + tname + ".update",
				Value:     []byte("initial"),
			})
			msgs.Append(MessageType_TABLE_EVENT, &TableEvent{
				Type:      TableEvent_CREATE,
				LTime:     uint64(ltime.Increment()),
				NodeName:  "node1",
				NetworkID: nid,
				TableName: tname,
				Key:       nid + "." + tname,
				Value:     []byte("a"),
			})
			msgs.Append(MessageType_TABLE_EVENT, &TableEvent{
				Type:      TableEvent_UPDATE,
				LTime:     uint64(ltime.Increment()),
				NodeName:  "node1",
				NetworkID: nid,
				TableName: tname,
				Key:       nid + "." + tname + ".update",
				Value:     []byte("updated"),
			})
		}
	}
	(&delegate{nDB}).NotifyMsg(msgs.Compound())

	watchAll, cancel := nDB.Watch("", "")
	defer cancel()
	watchNetwork1Tables, cancel := nDB.Watch("", "network1")
	defer cancel()
	watchTable1AllNetworks, cancel := nDB.Watch("table1", "")
	defer cancel()
	watchTable1Network1, cancel := nDB.Watch("table1", "network1")
	defer cancel()

	var gotAll, gotNetwork1Tables, gotTable1AllNetworks, gotTable1Network1 []events.Event
L:
	for {
		select {
		case ev := <-watchAll.C:
			gotAll = append(gotAll, ev)
		case ev := <-watchNetwork1Tables.C:
			gotNetwork1Tables = append(gotNetwork1Tables, ev)
		case ev := <-watchTable1AllNetworks.C:
			gotTable1AllNetworks = append(gotTable1AllNetworks, ev)
		case ev := <-watchTable1Network1.C:
			gotTable1Network1 = append(gotTable1Network1, ev)
		case <-time.After(time.Second):
			break L
		}
	}

	assert.Check(t, is.DeepEqual(gotAll, []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "network1.table1", Value: []byte("a")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "network1.table1.update", Value: []byte("updated")},
		WatchEvent{Table: "table2", NetworkID: "network1", Key: "network1.table2", Value: []byte("a")},
		WatchEvent{Table: "table2", NetworkID: "network1", Key: "network1.table2.update", Value: []byte("updated")},
		WatchEvent{Table: "table1", NetworkID: "network2", Key: "network2.table1", Value: []byte("a")},
		WatchEvent{Table: "table1", NetworkID: "network2", Key: "network2.table1.update", Value: []byte("updated")},
		WatchEvent{Table: "table2", NetworkID: "network2", Key: "network2.table2", Value: []byte("a")},
		WatchEvent{Table: "table2", NetworkID: "network2", Key: "network2.table2.update", Value: []byte("updated")},
	}))
	assert.Check(t, is.DeepEqual(gotNetwork1Tables, []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "network1.table1", Value: []byte("a")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "network1.table1.update", Value: []byte("updated")},
		WatchEvent{Table: "table2", NetworkID: "network1", Key: "network1.table2", Value: []byte("a")},
		WatchEvent{Table: "table2", NetworkID: "network1", Key: "network1.table2.update", Value: []byte("updated")},
	}))
	assert.Check(t, is.DeepEqual(gotTable1AllNetworks, []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "network1.table1", Value: []byte("a")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "network1.table1.update", Value: []byte("updated")},
		WatchEvent{Table: "table1", NetworkID: "network2", Key: "network2.table1", Value: []byte("a")},
		WatchEvent{Table: "table1", NetworkID: "network2", Key: "network2.table1.update", Value: []byte("updated")},
	}))
	assert.Check(t, is.DeepEqual(gotTable1Network1, []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "network1.table1", Value: []byte("a")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "network1.table1.update", Value: []byte("updated")},
	}))
}

func TestLeaveRejoinOutOfOrder(t *testing.T) {
	// Regression test for https://github.com/moby/moby/issues/47728

	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.Assert(t, nDB.JoinNetwork("network1"))

	(&eventDelegate{nDB}).NotifyJoin(&memberlist.Node{
		Name: "node1",
		Addr: net.IPv4(1, 2, 3, 4),
	})

	d := &delegate{nDB}

	msgs := messageBuffer{t: t}
	appendTableEvent := tableEventHelper(&msgs, "node1", "network1", "table1")

	msgs.Append(MessageType_NETWORK_EVENT, &NetworkEvent{
		Type:      NetworkEvent_JOIN,
		LTime:     1,
		NodeName:  "node1",
		NetworkID: "network1",
	})
	// Simulate node1 leaving, rejoining, and creating an entry,
	// but the table events are broadcast before the network events.
	appendTableEvent(1, TableEvent_CREATE, "key1", []byte("a"))
	msgs.Append(MessageType_NETWORK_EVENT, &NetworkEvent{
		Type:      NetworkEvent_LEAVE,
		LTime:     2,
		NodeName:  "node1",
		NetworkID: "network1",
	})
	msgs.Append(MessageType_NETWORK_EVENT, &NetworkEvent{
		Type:      NetworkEvent_JOIN,
		LTime:     3,
		NodeName:  "node1",
		NetworkID: "network1",
	})
	// Simulate a bulk sync or receiving a rebroadcasted copy of the table
	// event from another node.
	appendTableEvent(1, TableEvent_CREATE, "key1", []byte("a"))

	d.NotifyMsg(msgs.Compound())

	got := make(map[string]string)
	nDB.WalkTable("table1", func(nw, key string, value []byte, deleted bool) bool {
		got[nw+"/"+key] = fmt.Sprintf("%s (deleted=%t)", value, deleted)
		return false
	})
	want := map[string]string{
		"network1/key1": "a (deleted=false)",
	}
	assert.Check(t, is.DeepEqual(got, want))
}

func drainChannel(ch <-chan events.Event) []events.Event {
	var events []events.Event
	for {
		select {
		case ev := <-ch:
			events = append(events, ev)
		case <-time.After(time.Second):
			return events
		}
	}
}

type messageBuffer struct {
	t    *testing.T
	msgs [][]byte
}

func (mb *messageBuffer) Append(typ MessageType, msg vtMarshaler) {
	mb.t.Helper()
	buf, err := encodeMessage(typ, msg)
	if err != nil {
		mb.t.Fatalf("failed to encode message: %v", err)
	}
	mb.msgs = append(mb.msgs, buf)
}

func (mb *messageBuffer) Compound() []byte {
	return makeCompoundMessage(mb.msgs)
}

func (mb *messageBuffer) Reset() {
	mb.msgs = nil
}

func tableEventHelper(mb *messageBuffer, nodeName, networkID, tableName string) func(ltime lamport.Time, typ TableEvent_Type, key string, value []byte) {
	return func(ltime lamport.Time, typ TableEvent_Type, key string, value []byte) {
		mb.t.Helper()
		mb.Append(MessageType_TABLE_EVENT, &TableEvent{
			Type:      typ,
			LTime:     uint64(ltime),
			NodeName:  nodeName,
			NetworkID: networkID,
			TableName: tableName,
			Key:       key,
			Value:     value,
		})
	}
}
