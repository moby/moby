package networkdb

import (
	"maps"
	"net"
	"slices"
	"testing"

	"github.com/hashicorp/memberlist"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestFailedNodeKeepsNetworkMembership checks that a failed node's entries are
// deleted and that it stops being a peer of the networks it was attached to,
// but that the attachment is remembered, so that it is put back in the peer
// list and the entries it re-sends are accepted when it comes back.
func TestFailedNodeKeepsNetworkMembership(t *testing.T) {
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

	assert.NilError(t, nDB.CreateEntry("table1", "network1", "key1", []byte("v1")))

	// Owned by remote node1 — synthesize a remote entry.
	nDB.Lock()
	entry, err := nDB.getEntry("table1", "network1", "key1")
	assert.NilError(t, err)
	entry.node = "node1"
	nDB.createOrUpdateEntry("network1", "table1", "key1", entry)
	nDB.Unlock()

	ed.NotifyLeave(mn)

	// Entries owned by the failed node must be gone.
	err = nDB.WalkTable("table1", func(nw, key string, value []byte, deleted bool) bool {
		t.Fatalf("expected no table entries after NodeFailed, found %s/%s", nw, key)
		return false
	})
	assert.NilError(t, err)

	// It must no longer be a peer, but the attachment must be remembered.
	nDB.RLock()
	nn := slices.Clone(nDB.networkNodes["network1"])
	attachments := maps.Clone(nDB.networks["node1"])
	nDB.RUnlock()
	assert.Check(t, !slices.Contains(nn, "node1"), "node1 should not be a peer of network1 while failed")
	assert.Check(t, is.Contains(attachments, "network1"), "node1's attachment to network1 should be remembered")

	// Coming back must put it in the peer list again, and the entry it
	// re-sends must be accepted.
	ed.NotifyJoin(mn)

	nDB.RLock()
	nn = slices.Clone(nDB.networkNodes["network1"])
	nDB.RUnlock()
	assert.Check(t, is.Contains(nn, "node1"), "node1 should be a peer of network1 again")

	d := &delegate{nDB}
	msgs := messageBuffer{t: t}
	appendTableEvent := tableEventHelper(&msgs, "node1", "network1", "table1")
	appendTableEvent(2, TableEventTypeCreate, "key1", []byte("v1-rejoined"))
	d.NotifyMsg(msgs.Compound())

	var got string
	assert.NilError(t, nDB.WalkTable("table1", func(nw, key string, value []byte, deleted bool) bool {
		if nw == "network1" && key == "key1" && !deleted {
			got = string(value)
		}
		return false
	}))
	assert.Check(t, is.Equal("v1-rejoined", got))
}

// TestLeftNodeClearsNetworkMembership keeps graceful-leave behavior unchanged.
func TestLeftNodeClearsNetworkMembership(t *testing.T) {
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.NilError(t, nDB.JoinNetwork("network1"))

	(&eventDelegate{nDB}).NotifyJoin(&memberlist.Node{
		Name: "node1",
		Addr: net.IPv4(1, 2, 3, 4),
	})
	nDB.Lock()
	nDB.addNetworkNode("network1", "node1")
	_, err := nDB.changeNodeState("node1", nodeLeftState)
	nDB.Unlock()
	assert.NilError(t, err)

	nDB.RLock()
	present := slices.Contains(nDB.networkNodes["network1"], "node1")
	nDB.RUnlock()
	assert.Assert(t, !present, "node1 should be removed from networkNodes after leave")
}

// TestFailedNodeRelayedEventsDropped checks that entries owned by a failed node
// are not revived by another node relaying them, and are accepted again once
// the owner is back.
func TestFailedNodeRelayedEventsDropped(t *testing.T) {
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.NilError(t, nDB.JoinNetwork("network1"))

	(&eventDelegate{nDB}).NotifyJoin(&memberlist.Node{
		Name: "node1",
		Addr: net.IPv4(1, 2, 3, 4),
	})
	nDB.handleNetworkEvent(&NetworkEvent{
		Type:      NetworkEventTypeJoin,
		LTime:     1,
		NodeName:  "node1",
		NetworkID: "network1",
	})
	nDB.Lock()
	_, err := nDB.changeNodeState("node1", nodeFailedState)
	nDB.Unlock()
	assert.NilError(t, err)

	d := &delegate{nDB}
	msgs := messageBuffer{t: t}
	appendTableEvent := tableEventHelper(&msgs, "node1", "network1", "table1")
	appendTableEvent(2, TableEventTypeCreate, "key1", []byte("stale"))
	d.NotifyMsg(msgs.Compound())

	err = nDB.WalkTable("table1", func(nw, key string, value []byte, deleted bool) bool {
		t.Fatalf("expected relayed event from failed node to be dropped, found %s/%s", nw, key)
		return false
	})
	assert.NilError(t, err)

	nDB.Lock()
	_, err = nDB.changeNodeState("node1", nodeActiveState)
	nDB.Unlock()
	assert.NilError(t, err)

	msgs.Reset()
	appendTableEvent(3, TableEventTypeCreate, "key1", []byte("fresh"))
	d.NotifyMsg(msgs.Compound())

	var got string
	assert.NilError(t, nDB.WalkTable("table1", func(nw, key string, value []byte, deleted bool) bool {
		if nw == "network1" && key == "key1" && !deleted {
			got = string(value)
		}
		return false
	}))
	assert.Check(t, is.Equal("fresh", got), "entry from recovered owner should be accepted")
}

// TestReapFailedNodeClearsNetworkState checks that garbage collecting a node
// which never came back drops the attachments remembered for it.
func TestReapFailedNodeClearsNetworkState(t *testing.T) {
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
	ed.NotifyLeave(mn)

	// Leave a stray entry behind and make the node eligible for collection.
	nDB.Lock()
	nDB.createOrUpdateEntry("network1", "table1", "key1", &entry{ltime: 1, node: "node1", value: []byte("stray")})
	nDB.failedNodes["node1"].reapTime = nodeReapPeriod
	nDB.Unlock()

	// The failure took node1 out of the peer list, so what is left for the
	// reap to drop is the attachment it remembered.
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
