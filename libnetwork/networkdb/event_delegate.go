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
	e.nDB.purgeReincarnation(mn)

	e.nDB.nodes[mn.Name] = &node{Node: *mn}
	logrus.Infof("Node %s/%s, added to nodes list", mn.Name, mn.Addr)
}

func (e *eventDelegate) NotifyLeave(mn *memberlist.Node) {
	logrus.Infof("Node %s/%s, left gossip cluster", mn.Name, mn.Addr)
	e.broadcastNodeEvent(mn.Addr, opDelete)

	e.nDB.Lock()
	defer e.nDB.Unlock()

	n, currState, _ := e.nDB.findNode(mn.Name)
	if n == nil {
		logrus.Errorf("Node %s/%s not found in the node lists", mn.Name, mn.Addr)
		return
	}
	// if the node was active means that did not send the leave cluster message, so it's probable that
	// failed. Else would be already in the left list so nothing else has to be done
	if currState == nodeActiveState {
		moved, err := e.nDB.changeNodeState(mn.Name, nodeFailedState)
		if err != nil {
			logrus.WithError(err).Errorf("impossible condition, node %s/%s not present in the list", mn.Name, mn.Addr)
			return
		}
		if moved {
			logrus.Infof("Node %s/%s, added to failed nodes list", mn.Name, mn.Addr)
		}
	}
}

func (e *eventDelegate) NotifyUpdate(n *memberlist.Node) {
}
