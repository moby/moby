package networkdb

import (
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

type networkEventMessage struct {
	id   string
	node string
	msg  []byte
}

func (m *networkEventMessage) Invalidates(other memberlist.Broadcast) bool {
	otherm := other.(*networkEventMessage)
	return m.id == otherm.id && m.node == otherm.node
}

func (m *networkEventMessage) Message() []byte {
	return m.msg
}

func (m *networkEventMessage) Finished() {
}

func (nDB *NetworkDB) sendNetworkEvent(nid string, event NetworkEvent_Type, ltime serf.LamportTime) error {
	nEvent := NetworkEvent{
		Type:      event,
		LTime:     ltime,
		NodeName:  nDB.config.NodeName,
		NetworkID: nid,
	}

	raw, err := encodeMessage(MessageTypeNetworkEvent, &nEvent)
	if err != nil {
		return err
	}

	nDB.networkBroadcasts.QueueBroadcast(&networkEventMessage{
		msg:  raw,
		id:   nid,
		node: nDB.config.NodeName,
	})
	return nil
}

type tableEventMessage struct {
	id    string
	tname string
	key   string
	msg   []byte
	node  string
}

func (m *tableEventMessage) Invalidates(other memberlist.Broadcast) bool {
	otherm := other.(*tableEventMessage)
	return m.id == otherm.id && m.tname == otherm.tname && m.key == otherm.key
}

func (m *tableEventMessage) Message() []byte {
	return m.msg
}

func (m *tableEventMessage) Finished() {
}

func (nDB *NetworkDB) sendTableEvent(event TableEvent_Type, nid string, tname string, key string, entry *entry) error {
	tEvent := TableEvent{
		Type:      event,
		LTime:     entry.ltime,
		NodeName:  nDB.config.NodeName,
		NetworkID: nid,
		TableName: tname,
		Key:       key,
		Value:     entry.value,
	}

	raw, err := encodeMessage(MessageTypeTableEvent, &tEvent)
	if err != nil {
		return err
	}

	var broadcastQ *memberlist.TransmitLimitedQueue
	nDB.RLock()
	thisNodeNetworks, ok := nDB.networks[nDB.config.NodeName]
	if ok {
		// The network may have been removed
		network, networkOk := thisNodeNetworks[nid]
		if !networkOk {
			nDB.RUnlock()
			return nil
		}

		broadcastQ = network.tableBroadcasts
	}
	nDB.RUnlock()

	// The network may have been removed
	if broadcastQ == nil {
		return nil
	}

	broadcastQ.QueueBroadcast(&tableEventMessage{
		msg:   raw,
		id:    nid,
		tname: tname,
		key:   key,
		node:  nDB.config.NodeName,
	})
	return nil
}
