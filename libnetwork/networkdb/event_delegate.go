package networkdb

import (
	"encoding/json"
	"net"

	"github.com/hashicorp/memberlist"
	"github.com/sirupsen/logrus"
)

type eventDelegate struct {
	nDB *NetworkDB
}

func (e *eventDelegate) broadcastNodeEvent(addr net.IP, op opType) {
	value, err := json.Marshal(&NodeAddr{addr})
	if err == nil {
		e.nDB.broadcaster.Write(makeEvent(op, NodeTable, "", "", value))
	} else {
		logrus.Errorf("Error marshalling node broadcast event %s", addr.String())
	}
}

func (e *eventDelegate) purgeReincarnation(mn *memberlist.Node) {
	for name, node := range e.nDB.failedNodes {
		if node.Addr.Equal(mn.Addr) {
			logrus.Infof("Node %s/%s, is the new incarnation of the failed node %s/%s", mn.Name, mn.Addr, name, node.Addr)
			delete(e.nDB.failedNodes, name)
			return
		}
	}

	for name, node := range e.nDB.leftNodes {
		if node.Addr.Equal(mn.Addr) {
			logrus.Infof("Node %s/%s, is the new incarnation of the shutdown node %s/%s", mn.Name, mn.Addr, name, node.Addr)
			delete(e.nDB.leftNodes, name)
			return
		}
	}
}

func (e *eventDelegate) NotifyJoin(mn *memberlist.Node) {
	logrus.Infof("Node %s/%s, joined gossip cluster", mn.Name, mn.Addr)
	e.broadcastNodeEvent(mn.Addr, opCreate)
	e.nDB.Lock()
	defer e.nDB.Unlock()
	// In case the node is rejoining after a failure or leave,
	// wait until an explicit join message arrives before adding
	// it to the nodes just to make sure this is not a stale
	// join. If you don't know about this node add it immediately.
	_, fOk := e.nDB.failedNodes[mn.Name]
	_, lOk := e.nDB.leftNodes[mn.Name]
	if fOk || lOk {
		return
	}

	// Every node has a unique ID
	// Check on the base of the IP address if the new node that joined is actually a new incarnation of a previous
	// failed or shutdown one
	e.purgeReincarnation(mn)

	e.nDB.nodes[mn.Name] = &node{Node: *mn}
	logrus.Infof("Node %s/%s, added to nodes list", mn.Name, mn.Addr)
}

func (e *eventDelegate) NotifyLeave(mn *memberlist.Node) {
	var failed bool
	logrus.Infof("Node %s/%s, left gossip cluster", mn.Name, mn.Addr)
	e.broadcastNodeEvent(mn.Addr, opDelete)
	// The node left or failed, delete all the entries created by it.
	// If the node was temporary down, deleting the entries will guarantee that the CREATE events will be accepted
	// If the node instead left because was going down, then it makes sense to just delete all its state
	e.nDB.Lock()
	defer e.nDB.Unlock()
	e.nDB.deleteNodeFromNetworks(mn.Name)
	e.nDB.deleteNodeTableEntries(mn.Name)
	if n, ok := e.nDB.nodes[mn.Name]; ok {
		delete(e.nDB.nodes, mn.Name)

		// Check if a new incarnation of the same node already joined
		// In that case this node can simply be removed and no further action are needed
		for name, node := range e.nDB.nodes {
			if node.Addr.Equal(mn.Addr) {
				logrus.Infof("Node %s/%s, is the new incarnation of the failed node %s/%s", name, node.Addr, mn.Name, mn.Addr)
				return
			}
		}

		// In case of node failure, keep retrying to reconnect every retryInterval (1sec) for nodeReapInterval (24h)
		// Explicit leave will have already removed the node from the list of nodes (nDB.nodes) and put it into the leftNodes map
		n.reapTime = nodeReapInterval
		e.nDB.failedNodes[mn.Name] = n
		failed = true
	}

	if failed {
		logrus.Infof("Node %s/%s, added to failed nodes list", mn.Name, mn.Addr)
	}
}

func (e *eventDelegate) NotifyUpdate(n *memberlist.Node) {
}
