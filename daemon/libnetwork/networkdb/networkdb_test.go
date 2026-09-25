package networkdb

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"text/tabwriter"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/docker/go-events"
	"github.com/hashicorp/memberlist"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
)

var dbPort atomic.Int32

func init() {
	dbPort.Store(10000)
}

func TestMain(m *testing.M) {
	_ = os.WriteFile("/proc/sys/net/ipv6/conf/lo/disable_ipv6", []byte{'0', '\n'}, 0o644)
	_ = log.SetLevel(log.DebugLevel)
	os.Exit(m.Run())
}

type TestingT interface {
	assert.TestingT
	poll.TestingT
	Cleanup(func())
	Helper()
	Context() context.Context
}

func launchNode(t TestingT, conf Config) *NetworkDB {
	t.Helper()
	db, err := New(&conf)
	assert.NilError(t, err)
	return db
}

func createNetworkDBInstances(t TestingT, num int, namePrefix string, conf *Config) []*NetworkDB {
	t.Helper()
	var dbs []*NetworkDB
	for i := range num {
		localConfig := *conf
		localConfig.Hostname = fmt.Sprintf("%s%d", namePrefix, i+1)
		localConfig.NodeID = stringid.TruncateID(stringid.GenerateRandomID())
		localConfig.BindPort = int(dbPort.Add(1))
		localConfig.BindAddr = "127.0.0.1"
		localConfig.AdvertiseAddr = localConfig.BindAddr
		db := launchNode(t, localConfig)
		if i != 0 {
			assert.Check(t, db.Join([]string{net.JoinHostPort(db.config.AdvertiseAddr, strconv.Itoa(db.config.BindPort-1))}))
		}

		dbs = append(dbs, db)
	}

	// Wait till the cluster creation is successful
	check := func(t poll.LogT) poll.Result {
		// Check that the cluster is properly created
		for i := range num {
			if num != len(dbs[i].ClusterPeers()) {
				return poll.Continue("%s:Waiting for cluster peers to be established", dbs[i].config.Hostname)
			}
		}
		return poll.Success()
	}
	poll.WaitOn(t, check, poll.WithDelay(2*time.Second), poll.WithTimeout(20*time.Second+time.Duration(num-1)*10*time.Second))

	return dbs
}

func closeNetworkDBInstances(t TestingT, dbs []*NetworkDB) {
	t.Helper()
	log.G(t.Context()).Print("Closing DB instances...")
	for _, db := range dbs {
		db.Close()
	}
}

func (nDB *NetworkDB) verifyNodeExistence(t *testing.T, node string, present bool) {
	t.Helper()
	for range 80 {
		nDB.RLock()
		_, ok := nDB.nodes[node]
		nDB.RUnlock()
		if present == ok {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Errorf("%v(%v): expected node %s existence in the cluster = %v, got %v", nDB.config.Hostname, nDB.config.NodeID, node, present, !present)
}

func (nDB *NetworkDB) verifyNetworkExistence(t *testing.T, node string, id string, present bool) {
	t.Helper()

	const sleepInterval = 50 * time.Millisecond
	var maxRetries int64
	if dl, ok := t.Deadline(); ok {
		maxRetries = int64(time.Until(dl) / sleepInterval)
	} else {
		maxRetries = 80
	}
	var ok, leaving bool
	for i := int64(0); i < maxRetries; i++ {
		nDB.RLock()
		var vn *network
		if node == nDB.config.NodeID {
			if n, ok := nDB.thisNodeNetworks[id]; ok {
				vn = &n.network
			}
		} else {
			if nn, nnok := nDB.networks[node]; nnok {
				if n, ok := nn[id]; ok {
					vn = n
				}
			}
		}
		ok = vn != nil
		leaving = ok && vn.leaving
		nDB.RUnlock()

		if present == (ok && !leaving) {
			return
		}

		time.Sleep(sleepInterval)
	}

	if present {
		t.Errorf("%v(%v): want node %v to be a member of network %q, got that it is not a member (ok=%v, leaving=%v)",
			nDB.config.Hostname, nDB.config.NodeID, node, id, ok, leaving)
	} else {
		t.Errorf("%v(%v): want node %v to not be a member of network %q, got that it is a member (ok=%v, leaving=%v)",
			nDB.config.Hostname, nDB.config.NodeID, node, id, ok, leaving)
	}
}

func (nDB *NetworkDB) verifyEntryExistence(t *testing.T, tname, nid, key, value string, present bool) {
	t.Helper()
	n := 80
	var v []byte
	for range n {
		var err error
		v, err = nDB.GetEntry(tname, nid, key)
		if present && err == nil && string(v) == value {
			return
		}
		if cerrdefs.IsNotFound(err) && !present {
			return
		}
		if err != nil && !cerrdefs.IsNotFound(err) {
			t.Errorf("%v(%v): unexpected error while getting entry %v/%v in network %q: %v",
				nDB.config.Hostname, nDB.config.NodeID, tname, key, nid, err)
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Errorf("%v(%v): want entry %v/%v in network %q to be (present=%v, value=%q), got (present=%v, value=%q)",
		nDB.config.Hostname, nDB.config.NodeID, tname, key, nid, present, value, !present, string(v))
}

func testWatch(t *testing.T, ch chan events.Event, tname, nid, key, prev, value string) {
	t.Helper()
	select {
	case rcvdEv := <-ch:
		typ, ok := rcvdEv.(WatchEvent)
		if assert.Check(t, ok, "expected WatchEvent, got %T", rcvdEv) {
			assert.Check(t, is.Equal(tname, typ.Table))
			assert.Check(t, is.Equal(nid, typ.NetworkID))
			assert.Check(t, is.Equal(key, typ.Key))
			if prev == "" {
				assert.Check(t, is.Nil(typ.Prev))
			} else {
				assert.Check(t, is.Equal(prev, string(typ.Prev)))
			}
			if value == "" {
				assert.Check(t, is.Nil(typ.Value))
			} else {
				assert.Check(t, is.Equal(value, string(typ.Value)))
			}
		}
	case <-time.After(time.Second):
		t.Fail()
		return
	}
}

func TestNetworkDBSimple(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())
	closeNetworkDBInstances(t, dbs)
}

func TestNetworkDBJoinLeaveNetwork(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())

	err := dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, "network1", true)

	err = dbs[0].LeaveNetwork("network1")
	assert.NilError(t, err)

	dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, "network1", false)
	closeNetworkDBInstances(t, dbs)
}

func TestNetworkDBJoinLeaveNetworks(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())

	n := 10
	for i := 1; i <= n; i++ {
		err := dbs[0].JoinNetwork(fmt.Sprintf("network0%d", i))
		assert.NilError(t, err)
	}

	for i := 1; i <= n; i++ {
		err := dbs[1].JoinNetwork(fmt.Sprintf("network1%d", i))
		assert.NilError(t, err)
	}

	for i := 1; i <= n; i++ {
		dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, fmt.Sprintf("network0%d", i), true)
	}

	for i := 1; i <= n; i++ {
		dbs[0].verifyNetworkExistence(t, dbs[1].config.NodeID, fmt.Sprintf("network1%d", i), true)
	}

	for i := 1; i <= n; i++ {
		err := dbs[0].LeaveNetwork(fmt.Sprintf("network0%d", i))
		assert.NilError(t, err)
	}

	for i := 1; i <= n; i++ {
		err := dbs[1].LeaveNetwork(fmt.Sprintf("network1%d", i))
		assert.NilError(t, err)
	}

	for i := 1; i <= n; i++ {
		dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, fmt.Sprintf("network0%d", i), false)
	}

	for i := 1; i <= n; i++ {
		dbs[0].verifyNetworkExistence(t, dbs[1].config.NodeID, fmt.Sprintf("network1%d", i), false)
	}

	closeNetworkDBInstances(t, dbs)
}

func TestNetworkDBCRUDTableEntry(t *testing.T) {
	dbs := createNetworkDBInstances(t, 3, "node", DefaultConfig())

	err := dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, "network1", true)

	err = dbs[1].JoinNetwork("network1")
	assert.NilError(t, err)

	err = dbs[0].CreateEntry("test_table", "network1", "test_key", []byte("test_value"))
	assert.NilError(t, err)

	dbs[1].verifyEntryExistence(t, "test_table", "network1", "test_key", "test_value", true)
	dbs[2].verifyEntryExistence(t, "test_table", "network1", "test_key", "test_value", false)

	err = dbs[0].UpdateEntry("test_table", "network1", "test_key", []byte("test_updated_value"))
	assert.NilError(t, err)

	dbs[1].verifyEntryExistence(t, "test_table", "network1", "test_key", "test_updated_value", true)

	err = dbs[0].DeleteEntry("test_table", "network1", "test_key")
	assert.NilError(t, err)

	dbs[1].verifyEntryExistence(t, "test_table", "network1", "test_key", "", false)

	closeNetworkDBInstances(t, dbs)
}

// TestCreateEntryOverTombstone checks that a key can be created again while
// the tombstone of its previous incarnation is still lingering in the store,
// and that a live entry still holds its key against a create.
func TestCreateEntryOverTombstone(t *testing.T) {
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.NilError(t, nDB.JoinNetwork("network1"))

	assert.NilError(t, nDB.CreateEntry("test_table", "network1", "test_key", []byte("v1")))
	assert.NilError(t, nDB.DeleteEntry("test_table", "network1", "test_key"))

	nDB.RLock()
	tombstone, err := nDB.getEntry("test_table", "network1", "test_key")
	nDB.RUnlock()
	assert.NilError(t, err)
	assert.Assert(t, tombstone.deleting, "the deleted entry should still be lingering as a tombstone")

	assert.NilError(t, nDB.CreateEntry("test_table", "network1", "test_key", []byte("v2")))
	v, err := nDB.GetEntry("test_table", "network1", "test_key")
	assert.NilError(t, err)
	assert.Check(t, is.Equal("v2", string(v)))

	// The entry is live again, so it holds its key.
	assert.Check(t, is.ErrorContains(
		nDB.CreateEntry("test_table", "network1", "test_key", []byte("v3")), "already exists"))
}

// TestUpdateAndDeleteEntryOnTombstone checks that a tombstone counts as a
// non-existent entry for the write API, matching what a read of it reports:
// neither an update nor a delete of one succeeds, and a refused update does
// not resurrect it. TestCreateEntryOverTombstone covers the create side, where
// the key is instead free to be taken over.
func TestUpdateAndDeleteEntryOnTombstone(t *testing.T) {
	nDB := newNetworkDB(DefaultConfig())
	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{}
	nDB.nodeBroadcasts = &memberlist.TransmitLimitedQueue{}
	assert.NilError(t, nDB.JoinNetwork("network1"))

	assert.NilError(t, nDB.CreateEntry("test_table", "network1", "test_key", []byte("v1")))
	assert.NilError(t, nDB.DeleteEntry("test_table", "network1", "test_key"))

	nDB.RLock()
	tombstone, err := nDB.getEntry("test_table", "network1", "test_key")
	nDB.RUnlock()
	assert.NilError(t, err)
	assert.Assert(t, tombstone.deleting, "the deleted entry should still be lingering as a tombstone")

	// A read already reports the key as gone, so the writes must agree.
	_, err = nDB.GetEntry("test_table", "network1", "test_key")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(
		nDB.UpdateEntry("test_table", "network1", "test_key", []byte("v2")), "does not exist"))
	assert.Check(t, is.ErrorContains(
		nDB.DeleteEntry("test_table", "network1", "test_key"), "does not exist"))

	// The refused update must not have resurrected the entry nor taken its key.
	nDB.RLock()
	after, err := nDB.getEntry("test_table", "network1", "test_key")
	nDB.RUnlock()
	assert.NilError(t, err)
	assert.Check(t, after.deleting, "the tombstone should still be a tombstone")
	assert.Check(t, is.Equal("v1", string(after.value)), "the tombstone's value should be untouched")
	assert.Check(t, is.Equal(nDB.config.NodeID, after.node), "the tombstone's owner should be untouched")
}

func (nDB *NetworkDB) dumpTable(t *testing.T, tname string) {
	t.Helper()
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 10, 1, 1, ' ', 0)
	tw.Write([]byte("NetworkID\tKey\tValue\tFlags\n"))
	nDB.WalkTable(tname, func(nid, key string, value []byte, deleting bool) bool {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%v\n", nid, key, string(value), map[bool]string{true: "D"}[deleting])
		return false
	})
	tw.Flush()
	t.Logf("%s(%s): Table %s:\n%s", nDB.config.Hostname, nDB.config.NodeID, tname, b.String())
}

func TestNetworkDBCRUDTableEntries(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())

	err := dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, "network1", true)

	err = dbs[1].JoinNetwork("network1")
	assert.NilError(t, err)

	dbs[0].verifyNetworkExistence(t, dbs[1].config.NodeID, "network1", true)

	n := 10
	for i := 1; i <= n; i++ {
		err = dbs[0].CreateEntry("test_table", "network1",
			fmt.Sprintf("test_key0%d", i),
			fmt.Appendf(nil, "test_value0%d", i))
		assert.NilError(t, err)
	}

	for i := 1; i <= n; i++ {
		err = dbs[1].CreateEntry("test_table", "network1",
			fmt.Sprintf("test_key1%d", i),
			fmt.Appendf(nil, "test_value1%d", i))
		assert.NilError(t, err)
	}

	for n := range dbs {
		dbs[n].dumpTable(t, "test_table")
	}

	for i := 1; i <= n; i++ {
		dbs[0].verifyEntryExistence(t, "test_table", "network1",
			fmt.Sprintf("test_key1%d", i),
			fmt.Sprintf("test_value1%d", i), true)
	}

	for i := 1; i <= n; i++ {
		dbs[1].verifyEntryExistence(t, "test_table", "network1",
			fmt.Sprintf("test_key0%d", i),
			fmt.Sprintf("test_value0%d", i), true)
	}

	for n := range dbs {
		dbs[n].dumpTable(t, "test_table")
	}

	// Verify deletes
	for i := 1; i <= n; i++ {
		err = dbs[0].DeleteEntry("test_table", "network1",
			fmt.Sprintf("test_key0%d", i))
		assert.NilError(t, err)
	}

	for i := 1; i <= n; i++ {
		err = dbs[1].DeleteEntry("test_table", "network1",
			fmt.Sprintf("test_key1%d", i))
		assert.NilError(t, err)
	}

	for i := 1; i <= n; i++ {
		dbs[0].verifyEntryExistence(t, "test_table", "network1",
			fmt.Sprintf("test_key1%d", i), "", false)
	}

	for i := 1; i <= n; i++ {
		dbs[1].verifyEntryExistence(t, "test_table", "network1",
			fmt.Sprintf("test_key0%d", i), "", false)
	}

	for n := range dbs {
		dbs[n].dumpTable(t, "test_table")
	}

	closeNetworkDBInstances(t, dbs)
}

func TestNetworkDBNodeLeave(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())

	err := dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	err = dbs[1].JoinNetwork("network1")
	assert.NilError(t, err)

	err = dbs[0].CreateEntry("test_table", "network1", "test_key", []byte("test_value"))
	assert.NilError(t, err)

	dbs[1].verifyEntryExistence(t, "test_table", "network1", "test_key", "test_value", true)

	dbs[0].Close()
	dbs[1].verifyEntryExistence(t, "test_table", "network1", "test_key", "test_value", false)
	dbs[1].Close()
}

func TestNetworkDBWatch(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())
	err := dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	err = dbs[1].JoinNetwork("network1")
	assert.NilError(t, err)

	ch, cancel := dbs[1].Watch("", "")

	err = dbs[0].CreateEntry("test_table", "network1", "test_key", []byte("test_value"))
	assert.NilError(t, err)

	testWatch(t, ch.C, "test_table", "network1", "test_key", "", "test_value")

	err = dbs[0].UpdateEntry("test_table", "network1", "test_key", []byte("test_updated_value"))
	assert.NilError(t, err)

	testWatch(t, ch.C, "test_table", "network1", "test_key", "test_value", "test_updated_value")

	err = dbs[0].DeleteEntry("test_table", "network1", "test_key")
	assert.NilError(t, err)

	testWatch(t, ch.C, "test_table", "network1", "test_key", "test_updated_value", "")

	cancel()
	closeNetworkDBInstances(t, dbs)
}

func TestNetworkDBBulkSync(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())

	err := dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, "network1", true)

	n := 1000
	for i := 1; i <= n; i++ {
		err = dbs[0].CreateEntry("test_table", "network1",
			fmt.Sprintf("test_key0%d", i),
			fmt.Appendf(nil, "test_value0%d", i))
		assert.NilError(t, err)
	}

	err = dbs[1].JoinNetwork("network1")
	assert.NilError(t, err)

	dbs[0].verifyNetworkExistence(t, dbs[1].config.NodeID, "network1", true)

	for i := 1; i <= n; i++ {
		dbs[1].verifyEntryExistence(t, "test_table", "network1",
			fmt.Sprintf("test_key0%d", i),
			fmt.Sprintf("test_value0%d", i), true)
		assert.NilError(t, err)
	}

	closeNetworkDBInstances(t, dbs)
}

// TestBulkSyncWithFailedPeer checks that a bulk sync is still sent to a peer
// which memberlist has declared failed: such a peer can well be reachable, and
// waiting for a reply, before we learn that it is back. Only a peer nothing is
// known about is reported as unreachable, so that the caller does not count the
// sync as completed.
func TestBulkSyncWithFailedPeer(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())
	defer closeNetworkDBInstances(t, dbs)

	assert.NilError(t, dbs[0].JoinNetwork("network1"))
	dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, "network1", true)
	assert.NilError(t, dbs[1].JoinNetwork("network1"))
	dbs[0].verifyNetworkExistence(t, dbs[1].config.NodeID, "network1", true)

	// Make node0 consider node1 failed while node1 is still very much alive.
	// Nothing moves it back to active: a node event for a peer which is not in
	// the memberlist node list is ignored, and memberlist only notifies a join
	// for a node it sees coming back.
	dbs[0].Lock()
	ok := dbs[0].failNode(t.Context(), dbs[1].config.NodeID)
	dbs[0].Unlock()
	assert.Assert(t, ok)

	assert.NilError(t, dbs[0].bulkSyncNode([]string{"network1"}, dbs[1].config.NodeID, false))
	assert.Check(t, is.ErrorContains(
		dbs[0].bulkSyncNode([]string{"network1"}, "nosuchnode", false), "unknown node"))
}

// Regression test for https://github.com/moby/moby/issues/51701
func TestNetworkDBBulkSyncNodeConcurrent(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())
	defer closeNetworkDBInstances(t, dbs)

	err := dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, "network1", true)

	err = dbs[1].JoinNetwork("network1")
	assert.NilError(t, err)

	dbs[0].verifyNetworkExistence(t, dbs[1].config.NodeID, "network1", true)

	for i := range 50 {
		err = dbs[0].CreateEntry("test_table", "network1",
			fmt.Sprintf("key%d", i),
			fmt.Appendf(nil, "val%d", i))
		assert.NilError(t, err)
	}

	const N = 4
	start := make(chan struct{})
	var eg errgroup.Group
	for i := range N {
		eg.Go(func() error {
			<-start
			err := dbs[1].bulkSyncNode(
				[]string{"network1"}, dbs[0].config.NodeID, true)
			if err != nil {
				return fmt.Errorf("[%d] %w", i, err)
			}
			return nil
		})
	}
	close(start)
	assert.NilError(t, eg.Wait())

	// No bulk sync should have timed out. On broken code this is N-1.
	assert.Equal(t, dbs[1].bulkSyncAckTimeouts.Load(), uint64(0),
		"expected 0 bulk sync timeouts, but some ack channels were orphaned")

	dbs[1].RLock()
	defer dbs[1].RUnlock()
	assert.Assert(t, is.Len(dbs[1].bulkSyncAckTbl, 0),
		"bulkSyncAckTbl should be empty after all syncs complete")
}

func TestNetworkDBCRUDMediumCluster(t *testing.T) {
	n := 5

	dbs := createNetworkDBInstances(t, n, "node", DefaultConfig())

	// Shake out any data races.
	done := make(chan struct{})
	defer close(done)
	for _, db := range dbs {
		go func(db *NetworkDB) {
			for {
				select {
				case <-done:
					return
				default:
				}
				_ = db.GetTableByNetwork("test_table", "network1")
			}
		}(db)
	}

	for i := range n {
		for j := range n {
			if i == j {
				continue
			}

			dbs[i].verifyNodeExistence(t, dbs[j].config.NodeID, true)
		}
	}

	for i := range n {
		err := dbs[i].JoinNetwork("network1")
		assert.NilError(t, err)
	}

	for i := range n {
		for j := range n {
			dbs[i].verifyNetworkExistence(t, dbs[j].config.NodeID, "network1", true)
		}
	}

	err := dbs[0].CreateEntry("test_table", "network1", "test_key", []byte("test_value"))
	assert.NilError(t, err)

	for i := 1; i < n; i++ {
		dbs[i].verifyEntryExistence(t, "test_table", "network1", "test_key", "test_value", true)
	}

	err = dbs[0].UpdateEntry("test_table", "network1", "test_key", []byte("test_updated_value"))
	assert.NilError(t, err)

	for i := 1; i < n; i++ {
		dbs[i].verifyEntryExistence(t, "test_table", "network1", "test_key", "test_updated_value", true)
	}

	err = dbs[0].DeleteEntry("test_table", "network1", "test_key")
	assert.NilError(t, err)

	for i := 1; i < n; i++ {
		dbs[i].verifyEntryExistence(t, "test_table", "network1", "test_key", "", false)
	}

	for i := 1; i < n; i++ {
		_, err = dbs[i].GetEntry("test_table", "network1", "test_key")
		assert.Check(t, is.ErrorContains(err, ""))
		assert.Check(t, is.Contains(err.Error(), "deleted and pending garbage collection"), err)
	}

	closeNetworkDBInstances(t, dbs)
}

func TestNetworkDBNodeJoinLeaveIteration(t *testing.T) {
	dbs := createNetworkDBInstances(t, 2, "node", DefaultConfig())

	dbChangeWitness := func(nDB *NetworkDB) func(network string, expectNodeCount int) {
		staleNetworkTime := nDB.networkClock.Time()
		return func(network string, expectNodeCount int) {
			check := func(t poll.LogT) poll.Result {
				networkTime := nDB.networkClock.Time()
				if networkTime <= staleNetworkTime {
					return poll.Continue("network time is stale, no change registered yet.")
				}
				count := -1
				nDB.Lock()
				if nodes, ok := nDB.networkNodes[network]; ok {
					count = len(nodes)
				}
				nDB.Unlock()
				if count != expectNodeCount {
					return poll.Continue("current number of nodes is %d, expect %d.", count, expectNodeCount)
				}
				return poll.Success()
			}
			t.Helper()
			poll.WaitOn(t, check, poll.WithTimeout(3*time.Second), poll.WithDelay(5*time.Millisecond))
		}
	}

	// Single node Join/Leave
	witness0 := dbChangeWitness(dbs[0])
	err := dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)
	witness0("network1", 1)

	witness0 = dbChangeWitness(dbs[0])
	err = dbs[0].LeaveNetwork("network1")
	assert.NilError(t, err)
	witness0("network1", 0)

	// Multiple nodes Join/Leave
	witness0, witness1 := dbChangeWitness(dbs[0]), dbChangeWitness(dbs[1])
	err = dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	err = dbs[1].JoinNetwork("network1")
	assert.NilError(t, err)

	// Wait for the propagation on db[0]
	dbs[0].verifyNetworkExistence(t, dbs[1].config.NodeID, "network1", true)
	witness0("network1", 2)
	if n, ok := dbs[0].thisNodeNetworks["network1"]; !ok || n.leaving {
		t.Fatal("The network should not be marked as leaving")
	}

	// Wait for the propagation on db[1]
	dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, "network1", true)
	witness1("network1", 2)
	if n, ok := dbs[1].thisNodeNetworks["network1"]; !ok || n.leaving {
		t.Fatal("The network should not be marked as leaving")
	}

	// Try a quick leave/join
	witness0, witness1 = dbChangeWitness(dbs[0]), dbChangeWitness(dbs[1])
	err = dbs[0].LeaveNetwork("network1")
	assert.NilError(t, err)
	err = dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	dbs[0].verifyNetworkExistence(t, dbs[1].config.NodeID, "network1", true)
	witness0("network1", 2)

	dbs[1].verifyNetworkExistence(t, dbs[0].config.NodeID, "network1", true)
	witness1("network1", 2)

	closeNetworkDBInstances(t, dbs)
}

func TestNetworkDBGarbageCollection(t *testing.T) {
	keysWriteDelete := 5
	config := DefaultConfig()
	config.reapEntryInterval = 30 * time.Second
	config.StatsPrintPeriod = 15 * time.Second

	dbs := createNetworkDBInstances(t, 3, "node", config)

	// 2 Nodes join network
	err := dbs[0].JoinNetwork("network1")
	assert.NilError(t, err)

	err = dbs[1].JoinNetwork("network1")
	assert.NilError(t, err)

	for i := range keysWriteDelete {
		err = dbs[i%2].CreateEntry("testTable", "network1", "key-"+strconv.Itoa(i), []byte("value"))
		assert.NilError(t, err)
	}
	time.Sleep(time.Second)
	for i := range keysWriteDelete {
		err = dbs[i%2].DeleteEntry("testTable", "network1", "key-"+strconv.Itoa(i))
		assert.NilError(t, err)
	}
	for i := range 2 {
		dbs[i].Lock()
		assert.Check(t, is.Equal(int64(keysWriteDelete), dbs[i].thisNodeNetworks["network1"].entriesNumber.Load()), "entries number should match")
		dbs[i].Unlock()
	}

	// from this point the timer for the garbage collection started, wait 5 seconds and then join a new node
	time.Sleep(5 * time.Second)

	err = dbs[2].JoinNetwork("network1")
	assert.NilError(t, err)
	for i := range 3 {
		dbs[i].Lock()
		assert.Check(t, is.Equal(int64(keysWriteDelete), dbs[i].thisNodeNetworks["network1"].entriesNumber.Load()), "entries number should match")
		dbs[i].Unlock()
	}
	// Wait for the reaper instead of assuming it runs on schedule.
	poll.WaitOn(t, func(t poll.LogT) poll.Result {
		for _, db := range dbs {
			db.Lock()
			entries := db.thisNodeNetworks["network1"].entriesNumber.Load()
			db.Unlock()
			if entries != 0 {
				return poll.Continue("%s still has %d entries", db.config.Hostname, entries)
			}
		}
		return poll.Success()
	}, poll.WithDelay(time.Second), poll.WithTimeout(2*config.reapEntryInterval))

	// make sure that entries are not coming back
	time.Sleep(15 * time.Second)
	for i := range 3 {
		dbs[i].Lock()
		assert.Check(t, is.Equal(int64(0), dbs[i].thisNodeNetworks["network1"].entriesNumber.Load()), "entries should had been garbage collected")
		dbs[i].Unlock()
	}

	closeNetworkDBInstances(t, dbs)
}

func checkNodeIsForgotten(t *testing.T, db *NetworkDB, nodeName string, msgAndArgs ...any) {
	t.Helper()
	_, active := db.nodes[nodeName]
	assert.Check(t, !active, msgAndArgs...)
	_, failed := db.failedNodes[nodeName]
	assert.Check(t, !failed, msgAndArgs...)
}

func TestNodeStateTransitions(t *testing.T) {
	db := newNetworkDB(DefaultConfig())
	defer db.broadcaster.Close()

	db.nodes["node1"] = &node{Node: memberlist.Node{Name: "node1"}}
	db.nodes["node2"] = &node{Node: memberlist.Node{Name: "node2"}}
	db.nodes["node3"] = &node{Node: memberlist.Node{Name: "node3"}}

	assert.Check(t, is.Len(db.nodes, 3))

	_, failed := db.failedNodes["node1"]
	assert.Check(t, !failed)
	n, active := db.nodes["node1"]
	assert.Check(t, active)
	assert.Check(t, n != nil)
	assert.Check(t, is.Equal("node1", n.Name))

	// node1 to failed
	assert.Check(t, db.failNode(t.Context(), "node1"))

	_, active = db.nodes["node1"]
	assert.Check(t, !active)
	n, failed = db.failedNodes["node1"]
	assert.Check(t, failed)
	assert.Check(t, n != nil)
	assert.Check(t, is.Equal("node1", n.Name))
	assert.Check(t, time.Duration(0) != n.reapTime)

	// node1 back to active
	assert.Check(t, db.reactivateNode(t.Context(), "node1"))

	_, failed = db.failedNodes["node1"]
	assert.Check(t, !failed)
	n, active = db.nodes["node1"]
	assert.Check(t, active)
	assert.Check(t, n != nil)
	assert.Check(t, is.Equal("node1", n.Name))
	assert.Check(t, is.Equal(time.Duration(0), n.reapTime))

	// node1 to left. A node which has left is forgotten outright, so there is
	// nothing left to find it in.
	assert.Check(t, db.forgetNode(t.Context(), "node1"))
	assert.Check(t, db.forgetNode(t.Context(), "node2"))
	assert.Check(t, db.forgetNode(t.Context(), "node3"))

	for _, name := range []string{"node1", "node2", "node3"} {
		checkNodeIsForgotten(t, db, name, "%s should be forgotten once it has left", name)
	}

	assert.Check(t, is.Len(db.nodes, 0))
	assert.Check(t, is.Len(db.failedNodes, 0))
}

func TestNodeReincarnation(t *testing.T) {
	db := newNetworkDB(DefaultConfig())
	defer db.broadcaster.Close()

	const addrLive, addrDead = "192.168.1.1", "192.168.1.2"
	db.nodes["active1"] = &node{Node: memberlist.Node{Name: "active1", Addr: net.ParseIP(addrLive)}}
	db.failedNodes["failed1"] = &node{Node: memberlist.Node{Name: "failed1", Addr: net.ParseIP(addrDead)}}
	db.failedNodes["failed2"] = &node{Node: memberlist.Node{Name: "failed2", Addr: net.ParseIP(addrDead)}}

	// An active node is left where it is. An address collision says nothing
	// about which of the two names is the current one, and gossip about a
	// departed incarnation can arrive after its replacement is known, so
	// retiring active1 here would evict a peer which may well be the live one.
	assert.Check(t, is.Equal(db.purgeReincarnation(&memberlist.Node{Name: "new1", Addr: net.ParseIP(addrLive)}), 0),
		"an active node should survive an address collision")
	assert.Check(t, is.Contains(db.nodes, "active1"))

	// An address nobody is at supersedes nothing.
	assert.Check(t, is.Equal(db.purgeReincarnation(&memberlist.Node{Name: "new2", Addr: net.ParseIP("192.168.1.10")}), 0))

	// Every node memberlist has already given up on at that address is
	// retired, not only the first one found.
	assert.Check(t, is.Equal(db.purgeReincarnation(&memberlist.Node{Name: "new3", Addr: net.ParseIP(addrDead)}), 2))
	checkNodeIsForgotten(t, db, "failed1")
	checkNodeIsForgotten(t, db, "failed2")

	// The half of the decision purgeReincarnation defers: once memberlist has
	// given up on the active node and a replacement holds its address, it is
	// superseded and NotifyLeave retires it.
	db.nodes["new1"] = &node{Node: memberlist.Node{Name: "new1", Addr: net.ParseIP(addrLive)}}
	ed := &eventDelegate{db}
	ed.NotifyLeave(&memberlist.Node{Name: "active1", Addr: net.ParseIP(addrLive)})
	checkNodeIsForgotten(t, db, "active1", "superseded once memberlist gave up on it")
	assert.Check(t, is.Contains(db.nodes, "new1"), "the replacement is untouched")
}

func TestParallelCreate(t *testing.T) {
	dbs := createNetworkDBInstances(t, 1, "node", DefaultConfig())

	startCh := make(chan int)
	doneCh := make(chan error)
	var success atomic.Uint32
	for range 20 {
		go func() {
			<-startCh
			err := dbs[0].CreateEntry("testTable", "testNetwork", "key", []byte("value"))
			if err == nil {
				success.Add(1)
			}
			doneCh <- err
		}()
	}

	close(startCh)

	for range 20 {
		<-doneCh
	}
	close(doneCh)
	// Only 1 write should have succeeded
	assert.Check(t, is.Equal(uint32(1), success.Load()))

	closeNetworkDBInstances(t, dbs)
}

func TestParallelDelete(t *testing.T) {
	dbs := createNetworkDBInstances(t, 1, "node", DefaultConfig())

	err := dbs[0].CreateEntry("testTable", "testNetwork", "key", []byte("value"))
	assert.NilError(t, err)

	startCh := make(chan int)
	doneCh := make(chan error)
	var success atomic.Uint32
	for range 20 {
		go func() {
			<-startCh
			err := dbs[0].DeleteEntry("testTable", "testNetwork", "key")
			if err == nil {
				success.Add(1)
			}
			doneCh <- err
		}()
	}

	close(startCh)

	for range 20 {
		<-doneCh
	}
	close(doneCh)
	// Only 1 write should have succeeded
	assert.Check(t, is.Equal(uint32(1), success.Load()))

	closeNetworkDBInstances(t, dbs)
}

func TestNetworkDBIslands(t *testing.T) {
	t.Skip("FIXME: flaky test; see https://github.com/moby/moby/issues/42459")

	pollTimeout := func() time.Duration {
		const defaultTimeout = 120 * time.Second
		dl, ok := t.Deadline()
		if !ok {
			return defaultTimeout
		}
		if d := time.Until(dl); d <= defaultTimeout {
			return d
		}
		return defaultTimeout
	}

	_ = log.SetLevel(log.DebugLevel)
	conf := DefaultConfig()
	// Shorten durations to speed up test execution.
	conf.rejoinClusterDuration = conf.rejoinClusterDuration / 10
	conf.rejoinClusterInterval = conf.rejoinClusterInterval / 10
	dbs := createNetworkDBInstances(t, 5, "node", conf)

	// Get the node IP used currently
	node := dbs[0].nodes[dbs[0].config.NodeID]
	baseIPStr := node.Addr.String()
	// Node 0,1,2 are going to be the 3 bootstrap nodes
	members := []string{
		fmt.Sprintf("%s:%d", baseIPStr, dbs[0].config.BindPort),
		fmt.Sprintf("%s:%d", baseIPStr, dbs[1].config.BindPort),
		fmt.Sprintf("%s:%d", baseIPStr, dbs[2].config.BindPort),
	}
	// Rejoining will update the list of the bootstrap members
	for i := 3; i < 5; i++ {
		t.Logf("Re-joining: %d", i)
		assert.Check(t, dbs[i].Join(members))
	}

	// Now the 3 bootstrap nodes will cleanly leave, and will be properly removed from the other 2 nodes
	departed := make([]string, 3)
	for i := range departed {
		log.G(t.Context()).Infof("node %d leaving", i)
		// Record the node ID so we can check if their absence has
		// propagated to the survivor nodes.
		departed[i] = dbs[i].config.NodeID
		dbs[i].Close()
	}

	checkDBs := make(map[string]*NetworkDB)
	for i := 3; i < 5; i++ {
		db := dbs[i]
		checkDBs[db.config.Hostname] = db
	}

	// Give some time to let the system propagate the messages and free up the ports
	check := func(t poll.LogT) poll.Result {
		// Verify that the nodes are actually all gone. One which memberlist
		// declared failed rather than seeing leave is still on its way out,
		// which is the latitude the left-or-failed count used to allow.
		for name, db := range checkDBs {
			db.RLock()
			var stillPeers []string
			for _, id := range departed {
				if _, ok := db.nodes[id]; ok {
					stillPeers = append(stillPeers, id)
				}
			}
			db.RUnlock()
			if len(stillPeers) > 0 {
				return poll.Continue("%s:Waiting for nodes %v to leave", name, stillPeers)
			}
			t.Logf("%s: OK", name)
			delete(checkDBs, name)
		}
		return poll.Success()
	}
	poll.WaitOn(t, check, poll.WithDelay(time.Second), poll.WithTimeout(pollTimeout()))

	// Spawn again the first 3 nodes with different names but same IP:port
	for i := range 3 {
		log.G(t.Context()).Infof("node %d coming back", i)
		conf := *dbs[i].config
		conf.NodeID = stringid.TruncateID(stringid.GenerateRandomID())
		dbs[i] = launchNode(t, conf)
	}

	// Give some time for the reconnect routine to run, it runs every 6s.
	check = func(t poll.LogT) poll.Result {
		// Verify that the cluster is again all connected. Note that the 3 previous node did not do any join.
		// The two groups converge on the same state now: the survivors have
		// nothing left to remember the departed incarnations by, so there is
		// no longer an asymmetry between them and the nodes which came back.
		for i := range 5 {
			db := dbs[i]
			db.RLock()
			nNodes, nFailed := len(db.nodes), len(db.failedNodes)
			db.RUnlock()
			if nNodes != 5 {
				return poll.Continue("%s:Waiting to connect to all nodes", dbs[i].config.Hostname)
			}
			if nFailed != 0 {
				return poll.Continue("%s:Waiting for 0 failedNodes", dbs[i].config.Hostname)
			}
		}
		return poll.Success()
	}
	poll.WaitOn(t, check, poll.WithDelay(time.Second), poll.WithTimeout(pollTimeout()))
	closeNetworkDBInstances(t, dbs)
}
