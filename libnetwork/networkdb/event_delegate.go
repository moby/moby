package networkdb

import "github.com/hashicorp/memberlist"

type eventDelegate struct {
	nDB *NetworkDB
}

func (e *eventDelegate) NotifyJoin(n *memberlist.Node) {
	e.nDB.Lock()
	e.nDB.nodes[n.Name] = n
	e.nDB.Unlock()
}

func (e *eventDelegate) NotifyLeave(n *memberlist.Node) {
	e.nDB.deleteNodeTableEntries(n.Name)
	e.nDB.Lock()
	delete(e.nDB.nodes, n.Name)
	e.nDB.Unlock()
}

func (e *eventDelegate) NotifyUpdate(n *memberlist.Node) {
}
