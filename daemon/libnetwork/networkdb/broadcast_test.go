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
