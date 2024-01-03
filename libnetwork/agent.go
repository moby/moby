package libnetwork

//go:generate protoc -I=. -I=../vendor/ --gogofaster_out=import_path=github.com/docker/docker/libnetwork:. agent.proto

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"sync"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/cluster"
	"github.com/docker/docker/libnetwork/discoverapi"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/networkdb"
	"github.com/docker/docker/libnetwork/scope"
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

type nwAgent struct {
	networkDB         *networkdb.NetworkDB
	bindAddr          net.IP
	advertiseAddr     string
	dataPathAddr      string
	coreCancelFuncs   []func()
	driverCancelFuncs map[string][]func()
	mu                sync.Mutex
}

func (a *nwAgent) dataPathAddress() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dataPathAddr != "" {
		return a.dataPathAddr
	}
	return a.advertiseAddr
}

const libnetworkEPTable = "endpoint_table"

func getBindAddr(ifaceName string) (net.IP, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %v", ifaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface addresses: %v", err)
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

		return addrIP, nil
	}

	return nil, fmt.Errorf("failed to get bind address")
}

// resolveAddr resolves the given address, which can be one of, and
// parsed in the following order or priority:
//
// - a well-formed IP-address
// - a hostname
// - an interface-name
func resolveAddr(addrOrInterface string) (net.IP, error) {
	// Try and see if this is a valid IP address
	if ip := net.ParseIP(addrOrInterface); ip != nil {
		return ip, nil
	}

	// If not a valid IP address, it could be a hostname.
	addr, err := net.ResolveIPAddr("ip", addrOrInterface)
	if err != nil {
		// If hostname lookup failed, try to look for an interface with the given name.
		return getBindAddr(addrOrInterface)
	}
	return addr.IP, nil
}

func (c *Controller) handleKeyChange(keys []*types.EncryptionKey) error {
	drvEnc := discoverapi.DriverEncryptionUpdate{}

	agent := c.getAgent()
	if agent == nil {
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
		agent.networkDB.SetKey(added)
	}

	key, _, err := c.getPrimaryKeyTag(subsysGossip)
	if err != nil {
		return err
	}
	agent.networkDB.SetPrimaryKey(key)

	key, tag, err := c.getPrimaryKeyTag(subsysIPSec)
	if err != nil {
		return err
	}
	drvEnc.Primary = key
	drvEnc.PrimaryTag = tag

	if len(deleted) > 0 {
		agent.networkDB.RemoveKey(deleted)
	}

	c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
		dr, ok := driver.(discoverapi.Discover)
		if !ok {
			return false
		}
		if err := dr.DiscoverNew(discoverapi.EncryptionKeysUpdate, drvEnc); err != nil {
			log.G(context.TODO()).Warnf("Failed to update datapath keys in driver %s: %v", name, err)
			// Attempt to reconfigure keys in case of a update failure
			// which can arise due to a mismatch of keys
			// if worker nodes get temporarily disconnected
			log.G(context.TODO()).Warnf("Reconfiguring datapath keys for  %s", name)
			drvCfgEnc := discoverapi.DriverEncryptionConfig{}
			drvCfgEnc.Keys, drvCfgEnc.Tags = c.getKeys(subsysIPSec)
			err = dr.DiscoverNew(discoverapi.EncryptionKeysConfig, drvCfgEnc)
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
	if agent != nil {
		// agent is already present, so there is no need initialize it again.
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

	log.G(context.TODO()).WithFields(log.Fields{
		"listen-addr":               listenAddr,
		"local-addr":                bindAddr,
		"advertise-addr":            advAddr,
		"data-path-addr":            dataAddr,
		"remote-addr-list":          remoteAddrList,
		"network-control-plane-mtu": c.Config().NetworkControlPlaneMTU,
	}).Info("Initializing Libnetwork Agent")
	if advAddr != "" {
		if err := c.agentInit(listenAddr, bindAddr, advAddr, dataAddr); err != nil {
			log.G(context.TODO()).WithError(err).Errorf("Error in agentInit")
			return err
		}
		c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
			if capability.ConnectivityScope == scope.Global {
				if d, ok := driver.(discoverapi.Discover); ok {
					c.agentDriverNotify(d)
				}
			}
			return false
		})
	}

	if len(remoteAddrList) > 0 {
		if err := c.agentJoin(remoteAddrList); err != nil {
			log.G(context.TODO()).WithError(err).Error("Error in joining gossip cluster: join will be retried in background")
		}
	}

	return nil
}

// For a given subsystem getKeys sorts the keys by lamport time and returns
// slice of keys and lamport time which can used as a unique tag for the keys
func (c *Controller) getKeys(subsystem string) (keys [][]byte, tags []uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sort.Sort(ByTime(c.keys))

	keys = make([][]byte, 0, len(c.keys))
	tags = make([]uint64, 0, len(c.keys))
	for _, key := range c.keys {
		if key.Subsystem == subsystem {
			keys = append(keys, key.Key)
			tags = append(tags, key.LamportTime)
		}
	}

	if len(keys) > 1 {
		// TODO(thaJeztah): why are we swapping order here? This code was added in https://github.com/moby/libnetwork/commit/e83d68b7d1fd9c479120914024242238f791b4dc
		keys[0], keys[1] = keys[1], keys[0]
		tags[0], tags[1] = tags[1], tags[0]
	}
	return keys, tags
}

// getPrimaryKeyTag returns the primary key for a given subsystem from the
// list of sorted key and the associated tag
func (c *Controller) getPrimaryKeyTag(subsystem string) (key []byte, lamportTime uint64, _ error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sort.Sort(ByTime(c.keys))
	keys := make([]*types.EncryptionKey, 0, len(c.keys))
	for _, k := range c.keys {
		if k.Subsystem == subsystem {
			keys = append(keys, k)
		}
	}
	if len(keys) < 2 {
		return nil, 0, fmt.Errorf("no primary key found for %s subsystem: %d keys found on controller, expected at least 2", subsystem, len(keys))
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
	nDB.RegisterDiagnosticHandlers(c.DiagnosticServer)

	var cancelList []func()
	ch, cancel := nDB.Watch(libnetworkEPTable, "")
	cancelList = append(cancelList, cancel)
	nodeCh, cancel := nDB.Watch(networkdb.NodeTable, "")
	cancelList = append(cancelList, cancel)

	c.mu.Lock()
	c.agent = &nwAgent{
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

	keys, tags := c.getKeys(subsysIPSec)
	c.drvRegistry.WalkDrivers(func(name string, driver driverapi.Driver, capability driverapi.Capability) bool {
		if dr, ok := driver.(discoverapi.Discover); ok {
			if err := dr.DiscoverNew(discoverapi.EncryptionKeysConfig, discoverapi.DriverEncryptionConfig{
				Keys: keys,
				Tags: tags,
			}); err != nil {
				log.G(context.TODO()).Warnf("Failed to set datapath keys in driver %s: %v", name, err)
			}
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

func (c *Controller) agentDriverNotify(d discoverapi.Discover) {
	agent := c.getAgent()
	if agent == nil {
		return
	}

	if err := d.DiscoverNew(discoverapi.NodeDiscovery, discoverapi.NodeDiscoveryData{
		Address:     agent.dataPathAddress(),
		BindAddress: agent.bindAddr.String(),
		Self:        true,
	}); err != nil {
		log.G(context.TODO()).Warnf("Failed the node discovery in driver: %v", err)
	}

	keys, tags := c.getKeys(subsysIPSec)
	if err := d.DiscoverNew(discoverapi.EncryptionKeysConfig, discoverapi.DriverEncryptionConfig{
		Keys: keys,
		Tags: tags,
	}); err != nil {
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

	agent.mu.Lock()
	for _, cancelFuncs := range agent.driverCancelFuncs {
		cancelList = append(cancelList, cancelFuncs...)
	}

	// Add also the cancel functions for the network db
	cancelList = append(cancelList, agent.coreCancelFuncs...)
	agent.mu.Unlock()

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

// Services returns a map of services keyed by the service name with the details
// of all the tasks that belong to the service. Applicable only in swarm mode.
func (n *Network) Services() map[string]ServiceInfo {
	agent, ok := n.clusterAgent()
	if !ok {
		return nil
	}
	nwID := n.ID()
	d, err := n.driver(true)
	if err != nil {
		log.G(context.TODO()).Errorf("Could not resolve driver for network %s/%s while fetching services: %v", n.networkType, nwID, err)
		return nil
	}

	// Walk through libnetworkEPTable and fetch the driver agnostic endpoint info
	eps := make(map[string]epRecord)
	c := n.getController()
	for eid, value := range agent.networkDB.GetTableByNetwork(libnetworkEPTable, nwID) {
		var epRec EndpointRecord
		if err := proto.Unmarshal(value.Value, &epRec); err != nil {
			log.G(context.TODO()).Errorf("Unmarshal of libnetworkEPTable failed for endpoint %s in network %s, %v", eid, nwID, err)
			continue
		}
		eps[eid] = epRecord{
			ep:      epRec,
			lbIndex: c.getLBIndex(epRec.ServiceID, nwID, epRec.IngressPorts),
		}
	}

	// Walk through the driver's tables, have the driver decode the entries
	// and return the tuple {ep ID, value}. value is a string that coveys
	// relevant info about the endpoint.
	for _, table := range n.driverTables {
		if table.objType != driverapi.EndpointObject {
			continue
		}
		for key, value := range agent.networkDB.GetTableByNetwork(table.name, nwID) {
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
		s, ok := sinfo[epr.ep.ServiceName]
		if !ok {
			s = ServiceInfo{
				VIP:          epr.ep.VirtualIP,
				LocalLBIndex: epr.lbIndex,
			}
		}
		if s.Ports == nil {
			ports := make([]string, 0, len(epr.ep.IngressPorts))
			for _, port := range epr.ep.IngressPorts {
				ports = append(ports, fmt.Sprintf("Target: %d, Publish: %d", port.TargetPort, port.PublishedPort))
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

// clusterAgent returns the cluster agent if the network is a swarm-scoped,
// multi-host network.
func (n *Network) clusterAgent() (agent *nwAgent, ok bool) {
	if n.scope != scope.Swarm || !n.driverIsMultihost() {
		return nil, false
	}
	a := n.getController().getAgent()
	return a, a != nil
}

func (n *Network) joinCluster() error {
	agent, ok := n.clusterAgent()
	if !ok {
		return nil
	}
	return agent.networkDB.JoinNetwork(n.ID())
}

func (n *Network) leaveCluster() error {
	agent, ok := n.clusterAgent()
	if !ok {
		return nil
	}
	return agent.networkDB.LeaveNetwork(n.ID())
}

func (ep *Endpoint) addDriverInfoToCluster() error {
	if ep.joinInfo == nil || len(ep.joinInfo.driverTableEntries) == 0 {
		return nil
	}
	n := ep.getNetwork()
	agent, ok := n.clusterAgent()
	if !ok {
		return nil
	}

	nwID := n.ID()
	for _, te := range ep.joinInfo.driverTableEntries {
		if err := agent.networkDB.CreateEntry(te.tableName, nwID, te.key, te.value); err != nil {
			return err
		}
	}
	return nil
}

func (ep *Endpoint) deleteDriverInfoFromCluster() error {
	if ep.joinInfo == nil || len(ep.joinInfo.driverTableEntries) == 0 {
		return nil
	}
	n := ep.getNetwork()
	agent, ok := n.clusterAgent()
	if !ok {
		return nil
	}

	nwID := n.ID()
	for _, te := range ep.joinInfo.driverTableEntries {
		if err := agent.networkDB.DeleteEntry(te.tableName, nwID, te.key); err != nil {
			return err
		}
	}
	return nil
}

func (ep *Endpoint) addServiceInfoToCluster(sb *Sandbox) error {
	if len(ep.dnsNames) == 0 || ep.Iface() == nil || ep.Iface().Address() == nil {
		return nil
	}

	n := ep.getNetwork()
	agent, ok := n.clusterAgent()
	if !ok {
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
	if err := sb.getEndpoint(ep.ID()); err == nil {
		log.G(context.TODO()).Warnf("addServiceInfoToCluster suppressing service resolution ep is not anymore in the sandbox %s", ep.ID())
		return nil
	}

	dnsNames := ep.getDNSNames()
	primaryDNSName, dnsAliases := dnsNames[0], dnsNames[1:]

	var ingressPorts []*PortConfig
	if ep.svcID != "" {
		// This is a task part of a service
		// Gossip ingress ports only in ingress network.
		if n.ingress {
			ingressPorts = ep.ingressPorts
		}
		if err := n.getController().addServiceBinding(ep.svcName, ep.svcID, n.ID(), ep.ID(), primaryDNSName, ep.virtualIP, ingressPorts, ep.svcAliases, dnsAliases, ep.Iface().Address().IP, "addServiceInfoToCluster"); err != nil {
			return err
		}
	} else {
		// This is a container simply attached to an attachable network
		if err := n.getController().addContainerNameResolution(n.ID(), ep.ID(), primaryDNSName, dnsAliases, ep.Iface().Address().IP, "addServiceInfoToCluster"); err != nil {
			return err
		}
	}

	buf, err := proto.Marshal(&EndpointRecord{
		Name:            primaryDNSName,
		ServiceName:     ep.svcName,
		ServiceID:       ep.svcID,
		VirtualIP:       ep.virtualIP.String(),
		IngressPorts:    ingressPorts,
		Aliases:         ep.svcAliases,
		TaskAliases:     dnsAliases,
		EndpointIP:      ep.Iface().Address().IP.String(),
		ServiceDisabled: false,
	})
	if err != nil {
		return err
	}

	if err := agent.networkDB.CreateEntry(libnetworkEPTable, n.ID(), ep.ID(), buf); err != nil {
		log.G(context.TODO()).Warnf("addServiceInfoToCluster NetworkDB CreateEntry failed for %s %s err:%s", ep.id, n.id, err)
		return err
	}

	log.G(context.TODO()).Debugf("addServiceInfoToCluster END for %s %s", ep.svcName, ep.ID())

	return nil
}

func (ep *Endpoint) deleteServiceInfoFromCluster(sb *Sandbox, fullRemove bool, method string) error {
	if len(ep.dnsNames) == 0 {
		return nil
	}

	n := ep.getNetwork()
	agent, ok := n.clusterAgent()
	if !ok {
		return nil
	}

	sb.service.Lock()
	defer sb.service.Unlock()
	log.G(context.TODO()).Debugf("deleteServiceInfoFromCluster from %s START for %s %s", method, ep.svcName, ep.ID())

	// Avoid a race w/ with a container that aborts preemptively.  This would
	// get caught in disableServceInNetworkDB, but we check here to make the
	// nature of the condition more clear.
	// See comment in addServiceInfoToCluster()
	if err := sb.getEndpoint(ep.ID()); err == nil {
		log.G(context.TODO()).Warnf("deleteServiceInfoFromCluster suppressing service resolution ep is not anymore in the sandbox %s", ep.ID())
		return nil
	}

	dnsNames := ep.getDNSNames()
	primaryDNSName, dnsAliases := dnsNames[0], dnsNames[1:]

	// First update the networkDB then locally
	if fullRemove {
		if err := agent.networkDB.DeleteEntry(libnetworkEPTable, n.ID(), ep.ID()); err != nil {
			log.G(context.TODO()).Warnf("deleteServiceInfoFromCluster NetworkDB DeleteEntry failed for %s %s err:%s", ep.id, n.id, err)
		}
	} else {
		disableServiceInNetworkDB(agent, n, ep)
	}

	if ep.Iface() != nil && ep.Iface().Address() != nil {
		if ep.svcID != "" {
			// This is a task part of a service
			var ingressPorts []*PortConfig
			if n.ingress {
				ingressPorts = ep.ingressPorts
			}
			if err := n.getController().rmServiceBinding(ep.svcName, ep.svcID, n.ID(), ep.ID(), primaryDNSName, ep.virtualIP, ingressPorts, ep.svcAliases, dnsAliases, ep.Iface().Address().IP, "deleteServiceInfoFromCluster", true, fullRemove); err != nil {
				return err
			}
		} else {
			// This is a container simply attached to an attachable network
			if err := n.getController().delContainerNameResolution(n.ID(), ep.ID(), primaryDNSName, dnsAliases, ep.Iface().Address().IP, "deleteServiceInfoFromCluster"); err != nil {
				return err
			}
		}
	}

	log.G(context.TODO()).Debugf("deleteServiceInfoFromCluster from %s END for %s %s", method, ep.svcName, ep.ID())

	return nil
}

func disableServiceInNetworkDB(a *nwAgent, n *Network, ep *Endpoint) {
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

func (n *Network) addDriverWatches() {
	if len(n.driverTables) == 0 {
		return
	}
	agent, ok := n.clusterAgent()
	if !ok {
		return
	}

	c := n.getController()
	for _, table := range n.driverTables {
		ch, cancel := agent.networkDB.Watch(table.name, n.ID())
		agent.mu.Lock()
		agent.driverCancelFuncs[n.ID()] = append(agent.driverCancelFuncs[n.ID()], cancel)
		agent.mu.Unlock()
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

func (n *Network) cancelDriverWatches() {
	agent, ok := n.clusterAgent()
	if !ok {
		return
	}

	agent.mu.Lock()
	cancelFuncs := agent.driverCancelFuncs[n.ID()]
	delete(agent.driverCancelFuncs, n.ID())
	agent.mu.Unlock()

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

func (n *Network) handleDriverTableEvent(ev events.Event) {
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
