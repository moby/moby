package networkdb

import (
	"context"
	"fmt"

	"github.com/containerd/log"
	"github.com/hashicorp/memberlist"
)

type nodeState int

const (
	nodeNotFound    nodeState = -1
	nodeActiveState nodeState = 0
	nodeFailedState nodeState = 1
	nodeLeftState   nodeState = 2
)

var nodeStateName = map[nodeState]string{
	-1: "NodeNotFound",
	0:  "NodeActive",
	1:  "NodeFailed",
	2:  "NodeLeft",
}

// findNode search the node into the node lists and returns the node pointer and the list
// where it got found
func (nDB *NetworkDB) findNode(nodeName string) (*node, nodeState, map[string]*node) {
	for i, nodes := range []map[string]*node{
		nDB.nodes,
		nDB.failedNodes,
	} {
		if n, ok := nodes[nodeName]; ok {
			return n, nodeState(i), nodes
		}
	}
	return nil, nodeNotFound, nil
}

// changeNodeState changes the state of the node specified. It returns true if the
// node's state was changed. An error will be returned if the node does not
// exist.
func (nDB *NetworkDB) changeNodeState(nodeName string, newState nodeState) (bool, error) {
	n, currState, m := nDB.findNode(nodeName)
	if n == nil {
		return false, fmt.Errorf("node %s not found", nodeName)
	}

	switch newState {
	case nodeActiveState:
		if currState == nodeActiveState {
			return false, nil
		}

		delete(m, nodeName)
		// reset the node reap time
		n.reapTime = 0
		nDB.nodes[nodeName] = n
		// The node is reachable again, so put it back in the peer list of
		// the networks it was attached to when it failed, and make the
		// entries remembered across the failure visible again.
		nDB.restoreNodeInNetworks(n.Name)
		nDB.restoreNodeTableEntries(n.Name)
	case nodeLeftState:
		delete(m, nodeName)
		// The node is gone for good: delete all the entries created by
		// it along with the record of which networks it was attached to.
		deletedEvents := nDB.deleteNodeTableEntries(n.Name)
		if currState != nodeFailedState {
			// The entries were still visible: tell the watchers they
			// are gone. Had the node already failed, the watchers were
			// told when it failed.
			for _, ev := range deletedEvents {
				nDB.broadcaster.Write(ev)
			}
		}
		nDB.deleteNodeFromNetworks(n.Name)
	case nodeFailedState:
		if currState == nodeFailedState {
			return false, nil
		}

		delete(m, nodeName)
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
	default:
		// TODO(thaJeztah): make switch exhaustive; add networkdb.nodeNotFound
	}

	nDB.estNodes.Store(int32(len(nDB.nodes)))

	log.G(context.TODO()).Infof("Node %s change state %s --> %s", nodeName, nodeStateName[currState], nodeStateName[newState])

	return true, nil
}

func (nDB *NetworkDB) purgeReincarnation(mn *memberlist.Node) bool {
	for name, node := range nDB.nodes {
		if node.Addr.Equal(mn.Addr) && node.Port == mn.Port && mn.Name != name {
			log.G(context.TODO()).Infof("Node %s/%s, is the new incarnation of the active node %s/%s", mn.Name, mn.Addr, name, node.Addr)
			nDB.changeNodeState(name, nodeLeftState)
			return true
		}
	}

	for name, node := range nDB.failedNodes {
		if node.Addr.Equal(mn.Addr) && node.Port == mn.Port && mn.Name != name {
			log.G(context.TODO()).Infof("Node %s/%s, is the new incarnation of the failed node %s/%s", mn.Name, mn.Addr, name, node.Addr)
			nDB.changeNodeState(name, nodeLeftState)
			return true
		}
	}

	return false
}

func (nDB *NetworkDB) estNumNodes() int {
	return int(nDB.estNodes.Load())
}
