package networkdb

import (
	"context"
	"net"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/gogo/protobuf/proto"
)

type delegate struct {
	nDB *NetworkDB
}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (nDB *NetworkDB) handleNodeEvent(nEvent *NodeEvent) bool {
	// Update our local clock if the received messages has newer
	// time.
	nDB.networkClock.Witness(nEvent.LTime)

	nDB.Lock()
	defer nDB.Unlock()

	// check if the node exists
	n, _, _ := nDB.findNode(nEvent.NodeName)
	if n == nil {
		return false
	}

	// check if the event is fresh
	if n.ltime >= nEvent.LTime {
		return false
	}

	// If we are here means that the event is fresher and the node is known. Update the laport time
	n.ltime = nEvent.LTime

	// If the node is not known from memberlist we cannot process save any state of it else if it actually
	// dies we won't receive any notification and we will remain stuck with it
	if _, ok := nDB.nodes[nEvent.NodeName]; !ok {
		log.G(context.TODO()).Errorf("node: %s is unknown to memberlist", nEvent.NodeName)
		return false
	}

	switch nEvent.Type {
	case NodeEventTypeJoin:
		moved, err := nDB.changeNodeState(n.Name, nodeActiveState)
		if err != nil {
			log.G(context.TODO()).WithError(err).Error("unable to find the node to move")
			return false
		}
		if moved {
			log.G(context.TODO()).Infof("%v(%v): Node join event for %s/%s", nDB.config.Hostname, nDB.config.NodeID, n.Name, n.Addr)
		}
		return moved
	case NodeEventTypeLeave:
		moved, err := nDB.changeNodeState(n.Name, nodeLeftState)
		if err != nil {
			log.G(context.TODO()).WithError(err).Error("unable to find the node to move")
			return false
		}
		if moved {
			log.G(context.TODO()).Infof("%v(%v): Node leave event for %s/%s", nDB.config.Hostname, nDB.config.NodeID, n.Name, n.Addr)
		}
		return moved
	}

	return false
}

func (nDB *NetworkDB) handleNetworkEvent(nEvent *NetworkEvent) bool {
	// Update our local clock if the received messages has newer
	// time.
	nDB.networkClock.Witness(nEvent.LTime)

	nDB.Lock()
	defer nDB.Unlock()

	if nEvent.NodeName == nDB.config.NodeID {
		return false
	}

	nodeNetworks, ok := nDB.networks[nEvent.NodeName]
	if !ok {
		// We haven't heard about this node at all.  Ignore the leave
		if nEvent.Type == NetworkEventTypeLeave {
			return false
		}

		nodeNetworks = make(map[string]*network)
		nDB.networks[nEvent.NodeName] = nodeNetworks
	}

	if n, ok := nodeNetworks[nEvent.NetworkID]; ok {
		// We have the latest state. Ignore the event
		// since it is stale.
		if n.ltime >= nEvent.LTime {
			return false
		}

		n.ltime = nEvent.LTime
		n.leaving = nEvent.Type == NetworkEventTypeLeave
		if n.leaving {
			n.reapTime = nDB.config.reapNetworkInterval

			// The remote node is leaving the network, but not the gossip cluster.
			// Mark all its entries in deleted state, this will guarantee that
			// if some node bulk sync with us, the deleted state of
			// these entries will be propagated.
			nDB.deleteNodeNetworkEntries(nEvent.NetworkID, nEvent.NodeName)
		}

		if nEvent.Type == NetworkEventTypeLeave {
			nDB.deleteNetworkNode(nEvent.NetworkID, nEvent.NodeName)
		} else {
			nDB.addNetworkNode(nEvent.NetworkID, nEvent.NodeName)
		}

		return true
	}

	if nEvent.Type == NetworkEventTypeLeave {
		return false
	}

	// If the node is not known from memberlist we cannot process save any state of it else if it actually
	// dies we won't receive any notification and we will remain stuck with it
	if _, ok := nDB.nodes[nEvent.NodeName]; !ok {
		return false
	}

	// This remote network join is being seen the first time.
	nodeNetworks[nEvent.NetworkID] = &network{
		id:    nEvent.NetworkID,
		ltime: nEvent.LTime,
	}

	nDB.addNetworkNode(nEvent.NetworkID, nEvent.NodeName)
	return true
}

func (nDB *NetworkDB) handleTableEvent(tEvent *TableEvent, isBulkSync bool) bool {
	// Update our local clock if the received messages has newer time.
	nDB.tableClock.Witness(tEvent.LTime)

	// Ignore the table events for networks that are in the process of going away
	nDB.RLock()
	networks := nDB.networks[nDB.config.NodeID]
	network, ok := networks[tEvent.NetworkID]
	// Check if the owner of the event is still part of the network
	nodes := nDB.networkNodes[tEvent.NetworkID]
	var nodePresent bool
	for _, node := range nodes {
		if node == tEvent.NodeName {
			nodePresent = true
			break
		}
	}
	nDB.RUnlock()

	if !ok || network.leaving || !nodePresent {
		// I'm out of the network OR the event owner is not anymore part of the network so do not propagate
		return false
	}

	nDB.Lock()
	e, err := nDB.getEntry(tEvent.TableName, tEvent.NetworkID, tEvent.Key)
	if err == nil {
		// We have the latest state. Ignore the event
		// since it is stale.
		if e.ltime >= tEvent.LTime {
			nDB.Unlock()
			return false
		}
	} else if tEvent.Type == TableEventTypeDelete && !isBulkSync {
		nDB.Unlock()
		// We don't know the entry, the entry is being deleted and the message is an async message
		// In this case the safest approach is to ignore it, it is possible that the queue grew so much to
		// exceed the garbage collection time (the residual reap time that is in the message is not being
		// updated, to avoid inserting too many messages in the queue).
		// Instead the messages coming from TCP bulk sync are safe with the latest value for the garbage collection time
		return false
	}

	e = &entry{
		ltime:    tEvent.LTime,
		node:     tEvent.NodeName,
		value:    tEvent.Value,
		deleting: tEvent.Type == TableEventTypeDelete,
		reapTime: time.Duration(tEvent.ResidualReapTime) * time.Second,
	}

	// All the entries marked for deletion should have a reapTime set greater than 0
	// This case can happen if the cluster is running different versions of the engine where the old version does not have the
	// field. If that is not the case, this can be a BUG
	if e.deleting && e.reapTime == 0 {
		log.G(context.TODO()).Warnf("%v(%v) handleTableEvent object %+v has a 0 reapTime, is the cluster running the same docker engine version?",
			nDB.config.Hostname, nDB.config.NodeID, tEvent)
		e.reapTime = nDB.config.reapEntryInterval
	}
	nDB.createOrUpdateEntry(tEvent.NetworkID, tEvent.TableName, tEvent.Key, e)
	nDB.Unlock()

	if err != nil && tEvent.Type == TableEventTypeDelete {
		// Again we don't know the entry but this is coming from a TCP sync so the message body is up to date.
		// We had saved the state so to speed up convergence and be able to avoid accepting create events.
		// Now we will rebroadcast the message if 2 conditions are met:
		// 1) we had already synced this network (during the network join)
		// 2) the residual reapTime is higher than 1/6 of the total reapTime.
		// If the residual reapTime is lower or equal to 1/6 of the total reapTime don't bother broadcasting it around
		// most likely the cluster is already aware of it
		// This also reduce the possibility that deletion of entries close to their garbage collection ends up circuling around
		// forever
		// log.G(ctx).Infof("exiting on delete not knowing the obj with rebroadcast:%t", network.inSync)
		return network.inSync && e.reapTime > nDB.config.reapEntryInterval/6
	}

	var op opType
	switch tEvent.Type {
	case TableEventTypeCreate:
		op = opCreate
	case TableEventTypeUpdate:
		op = opUpdate
	case TableEventTypeDelete:
		op = opDelete
	}

	nDB.broadcaster.Write(makeEvent(op, tEvent.TableName, tEvent.NetworkID, tEvent.Key, tEvent.Value))
	return network.inSync
}

func (nDB *NetworkDB) handleCompound(buf []byte, isBulkSync bool) {
	// Decode the parts
	parts, err := decodeCompoundMessage(buf)
	if err != nil {
		log.G(context.TODO()).Errorf("Failed to decode compound request: %v", err)
		return
	}

	// Handle each message
	for _, part := range parts {
		nDB.handleMessage(part, isBulkSync)
	}
}

func (nDB *NetworkDB) handleTableMessage(buf []byte, isBulkSync bool) {
	var tEvent TableEvent
	if err := proto.Unmarshal(buf, &tEvent); err != nil {
		log.G(context.TODO()).Errorf("Error decoding table event message: %v", err)
		return
	}

	// Ignore messages that this node generated.
	if tEvent.NodeName == nDB.config.NodeID {
		return
	}

	if rebroadcast := nDB.handleTableEvent(&tEvent, isBulkSync); rebroadcast {
		var err error
		buf, err = encodeRawMessage(MessageTypeTableEvent, buf)
		if err != nil {
			log.G(context.TODO()).Errorf("Error marshalling gossip message for network event rebroadcast: %v", err)
			return
		}

		nDB.RLock()
		n, ok := nDB.networks[nDB.config.NodeID][tEvent.NetworkID]
		nDB.RUnlock()

		// if the network is not there anymore, OR we are leaving the network OR the broadcast queue is not present
		if !ok || n.leaving || n.tableBroadcasts == nil {
			return
		}

		// if the queue is over the threshold, avoid distributing information coming from TCP sync
		if isBulkSync && n.tableBroadcasts.NumQueued() > maxQueueLenBroadcastOnSync {
			return
		}

		n.tableBroadcasts.QueueBroadcast(&tableEventMessage{
			msg:   buf,
			id:    tEvent.NetworkID,
			tname: tEvent.TableName,
			key:   tEvent.Key,
		})
	}
}

func (nDB *NetworkDB) handleNodeMessage(buf []byte) {
	var nEvent NodeEvent
	if err := proto.Unmarshal(buf, &nEvent); err != nil {
		log.G(context.TODO()).Errorf("Error decoding node event message: %v", err)
		return
	}

	if rebroadcast := nDB.handleNodeEvent(&nEvent); rebroadcast {
		var err error
		buf, err = encodeRawMessage(MessageTypeNodeEvent, buf)
		if err != nil {
			log.G(context.TODO()).Errorf("Error marshalling gossip message for node event rebroadcast: %v", err)
			return
		}

		nDB.nodeBroadcasts.QueueBroadcast(&nodeEventMessage{
			msg: buf,
		})
	}
}

func (nDB *NetworkDB) handleNetworkMessage(buf []byte) {
	var nEvent NetworkEvent
	if err := proto.Unmarshal(buf, &nEvent); err != nil {
		log.G(context.TODO()).Errorf("Error decoding network event message: %v", err)
		return
	}

	if rebroadcast := nDB.handleNetworkEvent(&nEvent); rebroadcast {
		var err error
		buf, err = encodeRawMessage(MessageTypeNetworkEvent, buf)
		if err != nil {
			log.G(context.TODO()).Errorf("Error marshalling gossip message for network event rebroadcast: %v", err)
			return
		}

		nDB.networkBroadcasts.QueueBroadcast(&networkEventMessage{
			msg:  buf,
			id:   nEvent.NetworkID,
			node: nEvent.NodeName,
		})
	}
}

func (nDB *NetworkDB) handleBulkSync(buf []byte) {
	var bsm BulkSyncMessage
	if err := proto.Unmarshal(buf, &bsm); err != nil {
		log.G(context.TODO()).Errorf("Error decoding bulk sync message: %v", err)
		return
	}

	if bsm.LTime > 0 {
		nDB.tableClock.Witness(bsm.LTime)
	}

	nDB.handleMessage(bsm.Payload, true)

	// Don't respond to a bulk sync which was not unsolicited
	if !bsm.Unsolicited {
		nDB.Lock()
		ch, ok := nDB.bulkSyncAckTbl[bsm.NodeName]
		if ok {
			close(ch)
			delete(nDB.bulkSyncAckTbl, bsm.NodeName)
		}
		nDB.Unlock()

		return
	}

	var nodeAddr net.IP
	nDB.RLock()
	if node, ok := nDB.nodes[bsm.NodeName]; ok {
		nodeAddr = node.Addr
	}
	nDB.RUnlock()

	if err := nDB.bulkSyncNode(bsm.Networks, bsm.NodeName, false); err != nil {
		log.G(context.TODO()).Errorf("Error in responding to bulk sync from node %s: %v", nodeAddr, err)
	}
}

func (nDB *NetworkDB) handleMessage(buf []byte, isBulkSync bool) {
	mType, data, err := decodeMessage(buf)
	if err != nil {
		log.G(context.TODO()).Errorf("Error decoding gossip message to get message type: %v", err)
		return
	}

	switch mType {
	case MessageTypeNodeEvent:
		nDB.handleNodeMessage(data)
	case MessageTypeNetworkEvent:
		nDB.handleNetworkMessage(data)
	case MessageTypeTableEvent:
		nDB.handleTableMessage(data, isBulkSync)
	case MessageTypeBulkSync:
		nDB.handleBulkSync(data)
	case MessageTypeCompound:
		nDB.handleCompound(data, isBulkSync)
	default:
		log.G(context.TODO()).Errorf("%v(%v): unknown message type %d", nDB.config.Hostname, nDB.config.NodeID, mType)
	}
}

func (d *delegate) NotifyMsg(buf []byte) {
	if len(buf) == 0 {
		return
	}

	d.nDB.handleMessage(buf, false)
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	msgs := d.nDB.networkBroadcasts.GetBroadcasts(overhead, limit)
	msgs = append(msgs, d.nDB.nodeBroadcasts.GetBroadcasts(overhead, limit)...)
	return msgs
}

func (d *delegate) LocalState(join bool) []byte {
	if join {
		// Update all the local node/network state to a new time to
		// force update on the node we are trying to rejoin, just in
		// case that node has these in leaving state still. This is
		// facilitate fast convergence after recovering from a gossip
		// failure.
		d.nDB.updateLocalNetworkTime()
	}

	d.nDB.RLock()
	defer d.nDB.RUnlock()

	pp := NetworkPushPull{
		LTime:    d.nDB.networkClock.Time(),
		NodeName: d.nDB.config.NodeID,
	}

	for name, nn := range d.nDB.networks {
		for _, n := range nn {
			pp.Networks = append(pp.Networks, &NetworkEntry{
				LTime:     n.ltime,
				NetworkID: n.id,
				NodeName:  name,
				Leaving:   n.leaving,
			})
		}
	}

	buf, err := encodeMessage(MessageTypePushPull, &pp)
	if err != nil {
		log.G(context.TODO()).Errorf("Failed to encode local network state: %v", err)
		return nil
	}

	return buf
}

func (d *delegate) MergeRemoteState(buf []byte, isJoin bool) {
	if len(buf) == 0 {
		log.G(context.TODO()).Error("zero byte remote network state received")
		return
	}

	var gMsg GossipMessage
	err := proto.Unmarshal(buf, &gMsg)
	if err != nil {
		log.G(context.TODO()).Errorf("Error unmarshalling push pull message: %v", err)
		return
	}

	if gMsg.Type != MessageTypePushPull {
		log.G(context.TODO()).Errorf("Invalid message type %v received from remote", buf[0])
	}

	pp := NetworkPushPull{}
	if err := proto.Unmarshal(gMsg.Data, &pp); err != nil {
		log.G(context.TODO()).Errorf("Failed to decode remote network state: %v", err)
		return
	}

	nodeEvent := &NodeEvent{
		LTime:    pp.LTime,
		NodeName: pp.NodeName,
		Type:     NodeEventTypeJoin,
	}
	d.nDB.handleNodeEvent(nodeEvent)

	for _, n := range pp.Networks {
		nEvent := &NetworkEvent{
			LTime:     n.LTime,
			NodeName:  n.NodeName,
			NetworkID: n.NetworkID,
			Type:      NetworkEventTypeJoin,
		}

		if n.Leaving {
			nEvent.Type = NetworkEventTypeLeave
		}

		d.nDB.handleNetworkEvent(nEvent)
	}
}
