package overlay

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/ns"
	"github.com/docker/libnetwork/osl"
	"github.com/docker/libnetwork/resolvconf"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
)

var (
	hostMode    bool
	networkOnce sync.Once
	networkMu   sync.Mutex
	vniTbl      = make(map[uint32]string)
)

type networkTable map[string]*network

type subnet struct {
	once      *sync.Once
	vxlanName string
	brName    string
	vni       uint32
	initErr   error
	subnetIP  *net.IPNet
	gwIP      *net.IPNet
}

type subnetJSON struct {
	SubnetIP string
	GwIP     string
	Vni      uint32
}

type network struct {
	id        string
	dbIndex   uint64
	dbExists  bool
	sbox      osl.Sandbox
	endpoints endpointTable
	driver    *driver
	joinCnt   int
	once      *sync.Once
	initEpoch int
	initErr   error
	subnets   []*subnet
	secure    bool
	mtu       int
	sync.Mutex
}

func (d *driver) NetworkAllocate(id string, option map[string]string, ipV4Data, ipV6Data []driverapi.IPAMData) (map[string]string, error) {
	return nil, types.NotImplementedErrorf("not implemented")
}

func (d *driver) NetworkFree(id string) error {
	return types.NotImplementedErrorf("not implemented")
}

func (d *driver) CreateNetwork(id string, option map[string]interface{}, nInfo driverapi.NetworkInfo, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	if id == "" {
		return fmt.Errorf("invalid network id")
	}
	if len(ipV4Data) == 0 || ipV4Data[0].Pool.String() == "0.0.0.0/0" {
		return types.BadRequestErrorf("ipv4 pool is empty")
	}

	// Since we perform lazy configuration make sure we try
	// configuring the driver when we enter CreateNetwork
	if err := d.configure(); err != nil {
		return err
	}

	n := &network{
		id:        id,
		driver:    d,
		endpoints: endpointTable{},
		once:      &sync.Once{},
		subnets:   []*subnet{},
	}

	vnis := make([]uint32, 0, len(ipV4Data))
	if gval, ok := option[netlabel.GenericData]; ok {
		optMap := gval.(map[string]string)
		if val, ok := optMap[netlabel.OverlayVxlanIDList]; ok {
			logrus.Debugf("overlay: Received vxlan IDs: %s", val)
			vniStrings := strings.Split(val, ",")
			for _, vniStr := range vniStrings {
				vni, err := strconv.Atoi(vniStr)
				if err != nil {
					return fmt.Errorf("invalid vxlan id value %q passed", vniStr)
				}

				vnis = append(vnis, uint32(vni))
			}
		}
		if _, ok := optMap[secureOption]; ok {
			n.secure = true
		}
		if val, ok := optMap[netlabel.DriverMTU]; ok {
			var err error
			if n.mtu, err = strconv.Atoi(val); err != nil {
				return fmt.Errorf("failed to parse %v: %v", val, err)
			}
			if n.mtu < 0 {
				return fmt.Errorf("invalid MTU value: %v", n.mtu)
			}
		}
	}

	// If we are getting vnis from libnetwork, either we get for
	// all subnets or none.
	if len(vnis) != 0 && len(vnis) < len(ipV4Data) {
		return fmt.Errorf("insufficient vnis(%d) passed to overlay", len(vnis))
	}

	for i, ipd := range ipV4Data {
		s := &subnet{
			subnetIP: ipd.Pool,
			gwIP:     ipd.Gateway,
			once:     &sync.Once{},
		}

		if len(vnis) != 0 {
			s.vni = vnis[i]
		}

		n.subnets = append(n.subnets, s)
	}

	if err := n.writeToStore(); err != nil {
		return fmt.Errorf("failed to update data store for network %v: %v", n.id, err)
	}

	// Make sure no rule is on the way from any stale secure network
	if !n.secure {
		for _, vni := range vnis {
			programMangle(vni, false)
		}
	}

	if nInfo != nil {
		if err := nInfo.TableEventRegister(ovPeerTable); err != nil {
			return err
		}
	}

	d.addNetwork(n)
	return nil
}

func (d *driver) DeleteNetwork(nid string) error {
	if nid == "" {
		return fmt.Errorf("invalid network id")
	}

	// Make sure driver resources are initialized before proceeding
	if err := d.configure(); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("could not find network with id %s", nid)
	}

	for _, ep := range n.endpoints {
		if ep.ifName != "" {
			if link, err := ns.NlHandle().LinkByName(ep.ifName); err != nil {
				ns.NlHandle().LinkDel(link)
			}
		}

		if err := d.deleteEndpointFromStore(ep); err != nil {
			logrus.Warnf("Failed to delete overlay endpoint %s from local store: %v", ep.id[0:7], err)
		}

	}
	d.deleteNetwork(nid)

	vnis, err := n.releaseVxlanID()
	if err != nil {
		return err
	}

	if n.secure {
		for _, vni := range vnis {
			programMangle(vni, false)
		}
	}

	return nil
}

func (d *driver) ProgramExternalConnectivity(nid, eid string, options map[string]interface{}) error {
	return nil
}

func (d *driver) RevokeExternalConnectivity(nid, eid string) error {
	return nil
}

func (n *network) incEndpointCount() {
	n.Lock()
	defer n.Unlock()
	n.joinCnt++
}

func (n *network) joinSandbox(restore bool) error {
	// If there is a race between two go routines here only one will win
	// the other will wait.
	n.once.Do(func() {
		// save the error status of initSandbox in n.initErr so that
		// all the racing go routines are able to know the status.
		n.initErr = n.initSandbox(restore)
	})

	return n.initErr
}

func (n *network) joinSubnetSandbox(s *subnet, restore bool) error {
	s.once.Do(func() {
		s.initErr = n.initSubnetSandbox(s, restore)
	})
	return s.initErr
}

func (n *network) leaveSandbox() {
	n.Lock()
	defer n.Unlock()
	n.joinCnt--
	if n.joinCnt != 0 {
		return
	}

	// We are about to destroy sandbox since the container is leaving the network
	// Reinitialize the once variable so that we will be able to trigger one time
	// sandbox initialization(again) when another container joins subsequently.
	n.once = &sync.Once{}
	for _, s := range n.subnets {
		s.once = &sync.Once{}
	}

	n.destroySandbox()
}

// to be called while holding network lock
func (n *network) destroySandbox() {
	if n.sbox != nil {
		for _, iface := range n.sbox.Info().Interfaces() {
			if err := iface.Remove(); err != nil {
				logrus.Debugf("Remove interface %s failed: %v", iface.SrcName(), err)
			}
		}

		for _, s := range n.subnets {
			if hostMode {
				if err := removeFilters(n.id[:12], s.brName); err != nil {
					logrus.Warnf("Could not remove overlay filters: %v", err)
				}
			}

			if s.vxlanName != "" {
				err := deleteInterface(s.vxlanName)
				if err != nil {
					logrus.Warnf("could not cleanup sandbox properly: %v", err)
				}
			}
		}

		if hostMode {
			if err := removeNetworkChain(n.id[:12]); err != nil {
				logrus.Warnf("could not remove network chain: %v", err)
			}
		}

		n.sbox.Destroy()
		n.sbox = nil
	}
}

func populateVNITbl() {
	filepath.Walk(filepath.Dir(osl.GenerateKey("walk")),
		func(path string, info os.FileInfo, err error) error {
			_, fname := filepath.Split(path)

			if len(strings.Split(fname, "-")) <= 1 {
				return nil
			}

			ns, err := netns.GetFromPath(path)
			if err != nil {
				logrus.Errorf("Could not open namespace path %s during vni population: %v", path, err)
				return nil
			}
			defer ns.Close()

			nlh, err := netlink.NewHandleAt(ns, syscall.NETLINK_ROUTE)
			if err != nil {
				logrus.Errorf("Could not open netlink handle during vni population for ns %s: %v", path, err)
				return nil
			}
			defer nlh.Delete()

			err = nlh.SetSocketTimeout(soTimeout)
			if err != nil {
				logrus.Warnf("Failed to set the timeout on the netlink handle sockets for vni table population: %v", err)
			}

			links, err := nlh.LinkList()
			if err != nil {
				logrus.Errorf("Failed to list interfaces during vni population for ns %s: %v", path, err)
				return nil
			}

			for _, l := range links {
				if l.Type() == "vxlan" {
					vniTbl[uint32(l.(*netlink.Vxlan).VxlanId)] = path
				}
			}

			return nil
		})
}

func networkOnceInit() {
	populateVNITbl()

	if os.Getenv("_OVERLAY_HOST_MODE") != "" {
		hostMode = true
		return
	}

	err := createVxlan("testvxlan", 1, 0)
	if err != nil {
		logrus.Errorf("Failed to create testvxlan interface: %v", err)
		return
	}

	defer deleteInterface("testvxlan")

	path := "/proc/self/ns/net"
	hNs, err := netns.GetFromPath(path)
	if err != nil {
		logrus.Errorf("Failed to get network namespace from path %s while setting host mode: %v", path, err)
		return
	}
	defer hNs.Close()

	nlh := ns.NlHandle()

	iface, err := nlh.LinkByName("testvxlan")
	if err != nil {
		logrus.Errorf("Failed to get link testvxlan while setting host mode: %v", err)
		return
	}

	// If we are not able to move the vxlan interface to a namespace
	// then fallback to host mode
	if err := nlh.LinkSetNsFd(iface, int(hNs)); err != nil {
		hostMode = true
	}
}

func (n *network) generateVxlanName(s *subnet) string {
	id := n.id
	if len(n.id) > 5 {
		id = n.id[:5]
	}

	return "vx-" + fmt.Sprintf("%06x", n.vxlanID(s)) + "-" + id
}

func (n *network) generateBridgeName(s *subnet) string {
	id := n.id
	if len(n.id) > 5 {
		id = n.id[:5]
	}

	return n.getBridgeNamePrefix(s) + "-" + id
}

func (n *network) getBridgeNamePrefix(s *subnet) string {
	return "ov-" + fmt.Sprintf("%06x", n.vxlanID(s))
}

func checkOverlap(nw *net.IPNet) error {
	var nameservers []string

	if rc, err := resolvconf.Get(); err == nil {
		nameservers = resolvconf.GetNameserversAsCIDR(rc.Content)
	}

	if err := netutils.CheckNameserverOverlaps(nameservers, nw); err != nil {
		return fmt.Errorf("overlay subnet %s failed check with nameserver: %v: %v", nw.String(), nameservers, err)
	}

	if err := netutils.CheckRouteOverlaps(nw); err != nil {
		return fmt.Errorf("overlay subnet %s failed check with host route table: %v", nw.String(), err)
	}

	return nil
}

func (n *network) restoreSubnetSandbox(s *subnet, brName, vxlanName string) error {
	sbox := n.sandbox()

	// restore overlay osl sandbox
	Ifaces := make(map[string][]osl.IfaceOption)
	brIfaceOption := make([]osl.IfaceOption, 2)
	brIfaceOption = append(brIfaceOption, sbox.InterfaceOptions().Address(s.gwIP))
	brIfaceOption = append(brIfaceOption, sbox.InterfaceOptions().Bridge(true))
	Ifaces[fmt.Sprintf("%s+%s", brName, "br")] = brIfaceOption

	err := sbox.Restore(Ifaces, nil, nil, nil)
	if err != nil {
		return err
	}

	Ifaces = make(map[string][]osl.IfaceOption)
	vxlanIfaceOption := make([]osl.IfaceOption, 1)
	vxlanIfaceOption = append(vxlanIfaceOption, sbox.InterfaceOptions().Master(brName))
	Ifaces[fmt.Sprintf("%s+%s", vxlanName, "vxlan")] = vxlanIfaceOption
	err = sbox.Restore(Ifaces, nil, nil, nil)
	if err != nil {
		return err
	}
	return nil
}

func (n *network) setupSubnetSandbox(s *subnet, brName, vxlanName string) error {

	if hostMode {
		// Try to delete stale bridge interface if it exists
		if err := deleteInterface(brName); err != nil {
			deleteInterfaceBySubnet(n.getBridgeNamePrefix(s), s)
		}
		// Try to delete the vxlan interface by vni if already present
		deleteVxlanByVNI("", n.vxlanID(s))

		if err := checkOverlap(s.subnetIP); err != nil {
			return err
		}
	}

	if !hostMode {
		// Try to find this subnet's vni is being used in some
		// other namespace by looking at vniTbl that we just
		// populated in the once init. If a hit is found then
		// it must a stale namespace from previous
		// life. Destroy it completely and reclaim resourced.
		networkMu.Lock()
		path, ok := vniTbl[n.vxlanID(s)]
		networkMu.Unlock()

		if ok {
			deleteVxlanByVNI(path, n.vxlanID(s))
			if err := syscall.Unmount(path, syscall.MNT_FORCE); err != nil {
				logrus.Errorf("unmount of %s failed: %v", path, err)
			}
			os.Remove(path)

			networkMu.Lock()
			delete(vniTbl, n.vxlanID(s))
			networkMu.Unlock()
		}
	}

	// create a bridge and vxlan device for this subnet and move it to the sandbox
	sbox := n.sandbox()

	if err := sbox.AddInterface(brName, "br",
		sbox.InterfaceOptions().Address(s.gwIP),
		sbox.InterfaceOptions().Bridge(true)); err != nil {
		return fmt.Errorf("bridge creation in sandbox failed for subnet %q: %v", s.subnetIP.String(), err)
	}

	err := createVxlan(vxlanName, n.vxlanID(s), n.maxMTU())
	if err != nil {
		return err
	}

	if err := sbox.AddInterface(vxlanName, "vxlan",
		sbox.InterfaceOptions().Master(brName)); err != nil {
		return fmt.Errorf("vxlan interface creation failed for subnet %q: %v", s.subnetIP.String(), err)
	}

	if hostMode {
		if err := addFilters(n.id[:12], brName); err != nil {
			return err
		}
	}

	return nil
}

func (n *network) initSubnetSandbox(s *subnet, restore bool) error {
	brName := n.generateBridgeName(s)
	vxlanName := n.generateVxlanName(s)

	if restore {
		if err := n.restoreSubnetSandbox(s, brName, vxlanName); err != nil {
			return err
		}
	} else {
		if err := n.setupSubnetSandbox(s, brName, vxlanName); err != nil {
			return err
		}
	}

	n.Lock()
	s.vxlanName = vxlanName
	s.brName = brName
	n.Unlock()

	return nil
}

func (n *network) cleanupStaleSandboxes() {
	filepath.Walk(filepath.Dir(osl.GenerateKey("walk")),
		func(path string, info os.FileInfo, err error) error {
			_, fname := filepath.Split(path)

			pList := strings.Split(fname, "-")
			if len(pList) <= 1 {
				return nil
			}

			pattern := pList[1]
			if strings.Contains(n.id, pattern) {
				// Delete all vnis
				deleteVxlanByVNI(path, 0)
				syscall.Unmount(path, syscall.MNT_DETACH)
				os.Remove(path)

				// Now that we have destroyed this
				// sandbox, remove all references to
				// it in vniTbl so that we don't
				// inadvertently destroy the sandbox
				// created in this life.
				networkMu.Lock()
				for vni, tblPath := range vniTbl {
					if tblPath == path {
						delete(vniTbl, vni)
					}
				}
				networkMu.Unlock()
			}

			return nil
		})
}

func (n *network) initSandbox(restore bool) error {
	n.Lock()
	n.initEpoch++
	n.Unlock()

	networkOnce.Do(networkOnceInit)

	if !restore {
		if hostMode {
			if err := addNetworkChain(n.id[:12]); err != nil {
				return err
			}
		}

		// If there are any stale sandboxes related to this network
		// from previous daemon life clean it up here
		n.cleanupStaleSandboxes()
	}

	// In the restore case network sandbox already exist; but we don't know
	// what epoch number it was created with. It has to be retrieved by
	// searching the net namespaces.
	key := ""
	if restore {
		key = osl.GenerateKey("-" + n.id)
	} else {
		key = osl.GenerateKey(fmt.Sprintf("%d-", n.initEpoch) + n.id)
	}

	sbox, err := osl.NewSandbox(key, !hostMode, restore)
	if err != nil {
		return fmt.Errorf("could not get network sandbox (oper %t): %v", restore, err)
	}

	n.setSandbox(sbox)

	if !restore {
		n.driver.peerDbUpdateSandbox(n.id)
	}

	var nlSock *nl.NetlinkSocket
	sbox.InvokeFunc(func() {
		nlSock, err = nl.Subscribe(syscall.NETLINK_ROUTE, syscall.RTNLGRP_NEIGH)
		if err != nil {
			err = fmt.Errorf("failed to subscribe to neighbor group netlink messages")
		}
	})

	if nlSock != nil {
		go n.watchMiss(nlSock)
	}

	return nil
}

func (n *network) watchMiss(nlSock *nl.NetlinkSocket) {
	for {
		msgs, err := nlSock.Receive()
		if err != nil {
			logrus.Errorf("Failed to receive from netlink: %v ", err)
			continue
		}

		for _, msg := range msgs {
			if msg.Header.Type != syscall.RTM_GETNEIGH && msg.Header.Type != syscall.RTM_NEWNEIGH {
				continue
			}

			neigh, err := netlink.NeighDeserialize(msg.Data)
			if err != nil {
				logrus.Errorf("Failed to deserialize netlink ndmsg: %v", err)
				continue
			}

			if neigh.IP.To4() == nil {
				continue
			}

			// Not any of the network's subnets. Ignore.
			if !n.contains(neigh.IP) {
				continue
			}

			logrus.Debugf("miss notification for dest IP, %v", neigh.IP.String())

			if neigh.State&(netlink.NUD_STALE|netlink.NUD_INCOMPLETE) == 0 {
				continue
			}

			if !n.driver.isSerfAlive() {
				continue
			}

			mac, IPmask, vtep, err := n.driver.resolvePeer(n.id, neigh.IP)
			if err != nil {
				logrus.Errorf("could not resolve peer %q: %v", neigh.IP, err)
				continue
			}

			if err := n.driver.peerAdd(n.id, "dummy", neigh.IP, IPmask, mac, vtep, true); err != nil {
				logrus.Errorf("could not add neighbor entry for missed peer %q: %v", neigh.IP, err)
			}
		}
	}
}

func (d *driver) addNetwork(n *network) {
	d.Lock()
	d.networks[n.id] = n
	d.Unlock()
}

func (d *driver) deleteNetwork(nid string) {
	d.Lock()
	delete(d.networks, nid)
	d.Unlock()
}

func (d *driver) network(nid string) *network {
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()
	if !ok {
		n = d.getNetworkFromStore(nid)
		if n != nil {
			n.driver = d
			n.endpoints = endpointTable{}
			n.once = &sync.Once{}
			d.Lock()
			d.networks[nid] = n
			d.Unlock()
		}
	}

	return n
}

func (d *driver) getNetworkFromStore(nid string) *network {
	if d.store == nil {
		return nil
	}

	n := &network{id: nid}
	if err := d.store.GetObject(datastore.Key(n.Key()...), n); err != nil {
		return nil
	}

	return n
}

func (n *network) sandbox() osl.Sandbox {
	n.Lock()
	defer n.Unlock()

	return n.sbox
}

func (n *network) setSandbox(sbox osl.Sandbox) {
	n.Lock()
	n.sbox = sbox
	n.Unlock()
}

func (n *network) vxlanID(s *subnet) uint32 {
	n.Lock()
	defer n.Unlock()

	return s.vni
}

func (n *network) setVxlanID(s *subnet, vni uint32) {
	n.Lock()
	s.vni = vni
	n.Unlock()
}

func (n *network) Key() []string {
	return []string{"overlay", "network", n.id}
}

func (n *network) KeyPrefix() []string {
	return []string{"overlay", "network"}
}

func (n *network) Value() []byte {
	m := map[string]interface{}{}

	netJSON := []*subnetJSON{}

	for _, s := range n.subnets {
		sj := &subnetJSON{
			SubnetIP: s.subnetIP.String(),
			GwIP:     s.gwIP.String(),
			Vni:      s.vni,
		}
		netJSON = append(netJSON, sj)
	}

	b, err := json.Marshal(netJSON)
	if err != nil {
		return []byte{}
	}

	m["secure"] = n.secure
	m["subnets"] = netJSON
	m["mtu"] = n.mtu
	b, err = json.Marshal(m)
	if err != nil {
		return []byte{}
	}

	return b
}

func (n *network) Index() uint64 {
	return n.dbIndex
}

func (n *network) SetIndex(index uint64) {
	n.dbIndex = index
	n.dbExists = true
}

func (n *network) Exists() bool {
	return n.dbExists
}

func (n *network) Skip() bool {
	return false
}

func (n *network) SetValue(value []byte) error {
	var (
		m       map[string]interface{}
		newNet  bool
		isMap   = true
		netJSON = []*subnetJSON{}
	)

	if err := json.Unmarshal(value, &m); err != nil {
		err := json.Unmarshal(value, &netJSON)
		if err != nil {
			return err
		}
		isMap = false
	}

	if len(n.subnets) == 0 {
		newNet = true
	}

	if isMap {
		if val, ok := m["secure"]; ok {
			n.secure = val.(bool)
		}
		if val, ok := m["mtu"]; ok {
			n.mtu = int(val.(float64))
		}
		bytes, err := json.Marshal(m["subnets"])
		if err != nil {
			return err
		}
		if err := json.Unmarshal(bytes, &netJSON); err != nil {
			return err
		}
	}

	for _, sj := range netJSON {
		subnetIPstr := sj.SubnetIP
		gwIPstr := sj.GwIP
		vni := sj.Vni

		subnetIP, _ := types.ParseCIDR(subnetIPstr)
		gwIP, _ := types.ParseCIDR(gwIPstr)

		if newNet {
			s := &subnet{
				subnetIP: subnetIP,
				gwIP:     gwIP,
				vni:      vni,
				once:     &sync.Once{},
			}
			n.subnets = append(n.subnets, s)
		} else {
			sNet := n.getMatchingSubnet(subnetIP)
			if sNet != nil {
				sNet.vni = vni
			}
		}
	}
	return nil
}

func (n *network) DataScope() string {
	return datastore.GlobalScope
}

func (n *network) writeToStore() error {
	if n.driver.store == nil {
		return nil
	}

	return n.driver.store.PutObjectAtomic(n)
}

func (n *network) releaseVxlanID() ([]uint32, error) {
	if len(n.subnets) == 0 {
		return nil, nil
	}

	if n.driver.store != nil {
		if err := n.driver.store.DeleteObjectAtomic(n); err != nil {
			if err == datastore.ErrKeyModified || err == datastore.ErrKeyNotFound {
				// In both the above cases we can safely assume that the key has been removed by some other
				// instance and so simply get out of here
				return nil, nil
			}

			return nil, fmt.Errorf("failed to delete network to vxlan id map: %v", err)
		}
	}
	var vnis []uint32
	for _, s := range n.subnets {
		if n.driver.vxlanIdm != nil {
			vni := n.vxlanID(s)
			vnis = append(vnis, vni)
			n.driver.vxlanIdm.Release(uint64(vni))
		}

		n.setVxlanID(s, 0)
	}

	return vnis, nil
}

func (n *network) obtainVxlanID(s *subnet) error {
	//return if the subnet already has a vxlan id assigned
	if s.vni != 0 {
		return nil
	}

	if n.driver.store == nil {
		return fmt.Errorf("no valid vxlan id and no datastore configured, cannot obtain vxlan id")
	}

	for {
		if err := n.driver.store.GetObject(datastore.Key(n.Key()...), n); err != nil {
			return fmt.Errorf("getting network %q from datastore failed %v", n.id, err)
		}

		if s.vni == 0 {
			vxlanID, err := n.driver.vxlanIdm.GetID()
			if err != nil {
				return fmt.Errorf("failed to allocate vxlan id: %v", err)
			}

			n.setVxlanID(s, uint32(vxlanID))
			if err := n.writeToStore(); err != nil {
				n.driver.vxlanIdm.Release(uint64(n.vxlanID(s)))
				n.setVxlanID(s, 0)
				if err == datastore.ErrKeyModified {
					continue
				}
				return fmt.Errorf("network %q failed to update data store: %v", n.id, err)
			}
			return nil
		}
		return nil
	}
}

// contains return true if the passed ip belongs to one the network's
// subnets
func (n *network) contains(ip net.IP) bool {
	for _, s := range n.subnets {
		if s.subnetIP.Contains(ip) {
			return true
		}
	}

	return false
}

// getSubnetforIP returns the subnet to which the given IP belongs
func (n *network) getSubnetforIP(ip *net.IPNet) *subnet {
	for _, s := range n.subnets {
		// first check if the mask lengths are the same
		i, _ := s.subnetIP.Mask.Size()
		j, _ := ip.Mask.Size()
		if i != j {
			continue
		}
		if s.subnetIP.Contains(ip.IP) {
			return s
		}
	}
	return nil
}

// getMatchingSubnet return the network's subnet that matches the input
func (n *network) getMatchingSubnet(ip *net.IPNet) *subnet {
	if ip == nil {
		return nil
	}
	for _, s := range n.subnets {
		// first check if the mask lengths are the same
		i, _ := s.subnetIP.Mask.Size()
		j, _ := ip.Mask.Size()
		if i != j {
			continue
		}
		if s.subnetIP.IP.Equal(ip.IP) {
			return s
		}
	}
	return nil
}
