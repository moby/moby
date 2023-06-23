package libnetwork

//go:generate protoc -I=. -I=../vendor/ --gogofaster_out=import_path=github.com/docker/docker/libnetwork:. agent.proto

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"sync"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/cluster"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/discoverapi"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/networkdb"
	"github.com/docker/docker/libnetwork/types"
	"github.com/docker/go-events"
	"github.com/gogo/protobuf/proto"
)

const (
	subsysGossip = "networking:gossip"
	subsysIPSec  = "networking:ipsec"
	keyringSize  = 3
)

// ByTime implements sort.Interface for []*types.EncryptionKey based on
// the LamportTime field.
type ByTime []*types.EncryptionKey

func (b ByTime) Len() int           { return len(b) }
func (b ByTime) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b ByTime) Less(i, j int) bool { return b[i].LamportTime < b[j].LamportTime }

type agent struct {
	networkDB         *networkdb.NetworkDB
	bindAddr          string
	advertiseAddr     string
	dataPathAddr      string
	coreCancelFuncs   []func()
	driverCancelFuncs map[string][]func()
	sync.Mutex
}

func (a *agent) dataPathAddress() string {
	a.Lock()
	defer a.Unlock()
	if a.dataPathAddr != "" {
		return a.dataPathAddr
	}
	return a.advertiseAddr
}

const libnetworkEPTable = "endpoint_table"

func getBindAddr(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("failed to find interface %s: %v", ifaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("failed to get interface addresses: %v", err)
	}

	for _, a := range addrs {
		addr, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		addrIP := addr.IP

		if addrIP.IsLinkLocalUnicast() {
			continue
		}

		return addrIP.String(), nil
	}

	return "", fmt.Errorf("failed to get bind address")
}

func resolveAddr(addrOrInterface string) (string, error) {
	// Try and see if this is a valid IP address
	if net.ParseIP(addrOrInterface) != nil {
		return addrOrInterface, nil
	}

	addr, err := net.ResolveIPAddr("ip", addrOrInterface)
	if err != nil {
		// If not a valid IP address, it should be a valid interface
		return getBindAddr(addrOrInterface)
	}
	return addr.String(), nil
}

func (c *Controller) handleKeyChange(keys []*types.EncryptionKey) error {
	drvEnc := discoverapi.DriverEncryptionUpdate{}

	a := c.getAgent()
	if a == nil {
		log.G(context.TODO()).Debug("Skipping key change as agent is nil")
		return nil
	}

	// Find the deleted key. If the deleted key was the primary key,
	// a new primary key should be set before removing if from keyring.
	c.mu.Lock()
	added := []byte{}
	deleted := []byte{}
	j := len(c.keys)
	for i := 0; i < j; {
		same := false
		for _, key := range keys {
			if same = key.LamportTime == c.keys[i].LamportTime; same {
				break
			}
		}
		if !same {
			cKey := c.keys[i]
			if cKey.Subsystem == subsysGossip {
				deleted = cKey.Key
			}

			if cKey.Subsystem == subsysIPSec {
				drvEnc.Prune = cKey.Key
				drvEnc.PruneTag = cKey.LamportTime
			}
			c.keys[i], c.keys[j-1] = c.keys[j-1], c.keys[i]
			c.keys[j-1] = nil
			j--
		}
		i++
	}
	c.keys = c.keys[:j]

	// Find the new key and add it to the key ring
	for _, key := range keys {
		same := false
		for _, cKey := range c.keys {
			if same = cKey.LamportTime == key.LamportTime; same {
				break
			}
		}
		if !same {
			c.keys = append(c.keys, key)
			if key.Subsystem == subsysGossip {
				added = key.Key
			}

			if key.Subsystem == subsysIPSec {
				drvEnc.Key = key.Key
				drvEnc.Tag = key.LamportTime
			}
		}
	}
	c.mu.Unlock()

	if len(added) > 0 {
		a.networkDB.SetKey(added)
	}

	key, _, err := c.getPrimaryKeyTag(subsysGossip)
	if err != nil {
		return err
	}
	a.networkDB.SetPrimaryKey(key)

	key, tag, err := c.getPrimaryKeyTag(subsysIPSec)
	if err != nil {
		return err
	}
	drvEnc.Primary = key
	drvEnc.PrimaryTag = tag

	if len(deleted) > 0 {
		a.networkDB.RemoveKey(deleted)
	}

	c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
		err := driver.DiscoverNew(discoverapi.EncryptionKeysUpdate, drvEnc)
		if err != nil {
			log.G(context.TODO()).Warnf("Failed to update datapath keys in driver %s: %v", name, err)
			// Attempt to reconfigure keys in case of a update failure
			// which can arise due to a mismatch of keys
			// if worker nodes get temporarily disconnected
			log.G(context.TODO()).Warnf("Reconfiguring datapath keys for  %s", name)
			drvCfgEnc := discoverapi.DriverEncryptionConfig{}
			drvCfgEnc.Keys, drvCfgEnc.Tags = c.getKeys(subsysIPSec)
			err = driver.DiscoverNew(discoverapi.EncryptionKeysConfig, drvCfgEnc)
			if err != nil {
				log.G(context.TODO()).Warnf("Failed to reset datapath keys in driver %s: %v", name, err)
			}
		}
		return false
	})

	return nil
}

func (c *Controller) agentSetup(clusterProvider cluster.Provider) error {
	agent := c.getAgent()

	// If the agent is already present there is no need to try to initialize it again
	if agent != nil {
		return nil
	}

	bindAddr := clusterProvider.GetLocalAddress()
	advAddr := clusterProvider.GetAdvertiseAddress()
	dataAddr := clusterProvider.GetDataPathAddress()
	remoteList := clusterProvider.GetRemoteAddressList()
	remoteAddrList := make([]string, 0, len(remoteList))
	for _, remote := range remoteList {
		addr, _, _ := net.SplitHostPort(remote)
		remoteAddrList = append(remoteAddrList, addr)
	}

	listen := clusterProvider.GetListenAddress()
	listenAddr, _, _ := net.SplitHostPort(listen)

	log.G(context.TODO()).Infof("Initializing Libnetwork Agent Listen-Addr=%s Local-addr=%s Adv-addr=%s Data-addr=%s Remote-addr-list=%v MTU=%d",
		listenAddr, bindAddr, advAddr, dataAddr, remoteAddrList, c.Config().NetworkControlPlaneMTU)
	if advAddr != "" && agent == nil {
		if err := c.agentInit(listenAddr, bindAddr, advAddr, dataAddr); err != nil {
			log.G(context.TODO()).Errorf("error in agentInit: %v", err)
			return err
		}
		c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
			if capability.ConnectivityScope == datastore.GlobalScope {
				c.agentDriverNotify(driver)
			}
			return false
		})
	}

	if len(remoteAddrList) > 0 {
		if err := c.agentJoin(remoteAddrList); err != nil {
			log.G(context.TODO()).Errorf("Error in joining gossip cluster : %v(join will be retried in background)", err)
		}
	}

	return nil
}

// For a given subsystem getKeys sorts the keys by lamport time and returns
// slice of keys and lamport time which can used as a unique tag for the keys
func (c *Controller) getKeys(subsys string) ([][]byte, []uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sort.Sort(ByTime(c.keys))

	keys := [][]byte{}
	tags := []uint64{}
	for _, key := range c.keys {
		if key.Subsystem == subsys {
			keys = append(keys, key.Key)
			tags = append(tags, key.LamportTime)
		}
	}

	keys[0], keys[1] = keys[1], keys[0]
	tags[0], tags[1] = tags[1], tags[0]
	return keys, tags
}

// getPrimaryKeyTag returns the primary key for a given subsystem from the
// list of sorted key and the associated tag
func (c *Controller) getPrimaryKeyTag(subsys string) ([]byte, uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sort.Sort(ByTime(c.keys))
	keys := []*types.EncryptionKey{}
	for _, key := range c.keys {
		if key.Subsystem == subsys {
			keys = append(keys, key)
		}
	}
	return keys[1].Key, keys[1].LamportTime, nil
}

func (c *Controller) agentInit(listenAddr, bindAddrOrInterface, advertiseAddr, dataPathAddr string) error {
	bindAddr, err := resolveAddr(bindAddrOrInterface)
	if err != nil {
		return err
	}

	keys, _ := c.getKeys(subsysGossip)

	netDBConf := networkdb.DefaultConfig()
	netDBConf.BindAddr = listenAddr
	netDBConf.AdvertiseAddr = advertiseAddr
	netDBConf.Keys = keys
	if c.Config().NetworkControlPlaneMTU != 0 {
		// Consider the MTU remove the IP hdr (IPv4 or IPv6) and the TCP/UDP hdr.
		// To be on the safe side let's cut 100 bytes
		netDBConf.PacketBufferSize = (c.Config().NetworkControlPlaneMTU - 100)
		log.G(context.TODO()).Debugf("Control plane MTU: %d will initialize NetworkDB with: %d",
			c.Config().NetworkControlPlaneMTU, netDBConf.PacketBufferSize)
	}
	nDB, err := networkdb.New(netDBConf)
	if err != nil {
		return err
	}

	// Register the diagnostic handlers
	c.DiagnosticServer.RegisterHandler(nDB, networkdb.NetDbPaths2Func)

	var cancelList []func()
	ch, cancel := nDB.Watch(libnetworkEPTable, "", "")
	cancelList = append(cancelList, cancel)
	nodeCh, cancel := nDB.Watch(networkdb.NodeTable, "", "")
	cancelList = append(cancelList, cancel)

	c.mu.Lock()
	c.agent = &agent{
		networkDB:         nDB,
		bindAddr:          bindAddr,
		advertiseAddr:     advertiseAddr,
		dataPathAddr:      dataPathAddr,
		coreCancelFuncs:   cancelList,
		driverCancelFuncs: make(map[string][]func()),
	}
	c.mu.Unlock()

	go c.handleTableEvents(ch, c.handleEpTableEvent)
	go c.handleTableEvents(nodeCh, c.handleNodeTableEvent)

	drvEnc := discoverapi.DriverEncryptionConfig{}
	keys, tags := c.getKeys(subsysIPSec)
	drvEnc.Keys = keys
	drvEnc.Tags = tags

	c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
		err := driver.DiscoverNew(discoverapi.EncryptionKeysConfig, drvEnc)
		if err != nil {
			log.G(context.TODO()).Warnf("Failed to set datapath keys in driver %s: %v", name, err)
		}
		return false
	})

	c.WalkNetworks(joinCluster)

	return nil
}

func (c *Controller) agentJoin(remoteAddrList []string) error {
	agent := c.getAgent()
	if agent == nil {
		return nil
	}
	return agent.networkDB.Join(remoteAddrList)
}

func (c *Controller) agentDriverNotify(d driverapi.Driver) {
	agent := c.getAgent()
	if agent == nil {
		return
	}

	if err := d.DiscoverNew(discoverapi.NodeDiscovery, discoverapi.NodeDiscoveryData{
		Address:     agent.dataPathAddress(),
		BindAddress: agent.bindAddr,
		Self:        true,
	}); err != nil {
		log.G(context.TODO()).Warnf("Failed the node discovery in driver: %v", err)
	}

	drvEnc := discoverapi.DriverEncryptionConfig{}
	keys, tags := c.getKeys(subsysIPSec)
	drvEnc.Keys = keys
	drvEnc.Tags = tags

	if err := d.DiscoverNew(discoverapi.EncryptionKeysConfig, drvEnc); err != nil {
		log.G(context.TODO()).Warnf("Failed to set datapath keys in driver: %v", err)
	}
}

func (c *Controller) agentClose() {
	// Acquire current agent instance and reset its pointer
	// then run closing functions
	c.mu.Lock()
	agent := c.agent
	c.agent = nil
	c.mu.Unlock()

	// when the agent is closed the cluster provider should be cleaned up
	c.SetClusterProvider(nil)

	if agent == nil {
		return
	}

	var cancelList []func()

	agent.Lock()
	for _, cancelFuncs := range agent.driverCancelFuncs {
		cancelList = append(cancelList, cancelFuncs...)
	}

	// Add also the cancel functions for the network db
	cancelList = append(cancelList, agent.coreCancelFuncs...)
	agent.Unlock()

	for _, cancel := range cancelList {
		cancel()
	}

	agent.networkDB.Close()
}

// Task has the backend container details
type Task struct {
	Name       string
	EndpointID string
	EndpointIP string
	Info       map[string]string
}

// ServiceInfo has service specific details along with the list of backend tasks
type ServiceInfo struct {
	VIP          string
	LocalLBIndex int
	Tasks        []Task
	Ports        []string
}

type epRecord struct {
	ep      EndpointRecord
	info    map[string]string
	lbIndex int
}

func (n *network) Services() map[string]ServiceInfo {
	eps := make(map[string]epRecord)

	if !n.isClusterEligible() {
		return nil
	}
	agent := n.getController().getAgent()
	if agent == nil {
		return nil
	}

	// Walk through libnetworkEPTable and fetch the driver agnostic endpoint info
	entries := agent.networkDB.GetTableByNetwork(libnetworkEPTable, n.id)
	for eid, value := range entries {
		var epRec EndpointRecord
		nid := n.ID()
		if err := proto.Unmarshal(value.Value, &epRec); err != nil {
			log.G(context.TODO()).Errorf("Unmarshal of libnetworkEPTable failed for endpoint %s in network %s, %v", eid, nid, err)
			continue
		}
		i := n.getController().getLBIndex(epRec.ServiceID, nid, epRec.IngressPorts)
		eps[eid] = epRecord{
			ep:      epRec,
			lbIndex: i,
		}
	}

	// Walk through the driver's tables, have the driver decode the entries
	// and return the tuple {ep ID, value}. value is a string that coveys
	// relevant info about the endpoint.
	d, err := n.driver(true)
	if err != nil {
		log.G(context.TODO()).Errorf("Could not resolve driver for network %s/%s while fetching services: %v", n.networkType, n.ID(), err)
		return nil
	}
	for _, table := range n.driverTables {
		if table.objType != driverapi.EndpointObject {
			continue
		}
		entries := agent.networkDB.GetTableByNetwork(table.name, n.id)
		for key, value := range entries {
			epID, info := d.DecodeTableEntry(table.name, key, value.Value)
			if ep, ok := eps[epID]; !ok {
				log.G(context.TODO()).Errorf("Inconsistent driver and libnetwork state for endpoint %s", epID)
			} else {
				ep.info = info
				eps[epID] = ep
			}
		}
	}

	// group the endpoints into a map keyed by the service name
	sinfo := make(map[string]ServiceInfo)
	for ep, epr := range eps {
		var (
			s  ServiceInfo
			ok bool
		)
		if s, ok = sinfo[epr.ep.ServiceName]; !ok {
			s = ServiceInfo{
				VIP:          epr.ep.VirtualIP,
				LocalLBIndex: epr.lbIndex,
			}
		}
		ports := []string{}
		if s.Ports == nil {
			for _, port := range epr.ep.IngressPorts {
				p := fmt.Sprintf("Target: %d, Publish: %d", port.TargetPort, port.PublishedPort)
				ports = append(ports, p)
			}
			s.Ports = ports
		}
		s.Tasks = append(s.Tasks, Task{
			Name:       epr.ep.Name,
			EndpointID: ep,
			EndpointIP: epr.ep.EndpointIP,
			Info:       epr.info,
		})
		sinfo[epr.ep.ServiceName] = s
	}
	return sinfo
}

func (n *network) isClusterEligible() bool {
	if n.scope != datastore.SwarmScope || !n.driverIsMultihost() {
		return false
	}
	return n.getController().getAgent() != nil
}

func (n *network) joinCluster() error {
	if !n.isClusterEligible() {
		return nil
	}

	agent := n.getController().getAgent()
	if agent == nil {
		return nil
	}

	return agent.networkDB.JoinNetwork(n.ID())
}

func (n *network) leaveCluster() error {
	if !n.isClusterEligible() {
		return nil
	}

	agent := n.getController().getAgent()
	if agent == nil {
		return nil
	}

	return agent.networkDB.LeaveNetwork(n.ID())
}

func (ep *Endpoint) addDriverInfoToCluster() error {
	n := ep.getNetwork()
	if !n.isClusterEligible() {
		return nil
	}
	if ep.joinInfo == nil {
		return nil
	}

	agent := n.getController().getAgent()
	if agent == nil {
		return nil
	}

	for _, te := range ep.joinInfo.driverTableEntries {
		if err := agent.networkDB.CreateEntry(te.tableName, n.ID(), te.key, te.value); err != nil {
			return err
		}
	}
	return nil
}

func (ep *Endpoint) deleteDriverInfoFromCluster() error {
	n := ep.getNetwork()
	if !n.isClusterEligible() {
		return nil
	}
	if ep.joinInfo == nil {
		return nil
	}

	agent := n.getController().getAgent()
	if agent == nil {
		return nil
	}

	for _, te := range ep.joinInfo.driverTableEntries {
		if err := agent.networkDB.DeleteEntry(te.tableName, n.ID(), te.key); err != nil {
			return err
		}
	}
	return nil
}

func (ep *Endpoint) addServiceInfoToCluster(sb *Sandbox) error {
	if ep.isAnonymous() && len(ep.myAliases) == 0 || ep.Iface() == nil || ep.Iface().Address() == nil {
		return nil
	}

	n := ep.getNetwork()
	if !n.isClusterEligible() {
		return nil
	}

	sb.service.Lock()
	defer sb.service.Unlock()
	log.G(context.TODO()).Debugf("addServiceInfoToCluster START for %s %s", ep.svcName, ep.ID())

	// Check that the endpoint is still present on the sandbox before adding it to the service discovery.
	// This is to handle a race between the EnableService and the sbLeave
	// It is possible that the EnableService starts, fetches the list of the endpoints and
	// by the time the addServiceInfoToCluster is called the endpoint got removed from the sandbox
	// The risk is that the deleteServiceInfoToCluster happens before the addServiceInfoToCluster.
	// This check under the Service lock of the sandbox ensure the correct behavior.
	// If the addServiceInfoToCluster arrives first may find or not the endpoint and will proceed or exit
	// but in any case the deleteServiceInfoToCluster will follow doing the cleanup if needed.
	// In case the deleteServiceInfoToCluster arrives first, this one is happening after the endpoint is
	// removed from the list, in this situation the delete will bail out not finding any data to cleanup
	// and the add will bail out not finding the endpoint on the sandbox.
	if e := sb.getEndpoint(ep.ID()); e == nil {
		log.G(context.TODO()).Warnf("addServiceInfoToCluster suppressing service resolution ep is not anymore in the sandbox %s", ep.ID())
		return nil
	}

	c := n.getController()
	agent := c.getAgent()

	name := ep.Name()
	if ep.isAnonymous() {
		name = ep.MyAliases()[0]
	}

	var ingressPorts []*PortConfig
	if ep.svcID != "" {
		// This is a task part of a service
		// Gossip ingress ports only in ingress network.
		if n.ingress {
			ingressPorts = ep.ingressPorts
		}
		if err := c.addServiceBinding(ep.svcName, ep.svcID, n.ID(), ep.ID(), name, ep.virtualIP, ingressPorts, ep.svcAliases, ep.myAliases, ep.Iface().Address().IP, "addServiceInfoToCluster"); err != nil {
			return err
		}
	} else {
		// This is a container simply attached to an attachable network
		if err := c.addContainerNameResolution(n.ID(), ep.ID(), name, ep.myAliases, ep.Iface().Address().IP, "addServiceInfoToCluster"); err != nil {
			return err
		}
	}

	buf, err := proto.Marshal(&EndpointRecord{
		Name:            name,
		ServiceName:     ep.svcName,
		ServiceID:       ep.svcID,
		VirtualIP:       ep.virtualIP.String(),
		IngressPorts:    ingressPorts,
		Aliases:         ep.svcAliases,
		TaskAliases:     ep.myAliases,
		EndpointIP:      ep.Iface().Address().IP.String(),
		ServiceDisabled: false,
	})
	if err != nil {
		return err
	}

	if agent != nil {
		if err := agent.networkDB.CreateEntry(libnetworkEPTable, n.ID(), ep.ID(), buf); err != nil {
			log.G(context.TODO()).Warnf("addServiceInfoToCluster NetworkDB CreateEntry failed for %s %s err:%s", ep.id, n.id, err)
			return err
		}
	}

	log.G(context.TODO()).Debugf("addServiceInfoToCluster END for %s %s", ep.svcName, ep.ID())

	return nil
}

func (ep *Endpoint) deleteServiceInfoFromCluster(sb *Sandbox, fullRemove bool, method string) error {
	if ep.isAnonymous() && len(ep.myAliases) == 0 {
		return nil
	}

	n := ep.getNetwork()
	if !n.isClusterEligible() {
		return nil
	}

	sb.service.Lock()
	defer sb.service.Unlock()
	log.G(context.TODO()).Debugf("deleteServiceInfoFromCluster from %s START for %s %s", method, ep.svcName, ep.ID())

	// Avoid a race w/ with a container that aborts preemptively.  This would
	// get caught in disableServceInNetworkDB, but we check here to make the
	// nature of the condition more clear.
	// See comment in addServiceInfoToCluster()
	if e := sb.getEndpoint(ep.ID()); e == nil {
		log.G(context.TODO()).Warnf("deleteServiceInfoFromCluster suppressing service resolution ep is not anymore in the sandbox %s", ep.ID())
		return nil
	}

	c := n.getController()
	agent := c.getAgent()

	name := ep.Name()
	if ep.isAnonymous() {
		name = ep.MyAliases()[0]
	}

	if agent != nil {
		// First update the networkDB then locally
		if fullRemove {
			if err := agent.networkDB.DeleteEntry(libnetworkEPTable, n.ID(), ep.ID()); err != nil {
				log.G(context.TODO()).Warnf("deleteServiceInfoFromCluster NetworkDB DeleteEntry failed for %s %s err:%s", ep.id, n.id, err)
			}
		} else {
			disableServiceInNetworkDB(agent, n, ep)
		}
	}

	if ep.Iface() != nil && ep.Iface().Address() != nil {
		if ep.svcID != "" {
			// This is a task part of a service
			var ingressPorts []*PortConfig
			if n.ingress {
				ingressPorts = ep.ingressPorts
			}
			if err := c.rmServiceBinding(ep.svcName, ep.svcID, n.ID(), ep.ID(), name, ep.virtualIP, ingressPorts, ep.svcAliases, ep.myAliases, ep.Iface().Address().IP, "deleteServiceInfoFromCluster", true, fullRemove); err != nil {
				return err
			}
		} else {
			// This is a container simply attached to an attachable network
			if err := c.delContainerNameResolution(n.ID(), ep.ID(), name, ep.myAliases, ep.Iface().Address().IP, "deleteServiceInfoFromCluster"); err != nil {
				return err
			}
		}
	}

	log.G(context.TODO()).Debugf("deleteServiceInfoFromCluster from %s END for %s %s", method, ep.svcName, ep.ID())

	return nil
}

func disableServiceInNetworkDB(a *agent, n *network, ep *Endpoint) {
	var epRec EndpointRecord

	log.G(context.TODO()).Debugf("disableServiceInNetworkDB for %s %s", ep.svcName, ep.ID())

	// Update existing record to indicate that the service is disabled
	inBuf, err := a.networkDB.GetEntry(libnetworkEPTable, n.ID(), ep.ID())
	if err != nil {
		log.G(context.TODO()).Warnf("disableServiceInNetworkDB GetEntry failed for %s %s err:%s", ep.id, n.id, err)
		return
	}
	// Should never fail
	if err := proto.Unmarshal(inBuf, &epRec); err != nil {
		log.G(context.TODO()).Errorf("disableServiceInNetworkDB unmarshal failed for %s %s err:%s", ep.id, n.id, err)
		return
	}
	epRec.ServiceDisabled = true
	// Should never fail
	outBuf, err := proto.Marshal(&epRec)
	if err != nil {
		log.G(context.TODO()).Errorf("disableServiceInNetworkDB marshalling failed for %s %s err:%s", ep.id, n.id, err)
		return
	}
	// Send update to the whole cluster
	if err := a.networkDB.UpdateEntry(libnetworkEPTable, n.ID(), ep.ID(), outBuf); err != nil {
		log.G(context.TODO()).Warnf("disableServiceInNetworkDB UpdateEntry failed for %s %s err:%s", ep.id, n.id, err)
	}
}

func (n *network) addDriverWatches() {
	if !n.isClusterEligible() {
		return
	}

	c := n.getController()
	agent := c.getAgent()
	if agent == nil {
		return
	}
	for _, table := range n.driverTables {
		ch, cancel := agent.networkDB.Watch(table.name, n.ID(), "")
		agent.Lock()
		agent.driverCancelFuncs[n.ID()] = append(agent.driverCancelFuncs[n.ID()], cancel)
		agent.Unlock()
		go c.handleTableEvents(ch, n.handleDriverTableEvent)
		d, err := n.driver(false)
		if err != nil {
			log.G(context.TODO()).Errorf("Could not resolve driver %s while walking driver tabl: %v", n.networkType, err)
			return
		}

		err = agent.networkDB.WalkTable(table.name, func(nid, key string, value []byte, deleted bool) bool {
			// skip the entries that are mark for deletion, this is safe because this function is
			// called at initialization time so there is no state to delete
			if nid == n.ID() && !deleted {
				d.EventNotify(driverapi.Create, nid, table.name, key, value)
			}
			return false
		})
		if err != nil {
			log.G(context.TODO()).WithError(err).Warn("Error while walking networkdb")
		}
	}
}

func (n *network) cancelDriverWatches() {
	if !n.isClusterEligible() {
		return
	}

	agent := n.getController().getAgent()
	if agent == nil {
		return
	}

	agent.Lock()
	cancelFuncs := agent.driverCancelFuncs[n.ID()]
	delete(agent.driverCancelFuncs, n.ID())
	agent.Unlock()

	for _, cancel := range cancelFuncs {
		cancel()
	}
}

func (c *Controller) handleTableEvents(ch *events.Channel, fn func(events.Event)) {
	for {
		select {
		case ev := <-ch.C:
			fn(ev)
		case <-ch.Done():
			return
		}
	}
}

func (n *network) handleDriverTableEvent(ev events.Event) {
	d, err := n.driver(false)
	if err != nil {
		log.G(context.TODO()).Errorf("Could not resolve driver %s while handling driver table event: %v", n.networkType, err)
		return
	}

	var (
		etype driverapi.EventType
		tname string
		key   string
		value []byte
	)

	switch event := ev.(type) {
	case networkdb.CreateEvent:
		tname = event.Table
		key = event.Key
		value = event.Value
		etype = driverapi.Create
	case networkdb.DeleteEvent:
		tname = event.Table
		key = event.Key
		value = event.Value
		etype = driverapi.Delete
	case networkdb.UpdateEvent:
		tname = event.Table
		key = event.Key
		value = event.Value
		etype = driverapi.Delete
	}

	d.EventNotify(etype, n.ID(), tname, key, value)
}

func (c *Controller) handleNodeTableEvent(ev events.Event) {
	var (
		value    []byte
		isAdd    bool
		nodeAddr networkdb.NodeAddr
	)
	switch event := ev.(type) {
	case networkdb.CreateEvent:
		value = event.Value
		isAdd = true
	case networkdb.DeleteEvent:
		value = event.Value
	case networkdb.UpdateEvent:
		log.G(context.TODO()).Errorf("Unexpected update node table event = %#v", event)
	}

	err := json.Unmarshal(value, &nodeAddr)
	if err != nil {
		log.G(context.TODO()).Errorf("Error unmarshalling node table event %v", err)
		return
	}
	c.processNodeDiscovery([]net.IP{nodeAddr.Addr}, isAdd)
}

func (c *Controller) handleEpTableEvent(ev events.Event) {
	var (
		nid   string
		eid   string
		value []byte
		epRec EndpointRecord
	)

	switch event := ev.(type) {
	case networkdb.CreateEvent:
		nid = event.NetworkID
		eid = event.Key
		value = event.Value
	case networkdb.DeleteEvent:
		nid = event.NetworkID
		eid = event.Key
		value = event.Value
	case networkdb.UpdateEvent:
		nid = event.NetworkID
		eid = event.Key
		value = event.Value
	default:
		log.G(context.TODO()).Errorf("Unexpected update service table event = %#v", event)
		return
	}

	err := proto.Unmarshal(value, &epRec)
	if err != nil {
		log.G(context.TODO()).Errorf("Failed to unmarshal service table value: %v", err)
		return
	}

	containerName := epRec.Name
	svcName := epRec.ServiceName
	svcID := epRec.ServiceID
	vip := net.ParseIP(epRec.VirtualIP)
	ip := net.ParseIP(epRec.EndpointIP)
	ingressPorts := epRec.IngressPorts
	serviceAliases := epRec.Aliases
	taskAliases := epRec.TaskAliases

	if containerName == "" || ip == nil {
		log.G(context.TODO()).Errorf("Invalid endpoint name/ip received while handling service table event %s", value)
		return
	}

	switch ev.(type) {
	case networkdb.CreateEvent:
		log.G(context.TODO()).Debugf("handleEpTableEvent ADD %s R:%v", eid, epRec)
		if svcID != "" {
			// This is a remote task part of a service
			if err := c.addServiceBinding(svcName, svcID, nid, eid, containerName, vip, ingressPorts, serviceAliases, taskAliases, ip, "handleEpTableEvent"); err != nil {
				log.G(context.TODO()).Errorf("failed adding service binding for %s epRec:%v err:%v", eid, epRec, err)
				return
			}
		} else {
			// This is a remote container simply attached to an attachable network
			if err := c.addContainerNameResolution(nid, eid, containerName, taskAliases, ip, "handleEpTableEvent"); err != nil {
				log.G(context.TODO()).Errorf("failed adding container name resolution for %s epRec:%v err:%v", eid, epRec, err)
			}
		}

	case networkdb.DeleteEvent:
		log.G(context.TODO()).Debugf("handleEpTableEvent DEL %s R:%v", eid, epRec)
		if svcID != "" {
			// This is a remote task part of a service
			if err := c.rmServiceBinding(svcName, svcID, nid, eid, containerName, vip, ingressPorts, serviceAliases, taskAliases, ip, "handleEpTableEvent", true, true); err != nil {
				log.G(context.TODO()).Errorf("failed removing service binding for %s epRec:%v err:%v", eid, epRec, err)
				return
			}
		} else {
			// This is a remote container simply attached to an attachable network
			if err := c.delContainerNameResolution(nid, eid, containerName, taskAliases, ip, "handleEpTableEvent"); err != nil {
				log.G(context.TODO()).Errorf("failed removing container name resolution for %s epRec:%v err:%v", eid, epRec, err)
			}
		}
	case networkdb.UpdateEvent:
		log.G(context.TODO()).Debugf("handleEpTableEvent UPD %s R:%v", eid, epRec)
		// We currently should only get these to inform us that an endpoint
		// is disabled.  Report if otherwise.
		if svcID == "" || !epRec.ServiceDisabled {
			log.G(context.TODO()).Errorf("Unexpected update table event for %s epRec:%v", eid, epRec)
			return
		}
		// This is a remote task that is part of a service that is now disabled
		if err := c.rmServiceBinding(svcName, svcID, nid, eid, containerName, vip, ingressPorts, serviceAliases, taskAliases, ip, "handleEpTableEvent", true, false); err != nil {
			log.G(context.TODO()).Errorf("failed disabling service binding for %s epRec:%v err:%v", eid, epRec, err)
			return
		}
	}
}
