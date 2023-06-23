package networkdb

import (
	"context"
	"encoding/json"
	"net"

	"github.com/containerd/containerd/log"
	"github.com/hashicorp/memberlist"
)

type eventDelegate struct {
	nDB *NetworkDB
}

func (e *eventDelegate) broadcastNodeEvent(addr net.IP, op opType) {
	value, err := json.Marshal(&NodeAddr{addr})
	if err == nil {
		e.nDB.broadcaster.Write(makeEvent(op, NodeTable, "", "", value))
	} else {
		log.G(context.TODO()).Errorf("Error marshalling node broadcast event %s", addr.String())
	}
}

func (e *eventDelegate) NotifyJoin(mn *memberlist.Node) {
	log.G(context.TODO()).Infof("Node %s/%s, joined gossip cluster", mn.Name, mn.Addr)
	e.broadcastNodeEvent(mn.Addr, opCreate)
	e.nDB.Lock()
	defer e.nDB.Unlock()

	// In case the node is rejoining after a failure or leave,
	// just add the node back to active
	if moved, _ := e.nDB.changeNodeState(mn.Name, nodeActiveState); moved {
		return
	}

	// Every node has a unique ID
	// Check on the base of the IP address if the new node that joined is actually a new incarnation of a previous
	// failed or shutdown one
	e.nDB.purgeReincarnation(mn)

	e.nDB.nodes[mn.Name] = &node{Node: *mn}
	log.G(context.TODO()).Infof("Node %s/%s, added to nodes list", mn.Name, mn.Addr)
}

func (e *eventDelegate) NotifyLeave(mn *memberlist.Node) {
	log.G(context.TODO()).Infof("Node %s/%s, left gossip cluster", mn.Name, mn.Addr)
	e.broadcastNodeEvent(mn.Addr, opDelete)

	e.nDB.Lock()
	defer e.nDB.Unlock()

	n, currState, _ := e.nDB.findNode(mn.Name)
	if n == nil {
		log.G(context.TODO()).Errorf("Node %s/%s not found in the node lists", mn.Name, mn.Addr)
		return
	}
	// if the node was active means that did not send the leave cluster message, so it's probable that
	// failed. Else would be already in the left list so nothing else has to be done
	if currState == nodeActiveState {
		moved, err := e.nDB.changeNodeState(mn.Name, nodeFailedState)
		if err != nil {
			log.G(context.TODO()).WithError(err).Errorf("impossible condition, node %s/%s not present in the list", mn.Name, mn.Addr)
			return
		}
		if moved {
			log.G(context.TODO()).Infof("Node %s/%s, added to failed nodes list", mn.Name, mn.Addr)
		}
	}
}

func (e *eventDelegate) NotifyUpdate(n *memberlist.Node) {
}
