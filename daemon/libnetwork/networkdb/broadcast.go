package networkdb

import (
	"errors"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

const broadcastTimeout = 5 * time.Second

type networkEventMessage struct {
	id    string
	node  string
	ltime serf.LamportTime
	msg   []byte
}

func (m *networkEventMessage) Invalidates(other memberlist.Broadcast) bool {
	otherm := other.(*networkEventMessage)
	if m.id != otherm.id || m.node != otherm.node {
		return false
	}
	// Supersede an event only if this one is no older. handleNetworkMessage
	// queues a relay after releasing the lock it applied the event under, and
	// the two paths that deliver events do not share a goroutine: gossip is
	// handled by memberlist's single packetHandler, bulk syncs arrive over
	// streams with one goroutine per connection. Two handlers can therefore
	// apply in Lamport order and reach the queue in the opposite order.
	// Matching on (network, node) alone would let the straggler evict the
	// fresher relay and put a stale attachment on the wire in its place.
	return m.ltime >= otherm.ltime
}

func (m *networkEventMessage) Message() []byte {
	return m.msg
}

func (m *networkEventMessage) Finished() {
}

func (nDB *NetworkDB) sendNetworkEvent(nid string, event NetworkEvent_Type, ltime serf.LamportTime) error {
	nEvent := NetworkEvent{
		Type:      event,
		LTime:     ltime,
		NodeName:  nDB.config.NodeID,
		NetworkID: nid,
	}

	raw, err := encodeMessage(MessageTypeNetworkEvent, &nEvent)
	if err != nil {
		return err
	}

	nDB.networkBroadcasts.QueueBroadcast(&networkEventMessage{
		msg:   raw,
		id:    nid,
		ltime: ltime,
		node:  nDB.config.NodeID,
	})
	return nil
}

// ownNodeEventMessage is this node announcing its own arrival or departure.
//
// It is a [memberlist.UniqueBroadcast] so that the queue will neither invalidate
// it nor let it invalidate anything. sendNodeEvent waits to be told the message
// went out and Finished is how it is told, but memberlist calls Finished on
// whatever it invalidates -- so were this message invalidatable, that wait would
// be released as though the event had been broadcast when it had in fact been
// thrown away. There are only ever a handful of these in flight anyway:
// sendNodeEvent is called on cluster join, rejoin and leave.
type ownNodeEventMessage struct {
	msg    []byte
	notify chan<- struct{}
}

// The queue must treat this as unique; see ownNodeEventMessage.
var _ memberlist.UniqueBroadcast = (*ownNodeEventMessage)(nil)

func (m *ownNodeEventMessage) Invalidates(memberlist.Broadcast) bool { return false }

// UniqueBroadcast marks this message as one the queue must not deduplicate.
func (m *ownNodeEventMessage) UniqueBroadcast() {}

func (m *ownNodeEventMessage) Message() []byte {
	return m.msg
}

func (m *ownNodeEventMessage) Finished() {
	if m.notify != nil {
		close(m.notify)
	}
}

// relayedNodeEventMessage is a peer's node event being passed on.
type relayedNodeEventMessage struct {
	node  string
	ltime serf.LamportTime
	msg   []byte
}

func (m *relayedNodeEventMessage) Invalidates(other memberlist.Broadcast) bool {
	// Two message types share nDB.nodeBroadcasts, so this assertion is checked.
	// The queue does not in fact offer a UniqueBroadcast up for invalidation,
	// but nothing here should depend on that.
	otherm, ok := other.(*relayedNodeEventMessage)
	if !ok || m.node != otherm.node {
		return false
	}
	// Collapse a peer's churn to the latest state known of it. A node which
	// flaps queues a join and a leave on every cycle and only the newest of them
	// tells a receiver anything, since handleNodeEvent discards any event no
	// fresher than what it already holds -- so without this, a flapping peer
	// fills every other node's queue with events that will be dropped on
	// arrival.
	//
	// Comparing Lamport times rather than replacing unconditionally is what
	// keeps that safe: handleNodeMessage queues its relay after releasing the
	// lock handleNodeEvent applied the event under, so two handlers can apply in
	// Lamport order and reach the queue in the opposite one. That is also why
	// this cannot be a [memberlist.NamedBroadcast], whose deduplication is
	// unconditionally last-one-wins and would drop the fresher event.
	return m.ltime >= otherm.ltime
}

func (m *relayedNodeEventMessage) Message() []byte {
	return m.msg
}

func (m *relayedNodeEventMessage) Finished() {}

func (nDB *NetworkDB) sendNodeEvent(event NodeEvent_Type) error {
	nEvent := NodeEvent{
		Type:     event,
		LTime:    nDB.networkClock.Increment(),
		NodeName: nDB.config.NodeID,
	}

	raw, err := encodeMessage(MessageTypeNodeEvent, &nEvent)
	if err != nil {
		return err
	}

	notifyCh := make(chan struct{})
	nDB.nodeBroadcasts.QueueBroadcast(&ownNodeEventMessage{
		msg:    raw,
		notify: notifyCh,
	})

	nDB.RLock()
	noPeers := len(nDB.nodes) <= 1
	nDB.RUnlock()

	// Message enqueued, do not wait for a send if no peer is present
	if noPeers {
		return nil
	}

	// Wait for the broadcast
	select {
	case <-notifyCh:
	case <-time.After(broadcastTimeout):
		return errors.New("timed out broadcasting node event")
	}

	return nil
}

type tableEventMessage struct {
	id    string
	tname string
	key   string
	ltime serf.LamportTime
	msg   []byte
}

func (m *tableEventMessage) Invalidates(other memberlist.Broadcast) bool {
	otherm := other.(*tableEventMessage)
	if m.tname != otherm.tname || m.id != otherm.id || m.key != otherm.key {
		return false
	}
	// Supersede an event for this key only if this one is no older. Both paths
	// which queue a table event do so after releasing the lock they mutated the
	// entry under -- CreateEntry, UpdateEntry and DeleteEntry all Unlock before
	// calling sendTableEvent, and handleTableMessage queues its relay after
	// handleTableEvent returns -- so two writers to one key can take the lock in
	// Lamport order and reach the queue in the opposite order. Matching on the
	// key alone would then let the straggler evict the fresher event and gossip
	// a superseded value in its place, until anti-entropy corrected it.
	return m.ltime >= otherm.ltime
}

func (m *tableEventMessage) Message() []byte {
	return m.msg
}

func (m *tableEventMessage) Finished() {
}

func (nDB *NetworkDB) sendTableEvent(event TableEvent_Type, nid string, tname string, key string, entry *entry) error {
	tEvent := TableEvent{
		Type:      event,
		LTime:     entry.ltime,
		NodeName:  nDB.config.NodeID,
		NetworkID: nid,
		TableName: tname,
		Key:       key,
		Value:     entry.value,
		// The duration in second is a float that below would be truncated
		ResidualReapTime: int32(entry.reapTime.Seconds()),
	}

	raw, err := encodeMessage(MessageTypeTableEvent, &tEvent)
	if err != nil {
		return err
	}

	nDB.RLock()
	n, ok := nDB.thisNodeNetworks[nid]
	nDB.RUnlock()

	// The network may have been removed
	if !ok {
		return nil
	}

	n.tableBroadcasts.QueueBroadcast(&tableEventMessage{
		msg:   raw,
		id:    nid,
		tname: tname,
		key:   key,
		ltime: entry.ltime,
	})
	return nil
}

func getBroadcasts(overhead, limit int, queues ...*memberlist.TransmitLimitedQueue) [][]byte {
	var msgs [][]byte
	for _, q := range queues {
		b := q.GetBroadcasts(overhead, limit)
		for _, m := range b {
			limit -= overhead + len(m)
		}
		msgs = append(msgs, b...)
		if limit <= 0 {
			break
		}
	}
	return msgs
}
