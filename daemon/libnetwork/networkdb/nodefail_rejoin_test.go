package networkdb

import (
	"maps"
	"net"
	"slices"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/go-events"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// newTestNetworkDBWithPeer returns a NetworkDB which has joined network1, and
// a peer node1 attached to it, along with the delegates needed to drive them.
func newTestNetworkDBWithPeer(t *testing.T) (*NetworkDB, *eventDelegate, *memberlist.Node, func(ltime serf.LamportTime, typ TableEvent_Type, key string, value []byte)) {
	t.Helper()
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.NilError(t, nDB.JoinNetwork("network1"))

	ed := &eventDelegate{nDB}
	mn := &memberlist.Node{Name: "node1", Addr: net.IPv4(1, 2, 3, 4)}
	ed.NotifyJoin(mn)
	nDB.handleNetworkEvent(&NetworkEvent{
		Type:      NetworkEventTypeJoin,
		LTime:     1,
		NodeName:  "node1",
		NetworkID: "network1",
	})

	d := &delegate{nDB}
	msgs := &messageBuffer{t: t}
	appendTableEvent := tableEventHelper(msgs, "node1", "network1", "table1")
	deliver := func(ltime serf.LamportTime, typ TableEvent_Type, key string, value []byte) {
		t.Helper()
		msgs.Reset()
		appendTableEvent(ltime, typ, key, value)
		d.NotifyMsg(msgs.Compound())
	}
	return nDB, ed, mn, deliver
}

// TestFailedNodeEntriesHiddenAndRestored checks that the state owned by a
// failed node is hidden rather than dropped: the node stops being a peer of
// the networks it was attached to and its entries stop being visible, but
// both are remembered and put back as soon as it comes back, without anything
// being re-sent.
func TestFailedNodeEntriesHiddenAndRestored(t *testing.T) {
	nDB, ed, mn, deliver := newTestNetworkDBWithPeer(t)

	watch, cancel := nDB.Watch("table1", "network1")
	defer cancel()

	deliver(2, TableEventTypeCreate, "key1", []byte("v1"))
	ed.NotifyLeave(mn)

	// The entry must stop being visible, and the watchers must be told that
	// it is gone: what it describes cannot be reached either.
	_, err := nDB.GetEntry("table1", "network1", "key1")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound), "entry owned by a failed node should not be visible")
	assert.Check(t, is.Len(nDB.GetTableByNetwork("table1", "network1"), 0))
	assert.Check(t, is.DeepEqual(drainChannel(watch.C), []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Value: []byte("v1")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Prev: []byte("v1")},
	}))

	// The entry is still in the store, aging out.
	nDB.RLock()
	e, rawErr := nDB.getEntry("table1", "network1", "key1")
	nDB.RUnlock()
	assert.NilError(t, rawErr)
	assert.Check(t, is.Equal(nDB.config.reapEntryInterval, e.reapTime), "hidden entry should age out")

	// The node must no longer be a peer, but the attachment must be
	// remembered.
	nDB.RLock()
	nn := slices.Clone(nDB.networkNodes["network1"])
	attachments := maps.Clone(nDB.networks["node1"])
	nDB.RUnlock()
	assert.Check(t, !slices.Contains(nn, "node1"), "node1 should not be a peer of network1 while failed")
	assert.Check(t, is.Contains(attachments, "network1"), "node1's attachment to network1 should be remembered")

	// Coming back must put it in the peer list again and publish the entry
	// again, with nothing having to be re-sent.
	ed.NotifyJoin(mn)

	nDB.RLock()
	nn = slices.Clone(nDB.networkNodes["network1"])
	e, rawErr = nDB.getEntry("table1", "network1", "key1")
	nDB.RUnlock()
	assert.Check(t, is.Contains(nn, "node1"), "node1 should be a peer of network1 again")
	assert.NilError(t, rawErr)
	assert.Check(t, is.Equal(time.Duration(0), e.reapTime), "restored entry should no longer age")

	v, err := nDB.GetEntry("table1", "network1", "key1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal("v1", string(v)))
	assert.Check(t, is.DeepEqual(drainChannel(watch.C), []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Value: []byte("v1")},
	}))
}

// TestLeftNodeClearsNetworkMembership checks that a node which gracefully
// leaves the cluster is removed from the peer lists for good: nothing is
// remembered for it, unlike for a failed node.
func TestLeftNodeClearsNetworkMembership(t *testing.T) {
	nDB, _, _, deliver := newTestNetworkDBWithPeer(t)
	deliver(2, TableEventTypeCreate, "key1", []byte("v1"))

	watch, cancel := nDB.Watch("table1", "network1")
	defer cancel()

	nDB.Lock()
	_, err := nDB.changeNodeState("node1", nodeLeftState)
	nDB.Unlock()
	assert.NilError(t, err)

	nDB.RLock()
	present := slices.Contains(nDB.networkNodes["network1"], "node1")
	_, hasNetworks := nDB.networks["node1"]
	_, rawErr := nDB.getEntry("table1", "network1", "key1")
	nDB.RUnlock()
	assert.Check(t, !present, "node1 should be removed from networkNodes after leave")
	assert.Check(t, !hasNetworks, "node1's attachments should be forgotten after leave")
	assert.Check(t, is.ErrorType(rawErr, cerrdefs.IsNotFound), "node1's entries should be deleted after leave")

	// Watchers see the snapshot of the entry and then its deletion.
	assert.Check(t, is.DeepEqual(drainChannel(watch.C), []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Value: []byte("v1")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Prev: []byte("v1")},
	}))
}

// TestFailedNodeRelayedEventsRemembered checks that an update relayed by
// another node on behalf of a failed owner is remembered while the owner is
// down — hidden and with bounded retention — and is what gets published when
// the owner is back.
func TestFailedNodeRelayedEventsRemembered(t *testing.T) {
	nDB, ed, mn, deliver := newTestNetworkDBWithPeer(t)

	watch, cancel := nDB.Watch("table1", "network1")
	defer cancel()

	ed.NotifyLeave(mn)

	// An update for the failed owner, relayed by a node which can still
	// reach it.
	deliver(2, TableEventTypeCreate, "key1", []byte("relayed"))

	_, err := nDB.GetEntry("table1", "network1", "key1")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound), "relayed update should not be visible while the owner is down")

	// It is remembered, with bounded retention.
	nDB.RLock()
	e, rawErr := nDB.getEntry("table1", "network1", "key1")
	nDB.RUnlock()
	assert.NilError(t, rawErr)
	assert.Check(t, is.Equal("relayed", string(e.value)))
	assert.Check(t, is.Equal(nDB.config.reapEntryInterval, e.reapTime), "remembered entry should age out")

	// A fresher relayed update replaces what is remembered.
	deliver(3, TableEventTypeUpdate, "key1", []byte("fresher"))

	// Nothing was published while the owner was down, and the freshest state
	// remembered is what gets published when it returns.
	ed.NotifyJoin(mn)
	v, err := nDB.GetEntry("table1", "network1", "key1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal("fresher", string(v)), "the freshest state known should be restored")
	assert.Check(t, is.DeepEqual(drainChannel(watch.C), []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Value: []byte("fresher")},
	}))
}

// TestFailedNodeRelayedDeleteKept checks that an entry deleted while its owner
// was down does not come back when the owner does.
func TestFailedNodeRelayedDeleteKept(t *testing.T) {
	nDB, ed, mn, deliver := newTestNetworkDBWithPeer(t)
	deliver(2, TableEventTypeCreate, "key1", []byte("v1"))

	watch, cancel := nDB.Watch("table1", "network1")
	defer cancel()

	ed.NotifyLeave(mn)
	deliver(3, TableEventTypeDelete, "key1", []byte("v1"))

	ed.NotifyJoin(mn)
	_, err := nDB.GetEntry("table1", "network1", "key1")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound), "an entry deleted while its owner was down should stay deleted")
	assert.Check(t, is.DeepEqual(drainChannel(watch.C), []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Value: []byte("v1")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Prev: []byte("v1")},
	}), "watchers should see the delete once, and the entry should not come back")
}

// TestFailedThenLeftDropsRememberedState checks that a graceful leave of a
// failed node drops what was remembered for it without notifying watchers a
// second time.
func TestFailedThenLeftDropsRememberedState(t *testing.T) {
	nDB, ed, mn, deliver := newTestNetworkDBWithPeer(t)
	deliver(2, TableEventTypeCreate, "key1", []byte("v1"))

	watch, cancel := nDB.Watch("table1", "network1")
	defer cancel()

	ed.NotifyLeave(mn)

	nDB.Lock()
	_, err := nDB.changeNodeState("node1", nodeLeftState)
	nDB.Unlock()
	assert.NilError(t, err)

	nDB.RLock()
	_, rawErr := nDB.getEntry("table1", "network1", "key1")
	_, hasNetworks := nDB.networks["node1"]
	nDB.RUnlock()
	assert.Check(t, is.ErrorType(rawErr, cerrdefs.IsNotFound), "remembered entry should be dropped on leave")
	assert.Check(t, !hasNetworks, "attachments should be dropped on leave")

	// Watchers were told once, when the node failed.
	assert.Check(t, is.DeepEqual(drainChannel(watch.C), []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Value: []byte("v1")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Prev: []byte("v1")},
	}))
}

// TestFailedNodeEntriesExpire checks that entries remembered for a failed node
// age out, so that they cannot be restored after whatever would delete them
// expired.
func TestFailedNodeEntriesExpire(t *testing.T) {
	nDB, ed, mn, deliver := newTestNetworkDBWithPeer(t)
	deliver(2, TableEventTypeCreate, "key1", []byte("v1"))

	watch, cancel := nDB.Watch("table1", "network1")
	defer cancel()

	ed.NotifyLeave(mn)

	// Age the remembered entry past its retention.
	nDB.Lock()
	e, rawErr := nDB.getEntry("table1", "network1", "key1")
	if rawErr == nil {
		e.reapTime = time.Second
	}
	nDB.Unlock()
	assert.NilError(t, rawErr)

	nDB.reapTableEntries()

	nDB.RLock()
	_, rawErr = nDB.getEntry("table1", "network1", "key1")
	nDB.RUnlock()
	assert.Check(t, is.ErrorType(rawErr, cerrdefs.IsNotFound), "remembered entry should expire")

	// Nothing is left to restore when the node returns.
	ed.NotifyJoin(mn)
	assert.Check(t, is.DeepEqual(drainChannel(watch.C), []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Value: []byte("v1")},
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key1", Prev: []byte("v1")},
	}), "nothing should be restored after the remembered entry expired")
}

// TestWatchSnapshotSkipsHiddenEntries checks that a watch created while a node
// is failed does not receive synthesized create events for hidden entries.
func TestWatchSnapshotSkipsHiddenEntries(t *testing.T) {
	nDB, ed, mn, deliver := newTestNetworkDBWithPeer(t)

	// A second, healthy remote node with an entry of its own.
	ed.NotifyJoin(&memberlist.Node{Name: "node2", Addr: net.IPv4(1, 2, 3, 5)})
	nDB.handleNetworkEvent(&NetworkEvent{
		Type:      NetworkEventTypeJoin,
		LTime:     2,
		NodeName:  "node2",
		NetworkID: "network1",
	})

	deliver(2, TableEventTypeCreate, "key1", []byte("hidden"))

	d := &delegate{nDB}
	msgs := messageBuffer{t: t}
	appendNode2TableEvent := tableEventHelper(&msgs, "node2", "network1", "table1")
	appendNode2TableEvent(3, TableEventTypeCreate, "key2", []byte("visible"))
	d.NotifyMsg(msgs.Compound())

	ed.NotifyLeave(mn)

	watch, cancel := nDB.Watch("table1", "network1")
	defer cancel()
	assert.Check(t, is.DeepEqual(drainChannel(watch.C), []events.Event{
		WatchEvent{Table: "table1", NetworkID: "network1", Key: "key2", Value: []byte("visible")},
	}), "only the entry of the healthy node should be synthesized")
}

// TestBulkSyncFromInactiveNodeRemembered checks that a bulk sync from a node we
// consider failed (as happens across an asymmetric partition) is merged and
// remembered, hidden until the node returns.
func TestBulkSyncFromInactiveNodeRemembered(t *testing.T) {
	nDB, ed, mn, _ := newTestNetworkDBWithPeer(t)
	ed.NotifyLeave(mn)

	payload := &messageBuffer{t: t}
	tableEventHelper(payload, "node1", "network1", "table1")(2, TableEventTypeCreate, "key1", []byte("synced"))

	msgs := &messageBuffer{t: t}
	msgs.Append(MessageTypeBulkSync, &BulkSyncMessage{
		LTime:    2,
		NodeName: "node1",
		Networks: []string{"network1"},
		Payload:  payload.Compound(),
	})
	(&delegate{nDB}).NotifyMsg(msgs.Compound())

	nDB.RLock()
	e, rawErr := nDB.getEntry("table1", "network1", "key1")
	nDB.RUnlock()
	assert.NilError(t, rawErr)
	assert.Check(t, is.Equal("synced", string(e.value)))
	_, err := nDB.GetEntry("table1", "network1", "key1")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound), "the synced entry should stay hidden while its owner is inactive")
}

// TestHiddenEntriesDoNotExistForPublicAPI checks that the public write API
// treats entries hidden for a failed owner the same way it treated the
// deleted state before: a create takes the key over, and updates or deletes
// of a hidden entry fail as if it did not exist.
func TestHiddenEntriesDoNotExistForPublicAPI(t *testing.T) {
	nDB, ed, mn, deliver := newTestNetworkDBWithPeer(t)
	deliver(2, TableEventTypeCreate, "key1", []byte("remote"))
	deliver(2, TableEventTypeCreate, "key2", []byte("remote"))
	ed.NotifyLeave(mn)

	// A hidden entry does not hold its key against the local node.
	assert.NilError(t, nDB.CreateEntry("table1", "network1", "key1", []byte("local")))
	v, err := nDB.GetEntry("table1", "network1", "key1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal("local", string(v)))

	// Updating or deleting a hidden entry fails as if it did not exist.
	assert.Check(t, is.ErrorContains(nDB.UpdateEntry("table1", "network1", "key2", []byte("nope")), "does not exist"))
	assert.Check(t, is.ErrorContains(nDB.DeleteEntry("table1", "network1", "key2"), "does not exist"))

	// When the owner returns, only the entry it still owns is restored, and
	// its stale re-send of the taken-over key is rejected.
	ed.NotifyJoin(mn)
	deliver(2, TableEventTypeCreate, "key1", []byte("remote"))
	v, err = nDB.GetEntry("table1", "network1", "key1")
	assert.NilError(t, err)
	assert.Check(t, is.Equal("local", string(v)), "the taken-over key should not be reclaimed by a stale re-send")
	v, err = nDB.GetEntry("table1", "network1", "key2")
	assert.NilError(t, err)
	assert.Check(t, is.Equal("remote", string(v)))
}

// TestReapFailedNodeClearsNetworkState checks that garbage collecting a node
// which never came back drops the attachments and any entries still
// remembered for it.
func TestReapFailedNodeClearsNetworkState(t *testing.T) {
	nDB, ed, mn, deliver := newTestNetworkDBWithPeer(t)
	deliver(2, TableEventTypeCreate, "key1", []byte("v1"))

	ed.NotifyLeave(mn)

	// Make the node eligible for collection.
	nDB.Lock()
	nDB.failedNodes["node1"].reapTime = nodeReapPeriod
	nDB.Unlock()

	// The failure took node1 out of the peer list, so what is left for the
	// reap to drop is the attachment and the entry remembered for it.
	nDB.RLock()
	inNetwork := slices.Contains(nDB.networkNodes["network1"], "node1")
	attachments := maps.Clone(nDB.networks["node1"])
	nDB.RUnlock()
	assert.Check(t, !inNetwork, "node1 should already be out of the peer list before the reap")
	assert.Assert(t, is.Contains(attachments, "network1"), "node1's attachment should still be remembered before the reap")

	nDB.reapDeadNode()

	nDB.RLock()
	_, stillFailed := nDB.failedNodes["node1"]
	_, hasNetworks := nDB.networks["node1"]
	nDB.RUnlock()
	assert.Check(t, !stillFailed, "node1 should be garbage collected from failedNodes")
	assert.Check(t, !hasNetworks, "node1's network attachments should be removed when reaped")

	err := nDB.WalkTable("table1", func(nw, key string, value []byte, deleted bool) bool {
		t.Fatalf("expected no entries owned by the reaped node, found %s/%s", nw, key)
		return false
	})
	assert.NilError(t, err)
}
