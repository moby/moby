//go:build linux
// +build linux

package overlay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/netutils"
	"github.com/docker/docker/libnetwork/ns"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/resolvconf"
	"github.com/docker/docker/libnetwork/types"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

var (
	hostMode    bool
	networkOnce sync.Once
	networkMu   sync.Mutex
	vniTbl      = make(map[uint32]string)
)

type networkTable map[string]*network

type subnet struct {
	sboxInit  bool
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
	sboxInit  bool
	initEpoch int
	initErr   error
	subnets   []*subnet
	secure    bool
	mtu       int
	sync.Mutex
}

func init() {
	// Lock main() to the initial thread to exclude the goroutines executing
	// func setDefaultVLAN() from being scheduled onto that thread. Changes to
	// the network namespace of the initial thread alter /proc/self/ns/net,
	// which would break any code which (incorrectly) assumes that that file is
	// a handle to the network namespace for the thread it is currently
	// executing on.
	runtime.LockOSThread()
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
		subnets:   []*subnet{},
	}

	vnis := make([]uint32, 0, len(ipV4Data))
	gval, ok := option[netlabel.GenericData]
	if !ok {
		return fmt.Errorf("option %s is missing", netlabel.GenericData)
	}

	optMap := gval.(map[string]string)
	vnisOpt, ok := optMap[netlabel.OverlayVxlanIDList]
	if !ok {
		return errors.New("no VNI provided")
	}
	logrus.Debugf("overlay: Received vxlan IDs: %s", vnisOpt)
	vniStrings := strings.Split(vnisOpt, ",")
	for _, vniStr := range vniStrings {
		vni, err := strconv.Atoi(vniStr)
		if err != nil {
			return fmt.Errorf("invalid vxlan id value %q passed", vniStr)
		}

		vnis = append(vnis, uint32(vni))
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

	if len(vnis) == 0 {
		return errors.New("no VNI provided")
	} else if len(vnis) < len(ipV4Data) {
		return fmt.Errorf("insufficient vnis(%d) passed to overlay", len(vnis))
	}

	for i, ipd := range ipV4Data {
		s := &subnet{
			subnetIP: ipd.Pool,
			gwIP:     ipd.Gateway,
			vni:      vnis[i],
		}

		n.subnets = append(n.subnets, s)
	}

	d.Lock()
	defer d.Unlock()
	if d.networks[n.id] != nil {
		return fmt.Errorf("attempt to create overlay network %v that already exists", n.id)
	}

	// Make sure no rule is on the way from any stale secure network
	if !n.secure {
		for _, vni := range vnis {
			programMangle(vni, false)
			programInput(vni, false)
		}
	}

	if nInfo != nil {
		if err := nInfo.TableEventRegister(ovPeerTable, driverapi.EndpointObject); err != nil {
			// XXX Undo writeToStore?  No method to so.  Why?
			return err
		}
	}

	d.networks[id] = n

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

	d.Lock()
	// Only perform a peer flush operation (if required) AFTER unlocking
	// the driver lock to avoid deadlocking w/ the peerDB.
	var doPeerFlush bool
	defer func() {
		d.Unlock()
		if doPeerFlush {
			d.peerFlush(nid)
		}
	}()

	// This is similar to d.network(), but we need to keep holding the lock
	// until we are done removing this network.
	n := d.networks[nid]
	if n == nil {
		return fmt.Errorf("could not find network with id %s", nid)
	}

	for _, ep := range n.endpoints {
		if ep.ifName != "" {
			if link, err := ns.NlHandle().LinkByName(ep.ifName); err == nil {
				if err := ns.NlHandle().LinkDel(link); err != nil {
					logrus.WithError(err).Warnf("Failed to delete interface (%s)'s link on endpoint (%s) delete", ep.ifName, ep.id)
				}
			}
		}
	}

	doPeerFlush = true
	delete(d.networks, nid)

	return nil
}

func (d *driver) ProgramExternalConnectivity(nid, eid string, options map[string]interface{}) error {
	return nil
}

func (d *driver) RevokeExternalConnectivity(nid, eid string) error {
	return nil
}

func (n *network) joinSandbox(s *subnet, incJoinCount bool) error {
	// If there is a race between two go routines here only one will win
	// the other will wait.
	networkOnce.Do(networkOnceInit)

	n.Lock()
	// If initialization was successful then tell the peerDB to initialize the
	// sandbox with all the peers previously received from networkdb. But only
	// do this after unlocking the network. Otherwise we could deadlock with
	// on the peerDB channel while peerDB is waiting for the network lock.
	var doInitPeerDB bool
	defer func() {
		n.Unlock()
		if doInitPeerDB {
			go n.driver.initSandboxPeerDB(n.id)
		}
	}()

	if !n.sboxInit {
		n.initErr = n.initSandbox()
		doInitPeerDB = n.initErr == nil
		// If there was an error, we cannot recover it
		n.sboxInit = true
	}

	if n.initErr != nil {
		return fmt.Errorf("network sandbox join failed: %v", n.initErr)
	}

	subnetErr := s.initErr
	if !s.sboxInit {
		subnetErr = n.initSubnetSandbox(s)
		// We can recover from these errors
		if subnetErr == nil {
			s.initErr = subnetErr
			s.sboxInit = true
		}
	}
	if subnetErr != nil {
		return fmt.Errorf("subnet sandbox join failed for %q: %v", s.subnetIP.String(), subnetErr)
	}

	if incJoinCount {
		n.joinCnt++
	}

	return nil
}

func (n *network) leaveSandbox() {
	n.Lock()
	defer n.Unlock()
	n.joinCnt--
	if n.joinCnt != 0 {
		return
	}

	n.destroySandbox()

	n.sboxInit = false
	n.initErr = nil
	for _, s := range n.subnets {
		s.sboxInit = false
		s.initErr = nil
	}
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
	filepath.WalkDir(filepath.Dir(osl.GenerateKey("walk")),
		// NOTE(cpuguy83): The linter picked up on the fact that this walk function was not using this error argument
		// That seems wrong... however I'm not familiar with this code or if that error matters
		func(path string, _ os.DirEntry, _ error) error {
			_, fname := filepath.Split(path)

			if len(strings.Split(fname, "-")) <= 1 {
				return nil
			}

			n, err := netns.GetFromPath(path)
			if err != nil {
				logrus.Errorf("Could not open namespace path %s during vni population: %v", path, err)
				return nil
			}
			defer n.Close()

			nlh, err := netlink.NewHandleAt(n, unix.NETLINK_ROUTE)
			if err != nil {
				logrus.Errorf("Could not open netlink handle during vni population for ns %s: %v", path, err)
				return nil
			}
			defer nlh.Close()

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

	return fmt.Sprintf("vx-%06x-%v", s.vni, id)
}

func (n *network) generateBridgeName(s *subnet) string {
	id := n.id
	if len(n.id) > 5 {
		id = n.id[:5]
	}

	return n.getBridgeNamePrefix(s) + "-" + id
}

func (n *network) getBridgeNamePrefix(s *subnet) string {
	return fmt.Sprintf("ov-%06x", s.vni)
}

func checkOverlap(nw *net.IPNet) error {
	var nameservers []string

	if rc, err := os.ReadFile(resolvconf.Path()); err == nil {
		nameservers = resolvconf.GetNameserversAsCIDR(rc)
	}

	if err := netutils.CheckNameserverOverlaps(nameservers, nw); err != nil {
		return fmt.Errorf("overlay subnet %s failed check with nameserver: %v: %v", nw.String(), nameservers, err)
	}

	if err := netutils.CheckRouteOverlaps(nw); err != nil {
		return fmt.Errorf("overlay subnet %s failed check with host route table: %v", nw.String(), err)
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
		deleteVxlanByVNI("", s.vni)

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
		path, ok := vniTbl[s.vni]
		networkMu.Unlock()

		if ok {
			deleteVxlanByVNI(path, s.vni)
			if err := unix.Unmount(path, unix.MNT_FORCE); err != nil {
				logrus.Errorf("unmount of %s failed: %v", path, err)
			}
			os.Remove(path)

			networkMu.Lock()
			delete(vniTbl, s.vni)
			networkMu.Unlock()
		}
	}

	// create a bridge and vxlan device for this subnet and move it to the sandbox
	sbox := n.sbox

	if err := sbox.AddInterface(brName, "br",
		sbox.InterfaceOptions().Address(s.gwIP),
		sbox.InterfaceOptions().Bridge(true)); err != nil {
		return fmt.Errorf("bridge creation in sandbox failed for subnet %q: %v", s.subnetIP.String(), err)
	}

	err := createVxlan(vxlanName, s.vni, n.maxMTU())
	if err != nil {
		return err
	}

	if err := sbox.AddInterface(vxlanName, "vxlan",
		sbox.InterfaceOptions().Master(brName)); err != nil {
		// If adding vxlan device to the overlay namespace fails, remove the bridge interface we
		// already added to the namespace. This allows the caller to try the setup again.
		for _, iface := range sbox.Info().Interfaces() {
			if iface.SrcName() == brName {
				if ierr := iface.Remove(); ierr != nil {
					logrus.Errorf("removing bridge failed from ov ns %v failed, %v", n.sbox.Key(), ierr)
				}
			}
		}

		// Also, delete the vxlan interface. Since a global vni id is associated
		// with the vxlan interface, an orphaned vxlan interface will result in
		// failure of vxlan device creation if the vni is assigned to some other
		// network.
		if deleteErr := deleteInterface(vxlanName); deleteErr != nil {
			logrus.Warnf("could not delete vxlan interface, %s, error %v, after config error, %v", vxlanName, deleteErr, err)
		}
		return fmt.Errorf("vxlan interface creation failed for subnet %q: %v", s.subnetIP.String(), err)
	}

	if hostMode {
		return addFilters(n.id[:12], brName)
	}

	if err := setDefaultVLAN(sbox); err != nil {
		// not a fatal error
		logrus.WithError(err).Error("set bridge default vlan failed")
	}
	return nil
}

func setDefaultVLAN(sbox osl.Sandbox) error {
	var brName string
	for _, i := range sbox.Info().Interfaces() {
		if i.Bridge() {
			brName = i.DstName()
		}
	}

	// IFLA_BR_VLAN_DEFAULT_PVID was added in Linux v4.4 (see torvalds/linux@0f963b7), so we can't use netlink for
	// setting this until Docker drops support for CentOS/RHEL 7 (kernel 3.10, eol date: 2024-06-30).
	var innerErr error
	err := sbox.InvokeFunc(func() {
		// Contrary to what the sysfs(5) man page says, the entries of /sys/class/net
		// represent the networking devices visible in the network namespace of the
		// process which mounted the sysfs filesystem, irrespective of the network
		// namespace of the process accessing the directory. Remount sysfs in order to
		// see the network devices in sbox's network namespace, making sure the mount
		// doesn't propagate back.
		//
		// The Linux implementation of (osl.Sandbox).InvokeFunc() runs the function in a
		// dedicated goroutine. The effects of unshare(CLONE_NEWNS) on a thread cannot
		// be reverted so the thread needs to be terminated once the goroutine is
		// finished.
		runtime.LockOSThread()
		if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
			innerErr = os.NewSyscallError("unshare", err)
			return
		}
		if err := unix.Mount("", "/", "", unix.MS_SLAVE|unix.MS_REC, ""); err != nil {
			innerErr = &os.PathError{Op: "mount", Path: "/", Err: err}
			return
		}
		if err := unix.Mount("sysfs", "/sys", "sysfs", 0, ""); err != nil {
			innerErr = &os.PathError{Op: "mount", Path: "/sys", Err: err}
			return
		}

		path := filepath.Join("/sys/class/net", brName, "bridge/default_pvid")
		data := []byte{'0', '\n'}

		if err := os.WriteFile(path, data, 0o644); err != nil {
			innerErr = fmt.Errorf("failed to enable default vlan on bridge %s: %w", brName, err)
			return
		}
	})
	if err != nil {
		return err
	}
	return innerErr
}

// Must be called with the network lock
func (n *network) initSubnetSandbox(s *subnet) error {
	brName := n.generateBridgeName(s)
	vxlanName := n.generateVxlanName(s)

	// Program iptables rules for mandatory encryption of the secure
	// network, or clean up leftover rules for a stale secure network which
	// was previously assigned the same VNI.
	if err := programMangle(s.vni, n.secure); err != nil {
		return err
	}
	if err := programInput(s.vni, n.secure); err != nil {
		if n.secure {
			return multierror.Append(err, programMangle(s.vni, false))
		}
	}

	if err := n.setupSubnetSandbox(s, brName, vxlanName); err != nil {
		return err
	}

	s.vxlanName = vxlanName
	s.brName = brName

	return nil
}

func (n *network) cleanupStaleSandboxes() {
	filepath.WalkDir(filepath.Dir(osl.GenerateKey("walk")),
		func(path string, _ os.DirEntry, _ error) error {
			_, fname := filepath.Split(path)

			pList := strings.Split(fname, "-")
			if len(pList) <= 1 {
				return nil
			}

			pattern := pList[1]
			if strings.Contains(n.id, pattern) {
				// Delete all vnis
				deleteVxlanByVNI(path, 0)
				unix.Unmount(path, unix.MNT_DETACH)
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

func (n *network) initSandbox() error {
	n.initEpoch++

	if hostMode {
		if err := addNetworkChain(n.id[:12]); err != nil {
			return err
		}
	}

	// If there are any stale sandboxes related to this network
	// from previous daemon life clean it up here
	n.cleanupStaleSandboxes()

	key := osl.GenerateKey(fmt.Sprintf("%d-", n.initEpoch) + n.id)
	sbox, err := osl.NewSandbox(key, !hostMode, false)
	if err != nil {
		return fmt.Errorf("could not get network sandbox: %v", err)
	}

	// this is needed to let the peerAdd configure the sandbox
	n.sbox = sbox

	return nil
}

func (d *driver) network(nid string) *network {
	d.Lock()
	n := d.networks[nid]
	d.Unlock()

	return n
}

func (n *network) sandbox() osl.Sandbox {
	n.Lock()
	defer n.Unlock()
	return n.sbox
}

func (n *network) vxlanID(s *subnet) uint32 {
	n.Lock()
	defer n.Unlock()
	return s.vni
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

	m["secure"] = n.secure
	m["subnets"] = netJSON
	m["mtu"] = n.mtu
	b, err := json.Marshal(m)
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
