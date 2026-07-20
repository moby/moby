package networkdb

import (
	"net"
	"slices"
	"testing"

	"github.com/hashicorp/memberlist"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestFailedNodeKeepsNetworkMembership checks that a failed node's entries are
// deleted but the networks it was attached to are remembered, so that the
// entries it re-sends when it comes back are accepted.
func TestFailedNodeKeepsNetworkMembership(t *testing.T) {
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.Assert(t, nDB.JoinNetwork("network1"))

	(&eventDelegate{nDB}).NotifyJoin(&memberlist.Node{
		Name: "node1",
		Addr: net.IPv4(1, 2, 3, 4),
	})
	nDB.Lock()
	nDB.addNetworkNode("network1", "node1")
	nDB.Unlock()

	assert.Assert(t, nDB.CreateEntry("table1", "network1", "key1", []byte("v1")) == nil)

	// Owned by remote node1 — synthesize a remote entry.
	nDB.Lock()
	entry, err := nDB.getEntry("table1", "network1", "key1")
	assert.Assert(t, err == nil)
	entry.node = "node1"
	nDB.createOrUpdateEntry("network1", "table1", "key1", entry)
	nDB.Unlock()

	nDB.Lock()
	_, err = nDB.changeNodeState("node1", nodeFailedState)
	nDB.Unlock()
	assert.NilError(t, err)

	// Entries owned by the failed node must be gone.
	err = nDB.WalkTable("table1", func(nw, key string, value []byte, deleted bool) bool {
		t.Fatalf("expected no table entries after NodeFailed, found %s/%s", nw, key)
		return false
	})
	assert.NilError(t, err)

	// The networks it was attached to must be remembered.
	nDB.RLock()
	present := slices.Contains(nDB.networkNodes["network1"], "node1")
	nDB.RUnlock()
	assert.Check(t, present, "node1 should remain in networkNodes after NodeFailed")

	// The entry it re-sends on rejoin must be accepted.
	nDB.Lock()
	_, err = nDB.changeNodeState("node1", nodeActiveState)
	nDB.Unlock()
	assert.NilError(t, err)

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
	assert.Assert(t, nDB.JoinNetwork("network1"))

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
	assert.Check(t, !present, "node1 should be removed from networkNodes after leave")
}

// TestFailedNodeRelayedEventsDropped checks that entries owned by a failed node
// are not revived by another node relaying them, and are accepted again once
// the owner is back.
func TestFailedNodeRelayedEventsDropped(t *testing.T) {
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.Assert(t, nDB.JoinNetwork("network1"))

	(&eventDelegate{nDB}).NotifyJoin(&memberlist.Node{
		Name: "node1",
		Addr: net.IPv4(1, 2, 3, 4),
	})
	nDB.Lock()
	nDB.addNetworkNode("network1", "node1")
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
	assert.Check(t, is.Equal("fresh", got))
}

// TestReapFailedNodeClearsNetworkState checks that garbage collecting a node
// which never came back drops the network membership kept for it.
func TestReapFailedNodeClearsNetworkState(t *testing.T) {
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.Assert(t, nDB.JoinNetwork("network1"))

	(&eventDelegate{nDB}).NotifyJoin(&memberlist.Node{
		Name: "node1",
		Addr: net.IPv4(1, 2, 3, 4),
	})
	nDB.Lock()
	nDB.addNetworkNode("network1", "node1")
	nDB.networks["node1"] = map[string]*network{"network1": {ltime: 1}}
	_, err := nDB.changeNodeState("node1", nodeFailedState)
	nDB.Unlock()
	assert.NilError(t, err)

	// Leave a stray entry behind and make the node eligible for collection.
	nDB.Lock()
	nDB.createOrUpdateEntry("network1", "table1", "key1", &entry{ltime: 1, node: "node1", value: []byte("stray")})
	nDB.failedNodes["node1"].reapTime = nodeReapPeriod
	nDB.Unlock()

	nDB.reapDeadNode()

	nDB.RLock()
	_, stillFailed := nDB.failedNodes["node1"]
	inNetwork := slices.Contains(nDB.networkNodes["network1"], "node1")
	_, hasNetworks := nDB.networks["node1"]
	nDB.RUnlock()
	assert.Check(t, !stillFailed, "node1 should be garbage collected from failedNodes")
	assert.Check(t, !inNetwork, "node1 should be removed from networkNodes when reaped")
	assert.Check(t, !hasNetworks, "node1's network attachments should be removed when reaped")

	err = nDB.WalkTable("table1", func(nw, key string, value []byte, deleted bool) bool {
		t.Fatalf("expected no entries owned by the reaped node, found %s/%s", nw, key)
		return false
	})
	assert.NilError(t, err)
}
