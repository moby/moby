package networkdb

import (
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"pgregory.net/rapid"
)

// Virtual-time budget for the convergence check. Once the state machine has
// stopped mutating, convergence is terminal, so the step only decides how
// promptly the test notices -- it is not a measurement resolution. This test
// makes no timing claims; see TestNetworkDBConvergenceLatency for those.
const (
	convergenceStep    = 100 * time.Millisecond
	convergenceTimeout = 2 * time.Minute
)

// TestNetworkDBAlwaysConverges drives a cluster of NetworkDB instances with a
// random sequence of joins, leaves and table writes, then asserts that every
// node's view of the table ends up identical.
//
// This test exists to explore interleavings, and deliberately never quiesces
// the cluster between actions: gossip is still in flight when the next mutation
// lands, which is where the interesting bugs are. That is also precisely why it
// cannot measure convergence latency -- by the time the property function gets
// to look, most runs have already converged during the state machine's own
// Sleep actions, so any interval it could report would be an upper bound rather
// than a measurement. TestNetworkDBConvergenceLatency trades interleaving for
// precision and measures it properly.
//
// The whole property runs inside a [testing/synctest] bubble, gossiping over an
// in-memory network; see memcluster_test.go for why and how.
func TestNetworkDBAlwaysConverges(t *testing.T) {
	requireSynctest(t)
	rapid.Check(t, func(t *rapid.T) {
		rapid.SyncTest(t, testConvergence)
	})
}

func testConvergence(t *rapid.T) {
	numNodes := rapid.IntRange(2, 25).Draw(t, "numNodes")
	numNetworks := rapid.IntRange(1, 5).Draw(t, "numNetworks")

	// Draw the gossip seed rather than letting each node read crypto/rand, so
	// that it is recorded and shrunk like any other draw instead of being
	// invented. It does not pin the interleaving -- memberlist's own randomness
	// is unseedable and synctest orders the clock, not the scheduler -- but it
	// takes one uncontrolled input out of the picture. A fixed seed would be
	// worse than either: every run would gossip alike, and the point of this
	// test is to explore that space.
	conf := DefaultConfig()
	var seed [32]byte
	binary.LittleEndian.PutUint64(seed[:], rapid.Uint64().Draw(t, "rngSeed"))
	conf.rngSeed = &seed

	c := newMemCluster(t, numNodes, "node", conf)

	fsm := &networkDBFSM{
		nDB:      c.dbs,
		state:    make([]map[string]map[string]string, numNodes),
		keysUsed: make(map[string]map[string]bool),
	}
	for i := range fsm.state {
		fsm.state[i] = make(map[string]map[string]string)
	}
	for i := range numNetworks {
		nw := "nw" + strconv.Itoa(i)
		fsm.networks = append(fsm.networks, nw)
		fsm.keysUsed[nw] = make(map[string]bool)
	}
	// Drive the NetworkDB instances with a sequence of actions in random order.
	// We do not check for convergence until afterwards as NetworkDB is an
	// eventually consistent system.
	t.Repeat(rapid.StateMachineActions(fsm))

	// Take the union of all entries in all networks owned by all nodes.
	converged := make(map[string]map[string]string)
	for _, state := range fsm.state {
		for network, entries := range state {
			if converged[network] == nil {
				converged[network] = make(map[string]string)
			}
			maps.Copy(converged[network], entries)
		}
	}
	expected := make(tableState, numNodes)
	for i, st := range fsm.state {
		exp := make(map[string]map[string]string)
		for k := range st {
			exp[k] = converged[k]
		}
		expected[c.dbs[i].config.NodeID] = exp
	}

	t.Logf("Waiting for NetworkDB state to converge to %#v", converged)
	for i, st := range fsm.state {
		t.Logf("Node #%d (%s): %v", i, c.dbs[i].config.NodeID, slices.Sorted(maps.Keys(st)))
	}
	t.Log("Mutations:")
	for _, m := range fsm.mutations {
		t.Log(m)
	}
	t.Log("---------------------------")

	waitForTableState(t, c.dbs, expected)

	if drops := c.mn.dropCount(); drops != 0 {
		// Not a correctness failure -- gossip has to tolerate loss -- but it
		// means memPacketBuffer was too shallow for this load and the run
		// needed more retransmits than a real cluster would have.
		t.Logf("in-memory network dropped %d datagrams for want of receive buffer", drops)
	}
}

// tableStateCmp is shared by the check which ends the wait below and the diff
// which explains a timeout, so the two cannot disagree: on different options the
// wait could end on state the diff calls equal, and print an empty diff.
//
// A network a node has joined but holds no entries for is an empty map on one
// side and, depending on how it was built, nil on the other. That is not a
// difference worth failing on -- what matters is which networks a node lists and
// which keys it holds -- so equate them.
var tableStateCmp = cmp.Options{cmpopts.EquateEmpty()}

// waitForTableState blocks until every node's view of the table under test
// matches want, and fails the test if that does not happen in time.
func waitForTableState(t *rapid.T, dbs []*NetworkDB, want tableState) {
	deadline := time.Now().Add(convergenceTimeout)
	for {
		synctest.Wait()
		got := snapshotTableState(dbs)
		if cmp.Equal(want, got, tableStateCmp) {
			return
		}
		if !time.Now().Before(deadline) {
			t.Errorf("NetworkDB state did not converge within %v of virtual time:\n%v\n\n%v",
				convergenceTimeout, cmp.Diff(want, got, tableStateCmp), dumpTables(dbs))
			return
		}
		time.Sleep(convergenceStep)
	}
}

// tableState is every node's view of every network's entries in the table under
// test: node ID -> network -> key -> value.
type tableState map[string]map[string]map[string]string

func snapshotTableState(dbs []*NetworkDB) tableState {
	st := make(tableState, len(dbs))
	for _, nDB := range dbs {
		node := make(map[string]map[string]string)
		nDB.RLock()
		for k, nw := range nDB.thisNodeNetworks {
			if !nw.leaving {
				node[k] = make(map[string]string)
			}
		}
		nDB.RUnlock()
		nDB.WalkTable(tableUnderTest, func(network, key string, value []byte, deleting bool) bool {
			if deleting {
				return false
			}
			if node[network] == nil {
				node[network] = make(map[string]string)
			}
			node[network][key] = string(value)
			return false
		})
		st[nDB.config.NodeID] = node
	}
	return st
}

func dumpTables(dbs []*NetworkDB) string {
	dumps := make([]string, len(dbs))
	for i, nDB := range dbs {
		dumps[i] = fmt.Sprintf("Node #%d (%s):\n%v", i, nDB.config.NodeID, nDB.DebugDumpTable(tableUnderTest))
	}
	return strings.Join(dumps, "\n\n")
}

// networkDBFSM is a [rapid.StateMachine] providing the set of actions available
// for rapid to drive NetworkDB with in tests. See also
// [rapid.StateMachineActions] and [rapid.Repeat].
type networkDBFSM struct {
	nDB      []*NetworkDB
	networks []string // list of networks which can be joined
	// node -> joined-network -> key -> value
	state []map[string]map[string]string

	// Remember entry keys that have been used before to avoid trying to
	// create colliding keys. Due to how quickly the FSM runs, it is
	// possible for a node to not have learned that the previous generation
	// of the key was deleted before we try to create it again.
	// network -> key -> true
	keysUsed map[string]map[string]bool

	mutations []string
}

func (u *networkDBFSM) mutated(nodeidx int, action, network, key, value string) {
	desc := fmt.Sprintf("  [%v] #%d(%v):%v(%s", time.Now(), nodeidx, u.nDB[nodeidx].config.NodeID, action, network)
	if key != "" {
		desc += fmt.Sprintf(", %s=%s", key, value)
	}
	desc += ")"
	u.mutations = append(u.mutations, desc)
}

func (u *networkDBFSM) Check(t *rapid.T) {
	// This method is required to implement the [rapid.StateMachine]
	// interface. But there is nothing much to check stepwise as we are
	// testing an eventually consistent system. The checks happen after
	// rapid is done randomly driving the FSM.
}

func (u *networkDBFSM) JoinNetwork(t *rapid.T) {
	// Pick a node that has not joined all networks...
	var nodes []int
	for i, s := range u.state {
		if len(s) < len(u.networks) {
			nodes = append(nodes, i)
		}
	}
	if len(nodes) == 0 {
		t.Skip("All nodes are already joined to all networks")
	}
	nodeidx := rapid.SampledFrom(nodes).Draw(t, "node")

	// ... and a network to join.
	networks := slices.DeleteFunc(slices.Clone(u.networks), func(n string) bool {
		_, ok := u.state[nodeidx][n]
		return ok
	})
	nw := rapid.SampledFrom(networks).Draw(t, "network")

	if err := u.nDB[nodeidx].JoinNetwork(nw); err != nil {
		t.Errorf("Node %v failed to join network %s: %v", nodeidx, nw, err)
	} else {
		u.state[nodeidx][nw] = make(map[string]string)
		u.mutated(nodeidx, "JoinNetwork", nw, "", "")
	}
}

// drawJoinedNode returns a random node that has joined at least one network.
func (u *networkDBFSM) drawJoinedNodeAndNetwork(t *rapid.T) (nodeidx int, nw string) {
	var nodes []int
	for i, s := range u.state {
		if len(s) > 0 {
			nodes = append(nodes, i)
		}
	}
	if len(nodes) == 0 {
		t.Skip("No node is joined to any network")
	}
	nodeidx = rapid.SampledFrom(nodes).Draw(t, "node")

	// Sorted, not merely collected. SampledFrom records the index it chose,
	// and map iteration order is randomised per run, so an unsorted candidate
	// list makes that index resolve to a different element on replay. The
	// model then diverges, later candidate lists differ in size, draws consume
	// different widths, and the rest of the stream desynchronises -- which is
	// why an unsorted list defeats both failfile replay and shrinking.
	nw = rapid.SampledFrom(slices.Sorted(maps.Keys(u.state[nodeidx]))).Draw(t, "network")
	return nodeidx, nw
}

func (u *networkDBFSM) LeaveNetwork(t *rapid.T) {
	nodeidx, nw := u.drawJoinedNodeAndNetwork(t)
	if err := u.nDB[nodeidx].LeaveNetwork(nw); err != nil {
		t.Errorf("Node %v failed to leave network %s: %v", nodeidx, nw, err)
	} else {
		delete(u.state[nodeidx], nw)
		u.mutated(nodeidx, "LeaveNetwork", nw, "", "")
	}
}

func (u *networkDBFSM) CreateEntry(t *rapid.T) {
	nodeidx, nw := u.drawJoinedNodeAndNetwork(t)
	key := rapid.StringMatching(`[a-z]{3,25}`).
		Filter(func(s string) bool { return !u.keysUsed[nw][s] }).
		Draw(t, "key")
	value := rapid.StringMatching(`[a-z]{5,20}`).Draw(t, "value")

	if err := u.nDB[nodeidx].CreateEntry(tableUnderTest, nw, key, []byte(value)); err != nil {
		t.Errorf("Node %v failed to create entry %s=%s in network %s: %v", nodeidx, key, value, nw, err)
	} else {
		u.state[nodeidx][nw][key] = value
		u.keysUsed[nw][key] = true
		u.mutated(nodeidx, "CreateEntry", nw, key, value)
	}
}

// drawOwnedDBKey returns a random key in nw owned by the node at nodeidx.
func (u *networkDBFSM) drawOwnedDBKey(t *rapid.T, nodeidx int, nw string) string {
	keys := slices.Sorted(maps.Keys(u.state[nodeidx][nw])) // sorted: see drawJoinedNodeAndNetwork
	if len(keys) == 0 {
		t.Skipf("Node %v owns no entries in network %s", nodeidx, nw)
		panic("unreachable")
	}
	return rapid.SampledFrom(keys).Draw(t, "key")
}

func (u *networkDBFSM) UpdateEntry(t *rapid.T) {
	nodeidx, nw := u.drawJoinedNodeAndNetwork(t)
	key := u.drawOwnedDBKey(t, nodeidx, nw)
	value := rapid.StringMatching(`[a-z]{5,20}`).Draw(t, "value")

	if err := u.nDB[nodeidx].UpdateEntry(tableUnderTest, nw, key, []byte(value)); err != nil {
		t.Errorf("Node %v failed to update entry %s=%s in network %s: %v", nodeidx, key, value, nw, err)
	} else {
		u.state[nodeidx][nw][key] = value
		u.mutated(nodeidx, "UpdateEntry", nw, key, value)
	}
}

func (u *networkDBFSM) DeleteEntry(t *rapid.T) {
	nodeidx, nw := u.drawJoinedNodeAndNetwork(t)
	key := u.drawOwnedDBKey(t, nodeidx, nw)

	if err := u.nDB[nodeidx].DeleteEntry(tableUnderTest, nw, key); err != nil {
		t.Errorf("Node %v failed to delete entry %s in network %s: %v", nodeidx, key, nw, err)
	} else {
		delete(u.state[nodeidx][nw], key)
		u.mutated(nodeidx, "DeleteEntry", nw, key, "")
	}
}

func (u *networkDBFSM) Sleep(t *rapid.T) {
	duration := time.Duration(rapid.IntRange(10, 500).Draw(t, "duration")) * time.Millisecond
	time.Sleep(duration)
}
