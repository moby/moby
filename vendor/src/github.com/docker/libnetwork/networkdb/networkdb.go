package networkdb

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/armon/go-radix"
	"github.com/docker/go-events"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

const (
	byTable int = 1 + iota
	byNetwork
)

// NetworkDB instance drives the networkdb cluster and acts the broker
// for cluster-scoped and network-scoped gossip and watches.
type NetworkDB struct {
	sync.RWMutex

	// NetworkDB configuration.
	config *Config

	// local copy of memberlist config that we use to driver
	// network scoped gossip and bulk sync.
	mConfig *memberlist.Config

	// All the tree index (byTable, byNetwork) that we maintain
	// the db.
	indexes map[int]*radix.Tree

	// Memberlist we use to drive the cluster.
	memberlist *memberlist.Memberlist

	// List of all peer nodes in the cluster not-limited to any
	// network.
	nodes map[string]*memberlist.Node

	// A multi-dimensional map of network/node attachmemts. The
	// first key is a node name and the second key is a network ID
	// for the network that node is participating in.
	networks map[string]map[string]*network

	// A map of nodes which are participating in a given
	// network. The key is a network ID.

	networkNodes map[string][]string

	// A table of ack channels for every node from which we are
	// waiting for an ack.
	bulkSyncAckTbl map[string]chan struct{}

	// Global lamport clock for node network attach events.
	networkClock serf.LamportClock

	// Global lamport clock for table events.
	tableClock serf.LamportClock

	// Broadcast queue for network event gossip.
	networkBroadcasts *memberlist.TransmitLimitedQueue

	// A central stop channel to stop all go routines running on
	// behalf of the NetworkDB instance.
	stopCh chan struct{}

	// A central broadcaster for all local watchers watching table
	// events.
	broadcaster *events.Broadcaster

	// List of all tickers which needed to be stopped when
	// cleaning up.
	tickers []*time.Ticker
}

// network describes the node/network attachment.
type network struct {
	// Network ID
	id string

	// Lamport time for the latest state of the entry.
	ltime serf.LamportTime

	// Node leave is in progress.
	leaving bool

	// The time this node knew about the node's network leave.
	leaveTime time.Time

	// The broadcast queue for table event gossip. This is only
	// initialized for this node's network attachment entries.
	tableBroadcasts *memberlist.TransmitLimitedQueue
}

// Config represents the configuration of the networdb instance and
// can be passed by the caller.
type Config struct {
	// NodeName is the cluster wide unique name for this node.
	NodeName string

	// BindAddr is the local node's IP address that we bind to for
	// cluster communication.
	BindAddr string

	// BindPort is the local node's port to which we bind to for
	// cluster communication.
	BindPort int
}

// entry defines a table entry
type entry struct {
	// node from which this entry was learned.
	node string

	// Lamport time for the most recent update to the entry
	ltime serf.LamportTime

	// Opaque value store in the entry
	value []byte

	// Deleting the entry is in progress. All entries linger in
	// the cluster for certain amount of time after deletion.
	deleting bool

	// The wall clock time when this node learned about this deletion.
	deleteTime time.Time
}

// New creates a new instance of NetworkDB using the Config passed by
// the caller.
func New(c *Config) (*NetworkDB, error) {
	nDB := &NetworkDB{
		config:         c,
		indexes:        make(map[int]*radix.Tree),
		networks:       make(map[string]map[string]*network),
		nodes:          make(map[string]*memberlist.Node),
		networkNodes:   make(map[string][]string),
		bulkSyncAckTbl: make(map[string]chan struct{}),
		broadcaster:    events.NewBroadcaster(),
	}

	nDB.indexes[byTable] = radix.New()
	nDB.indexes[byNetwork] = radix.New()

	if err := nDB.clusterInit(); err != nil {
		return nil, err
	}

	return nDB, nil
}

// Join joins this NetworkDB instance with a list of peer NetworkDB
// instances passed by the caller in the form of addr:port
func (nDB *NetworkDB) Join(members []string) error {
	return nDB.clusterJoin(members)
}

// Close destroys this NetworkDB instance by leave the cluster,
// stopping timers, canceling goroutines etc.
func (nDB *NetworkDB) Close() {
	if err := nDB.clusterLeave(); err != nil {
		logrus.Errorf("Could not close DB %s: %v", nDB.config.NodeName, err)
	}
}

// GetEntry retrieves the value of a table entry in a given (network,
// table, key) tuple
func (nDB *NetworkDB) GetEntry(tname, nid, key string) ([]byte, error) {
	entry, err := nDB.getEntry(tname, nid, key)
	if err != nil {
		return nil, err
	}

	return entry.value, nil
}

func (nDB *NetworkDB) getEntry(tname, nid, key string) (*entry, error) {
	nDB.RLock()
	defer nDB.RUnlock()

	e, ok := nDB.indexes[byTable].Get(fmt.Sprintf("/%s/%s/%s", tname, nid, key))
	if !ok {
		return nil, fmt.Errorf("could not get entry in table %s with network id %s and key %s", tname, nid, key)
	}

	return e.(*entry), nil
}

// CreateEntry creates a table entry in NetworkDB for given (network,
// table, key) tuple and if the NetworkDB is part of the cluster
// propogates this event to the cluster. It is an error to create an
// entry for the same tuple for which there is already an existing
// entry.
func (nDB *NetworkDB) CreateEntry(tname, nid, key string, value []byte) error {
	if _, err := nDB.GetEntry(tname, nid, key); err == nil {
		return fmt.Errorf("cannot create entry as the entry in table %s with network id %s and key %s already exists", tname, nid, key)
	}

	entry := &entry{
		ltime: nDB.tableClock.Increment(),
		node:  nDB.config.NodeName,
		value: value,
	}

	if err := nDB.sendTableEvent(tableEntryCreate, nid, tname, key, entry); err != nil {
		return fmt.Errorf("cannot send table create event: %v", err)
	}

	nDB.Lock()
	nDB.indexes[byTable].Insert(fmt.Sprintf("/%s/%s/%s", tname, nid, key), entry)
	nDB.indexes[byNetwork].Insert(fmt.Sprintf("/%s/%s/%s", nid, tname, key), entry)
	nDB.Unlock()

	nDB.broadcaster.Write(makeEvent(opCreate, tname, nid, key, value))
	return nil
}

// UpdateEntry updates a table entry in NetworkDB for given (network,
// table, key) tuple and if the NetworkDB is part of the cluster
// propogates this event to the cluster. It is an error to update a
// non-existent entry.
func (nDB *NetworkDB) UpdateEntry(tname, nid, key string, value []byte) error {
	if _, err := nDB.GetEntry(tname, nid, key); err != nil {
		return fmt.Errorf("cannot update entry as the entry in table %s with network id %s and key %s does not exist", tname, nid, key)
	}

	entry := &entry{
		ltime: nDB.tableClock.Increment(),
		node:  nDB.config.NodeName,
		value: value,
	}

	if err := nDB.sendTableEvent(tableEntryUpdate, nid, tname, key, entry); err != nil {
		return fmt.Errorf("cannot send table update event: %v", err)
	}

	nDB.Lock()
	nDB.indexes[byTable].Insert(fmt.Sprintf("/%s/%s/%s", tname, nid, key), entry)
	nDB.indexes[byNetwork].Insert(fmt.Sprintf("/%s/%s/%s", nid, tname, key), entry)
	nDB.Unlock()

	nDB.broadcaster.Write(makeEvent(opUpdate, tname, nid, key, value))
	return nil
}

// DeleteEntry deletes a table entry in NetworkDB for given (network,
// table, key) tuple and if the NetworkDB is part of the cluster
// propogates this event to the cluster.
func (nDB *NetworkDB) DeleteEntry(tname, nid, key string) error {
	value, err := nDB.GetEntry(tname, nid, key)
	if err != nil {
		return fmt.Errorf("cannot delete entry as the entry in table %s with network id %s and key %s does not exist", tname, nid, key)
	}

	entry := &entry{
		ltime:      nDB.tableClock.Increment(),
		node:       nDB.config.NodeName,
		value:      value,
		deleting:   true,
		deleteTime: time.Now(),
	}

	if err := nDB.sendTableEvent(tableEntryDelete, nid, tname, key, entry); err != nil {
		return fmt.Errorf("cannot send table delete event: %v", err)
	}

	nDB.Lock()
	nDB.indexes[byTable].Insert(fmt.Sprintf("/%s/%s/%s", tname, nid, key), entry)
	nDB.indexes[byNetwork].Insert(fmt.Sprintf("/%s/%s/%s", nid, tname, key), entry)
	nDB.Unlock()

	nDB.broadcaster.Write(makeEvent(opDelete, tname, nid, key, value))
	return nil
}

func (nDB *NetworkDB) deleteNodeTableEntries(node string) {
	nDB.Lock()
	nDB.indexes[byTable].Walk(func(path string, v interface{}) bool {
		oldEntry := v.(*entry)
		if oldEntry.node != node {
			return false
		}

		params := strings.Split(path[1:], "/")
		tname := params[0]
		nid := params[1]
		key := params[2]

		entry := &entry{
			ltime:      oldEntry.ltime,
			node:       node,
			value:      oldEntry.value,
			deleting:   true,
			deleteTime: time.Now(),
		}

		nDB.indexes[byTable].Insert(fmt.Sprintf("/%s/%s/%s", tname, nid, key), entry)
		nDB.indexes[byNetwork].Insert(fmt.Sprintf("/%s/%s/%s", nid, tname, key), entry)
		return false
	})
	nDB.Unlock()
}

// WalkTable walks a single table in NetworkDB and invokes the passed
// function for each entry in the table passing the network, key,
// value. The walk stops if the passed function returns a true.
func (nDB *NetworkDB) WalkTable(tname string, fn func(string, string, []byte) bool) error {
	nDB.RLock()
	values := make(map[string]interface{})
	nDB.indexes[byTable].WalkPrefix(fmt.Sprintf("/%s", tname), func(path string, v interface{}) bool {
		values[path] = v
		return false
	})
	nDB.RUnlock()

	for k, v := range values {
		params := strings.Split(k[1:], "/")
		nid := params[1]
		key := params[2]
		if fn(nid, key, v.(*entry).value) {
			return nil
		}
	}

	return nil
}

// JoinNetwork joins this node to a given network and propogates this
// event across the cluster. This triggers this node joining the
// sub-cluster of this network and participates in the network-scoped
// gossip and bulk sync for this network.
func (nDB *NetworkDB) JoinNetwork(nid string) error {
	ltime := nDB.networkClock.Increment()

	nDB.Lock()
	nodeNetworks, ok := nDB.networks[nDB.config.NodeName]
	if !ok {
		nodeNetworks = make(map[string]*network)
		nDB.networks[nDB.config.NodeName] = nodeNetworks
	}
	nodeNetworks[nid] = &network{id: nid, ltime: ltime}
	nodeNetworks[nid].tableBroadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return len(nDB.networkNodes[nid])
		},
		RetransmitMult: 4,
	}
	nDB.networkNodes[nid] = append(nDB.networkNodes[nid], nDB.config.NodeName)
	nDB.Unlock()

	if err := nDB.sendNetworkEvent(nid, networkJoin, ltime); err != nil {
		return fmt.Errorf("failed to send leave network event for %s: %v", nid, err)
	}

	logrus.Debugf("%s: joined network %s", nDB.config.NodeName, nid)
	if _, err := nDB.bulkSync(nid, true); err != nil {
		logrus.Errorf("Error bulk syncing while joining network %s: %v", nid, err)
	}

	return nil
}

// LeaveNetwork leaves this node from a given network and propogates
// this event across the cluster. This triggers this node leaving the
// sub-cluster of this network and as a result will no longer
// participate in the network-scoped gossip and bulk sync for this
// network.
func (nDB *NetworkDB) LeaveNetwork(nid string) error {
	ltime := nDB.networkClock.Increment()
	if err := nDB.sendNetworkEvent(nid, networkLeave, ltime); err != nil {
		return fmt.Errorf("failed to send leave network event for %s: %v", nid, err)
	}

	nDB.Lock()
	defer nDB.Unlock()
	nodeNetworks, ok := nDB.networks[nDB.config.NodeName]
	if !ok {
		return fmt.Errorf("could not find self node for network %s while trying to leave", nid)
	}

	n, ok := nodeNetworks[nid]
	if !ok {
		return fmt.Errorf("could not find network %s while trying to leave", nid)
	}

	n.ltime = ltime
	n.leaving = true
	return nil
}

// Deletes the node from the list of nodes which participate in the
// passed network. Caller should hold the NetworkDB lock while calling
// this
func (nDB *NetworkDB) deleteNetworkNode(nid string, nodeName string) {
	nodes := nDB.networkNodes[nid]
	for i, name := range nodes {
		if name == nodeName {
			nodes[i] = nodes[len(nodes)-1]
			nodes = nodes[:len(nodes)-1]
			break
		}
	}
	nDB.networkNodes[nid] = nodes
}

// findCommonnetworks find the networks that both this node and the
// passed node have joined.
func (nDB *NetworkDB) findCommonNetworks(nodeName string) []string {
	nDB.RLock()
	defer nDB.RUnlock()

	var networks []string
	for nid := range nDB.networks[nDB.config.NodeName] {
		if _, ok := nDB.networks[nodeName][nid]; ok {
			networks = append(networks, nid)
		}
	}

	return networks
}
