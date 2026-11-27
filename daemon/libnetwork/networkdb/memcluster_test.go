package networkdb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/docker/go-events"
	"github.com/hashicorp/memberlist"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// Helpers for running a cluster of NetworkDB instances inside a
// [testing/synctest] bubble, gossiping over the in-memory network implemented at
// the bottom of this file rather than over real sockets.
//
// A goroutine parked on a socket is not *durably* blocked, so a bubble
// containing real network I/O never quiesces and its clock never advances.
// Substituting the transport makes the whole cluster timer-driven, which buys
// two things: waits cost nothing, and [synctest.Wait] becomes an exact "the
// cluster has finished reacting to everything it already knows" signal.
//
// Everything in this file may only be called from inside a bubble, apart from
// runInCluster and runBubbled, which are how a test gets into one.

// tableUnderTest is the single NetworkDB table these tests read and write.
const tableUnderTest = "some_table"

// Virtual-time budgets. These are spent from the bubble's fake clock, so a
// generous budget costs nothing but the CPU of the checks it performs.
const (
	formationStep    = 100 * time.Millisecond
	formationTimeout = 60 * time.Second

	// propagation budgets bound the verify* helpers below: how long one
	// change is given to reach the node being asked about. Sized like
	// settleTimeout rather than like a gossip round, because the tail of
	// NetworkDB's convergence is its anti-entropy sweep rather than gossip,
	// and a budget which only covers the gossip path turns the slow half of
	// a bimodal distribution into a test failure.
	propagationStep    = 10 * time.Millisecond
	propagationTimeout = 90 * time.Second

	// watchTimeout bounds a wait for one watch event. Same reasoning.
	watchTimeout = 90 * time.Second

	// shutdownDrain has to exceed the longest wait Close() cannot cut short.
	// That is bulkSyncNode's ack wait: a flat 30s timer with no ctx case
	// (cluster.go), so cancelCtx does not touch it, and bulkSyncTables works
	// through its networks serially, so one latched tick can arm it once per
	// network -- 5 in TestNetworkDBAlwaysConverges, hence 150s. At exactly 30s
	// the budget covered one network and nothing else, and a sync armed at or
	// after the moment the teardown starts counting outlived it, leaving a
	// goroutine parked when the bubble ended: a deadlock panic that takes the
	// whole test binary with it.
	shutdownDrain = 3 * time.Minute
)

// memCluster is a cluster of NetworkDB instances on a private in-memory network.
type memCluster struct {
	dbs []*NetworkDB
	mn  *memNetwork

	// closed is the set of instances the test has already shut down itself,
	// which the teardown must not shut down again. See closeNode.
	closed map[*NetworkDB]struct{}

	// relaunches counts the nodes brought back by relaunch, so each gets a
	// node ID of its own without reaching for randomness.
	relaunches int
}

// closeNode shuts node i down and records that it is gone, so that the teardown
// leaves it alone.
//
// A test whose scenario is a node departing has to go through here rather than
// calling Close directly: [NetworkDB.Close] is not idempotent, and the second
// call reaches memberlist.Leave on an already shut-down memberlist, which
// panics outright rather than returning an error.
//
// The node keeps its place in c.dbs. Callers index the cluster positionally and
// a departed node is still the subject of the assertions that follow -- its
// peers are expected to notice it left -- so removing it would renumber the
// cluster out from under them.
func (c *memCluster) closeNode(i int) {
	db := c.dbs[i]
	db.Close()
	if c.closed == nil {
		c.closed = make(map[*NetworkDB]struct{})
	}
	c.closed[db] = struct{}{}
}

// live is the subset of the cluster which is still running.
func (c *memCluster) live() []*NetworkDB {
	dbs := make([]*NetworkDB, 0, len(c.dbs))
	for _, db := range c.dbs {
		if _, done := c.closed[db]; !done {
			dbs = append(dbs, db)
		}
	}
	return dbs
}

// newMemCluster brings up num NetworkDB instances wired to a private in-memory
// network and waits for every node to see every other. The cluster is torn down
// by a [testing.T.Cleanup] registered here, so callers need no teardown of their
// own.
//
// Owning the teardown matters, rather than leaving a deferred one to each
// caller. launchNode and the formation wait below are both fatal, and a fatal
// inside a bubble runtime.Goexit()s the bubble's root goroutine -- at which
// point a caller's defer has not been reached yet, because this function has not
// returned. The nodes already launched would still be running when the bubble
// ends, which synctest reports as a deadlock panic: it kills the whole test
// binary and buries the diagnostic those two were trying to print. Cleanups run
// on the Goexit and, in a bubble, run inside it -- synctest.Test invokes the
// property through tRunner, and rapid.SyncTest calls t.cleanup() from a defer
// within its bubble -- so the teardown can still use the fake clock.
//
// conf is used as a template; the per-node identity and address fields are
// filled in here. If conf.tableEventObserver is set it is shared by every node,
// so an observer must be safe for concurrent use.
//
// Log output is capped at warn for the cluster's lifetime. TestMain runs the
// package at debug, which a socket-based test with a handful of nodes can
// afford; a bubble cannot. NetworkDB logs a line per peer pair at info, so
// cluster formation alone is O(n^2), and these tests build hundreds of clusters
// of up to 25 nodes. Left at debug they bury a failing subtest under tens of
// thousands of lines from the ones that passed.
func newMemCluster(t TestingT, num int, namePrefix string, conf *Config) *memCluster {
	t.Helper()

	if logLevel := log.GetLevel(); logLevel > log.WarnLevel {
		_ = log.SetLevel(log.WarnLevel)
		t.Cleanup(func() { _ = log.SetLevel(logLevel) })
	}

	c := &memCluster{mn: &memNetwork{}}

	t.Cleanup(func() {
		closeNetworkDBInstances(t, c.live())
		// [NetworkDB.Close] stops the cluster but does not join
		// everything it stops: some goroutines are parked in a bounded
		// wait -- a broadcast ack (5s), a probe RTT, a stream timeout
		// (10s) -- which has to lapse before they notice the shutdown
		// and return. The bubble ends the moment the test function
		// returns and reports any goroutine still running as a
		// deadlock, so the budget here has to exceed the longest such
		// wait. It is spent from the fake clock, so a generous one is
		// free: nothing is runnable, and the sleep returns at once.
		time.Sleep(shutdownDrain)
		synctest.Wait()
	})

	// Configs first, and sequentially: memNetwork hands out addresses from a
	// counter, so allocating them concurrently would leave a node's address
	// decided by the scheduler and stop a recorded failure from replaying.
	configs := make([]Config, num)
	for i := range num {
		tr := c.mn.NewTransport()
		addr := tr.AddrPort()

		localConfig := *conf
		localConfig.Hostname = fmt.Sprintf("%s%d", namePrefix, i+1)
		// Derived from the index, not stringid.GenerateRandomID, which reads
		// crypto/rand: identical clusters across runs make a failure dump
		// diffable against the next run's, and stop node identity from being
		// one more thing a replay cannot reproduce. Twelve characters, matching
		// what TruncateID would have produced, and never all-digits.
		localConfig.NodeID = fmt.Sprintf("node%08d", i+1)
		if conf.rngSeed != nil {
			// One stream per node, all derived from the template's seed, so a
			// single recorded value reproduces the whole cluster's gossip
			// without every node making identical choices.
			seed := *conf.rngSeed
			binary.LittleEndian.PutUint64(seed[24:], uint64(i))
			localConfig.rngSeed = &seed
		}
		localConfig.transport = tr
		localConfig.BindAddr = addr.Addr().String()
		localConfig.AdvertiseAddr = localConfig.BindAddr
		localConfig.BindPort = int(addr.Port())

		configs[i] = localConfig
	}

	// Then bring every node up from its own goroutine, all at one virtual
	// instant. Creating them together is deliberate, and here is the only place
	// it can be decided: Create calls memberlist's schedule(), which builds the
	// probe and gossip tickers, and clusterInit starts NetworkDB's own triggers,
	// so the instant a node is created fixes the phase of every periodic task it
	// will ever run, and nothing afterwards moves it -- memberlist's randStagger
	// cannot, since schedule() builds the ticker before the goroutine which reads
	// it, and a stagger shorter than the interval never even skips a tick.
	//
	// Launching one at a time left those phases to whatever each Join happened to
	// cost, which came out a multiple of the gossip interval: they landed in
	// phase by accident rather than by intent, and would have drifted the moment
	// a join got faster or slower.
	dbs := make([]*NetworkDB, num)
	errs := make([]error, num)
	var wg sync.WaitGroup
	for i := range num {
		wg.Go(func() {
			dbs[i], errs[i] = New(&configs[i])
		})
	}
	wg.Wait()
	// Register whatever came up before reporting any failure, so the cleanup
	// above closes it either way. The reporting happens here and not inside the
	// goroutines because t.Fatal has to run on the test's own goroutine.
	for _, db := range dbs {
		if db != nil {
			c.dbs = append(c.dbs, db)
		}
	}
	for _, err := range errs {
		assert.NilError(t, err)
	}

	for i, db := range c.dbs {
		if i > 0 {
			// Seed the newcomer with every node already up, not just the
			// previous one. Join push/pulls with each seed synchronously, so
			// all of them learn the newcomer before Join returns and formation
			// never waits on an epidemic to spread it. That is not the same as
			// costing no clock -- a join spends a few hundred milliseconds of
			// it, and forming nineteen nodes some seconds in total -- but what
			// it costs does not turn on a race.
			//
			// Chain-joining left the newcomer known only to its one seed, and
			// its spread to everyone else rode memberlist's alive broadcast:
			// a one-shot epidemic whose budget is
			// RetransmitMult*ceil(log10(N+1)) transmissions, spent one per
			// recipient, so gossiping to GossipNodes peers costs that many at
			// once. At N=9 the budget is 4 against 8 peers -- the worst ratio
			// in the range these tests use, since the ceil(log10) step to 8
			// lands at N=10 -- and roughly 10% of formations lost the race and
			// fell back to memberlist's push/pull anti-entropy: one random
			// peer per node per 30s, several rounds to spread from a single
			// origin, 15-60s of virtual time. That tail sat right on
			// formationTimeout, so about 1 run in 171 timed out here.
			seeds := make([]string, 0, i)
			for _, prev := range c.dbs[:i] {
				seeds = append(seeds, addrOf(prev))
			}
			assert.Check(t, db.Join(seeds))
		}
	}

	// Formation is synchronous now, so this should pass on its first look. It
	// stays as a guard: if a future change makes membership depend on gossip
	// again, this reports it here rather than as a puzzling failure later.
	incomplete := func(db *NetworkDB) bool { return len(db.ClusterPeers()) != len(c.dbs) }
	start := time.Now()
	for {
		// Wait before looking, so a check never races the gossip it is
		// waiting on, and so the cluster is always quiesced on return.
		synctest.Wait()
		if !slices.ContainsFunc(c.dbs, incomplete) {
			break
		}
		if time.Since(start) >= formationTimeout {
			var b strings.Builder
			for _, db := range c.dbs {
				fmt.Fprintf(&b, "  %s (%s): %d of %d peers\n",
					db.config.Hostname, db.config.NodeID, len(db.ClusterPeers()), len(c.dbs))
			}
			t.Fatalf("cluster of %d did not form within %v of virtual time:\n%s", len(c.dbs), formationTimeout, b.String())
		}
		time.Sleep(formationStep)
	}
	return c
}

// runInCluster runs fn against a freshly formed cluster of num nodes inside a
// [testing/synctest] bubble, skipping the test on toolchains where a bubble
// would take the test binary with it.
//
// Every bubbled cluster test goes through here rather than opening its own
// bubble, so that the skip cannot be forgotten and so that "which tests are
// bubbled" is answerable by grep.
func runInCluster(t *testing.T, num int, conf *Config, fn func(t *testing.T, c *memCluster)) {
	t.Helper()
	runBubbled(t, func(t *testing.T) {
		fn(t, newMemCluster(t, num, "node", conf))
	})
}

// runBubbled runs fn inside a [testing/synctest] bubble, skipping the test on
// toolchains where a bubble would take the test binary with it. It is the entry
// point for bubbled tests which drive a NetworkDB directly rather than through
// a cluster.
func runBubbled(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	requireSynctest(t)
	synctest.Test(t, fn)
}

// relaunch brings node i back up at the address it was using before, with a
// fresh node ID, and points c.dbs[i] at the new instance so that the teardown
// closes the replacement rather than the corpse.
//
// The caller closes the old node first: going away is usually the point of the
// scenario, and how it goes -- a clean leave or a silent failure -- decides
// what its peers are expected to conclude.
//
// Reusing the address is the whole reason this exists. To its peers the
// returned node is the endpoint they still hold in their member lists paired
// with a NetworkDB identity they have never seen, which is what a restarted
// daemon looks like from the outside.
//
// The returned node does no join and is given no bootstrap peers, so it cannot
// reach out on its own: its peers have to find it.
func (c *memCluster) relaunch(t TestingT, i int) *NetworkDB {
	t.Helper()

	conf := *c.dbs[i].config
	// A new identity at the old address, derived rather than random for the
	// reason newMemCluster gives: a replay should get the same node IDs. The
	// counter distinguishes repeated relaunches of the same slot.
	c.relaunches++
	conf.NodeID = fmt.Sprintf("redo%08d", c.relaunches)
	conf.BootstrapPeers = nil
	conf.transport = c.mn.NewTransportAt(addrPortOf(t, c.dbs[i]))
	c.dbs[i] = launchNode(t, conf)
	return c.dbs[i]
}

// launch brings up one more node at an address the cluster has never seen,
// and adds it to c.dbs so that the teardown closes it. It joins nothing;
// everything else about it comes from conf.
func (c *memCluster) launch(t TestingT, conf Config) *NetworkDB {
	t.Helper()

	tr := c.mn.NewTransport()
	addr := tr.AddrPort()
	conf.transport = tr
	conf.BindAddr = addr.Addr().String()
	conf.AdvertiseAddr = conf.BindAddr
	conf.BindPort = int(addr.Port())
	db := launchNode(t, conf)
	c.dbs = append(c.dbs, db)
	return db
}

// addrPortOf is [addrOf] in the form a memNetwork indexes its nodes by.
func addrPortOf(t TestingT, db *NetworkDB) netip.AddrPort {
	t.Helper()
	ap, err := netip.ParseAddrPort(addrOf(db))
	assert.NilError(t, err)
	return ap
}

// waitFor polls cond until it holds, letting the bubble's clock run in between,
// and reports whether it held within budget of virtual time.
//
// Every round begins with a [synctest.Wait], so cond never races the gossip it
// is waiting on and always runs against a quiesced cluster. That is also why a
// false return is trustworthy rather than merely impatient: the cluster had
// nothing left to do at the moment of each check.
func waitFor(budget time.Duration, cond func() bool) bool {
	start := time.Now()
	for {
		synctest.Wait()
		if cond() {
			return true
		}
		if time.Since(start) >= budget {
			return false
		}
		time.Sleep(propagationStep)
	}
}

// verifyNodeExistence waits for nDB to agree that node is, or is not, a peer.
func (nDB *NetworkDB) verifyNodeExistence(t *testing.T, node string, present bool) {
	t.Helper()

	if waitFor(propagationTimeout, func() bool {
		nDB.RLock()
		defer nDB.RUnlock()
		_, ok := nDB.nodes[node]
		return ok == present
	}) {
		return
	}
	t.Errorf("%v(%v): expected node %s existence in the cluster = %v, got %v",
		nDB.config.Hostname, nDB.config.NodeID, node, present, !present)
}

// verifyNetworkExistence waits for nDB to agree that node is, or is not, a
// member of network id. A node on its way out counts as a non-member.
func (nDB *NetworkDB) verifyNetworkExistence(t *testing.T, node string, id string, present bool) {
	t.Helper()

	var ok, leaving bool
	if waitFor(propagationTimeout, func() bool {
		nDB.RLock()
		defer nDB.RUnlock()
		var vn *network
		if node == nDB.config.NodeID {
			if n, found := nDB.thisNodeNetworks[id]; found {
				vn = &n.network
			}
		} else if nn, found := nDB.networks[node]; found {
			vn = nn[id]
		}
		ok = vn != nil
		leaving = ok && vn.leaving
		return present == (ok && !leaving)
	}) {
		return
	}

	if present {
		t.Errorf("%v(%v): want node %v to be a member of network %q, got that it is not a member (ok=%v, leaving=%v)",
			nDB.config.Hostname, nDB.config.NodeID, node, id, ok, leaving)
	} else {
		t.Errorf("%v(%v): want node %v to not be a member of network %q, got that it is a member (ok=%v, leaving=%v)",
			nDB.config.Hostname, nDB.config.NodeID, node, id, ok, leaving)
	}
}

// verifyEntryExistence waits for nDB to hold the given value for an entry, or
// to not hold the entry at all.
func (nDB *NetworkDB) verifyEntryExistence(t *testing.T, tname, nid, key, value string, present bool) {
	t.Helper()

	var v []byte
	var unexpected error
	if waitFor(propagationTimeout, func() bool {
		var err error
		v, err = nDB.GetEntry(tname, nid, key)
		if err != nil && !cerrdefs.IsNotFound(err) {
			// Not something waiting will fix; remember it and stop
			// spending the budget on it.
			unexpected = err
			return true
		}
		if present {
			return err == nil && string(v) == value
		}
		return cerrdefs.IsNotFound(err)
	}) && unexpected == nil {
		return
	}

	if unexpected != nil {
		t.Errorf("%v(%v): unexpected error while getting entry %v/%v in network %q: %v",
			nDB.config.Hostname, nDB.config.NodeID, tname, key, nid, unexpected)
		return
	}
	t.Errorf("%v(%v): want entry %v/%v in network %q to be (present=%v, value=%q), got (present=%v, value=%q)",
		nDB.config.Hostname, nDB.config.NodeID, tname, key, nid, present, value, !present, string(v))
}

// testWatch waits for one watch event and checks it describes the expected
// transition. The budget is virtual, so it is generous rather than tuned: the
// clock only advances once every node has run out of work, which makes the
// timeout an assertion that the event is never coming rather than a guess about
// how long it ought to take.
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
	case <-time.After(watchTimeout):
		t.Errorf("no watch event for %s/%s/%s within %v of virtual time", tname, nid, key, watchTimeout)
	}
}

// dumpNodeState renders what each node believes about its peers, for the
// failure message of a wait which timed out on membership.
func dumpNodeState(dbs []*NetworkDB) string {
	var b strings.Builder
	for _, db := range dbs {
		db.RLock()
		fmt.Fprintf(&b, "  %s (%s): %d active %v, %d failed %v\n",
			db.config.Hostname, db.config.NodeID,
			len(db.nodes), slices.Sorted(maps.Keys(db.nodes)),
			len(db.failedNodes), slices.Sorted(maps.Keys(db.failedNodes)))
		db.RUnlock()
	}
	return b.String()
}

// memNetwork is the in-memory stand-in for the UDP + TCP network that memberlist
// nodes gossip over, built from channels and [net.Pipe] for the reason given at
// the top of this file.
//
// memberlist ships its own [memberlist.MockTransport], which is not usable here
// for three reasons that this type fixes:
//
//   - Its sends block until the peer's handler picks the message up, so a cycle
//     of nodes gossiping at each other can deadlock. Real UDP never blocks the
//     sender, it drops; so does this.
//   - Its Shutdown is a no-op, leaving a closed node reachable. Delivering to a
//     channel nobody will ever read again is a goroutine leak outside a bubble
//     and a hard deadlock inside one.
//   - Its address maps are read and written without synchronization.
type memNetwork struct {
	mu    sync.Mutex
	nodes map[netip.AddrPort]*memTransport
	all   []*memTransport // every transport ever created, for accounting
	next  uint32          // host part of the next address to hand out
}

// memNetworkPort is the port every node on a memNetwork listens on. Nodes are
// distinguished by address, one fictional host each, as they would be in a real
// cluster.
const memNetworkPort = 7946

// packet and stream channel depths. Both are far larger than the number of
// messages a healthy cluster has in flight at once, so the drop path below is
// only reached if something is genuinely wedged.
const (
	memPacketBuffer = 512
	memStreamBuffer = 64
)

// NewTransport returns a transport with a freshly allocated address, wired up to
// every other transport on the same memNetwork.
func (mn *memNetwork) NewTransport() *memTransport {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	mn.next++
	if mn.next > 0xfffe {
		panic("memnet: out of addresses")
	}
	return mn.bind(netip.AddrPortFrom(netip.AddrFrom4([4]byte{10, 0, byte(mn.next >> 8), byte(mn.next)}), memNetworkPort))
}

// NewTransportAt returns a transport bound to addr, which must not currently be
// in use. It exists for the node which goes away and comes back at the address
// it had before -- a different process, and to NetworkDB a different node ID,
// but the same endpoint its peers still hold in their member lists. Allocating
// a fresh address instead would make that a new node joining rather than an old
// one reincarnating, which is the distinction under test.
func (mn *memNetwork) NewTransportAt(addr netip.AddrPort) *memTransport {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	if _, taken := mn.nodes[addr]; taken {
		panic("memnet: address already in use: " + addr.String())
	}
	return mn.bind(addr)
}

// bind creates a transport listening on addr and registers it. mn.mu must be
// held.
func (mn *memNetwork) bind(addr netip.AddrPort) *memTransport {
	t := &memTransport{
		mn:       mn,
		addr:     addr,
		packetCh: make(chan *memberlist.Packet, memPacketBuffer),
		streamCh: make(chan net.Conn, memStreamBuffer),
		shutdown: make(chan struct{}),
	}
	if mn.nodes == nil {
		mn.nodes = make(map[netip.AddrPort]*memTransport)
	}
	mn.nodes[t.addr] = t
	mn.all = append(mn.all, t)
	return t
}

func (mn *memNetwork) lookup(addr string) *memTransport {
	ap, err := netip.ParseAddrPort(addr)
	if err != nil {
		return nil
	}
	mn.mu.Lock()
	defer mn.mu.Unlock()
	return mn.nodes[ap]
}

func (mn *memNetwork) deregister(addr netip.AddrPort) {
	mn.mu.Lock()
	defer mn.mu.Unlock()
	delete(mn.nodes, addr)
}

// memAddr adapts a [netip.AddrPort] to [net.Addr]. Its String form is the usual
// "ip:port", so anything which parses a peer address off the wire still works.
type memAddr netip.AddrPort

func (a memAddr) Network() string { return "memnet" }
func (a memAddr) String() string  { return netip.AddrPort(a).String() }

// memTransport is one node's attachment to a memNetwork.
type memTransport struct {
	mn       *memNetwork
	addr     netip.AddrPort
	packetCh chan *memberlist.Packet
	streamCh chan net.Conn

	shutdown  chan struct{}
	closeOnce sync.Once

	dropped atomic.Int64
}

var _ memberlist.NodeAwareTransport = (*memTransport)(nil)

// AddrPort returns the address this transport is reachable at.
func (t *memTransport) AddrPort() netip.AddrPort { return t.addr }

func (t *memTransport) FinalAdvertiseAddr(string, int) (net.IP, int, error) {
	return net.IP(t.addr.Addr().AsSlice()), int(t.addr.Port()), nil
}

func (t *memTransport) WriteTo(b []byte, addr string) (time.Time, error) {
	return t.WriteToAddress(b, memberlist.Address{Addr: addr})
}

func (t *memTransport) WriteToAddress(b []byte, a memberlist.Address) (time.Time, error) {
	now := time.Now()

	dst := t.mn.lookup(a.Addr)
	if dst == nil {
		// A datagram addressed to a node which is not listening is
		// silently discarded, with nothing reported to the sender. Do
		// not return an error here: memberlist would treat it as a
		// send failure, which a real UDP socket would never observe.
		return now, nil
	}
	select {
	case <-dst.shutdown:
		return now, nil
	case <-t.shutdown:
		// This node's own transport is down. memberlist shuts the transport
		// down before it stops its own handlers, so a send already in flight
		// can reach here afterwards; the real transport has closed its UDP
		// sockets by then and the write fails. Fail here too, or a closed node
		// goes on originating fresh probes against live peers whose acks can
		// no longer reach it.
		//
		// This does not silence teardown's "UDP probes failed, network may be
		// misconfigured" warnings, and should not: a real node logs them too,
		// for probes already awaiting an ack when Shutdown runs.
		// NetTransport.Shutdown closes listeners only, and its
		// DialAddressTimeout is a bare dialer.Dial, so the UDP ack is lost
		// while the TCP fallback still connects. What this removes is the
		// probes a departed node would otherwise keep starting, which a real
		// one cannot.
		//
		// Deliberately not a *net.OpError: memberlist's failedRemote treats a
		// udp/write OpError as the *peer* having failed and runs the indirect
		// probe and suspicion machinery, which is the opposite of what a node
		// on its way out should provoke. A plain error is a local failure, so
		// probeNode logs and returns.
		return now, fmt.Errorf("memnet: write on shut-down transport %s", t.addr)
	default:
	}

	// memberlist pools the buffers it hands us, and a broadcast passes the
	// same buffer to every recipient, so the receiver needs its own copy.
	pkt := &memberlist.Packet{
		Buf:       bytes.Clone(b),
		From:      memAddr(t.addr),
		Timestamp: now,
	}
	select {
	case dst.packetCh <- pkt:
	default:
		dst.dropped.Add(1)
		// Receive buffer full. Drop it, as a kernel socket buffer
		// would, rather than blocking the sender and risking a cycle of
		// nodes each waiting on the other.
	}
	return now, nil
}

func (t *memTransport) PacketCh() <-chan *memberlist.Packet { return t.packetCh }

func (t *memTransport) DialTimeout(addr string, timeout time.Duration) (net.Conn, error) {
	return t.DialAddressTimeout(memberlist.Address{Addr: addr}, timeout)
}

func (t *memTransport) DialAddressTimeout(a memberlist.Address, timeout time.Duration) (net.Conn, error) {
	dst := t.mn.lookup(a.Addr)
	if dst == nil {
		return nil, fmt.Errorf("memnet: dial %s: connection refused", a.String())
	}

	local, remote := net.Pipe()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case dst.streamCh <- remote:
		// The peer may have shut down between the lookup above and this send.
		// Both this case and the one below are then ready at once and the
		// select picks at random, so half the time the conn is queued anyway --
		// into a channel whose one-shot drain has already run and whose
		// streamListen has already returned. Nothing would ever read or close
		// it, and the caller is memberlist's sendUserMsg, the one stream path
		// which sets no deadline, so its write parks forever and the bubble
		// deadlocks. Re-check and hand back a failure rather than a conn nobody
		// is listening on. If shutdown instead begins just after this check,
		// its drain still runs after ours was queued and closes it.
		select {
		case <-dst.shutdown:
		default:
			return local, nil
		}
	case <-dst.shutdown:
	case <-t.shutdown:
	case <-timer.C:
	}
	_ = local.Close()
	_ = remote.Close()
	return nil, fmt.Errorf("memnet: dial %s: connection refused or timed out", a.String())
}

func (t *memTransport) StreamCh() <-chan net.Conn { return t.streamCh }

func (t *memTransport) Shutdown() error {
	t.closeOnce.Do(func() {
		// Stop accepting first, so no peer can queue anything new, then
		// hang up on whatever is already queued but not yet accepted.
		// Otherwise the dialer at the far end of the pipe stays blocked
		// on a write that will never be read -- a deadlocked bubble.
		t.mn.deregister(t.addr)
		close(t.shutdown)
		for {
			select {
			case c := <-t.streamCh:
				_ = c.Close()
			default:
				return
			}
		}
	})
	return nil
}

// dropCount reports how many datagrams this network discarded for want of
// receive buffer, across every node, including nodes since shut down.
//
// A healthy run drops nothing. A non-zero count means memPacketBuffer was too
// shallow for the load, and any convergence time measured over that run has
// been inflated by retransmits that a real cluster would not have needed --
// which is worth knowing, because it is otherwise indistinguishable from
// NetworkDB genuinely falling back to its anti-entropy sweep.
func (mn *memNetwork) dropCount() int64 {
	mn.mu.Lock()
	defer mn.mu.Unlock()
	var n int64
	for _, t := range mn.all {
		n += t.dropped.Load()
	}
	return n
}

// resetDropCount zeroes the tally, so that a measurement can assert on drops
// which happened during the measurement rather than on every drop since the
// cluster was created. Cluster formation and the pre-measurement settle are
// the noisiest parts of a run and cannot contaminate a sample taken after
// settle has seen every node agree.
func (mn *memNetwork) resetDropCount() {
	mn.mu.Lock()
	defer mn.mu.Unlock()
	for _, t := range mn.all {
		t.dropped.Store(0)
	}
}
