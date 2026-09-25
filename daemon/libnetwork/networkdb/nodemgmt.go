package networkdb

import (
	"context"
	"iter"

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

// purgeReincarnation retires a node which mn has taken the place of: one at
// the same address, under a name the cluster is no longer using.
//
// Only nodes memberlist has already given up on are candidates. An address
// collision on its own does not say which of the two names is the current one,
// and the answer is not ours to guess: a node still in the active list is one
// memberlist last told us was alive, and gossip about a departed incarnation
// can arrive after its replacement is already known. Retiring the active node
// on the strength of the address would evict a live peer -- deleting its
// entries and its network attachments -- and nothing would bring it back,
// since memberlist does not re-announce a node it still believes in.
//
// So leave the active list alone and let memberlist settle it. It probes by
// name and a node refuses to ack a ping addressed to someone else, so whichever
// of the two is gone stops acking within a probe cycle or two and is reported
// to us as having left. [eventDelegate.NotifyLeave] retires it then, once the
// question has an answer.
func (nDB *NetworkDB) purgeReincarnation(mn *memberlist.Node) int {
	var purged int
	for name, node := range nDB.failedNodes.AllColliding(mn) {
		log.G(context.TODO()).Infof("Node %s/%s, is the new incarnation of the failed node %s/%s", mn.Name, mn.Addr, name, node.Addr)
		if nDB.forgetNode(context.TODO(), name) {
			purged++
		}
	}

	return purged
}

func (nDB *NetworkDB) estNumNodes() int {
	return int(nDB.estNodes.Load())
}

type nodeMap map[string]*node

// AllColliding yields all nodes in the map that have the same address and port,
// but a different name from, the given node.
func (nm nodeMap) AllColliding(needle *memberlist.Node) iter.Seq2[string, *node] {
	return func(yield func(string, *node) bool) {
		for name, n := range nm {
			if n.Addr.Equal(needle.Addr) && n.Port == needle.Port && needle.Name != name {
				if !yield(name, n) {
					return
				}
			}
		}
	}
}
