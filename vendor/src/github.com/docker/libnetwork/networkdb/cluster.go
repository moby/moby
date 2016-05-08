package networkdb

import (
	"crypto/rand"
	"fmt"
	"math/big"
	rnd "math/rand"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
)

const reapInterval = 2 * time.Second

type logWriter struct{}

func (l *logWriter) Write(p []byte) (int, error) {
	str := string(p)

	switch {
	case strings.Contains(str, "[WARN]"):
		logrus.Warn(str)
	case strings.Contains(str, "[DEBUG]"):
		logrus.Debug(str)
	case strings.Contains(str, "[INFO]"):
		logrus.Info(str)
	case strings.Contains(str, "[ERR]"):
		logrus.Warn(str)
	}

	return len(p), nil
}

func (nDB *NetworkDB) clusterInit() error {
	config := memberlist.DefaultLANConfig()
	config.Name = nDB.config.NodeName
	config.BindAddr = nDB.config.BindAddr

	if nDB.config.BindPort != 0 {
		config.BindPort = nDB.config.BindPort
	}

	config.ProtocolVersion = memberlist.ProtocolVersionMax
	config.Delegate = &delegate{nDB: nDB}
	config.Events = &eventDelegate{nDB: nDB}
	config.LogOutput = &logWriter{}

	nDB.networkBroadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return len(nDB.nodes)
		},
		RetransmitMult: config.RetransmitMult,
	}

	mlist, err := memberlist.Create(config)
	if err != nil {
		return fmt.Errorf("failed to create memberlist: %v", err)
	}

	nDB.stopCh = make(chan struct{})
	nDB.memberlist = mlist
	nDB.mConfig = config

	for _, trigger := range []struct {
		interval time.Duration
		fn       func()
	}{
		{reapInterval, nDB.reapState},
		{config.GossipInterval, nDB.gossip},
		{config.PushPullInterval, nDB.bulkSyncTables},
	} {
		t := time.NewTicker(trigger.interval)
		go nDB.triggerFunc(trigger.interval, t.C, nDB.stopCh, trigger.fn)
		nDB.tickers = append(nDB.tickers, t)
	}

	return nil
}

func (nDB *NetworkDB) clusterJoin(members []string) error {
	mlist := nDB.memberlist

	if _, err := mlist.Join(members); err != nil {
		return fmt.Errorf("could not join node to memberlist: %v", err)
	}

	return nil
}

func (nDB *NetworkDB) clusterLeave() error {
	mlist := nDB.memberlist

	if err := mlist.Leave(time.Second); err != nil {
		return err
	}

	close(nDB.stopCh)

	for _, t := range nDB.tickers {
		t.Stop()
	}

	return mlist.Shutdown()
}

func (nDB *NetworkDB) triggerFunc(stagger time.Duration, C <-chan time.Time, stop <-chan struct{}, f func()) {
	// Use a random stagger to avoid syncronizing
	randStagger := time.Duration(uint64(rnd.Int63()) % uint64(stagger))
	select {
	case <-time.After(randStagger):
	case <-stop:
		return
	}
	for {
		select {
		case <-C:
			f()
		case <-stop:
			return
		}
	}
}

func (nDB *NetworkDB) reapState() {
	nDB.reapNetworks()
	nDB.reapTableEntries()
}

func (nDB *NetworkDB) reapNetworks() {
	now := time.Now()
	nDB.Lock()
	for name, nn := range nDB.networks {
		for id, n := range nn {
			if n.leaving && now.Sub(n.leaveTime) > reapInterval {
				delete(nn, id)
				nDB.deleteNetworkNode(id, name)
			}
		}
	}
	nDB.Unlock()
}

func (nDB *NetworkDB) reapTableEntries() {
	var paths []string

	now := time.Now()

	nDB.RLock()
	nDB.indexes[byTable].Walk(func(path string, v interface{}) bool {
		entry, ok := v.(*entry)
		if !ok {
			return false
		}

		if !entry.deleting || now.Sub(entry.deleteTime) <= reapInterval {
			return false
		}

		paths = append(paths, path)
		return false
	})
	nDB.RUnlock()

	nDB.Lock()
	for _, path := range paths {
		params := strings.Split(path[1:], "/")
		tname := params[0]
		nid := params[1]
		key := params[2]

		if _, ok := nDB.indexes[byTable].Delete(fmt.Sprintf("/%s/%s/%s", tname, nid, key)); !ok {
			logrus.Errorf("Could not delete entry in table %s with network id %s and key %s as it does not exist", tname, nid, key)
		}

		if _, ok := nDB.indexes[byNetwork].Delete(fmt.Sprintf("/%s/%s/%s", nid, tname, key)); !ok {
			logrus.Errorf("Could not delete entry in network %s with table name %s and key %s as it does not exist", nid, tname, key)
		}
	}
	nDB.Unlock()
}

func (nDB *NetworkDB) gossip() {
	networkNodes := make(map[string][]string)
	nDB.RLock()
	for nid := range nDB.networks[nDB.config.NodeName] {
		networkNodes[nid] = nDB.networkNodes[nid]

	}
	nDB.RUnlock()

	for nid, nodes := range networkNodes {
		mNodes := nDB.mRandomNodes(3, nodes)
		bytesAvail := udpSendBuf - compoundHeaderOverhead

		nDB.RLock()
		broadcastQ := nDB.networks[nDB.config.NodeName][nid].tableBroadcasts
		nDB.RUnlock()

		if broadcastQ == nil {
			logrus.Errorf("Invalid broadcastQ encountered while gossiping for network %s", nid)
			continue
		}

		msgs := broadcastQ.GetBroadcasts(compoundOverhead, bytesAvail)
		if len(msgs) == 0 {
			continue
		}

		// Create a compound message
		compound := makeCompoundMessage(msgs)

		for _, node := range mNodes {
			nDB.RLock()
			mnode := nDB.nodes[node]
			nDB.RUnlock()

			if mnode == nil {
				break
			}

			// Send the compound message
			if err := nDB.memberlist.SendToUDP(mnode, compound.Bytes()); err != nil {
				logrus.Errorf("Failed to send gossip to %s: %s", mnode.Addr, err)
			}
		}
	}
}

type bulkSyncMessage struct {
	LTime       serf.LamportTime
	Unsolicited bool
	NodeName    string
	Networks    []string
	Payload     []byte
}

func (nDB *NetworkDB) bulkSyncTables() {
	var networks []string
	nDB.RLock()
	for nid := range nDB.networks[nDB.config.NodeName] {
		networks = append(networks, nid)
	}
	nDB.RUnlock()

	for {
		if len(networks) == 0 {
			break
		}

		nid := networks[0]
		networks = networks[1:]

		completed, err := nDB.bulkSync(nid, false)
		if err != nil {
			logrus.Errorf("periodic bulk sync failure for network %s: %v", nid, err)
			continue
		}

		// Remove all the networks for which we have
		// successfully completed bulk sync in this iteration.
		updatedNetworks := make([]string, 0, len(networks))
		for _, nid := range networks {
			for _, completedNid := range completed {
				if nid == completedNid {
					continue
				}

				updatedNetworks = append(updatedNetworks, nid)
			}
		}

		networks = updatedNetworks
	}
}

func (nDB *NetworkDB) bulkSync(nid string, all bool) ([]string, error) {
	nDB.RLock()
	nodes := nDB.networkNodes[nid]
	nDB.RUnlock()

	if !all {
		// If not all, then just pick one.
		nodes = nDB.mRandomNodes(1, nodes)
	}

	logrus.Debugf("%s: Initiating bulk sync with nodes %v", nDB.config.NodeName, nodes)
	var err error
	var networks []string
	for _, node := range nodes {
		if node == nDB.config.NodeName {
			continue
		}

		networks = nDB.findCommonNetworks(node)
		err = nDB.bulkSyncNode(networks, node, true)
		if err != nil {
			err = fmt.Errorf("bulk sync failed on node %s: %v", node, err)
		}
	}

	if err != nil {
		return nil, err
	}

	return networks, nil
}

// Bulk sync all the table entries belonging to a set of networks to a
// single peer node. It can be unsolicited or can be in response to an
// unsolicited bulk sync
func (nDB *NetworkDB) bulkSyncNode(networks []string, node string, unsolicited bool) error {
	var msgs [][]byte

	logrus.Debugf("%s: Initiating bulk sync for networks %v with node %s", nDB.config.NodeName, networks, node)

	nDB.RLock()
	mnode := nDB.nodes[node]
	if mnode == nil {
		nDB.RUnlock()
		return nil
	}

	for _, nid := range networks {
		nDB.indexes[byNetwork].WalkPrefix(fmt.Sprintf("/%s", nid), func(path string, v interface{}) bool {
			entry, ok := v.(*entry)
			if !ok {
				return false
			}

			params := strings.Split(path[1:], "/")
			tEvent := tableEventData{
				Event:     tableEntryCreate,
				LTime:     entry.ltime,
				NodeName:  entry.node,
				NetworkID: nid,
				TableName: params[1],
				Key:       params[2],
				Value:     entry.value,
			}

			msg, err := encodeMessage(tableEventMsg, &tEvent)
			if err != nil {
				logrus.Errorf("Encode failure during bulk sync: %#v", tEvent)
				return false
			}

			msgs = append(msgs, msg)
			return false
		})
	}
	nDB.RUnlock()

	// Create a compound message
	compound := makeCompoundMessage(msgs)

	bsm := bulkSyncMessage{
		LTime:       nDB.tableClock.Time(),
		Unsolicited: unsolicited,
		NodeName:    nDB.config.NodeName,
		Networks:    networks,
		Payload:     compound.Bytes(),
	}

	buf, err := encodeMessage(bulkSyncMsg, &bsm)
	if err != nil {
		return fmt.Errorf("failed to encode bulk sync message: %v", err)
	}

	nDB.Lock()
	ch := make(chan struct{})
	nDB.bulkSyncAckTbl[node] = ch
	nDB.Unlock()

	err = nDB.memberlist.SendToTCP(mnode, buf)
	if err != nil {
		nDB.Lock()
		delete(nDB.bulkSyncAckTbl, node)
		nDB.Unlock()

		return fmt.Errorf("failed to send a TCP message during bulk sync: %v", err)
	}

	startTime := time.Now()
	select {
	case <-time.After(30 * time.Second):
		logrus.Errorf("Bulk sync to node %s timed out", node)
	case <-ch:
		nDB.Lock()
		delete(nDB.bulkSyncAckTbl, node)
		nDB.Unlock()

		logrus.Debugf("%s: Bulk sync to node %s took %s", nDB.config.NodeName, node, time.Now().Sub(startTime))
	}

	return nil
}

// Returns a random offset between 0 and n
func randomOffset(n int) int {
	if n == 0 {
		return 0
	}

	val, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		logrus.Errorf("Failed to get a random offset: %v", err)
		return 0
	}

	return int(val.Int64())
}

// mRandomNodes is used to select up to m random nodes. It is possible
// that less than m nodes are returned.
func (nDB *NetworkDB) mRandomNodes(m int, nodes []string) []string {
	n := len(nodes)
	mNodes := make([]string, 0, m)
OUTER:
	// Probe up to 3*n times, with large n this is not necessary
	// since k << n, but with small n we want search to be
	// exhaustive
	for i := 0; i < 3*n && len(mNodes) < m; i++ {
		// Get random node
		idx := randomOffset(n)
		node := nodes[idx]

		if node == nDB.config.NodeName {
			continue
		}

		// Check if we have this node already
		for j := 0; j < len(mNodes); j++ {
			if node == mNodes[j] {
				continue OUTER
			}
		}

		// Append the node
		mNodes = append(mNodes, node)
	}

	return mNodes
}
