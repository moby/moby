package networkdb

import (
	"slices"
	"testing"

	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
	"gotest.tools/v3/assert"
)

// queuedMessages returns what the queue would hand out after the given
// broadcasts have been enqueued in order, sorted for comparison.
func queuedMessages(t *testing.T, order ...memberlist.Broadcast) []string {
	t.Helper()
	q := &memberlist.TransmitLimitedQueue{
		NumNodes:       func() int { return 3 },
		RetransmitMult: 4,
	}
	for _, m := range order {
		q.QueueBroadcast(m)
	}
	var got []string
	for _, raw := range q.GetBroadcasts(0, 1<<20) {
		got = append(got, string(raw))
	}
	slices.Sort(got)
	return got
}

func TestTableEventMessageInvalidates(t *testing.T) {
	msg := func(nid, tname, key string, ltime serf.LamportTime) *tableEventMessage {
		return &tableEventMessage{id: nid, tname: tname, key: key, ltime: ltime}
	}

	for _, tc := range []struct {
		name       string
		m, other   *tableEventMessage
		invalidate bool
	}{
		{"fresher supersedes staler", msg("nw", "t", "k", 7), msg("nw", "t", "k", 5), true},
		{"same ltime is a duplicate", msg("nw", "t", "k", 5), msg("nw", "t", "k", 5), true},
		{"staler must not evict fresher", msg("nw", "t", "k", 5), msg("nw", "t", "k", 7), false},
		{"another key", msg("nw", "t", "k1", 9), msg("nw", "t", "k2", 1), false},
		{"another table", msg("nw", "t1", "k", 9), msg("nw", "t2", "k", 1), false},
		{"another network", msg("nw1", "t", "k", 9), msg("nw2", "t", "k", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.m.Invalidates(tc.other), tc.invalidate)
		})
	}
}

// The same property through the real queue, which is what actually decides
// which message survives. Queueing in the reverse of Lamport order is the
// interleaving both write paths allow: they release the lock they mutated the
// entry under before queueing, so a writer can win the lock and lose the race
// to the queue.
func TestTableEventQueueKeepsFreshest(t *testing.T) {
	fresh := &tableEventMessage{id: "nw", tname: "t", key: "k", ltime: 7, msg: []byte("fresh")}
	stale := &tableEventMessage{id: "nw", tname: "t", key: "k", ltime: 5, msg: []byte("stale")}

	// Straggler arrives last and must not displace what beat it there.
	got := queuedMessages(t, fresh, stale)
	assert.Check(t, slices.Contains(got, "fresh"), "queue dropped the fresher event: %v", got)

	// The ordinary case still collapses to one message.
	assert.DeepEqual(t, queuedMessages(t, stale, fresh), []string{"fresh"})
}

func TestRelayedNodeEventMessageInvalidates(t *testing.T) {
	msg := func(node string, ltime serf.LamportTime) *relayedNodeEventMessage {
		return &relayedNodeEventMessage{node: node, ltime: ltime}
	}

	for _, tc := range []struct {
		name       string
		m, other   *relayedNodeEventMessage
		invalidate bool
	}{
		{"fresher supersedes staler", msg("n1", 7), msg("n1", 5), true},
		{"same ltime is a duplicate", msg("n1", 5), msg("n1", 5), true},
		{"staler must not evict fresher", msg("n1", 5), msg("n1", 7), false},
		{"another node", msg("n1", 9), msg("n2", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.m.Invalidates(tc.other), tc.invalidate)
		})
	}

	// It shares a queue with ownNodeEventMessage, so it must tolerate one.
	assert.Check(t, !msg("n1", 9).Invalidates(&ownNodeEventMessage{}))
}

// A flapping peer's churn collapses to the newest event known of it, since
// handleNodeEvent would discard the rest on arrival anyway.
func TestRelayedNodeEventQueueCollapsesChurn(t *testing.T) {
	relay := func(node string, ltime serf.LamportTime, body string) *relayedNodeEventMessage {
		return &relayedNodeEventMessage{node: node, ltime: ltime, msg: []byte(body)}
	}

	assert.DeepEqual(t, queuedMessages(t,
		relay("n1", 5, "join5"),
		relay("n1", 6, "leave6"),
		relay("n1", 7, "join7"),
	), []string{"join7"})

	// A straggler must not displace the event it lost the race to.
	got := queuedMessages(t, relay("n1", 7, "join7"), relay("n1", 5, "join5"))
	assert.Check(t, slices.Contains(got, "join7"), "queue dropped the fresher event: %v", got)

	// Different peers do not collapse into each other.
	assert.DeepEqual(t, queuedMessages(t,
		relay("n1", 9, "n1-event"),
		relay("n2", 1, "n2-event"),
	), []string{"n1-event", "n2-event"})
}

// Queueing a relayed event must not release the wait in sendNodeEvent: the
// queue calls Finished on whatever it invalidates, and this node's own message
// signals that channel from Finished.
func TestOwnNodeEventSurvivesRelayedEvents(t *testing.T) {
	q := &memberlist.TransmitLimitedQueue{
		NumNodes:       func() int { return 3 },
		RetransmitMult: 4,
	}
	notify := make(chan struct{})
	q.QueueBroadcast(&ownNodeEventMessage{msg: []byte("own"), notify: notify})
	q.QueueBroadcast(&relayedNodeEventMessage{node: "n1", ltime: 9, msg: []byte("relay")})

	select {
	case <-notify:
		t.Fatal("a relayed node event invalidated this node's own event, releasing sendNodeEvent as though it had been broadcast")
	default:
	}

	var got []string
	for _, raw := range q.GetBroadcasts(0, 1<<20) {
		got = append(got, string(raw))
	}
	slices.Sort(got)
	assert.DeepEqual(t, got, []string{"own", "relay"})
}

// A queued network event may only be superseded by one which is at least as
// fresh. handleNetworkMessage queues a relay after releasing the lock it
// applied the event under, and gossip and bulk sync do not share a goroutine,
// so a stale relay can reach the queue behind a fresher one.
func TestNetworkEventMessageInvalidates(t *testing.T) {
	msg := func(nid, node string, ltime serf.LamportTime) *networkEventMessage {
		return &networkEventMessage{id: nid, node: node, ltime: ltime}
	}

	for _, tc := range []struct {
		name       string
		m, other   *networkEventMessage
		invalidate bool
	}{
		{"fresher supersedes staler", msg("nw", "n1", 7), msg("nw", "n1", 5), true},
		{"same ltime is a duplicate", msg("nw", "n1", 5), msg("nw", "n1", 5), true},
		{"staler must not evict fresher", msg("nw", "n1", 5), msg("nw", "n1", 7), false},
		{"another node's attachment", msg("nw", "n1", 9), msg("nw", "n2", 1), false},
		{"another network", msg("nw2", "n1", 9), msg("nw", "n1", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.m.Invalidates(tc.other), tc.invalidate)
		})
	}
}
