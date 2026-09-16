package networkdb

import (
	"context"

	"github.com/containerd/log"
	"github.com/hashicorp/memberlist"
)

// reactivateNode moves a node from the failed state back to the active state.
// Caller should hold the NetworkDB lock while calling this.
func (nDB *NetworkDB) reactivateNode(ctx context.Context, nodeName string) bool {
	n, ok := nDB.failedNodes[nodeName]
	if !ok {
		return false
	}
	delete(nDB.failedNodes, nodeName)
	// reset the node reap time
	n.reapTime = 0
	nDB.nodes[nodeName] = n
	nDB.estNodes.Store(int32(len(nDB.nodes)))
	// The node is reachable again, so put it back in the peer list of
	// the networks it was attached to when it failed, and make the
	// entries remembered across the failure visible again.
	nDB.restoreNodeInNetworks(n.Name)
	nDB.restoreNodeTableEntries(n.Name)

	log.G(ctx).Infof("Node %s change state NodeFailed --> NodeActive", nodeName)
	return true
}

// forgetNode handles the transition of a node leaving the NetworkDB cluster.
// Caller should hold the NetworkDB lock while calling this.
func (nDB *NetworkDB) forgetNode(ctx context.Context, nodeName string) bool {
	var m map[string]*node
	var oldState string
	n, active := nDB.nodes[nodeName]
	if active {
		m = nDB.nodes
		oldState = "NodeActive"
	} else {
		var ok bool
		n, ok = nDB.failedNodes[nodeName]
		if !ok {
			return false
		}
		m = nDB.failedNodes
		oldState = "NodeFailed"
	}

	delete(m, nodeName)
	if active {
		nDB.estNodes.Store(int32(len(nDB.nodes)))
	}
	// The node is gone for good: delete all the entries created by
	// it along with the record of which networks it was attached to.
	deletedEvents := nDB.deleteNodeTableEntries(n.Name)
	if active {
		// The entries were still visible: tell the watchers they
		// are gone. Had the node already failed, the watchers were
		// told when it failed.
		for _, ev := range deletedEvents {
			nDB.broadcaster.Write(ev)
		}
	}
	nDB.deleteNodeFromNetworks(n.Name)

	log.G(ctx).Infof("Node %s change state %s --> NodeLeft", nodeName, oldState)
	return true
}

// failNode handles the transition of an active node to the failed state.
// Caller should hold the NetworkDB lock while calling this.
func (nDB *NetworkDB) failNode(ctx context.Context, nodeName string) bool {
	n, active := nDB.nodes[nodeName]
	if !active {
		return false
	}
	delete(nDB.nodes, nodeName)
	nDB.estNodes.Store(int32(len(nDB.nodes)))
	nDB.failedNodes[nodeName] = n

	// set the node reap time, if not already set
	if n.reapTime == 0 {
		n.reapTime = nodeReapInterval
	}
	// The node may only be temporarily down. Hide its entries and
	// remember its attachments, so the freshest state possible can
	// be put straight back if it returns. What is remembered ages
	// out on the entry reap timer and is dropped by reapDeadNode.
	nDB.suspendNodeTableEntries(n.Name)
	nDB.suspendNodeInNetworks(n.Name)

	log.G(ctx).Infof("Node %s change state NodeActive --> NodeFailed", nodeName)
	return true
}

func (nDB *NetworkDB) purgeReincarnation(mn *memberlist.Node) bool {
	for name, node := range nDB.nodes {
		if node.Addr.Equal(mn.Addr) && node.Port == mn.Port && mn.Name != name {
			log.G(context.TODO()).Infof("Node %s/%s, is the new incarnation of the active node %s/%s", mn.Name, mn.Addr, name, node.Addr)
			nDB.forgetNode(context.TODO(), name)
			return true
		}
	}

	for name, node := range nDB.failedNodes {
		if node.Addr.Equal(mn.Addr) && node.Port == mn.Port && mn.Name != name {
			log.G(context.TODO()).Infof("Node %s/%s, is the new incarnation of the failed node %s/%s", mn.Name, mn.Addr, name, node.Addr)
			nDB.forgetNode(context.TODO(), name)
			return true
		}
	}

	return false
}

func (nDB *NetworkDB) estNumNodes() int {
	return int(nDB.estNodes.Load())
}
