package networkdb

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/hashicorp/serf/serf"
)

type networkData struct {
	LTime    serf.LamportTime
	ID       string
	NodeName string
	Leaving  bool
}

type networkPushPull struct {
	LTime    serf.LamportTime
	Networks []networkData
}

type delegate struct {
	nDB *NetworkDB
}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (nDB *NetworkDB) handleNetworkEvent(nEvent *networkEventData) bool {
	// Update our local clock if the received messages has newer
	// time.
	nDB.networkClock.Witness(nEvent.LTime)

	nDB.Lock()
	defer nDB.Unlock()

	nodeNetworks, ok := nDB.networks[nEvent.NodeName]
	if !ok {
		// We haven't heard about this node at all.  Ignore the leave
		if nEvent.Event == networkLeave {
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
		n.leaving = nEvent.Event == networkLeave
		if n.leaving {
			n.leaveTime = time.Now()
		}

		return true
	}

	if nEvent.Event == networkLeave {
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

func (nDB *NetworkDB) handleTableEvent(tEvent *tableEventData) bool {
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
		deleting: tEvent.Event == tableEntryDelete,
	}

	if entry.deleting {
		entry.deleteTime = time.Now()
	}

	nDB.Lock()
	nDB.indexes[byTable].Insert(fmt.Sprintf("/%s/%s/%s", tEvent.TableName, tEvent.NetworkID, tEvent.Key), entry)
	nDB.indexes[byNetwork].Insert(fmt.Sprintf("/%s/%s/%s", tEvent.NetworkID, tEvent.TableName, tEvent.Key), entry)
	nDB.Unlock()

	var op opType
	switch tEvent.Event {
	case tableEntryCreate:
		op = opCreate
	case tableEntryUpdate:
		op = opUpdate
	case tableEntryDelete:
		op = opDelete
	}

	nDB.broadcaster.Write(makeEvent(op, tEvent.TableName, tEvent.NetworkID, tEvent.Key, tEvent.Value))
	return true
}

func (nDB *NetworkDB) handleCompound(buf []byte) {
	// Decode the parts
	trunc, parts, err := decodeCompoundMessage(buf[1:])
	if err != nil {
		logrus.Errorf("Failed to decode compound request: %v", err)
		return
	}

	// Log any truncation
	if trunc > 0 {
		logrus.Warnf("Compound request had %d truncated messages", trunc)
	}

	// Handle each message
	for _, part := range parts {
		nDB.handleMessage(part)
	}
}

func (nDB *NetworkDB) handleTableMessage(buf []byte) {
	var tEvent tableEventData
	if err := decodeMessage(buf[1:], &tEvent); err != nil {
		logrus.Errorf("Error decoding table event message: %v", err)
		return
	}

	if rebroadcast := nDB.handleTableEvent(&tEvent); rebroadcast {
		// Copy the buffer since we cannot rely on the slice not changing
		newBuf := make([]byte, len(buf))
		copy(newBuf, buf)

		nDB.RLock()
		n, ok := nDB.networks[nDB.config.NodeName][tEvent.NetworkID]
		nDB.RUnlock()

		if !ok {
			return
		}

		broadcastQ := n.tableBroadcasts
		broadcastQ.QueueBroadcast(&tableEventMessage{
			msg:   newBuf,
			id:    tEvent.NetworkID,
			tname: tEvent.TableName,
			key:   tEvent.Key,
			node:  nDB.config.NodeName,
		})
	}
}

func (nDB *NetworkDB) handleNetworkMessage(buf []byte) {
	var nEvent networkEventData
	if err := decodeMessage(buf[1:], &nEvent); err != nil {
		logrus.Errorf("Error decoding network event message: %v", err)
		return
	}

	if rebroadcast := nDB.handleNetworkEvent(&nEvent); rebroadcast {
		// Copy the buffer since it we cannot rely on the slice not changing
		newBuf := make([]byte, len(buf))
		copy(newBuf, buf)

		nDB.networkBroadcasts.QueueBroadcast(&networkEventMessage{
			msg:  newBuf,
			id:   nEvent.NetworkID,
			node: nEvent.NodeName,
		})
	}
}

func (nDB *NetworkDB) handleBulkSync(buf []byte) {
	var bsm bulkSyncMessage
	if err := decodeMessage(buf[1:], &bsm); err != nil {
		logrus.Errorf("Error decoding bulk sync message: %v", err)
		return
	}

	if bsm.LTime > 0 {
		nDB.tableClock.Witness(bsm.LTime)
	}

	nDB.handleMessage(bsm.Payload)

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

	if err := nDB.bulkSyncNode(bsm.Networks, bsm.NodeName, false); err != nil {
		logrus.Errorf("Error in responding to bulk sync from node %s: %v", nDB.nodes[bsm.NodeName].Addr, err)
	}
}

func (nDB *NetworkDB) handleMessage(buf []byte) {
	msgType := messageType(buf[0])

	switch msgType {
	case networkEventMsg:
		nDB.handleNetworkMessage(buf)
	case tableEventMsg:
		nDB.handleTableMessage(buf)
	case compoundMsg:
		nDB.handleCompound(buf)
	case bulkSyncMsg:
		nDB.handleBulkSync(buf)
	default:
		logrus.Errorf("%s: unknown message type %d payload = %v", nDB.config.NodeName, msgType, buf[:8])
	}
}

func (d *delegate) NotifyMsg(buf []byte) {
	if len(buf) == 0 {
		return
	}

	d.nDB.handleMessage(buf)
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.nDB.networkBroadcasts.GetBroadcasts(overhead, limit)
}

func (d *delegate) LocalState(join bool) []byte {
	d.nDB.RLock()
	defer d.nDB.RUnlock()

	pp := networkPushPull{
		LTime: d.nDB.networkClock.Time(),
	}

	for name, nn := range d.nDB.networks {
		for _, n := range nn {
			pp.Networks = append(pp.Networks, networkData{
				LTime:    n.ltime,
				ID:       n.id,
				NodeName: name,
				Leaving:  n.leaving,
			})
		}
	}

	buf, err := encodeMessage(networkPushPullMsg, &pp)
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

	if messageType(buf[0]) != networkPushPullMsg {
		logrus.Errorf("Invalid message type %v received from remote", buf[0])
	}

	pp := networkPushPull{}
	if err := decodeMessage(buf[1:], &pp); err != nil {
		logrus.Errorf("Failed to decode remote network state: %v", err)
		return
	}

	if pp.LTime > 0 {
		d.nDB.networkClock.Witness(pp.LTime)
	}

	for _, n := range pp.Networks {
		nEvent := &networkEventData{
			LTime:     n.LTime,
			NodeName:  n.NodeName,
			NetworkID: n.ID,
			Event:     networkJoin,
		}

		if n.Leaving {
			nEvent.Event = networkLeave
		}

		d.nDB.handleNetworkEvent(nEvent)
	}

}
