package networkdb

import (
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

type networkEventType uint8

const (
	networkJoin networkEventType = 1 + iota
	networkLeave
)

type networkEventData struct {
	Event     networkEventType
	LTime     serf.LamportTime
	NodeName  string
	NetworkID string
}

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

func (nDB *NetworkDB) sendNetworkEvent(nid string, event networkEventType, ltime serf.LamportTime) error {
	nEvent := networkEventData{
		Event:     event,
		LTime:     ltime,
		NodeName:  nDB.config.NodeName,
		NetworkID: nid,
	}

	raw, err := encodeMessage(networkEventMsg, &nEvent)
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

type tableEventType uint8

const (
	tableEntryCreate tableEventType = 1 + iota
	tableEntryUpdate
	tableEntryDelete
)

type tableEventData struct {
	Event     tableEventType
	LTime     serf.LamportTime
	NetworkID string
	TableName string
	NodeName  string
	Value     []byte
	Key       string
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

func (nDB *NetworkDB) sendTableEvent(event tableEventType, nid string, tname string, key string, entry *entry) error {
	tEvent := tableEventData{
		Event:     event,
		LTime:     entry.ltime,
		NodeName:  nDB.config.NodeName,
		NetworkID: nid,
		TableName: tname,
		Key:       key,
		Value:     entry.value,
	}

	raw, err := encodeMessage(tableEventMsg, &tEvent)
	if err != nil {
		return err
	}

	nDB.RLock()
	broadcastQ := nDB.networks[nDB.config.NodeName][nid].tableBroadcasts
	nDB.RUnlock()

	broadcastQ.QueueBroadcast(&tableEventMessage{
		msg:   raw,
		id:    nid,
		tname: tname,
		key:   key,
		node:  nDB.config.NodeName,
	})
	return nil
}
