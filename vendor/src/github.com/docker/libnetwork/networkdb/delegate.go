package networkdb

import (
	"fmt"
	"net"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gogo/protobuf/proto"
)

type delegate struct {
	nDB *NetworkDB
}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (nDB *NetworkDB) handleNetworkEvent(nEvent *NetworkEvent) bool {
	// Update our local clock if the received messages has newer
	// time.
	nDB.networkClock.Witness(nEvent.LTime)

	nDB.Lock()
	defer nDB.Unlock()

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
			n.leaveTime = time.Now()
		}

		return true
	}

	if nEvent.Type == NetworkEventTypeLeave {
		return false
	}

	// This remote network join is being seen the first time.
	nodeNetworks[nEvent.NetworkID] = &network{
		id:    nEvent.NetworkID,
		ltime: nEvent.LTime,
	}

	nDB.networkNodes[nEvent.NetworkID] = append(nDB.networkNodes[nEvent.NetworkID], nEvent.NodeName)
	return true
}

func (nDB *NetworkDB) handleTableEvent(tEvent *TableEvent) bool {
	// Update our local clock if the received messages has newer
	// time.
	nDB.tableClock.Witness(tEvent.LTime)

	if entry, err := nDB.getEntry(tEvent.TableName, tEvent.NetworkID, tEvent.Key); err == nil {
		// We have the latest state. Ignore the event
		// since it is stale.
		if entry.ltime >= tEvent.LTime {
			return false
		}
	}

	entry := &entry{
		ltime:    tEvent.LTime,
		node:     tEvent.NodeName,
		value:    tEvent.Value,
		deleting: tEvent.Type == TableEventTypeDelete,
	}

	if entry.deleting {
		entry.deleteTime = time.Now()
	}

	nDB.Lock()
	nDB.indexes[byTable].Insert(fmt.Sprintf("/%s/%s/%s", tEvent.TableName, tEvent.NetworkID, tEvent.Key), entry)
	nDB.indexes[byNetwork].Insert(fmt.Sprintf("/%s/%s/%s", tEvent.NetworkID, tEvent.TableName, tEvent.Key), entry)
	nDB.Unlock()

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
	return true
}

func (nDB *NetworkDB) handleCompound(buf []byte, isBulkSync bool) {
	// Decode the parts
	parts, err := decodeCompoundMessage(buf)
	if err != nil {
		logrus.Errorf("Failed to decode compound request: %v", err)
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
		logrus.Errorf("Error decoding table event message: %v", err)
		return
	}

	// Ignore messages that this node generated.
	if tEvent.NodeName == nDB.config.NodeName {
		return
	}

	// Do not rebroadcast a bulk sync
	if rebroadcast := nDB.handleTableEvent(&tEvent); rebroadcast && !isBulkSync {
		var err error
		buf, err = encodeRawMessage(MessageTypeTableEvent, buf)
		if err != nil {
			logrus.Errorf("Error marshalling gossip message for network event rebroadcast: %v", err)
			return
		}

		nDB.RLock()
		n, ok := nDB.networks[nDB.config.NodeName][tEvent.NetworkID]
		nDB.RUnlock()

		if !ok {
			return
		}

		broadcastQ := n.tableBroadcasts

		if broadcastQ == nil {
			return
		}

		broadcastQ.QueueBroadcast(&tableEventMessage{
			msg:   buf,
			id:    tEvent.NetworkID,
			tname: tEvent.TableName,
			key:   tEvent.Key,
			node:  nDB.config.NodeName,
		})
	}
}

func (nDB *NetworkDB) handleNetworkMessage(buf []byte) {
	var nEvent NetworkEvent
	if err := proto.Unmarshal(buf, &nEvent); err != nil {
		logrus.Errorf("Error decoding network event message: %v", err)
		return
	}

	if rebroadcast := nDB.handleNetworkEvent(&nEvent); rebroadcast {
		var err error
		buf, err = encodeRawMessage(MessageTypeNetworkEvent, buf)
		if err != nil {
			logrus.Errorf("Error marshalling gossip message for network event rebroadcast: %v", err)
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
		logrus.Errorf("Error decoding bulk sync message: %v", err)
		return
	}

	if bsm.LTime > 0 {
		nDB.tableClock.Witness(bsm.LTime)
	}

	nDB.handleMessage(bsm.Payload, true)

	// Don't respond to a bulk sync which was not unsolicited
	if !bsm.Unsolicited {
		nDB.RLock()
		ch, ok := nDB.bulkSyncAckTbl[bsm.NodeName]
		nDB.RUnlock()
		if ok {
			close(ch)
		}

		return
	}

	var nodeAddr net.IP
	if node, ok := nDB.nodes[bsm.NodeName]; ok {
		nodeAddr = node.Addr
	}

	if err := nDB.bulkSyncNode(bsm.Networks, bsm.NodeName, false); err != nil {
		logrus.Errorf("Error in responding to bulk sync from node %s: %v", nodeAddr, err)
	}
}

func (nDB *NetworkDB) handleMessage(buf []byte, isBulkSync bool) {
	mType, data, err := decodeMessage(buf)
	if err != nil {
		logrus.Errorf("Error decoding gossip message to get message type: %v", err)
		return
	}

	switch mType {
	case MessageTypeNetworkEvent:
		nDB.handleNetworkMessage(data)
	case MessageTypeTableEvent:
		nDB.handleTableMessage(data, isBulkSync)
	case MessageTypeBulkSync:
		nDB.handleBulkSync(data)
	case MessageTypeCompound:
		nDB.handleCompound(data, isBulkSync)
	default:
		logrus.Errorf("%s: unknown message type %d", nDB.config.NodeName, mType)
	}
}

func (d *delegate) NotifyMsg(buf []byte) {
	if len(buf) == 0 {
		return
	}

	d.nDB.handleMessage(buf, false)
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.nDB.networkBroadcasts.GetBroadcasts(overhead, limit)
}

func (d *delegate) LocalState(join bool) []byte {
	d.nDB.RLock()
	defer d.nDB.RUnlock()

	pp := NetworkPushPull{
		LTime: d.nDB.networkClock.Time(),
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
		logrus.Errorf("Failed to encode local network state: %v", err)
		return nil
	}

	return buf
}

func (d *delegate) MergeRemoteState(buf []byte, isJoin bool) {
	if len(buf) == 0 {
		logrus.Error("zero byte remote network state received")
		return
	}

	var gMsg GossipMessage
	err := proto.Unmarshal(buf, &gMsg)
	if err != nil {
		logrus.Errorf("Error unmarshalling push pull messsage: %v", err)
		return
	}

	if gMsg.Type != MessageTypePushPull {
		logrus.Errorf("Invalid message type %v received from remote", buf[0])
	}

	pp := NetworkPushPull{}
	if err := proto.Unmarshal(gMsg.Data, &pp); err != nil {
		logrus.Errorf("Failed to decode remote network state: %v", err)
		return
	}

	if pp.LTime > 0 {
		d.nDB.networkClock.Witness(pp.LTime)
	}

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
