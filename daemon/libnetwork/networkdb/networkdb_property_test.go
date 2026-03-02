//go:build slowtests

package networkdb

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/poll"
	"pgregory.net/rapid"
)

func TestNetworkDBAlwaysConverges(t *testing.T) {
	rapid.Check(t, testConvergence)
}

func testConvergence(t *rapid.T) {
	numNodes := rapid.IntRange(2, 25).Draw(t, "numNodes")
	numNetworks := rapid.IntRange(1, 5).Draw(t, "numNetworks")

	fsm := &networkDBFSM{
		nDB:      createNetworkDBInstances(t, numNodes, "node", DefaultConfig()),
		state:    make([]map[string]map[string]string, numNodes),
		keysUsed: make(map[string]map[string]bool),
	}
	defer closeNetworkDBInstances(t, fsm.nDB)
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
	expected := make(map[string]map[string]map[string]string, numNodes)
	for i, st := range fsm.state {
		exp := make(map[string]map[string]string)
		for k := range st {
			exp[k] = converged[k]
		}
		expected[fsm.nDB[i].config.NodeID] = exp
	}

	t.Logf("Waiting for NetworkDB state to converge to %#v", converged)
	for i, st := range fsm.state {
		t.Logf("Node #%d (%s): %v", i, fsm.nDB[i].config.NodeID, slices.Collect(maps.Keys(st)))
	}
	t.Log("Mutations:")
	for _, m := range fsm.mutations {
		t.Log(m)
	}
	t.Log("---------------------------")

	poll.WaitOn(t, func(t poll.LogT) poll.Result {
		actualState := make(map[string]map[string]map[string]string, numNodes)
		for _, nDB := range fsm.nDB {
			actual := make(map[string]map[string]string)
			for k, nw := range nDB.thisNodeNetworks {
				if !nw.leaving {
					actual[k] = make(map[string]string)
				}
			}
			actualState[nDB.config.NodeID] = actual
		}
		tableContent := make([]string, len(fsm.nDB))
		for i, nDB := range fsm.nDB {
			tableContent[i] = fmt.Sprintf("Node #%d (%s):\n%v", i, nDB.config.NodeID, nDB.DebugDumpTable("some_table"))
			nDB.WalkTable("some_table", func(network, key string, value []byte, deleting bool) bool {
				if deleting {
					return false
				}
				if actualState[nDB.config.NodeID][network] == nil {
					actualState[nDB.config.NodeID][network] = make(map[string]string)
				}
				actualState[nDB.config.NodeID][network][key] = string(value)
				return false
			})
		}
		diff := cmp.Diff(expected, actualState)
		if diff != "" {
			return poll.Continue("NetworkDB state has not converged:\n%v\n%v", diff, strings.Join(tableContent, "\n\n"))
		}
		return poll.Success()
	}, poll.WithTimeout(5*time.Minute), poll.WithDelay(200*time.Millisecond))

	convergenceTime := time.Since(fsm.lastMutation)
	t.Logf("NetworkDB state converged in %v", convergenceTime)

	// Log the convergence time to disk for later statistical analysis.

	if err := os.Mkdir("testdata", 0o755); err != nil && !os.IsExist(err) {
		t.Logf("Could not log convergence time to disk: failed to create testdata directory: %v", err)
		return
	}
	f, err := os.OpenFile("testdata/convergence_time.csv", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Logf("Could not log convergence time to disk: failed to open file: %v", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Logf("Could not log convergence time to disk: error closing file: %v", err)
		}
	}()
	if st, err := f.Stat(); err != nil {
		t.Logf("Could not log convergence time to disk: failed to stat file: %v", err)
		return
	} else if st.Size() == 0 {
		f.WriteString("Nodes,Networks,#Mutations,Convergence(ns)\n")
	}
	if _, err := fmt.Fprintf(f, "%v,%v,%v,%d\n", numNodes, numNetworks, len(fsm.mutations), convergenceTime); err != nil {
		t.Logf("Could not log convergence time to disk: failed to write to file: %v", err)
		return
	}
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

	// Timestamp of the most recent state-machine action which perturbed the
	// system state.
	lastMutation time.Time
	mutations    []string
}

func (u *networkDBFSM) mutated(nodeidx int, action, network, key, value string) {
	u.lastMutation = time.Now()
	desc := fmt.Sprintf("  [%v] #%d(%v):%v(%s", u.lastMutation, nodeidx, u.nDB[nodeidx].config.NodeID, action, network)
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

	nw = rapid.SampledFrom(slices.Collect(maps.Keys(u.state[nodeidx]))).Draw(t, "network")
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

	if err := u.nDB[nodeidx].CreateEntry("some_table", nw, key, []byte(value)); err != nil {
		t.Errorf("Node %v failed to create entry %s=%s in network %s: %v", nodeidx, key, value, nw, err)
	} else {
		u.state[nodeidx][nw][key] = value
		u.keysUsed[nw][key] = true
		u.mutated(nodeidx, "CreateEntry", nw, key, value)
	}
}

// drawOwnedDBKey returns a random key in nw owned by the node at nodeidx.
func (u *networkDBFSM) drawOwnedDBKey(t *rapid.T, nodeidx int, nw string) string {
	keys := slices.Collect(maps.Keys(u.state[nodeidx][nw]))
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

	if err := u.nDB[nodeidx].UpdateEntry("some_table", nw, key, []byte(value)); err != nil {
		t.Errorf("Node %v failed to update entry %s=%s in network %s: %v", nodeidx, key, value, nw, err)
	} else {
		u.state[nodeidx][nw][key] = value
		u.mutated(nodeidx, "UpdateEntry", nw, key, value)
	}
}

func (u *networkDBFSM) DeleteEntry(t *rapid.T) {
	nodeidx, nw := u.drawJoinedNodeAndNetwork(t)
	key := u.drawOwnedDBKey(t, nodeidx, nw)

	if err := u.nDB[nodeidx].DeleteEntry("some_table", nw, key); err != nil {
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
