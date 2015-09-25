package libnetwork

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/etchosts"
	"github.com/docker/libnetwork/osl"
	"github.com/docker/libnetwork/resolvconf"
	"github.com/docker/libnetwork/types"
)

// Sandbox provides the control over the network container entity. It is a one to one mapping with the container.
type Sandbox interface {
	// ID returns the ID of the sandbox
	ID() string
	// Key returns the sandbox's key
	Key() string
	// ContainerID returns the container id associated to this sandbox
	ContainerID() string
	// Labels returns the sandbox's labels
	Labels() map[string]interface{}
	// Statistics retrieves the interfaces' statistics for the sandbox
	Statistics() (map[string]*osl.InterfaceStatistics, error)
	// Refresh leaves all the endpoints, resets and re-apply the options,
	// re-joins all the endpoints without destroying the osl sandbox
	Refresh(options ...SandboxOption) error
	// SetKey updates the Sandbox Key
	SetKey(key string) error
	// Delete destroys this container after detaching it from all connected endpoints.
	Delete() error
}

// SandboxOption is a option setter function type used to pass varios options to
// NewNetContainer method. The various setter functions of type SandboxOption are
// provided by libnetwork, they look like ContainerOptionXXXX(...)
type SandboxOption func(sb *sandbox)

func (sb *sandbox) processOptions(options ...SandboxOption) {
	for _, opt := range options {
		if opt != nil {
			opt(sb)
		}
	}
}

type epHeap []*endpoint

type sandbox struct {
	id            string
	containerID   string
	config        containerConfig
	osSbox        osl.Sandbox
	controller    *controller
	refCnt        int
	hostsOnce     sync.Once
	endpoints     epHeap
	epPriority    map[string]int
	joinLeaveDone chan struct{}
	sync.Mutex
}

// These are the container configs used to customize container /etc/hosts file.
type hostsPathConfig struct {
	hostName        string
	domainName      string
	hostsPath       string
	originHostsPath string
	extraHosts      []extraHost
	parentUpdates   []parentUpdate
}

type parentUpdate struct {
	cid  string
	name string
	ip   string
}

type extraHost struct {
	name string
	IP   string
}

// These are the container configs used to customize container /etc/resolv.conf file.
type resolvConfPathConfig struct {
	resolvConfPath       string
	originResolvConfPath string
	resolvConfHashFile   string
	dnsList              []string
	dnsSearchList        []string
	dnsOptionsList       []string
}

type containerConfig struct {
	hostsPathConfig
	resolvConfPathConfig
	generic           map[string]interface{}
	useDefaultSandBox bool
	useExternalKey    bool
	prio              int // higher the value, more the priority
}

func (sb *sandbox) ID() string {
	return sb.id
}

func (sb *sandbox) ContainerID() string {
	return sb.containerID
}

func (sb *sandbox) Key() string {
	if sb.config.useDefaultSandBox {
		return osl.GenerateKey("default")
	}
	return osl.GenerateKey(sb.id)
}

func (sb *sandbox) Labels() map[string]interface{} {
	return sb.config.generic
}

func (sb *sandbox) Statistics() (map[string]*osl.InterfaceStatistics, error) {
	m := make(map[string]*osl.InterfaceStatistics)

	if sb.osSbox == nil {
		return m, nil
	}

	var err error
	for _, i := range sb.osSbox.Info().Interfaces() {
		if m[i.DstName()], err = i.Statistics(); err != nil {
			return m, err
		}
	}

	return m, nil
}

func (sb *sandbox) Delete() error {
	c := sb.controller

	// Detach from all endpoints
	for _, ep := range sb.getConnectedEndpoints() {
		// endpoint in the Gateway network will be cleaned up
		// when when sandbox no longer needs external connectivity
		if ep.endpointInGWNetwork() {
			continue
		}
		if err := ep.Leave(sb); err != nil {
			log.Warnf("Failed detaching sandbox %s from endpoint %s: %v\n", sb.ID(), ep.ID(), err)
		}
	}

	if sb.osSbox != nil {
		sb.osSbox.Destroy()
	}

	c.Lock()
	delete(c.sandboxes, sb.ID())
	c.Unlock()

	return nil
}

func (sb *sandbox) Refresh(options ...SandboxOption) error {
	// Store connected endpoints
	epList := sb.getConnectedEndpoints()

	// Detach from all endpoints
	for _, ep := range epList {
		if err := ep.Leave(sb); err != nil {
			log.Warnf("Failed detaching sandbox %s from endpoint %s: %v\n", sb.ID(), ep.ID(), err)
		}
	}

	// Re-apply options
	sb.config = containerConfig{}
	sb.processOptions(options...)

	// Setup discovery files
	if err := sb.setupResolutionFiles(); err != nil {
		return err
	}

	// Re -connect to all endpoints
	for _, ep := range epList {
		if err := ep.Join(sb); err != nil {
			log.Warnf("Failed attach sandbox %s to endpoint %s: %v\n", sb.ID(), ep.ID(), err)
		}
	}

	return nil
}

func (sb *sandbox) MarshalJSON() ([]byte, error) {
	sb.Lock()
	defer sb.Unlock()

	// We are just interested in the container ID. This can be expanded to include all of containerInfo if there is a need
	return json.Marshal(sb.id)
}

func (sb *sandbox) UnmarshalJSON(b []byte) (err error) {
	sb.Lock()
	defer sb.Unlock()

	var id string
	if err := json.Unmarshal(b, &id); err != nil {
		return err
	}
	sb.id = id
	return nil
}

func (sb *sandbox) setupResolutionFiles() error {
	if err := sb.buildHostsFile(); err != nil {
		return err
	}

	if err := sb.updateParentHosts(); err != nil {
		return err
	}

	if err := sb.setupDNS(); err != nil {
		return err
	}

	return nil
}

func (sb *sandbox) getConnectedEndpoints() []*endpoint {
	sb.Lock()
	defer sb.Unlock()

	eps := make([]*endpoint, len(sb.endpoints))
	for i, ep := range sb.endpoints {
		eps[i] = ep
	}

	return eps
}

func (sb *sandbox) updateGateway(ep *endpoint) error {
	sb.Lock()
	osSbox := sb.osSbox
	sb.Unlock()
	if osSbox == nil {
		return nil
	}
	osSbox.UnsetGateway()
	osSbox.UnsetGatewayIPv6()

	if ep == nil {
		return nil
	}

	ep.Lock()
	joinInfo := ep.joinInfo
	ep.Unlock()

	if err := osSbox.SetGateway(joinInfo.gw); err != nil {
		return fmt.Errorf("failed to set gateway while updating gateway: %v", err)
	}

	if err := osSbox.SetGatewayIPv6(joinInfo.gw6); err != nil {
		return fmt.Errorf("failed to set IPv6 gateway while updating gateway: %v", err)
	}

	return nil
}

func (sb *sandbox) SetKey(basePath string) error {
	var err error
	if basePath == "" {
		return types.BadRequestErrorf("invalid sandbox key")
	}

	sb.Lock()
	if sb.osSbox != nil {
		sb.Unlock()
		return types.ForbiddenErrorf("failed to set sandbox key : already assigned")
	}
	sb.Unlock()
	osSbox, err := osl.GetSandboxForExternalKey(basePath, sb.Key())
	if err != nil {
		return err
	}
	sb.Lock()
	sb.osSbox = osSbox
	sb.Unlock()
	defer func() {
		if err != nil {
			sb.Lock()
			sb.osSbox = nil
			sb.Unlock()
		}
	}()

	for _, ep := range sb.getConnectedEndpoints() {
		if err = sb.populateNetworkResources(ep); err != nil {
			return err
		}
	}
	return nil
}

func (sb *sandbox) populateNetworkResources(ep *endpoint) error {
	sb.Lock()
	if sb.osSbox == nil {
		sb.Unlock()
		return nil
	}
	sb.Unlock()

	ep.Lock()
	joinInfo := ep.joinInfo
	i := ep.iface
	ep.Unlock()

	if i != nil {
		var ifaceOptions []osl.IfaceOption

		ifaceOptions = append(ifaceOptions, sb.osSbox.InterfaceOptions().Address(&i.addr), sb.osSbox.InterfaceOptions().Routes(i.routes))
		if i.addrv6.IP.To16() != nil {
			ifaceOptions = append(ifaceOptions, sb.osSbox.InterfaceOptions().AddressIPv6(&i.addrv6))
		}

		if err := sb.osSbox.AddInterface(i.srcName, i.dstPrefix, ifaceOptions...); err != nil {
			return fmt.Errorf("failed to add interface %s to sandbox: %v", i.srcName, err)
		}
	}

	if joinInfo != nil {
		// Set up non-interface routes.
		for _, r := range joinInfo.StaticRoutes {
			if err := sb.osSbox.AddStaticRoute(r); err != nil {
				return fmt.Errorf("failed to add static route %s: %v", r.Destination.String(), err)
			}
		}
	}

	for _, gwep := range sb.getConnectedEndpoints() {
		if len(gwep.Gateway()) > 0 {
			if gwep != ep {
				return nil
			}
			if err := sb.updateGateway(gwep); err != nil {
				return err
			}
		}
	}
	return nil
}

func (sb *sandbox) clearNetworkResources(ep *endpoint) error {
	sb.Lock()
	osSbox := sb.osSbox
	sb.Unlock()
	if osSbox != nil {
		for _, i := range osSbox.Info().Interfaces() {
			// Only remove the interfaces owned by this endpoint from the sandbox.
			if ep.hasInterface(i.SrcName()) {
				if err := i.Remove(); err != nil {
					log.Debugf("Remove interface failed: %v", err)
				}
			}
		}

		ep.Lock()
		joinInfo := ep.joinInfo
		ep.Unlock()

		// Remove non-interface routes.
		for _, r := range joinInfo.StaticRoutes {
			if err := osSbox.RemoveStaticRoute(r); err != nil {
				log.Debugf("Remove route failed: %v", err)
			}
		}
	}

	sb.Lock()
	if len(sb.endpoints) == 0 {
		// sb.endpoints should never be empty and this is unexpected error condition
		// We log an error message to note this down for debugging purposes.
		log.Errorf("No endpoints in sandbox while trying to remove endpoint %s", ep.Name())
		sb.Unlock()
		return nil
	}

	var (
		gwepBefore, gwepAfter *endpoint
		index                 = -1
	)
	for i, e := range sb.endpoints {
		if e == ep {
			index = i
		}
		if len(e.Gateway()) > 0 && gwepBefore == nil {
			gwepBefore = e
		}
		if index != -1 && gwepBefore != nil {
			break
		}
	}
	heap.Remove(&sb.endpoints, index)
	for _, e := range sb.endpoints {
		if len(e.Gateway()) > 0 {
			gwepAfter = e
			break
		}
	}
	delete(sb.epPriority, ep.ID())
	sb.Unlock()

	if gwepAfter != nil && gwepBefore != gwepAfter {
		sb.updateGateway(gwepAfter)
	}

	return nil
}

const (
	defaultPrefix = "/var/lib/docker/network/files"
	dirPerm       = 0755
	filePerm      = 0644
)

func (sb *sandbox) buildHostsFile() error {
	if sb.config.hostsPath == "" {
		sb.config.hostsPath = defaultPrefix + "/" + sb.id + "/hosts"
	}

	dir, _ := filepath.Split(sb.config.hostsPath)
	if err := createBasePath(dir); err != nil {
		return err
	}

	// This is for the host mode networking
	if sb.config.originHostsPath != "" {
		if err := copyFile(sb.config.originHostsPath, sb.config.hostsPath); err != nil && !os.IsNotExist(err) {
			return types.InternalErrorf("could not copy source hosts file %s to %s: %v", sb.config.originHostsPath, sb.config.hostsPath, err)
		}
		return nil
	}

	extraContent := make([]etchosts.Record, 0, len(sb.config.extraHosts))
	for _, extraHost := range sb.config.extraHosts {
		extraContent = append(extraContent, etchosts.Record{Hosts: extraHost.name, IP: extraHost.IP})
	}

	return etchosts.Build(sb.config.hostsPath, "", sb.config.hostName, sb.config.domainName, extraContent)
}

func (sb *sandbox) updateHostsFile(ifaceIP string, svcRecords []etchosts.Record) error {
	var err error

	if sb.config.originHostsPath != "" {
		return nil
	}

	max := func(a, b int) int {
		if a < b {
			return b
		}

		return a
	}

	extraContent := make([]etchosts.Record, 0,
		max(len(sb.config.extraHosts), len(svcRecords)))

	sb.hostsOnce.Do(func() {
		// Rebuild the hosts file accounting for the passed
		// interface IP and service records

		for _, extraHost := range sb.config.extraHosts {
			extraContent = append(extraContent,
				etchosts.Record{Hosts: extraHost.name, IP: extraHost.IP})
		}

		err = etchosts.Build(sb.config.hostsPath, ifaceIP,
			sb.config.hostName, sb.config.domainName, extraContent)
	})

	if err != nil {
		return err
	}

	extraContent = extraContent[:0]
	for _, svc := range svcRecords {
		extraContent = append(extraContent, svc)
	}

	sb.addHostsEntries(extraContent)
	return nil
}

func (sb *sandbox) addHostsEntries(recs []etchosts.Record) {
	if err := etchosts.Add(sb.config.hostsPath, recs); err != nil {
		log.Warnf("Failed adding service host entries to the running container: %v", err)
	}
}

func (sb *sandbox) deleteHostsEntries(recs []etchosts.Record) {
	if err := etchosts.Delete(sb.config.hostsPath, recs); err != nil {
		log.Warnf("Failed deleting service host entries to the running container: %v", err)
	}
}

func (sb *sandbox) updateParentHosts() error {
	var pSb Sandbox

	for _, update := range sb.config.parentUpdates {
		sb.controller.WalkSandboxes(SandboxContainerWalker(&pSb, update.cid))
		if pSb == nil {
			continue
		}
		if err := etchosts.Update(pSb.(*sandbox).config.hostsPath, update.ip, update.name); err != nil {
			return err
		}
	}

	return nil
}

func (sb *sandbox) setupDNS() error {
	var newRC *resolvconf.File

	if sb.config.resolvConfPath == "" {
		sb.config.resolvConfPath = defaultPrefix + "/" + sb.id + "/resolv.conf"
	}

	sb.config.resolvConfHashFile = sb.config.resolvConfPath + ".hash"

	dir, _ := filepath.Split(sb.config.resolvConfPath)
	if err := createBasePath(dir); err != nil {
		return err
	}

	// This is for the host mode networking
	if sb.config.originResolvConfPath != "" {
		if err := copyFile(sb.config.originResolvConfPath, sb.config.resolvConfPath); err != nil {
			return fmt.Errorf("could not copy source resolv.conf file %s to %s: %v", sb.config.originResolvConfPath, sb.config.resolvConfPath, err)
		}
		return nil
	}

	currRC, err := resolvconf.Get()
	if err != nil {
		return err
	}

	if len(sb.config.dnsList) > 0 || len(sb.config.dnsSearchList) > 0 || len(sb.config.dnsOptionsList) > 0 {
		var (
			err            error
			dnsList        = resolvconf.GetNameservers(currRC.Content)
			dnsSearchList  = resolvconf.GetSearchDomains(currRC.Content)
			dnsOptionsList = resolvconf.GetOptions(currRC.Content)
		)
		if len(sb.config.dnsList) > 0 {
			dnsList = sb.config.dnsList
		}
		if len(sb.config.dnsSearchList) > 0 {
			dnsSearchList = sb.config.dnsSearchList
		}
		if len(sb.config.dnsOptionsList) > 0 {
			dnsOptionsList = sb.config.dnsOptionsList
		}
		newRC, err = resolvconf.Build(sb.config.resolvConfPath, dnsList, dnsSearchList, dnsOptionsList)
		if err != nil {
			return err
		}
	} else {
		// Replace any localhost/127.* (at this point we have no info about ipv6, pass it as true)
		if newRC, err = resolvconf.FilterResolvDNS(currRC.Content, true); err != nil {
			return err
		}
		// No contention on container resolv.conf file at sandbox creation
		if err := ioutil.WriteFile(sb.config.resolvConfPath, newRC.Content, filePerm); err != nil {
			return types.InternalErrorf("failed to write unhaltered resolv.conf file content when setting up dns for sandbox %s: %v", sb.ID(), err)
		}
	}

	// Write hash
	if err := ioutil.WriteFile(sb.config.resolvConfHashFile, []byte(newRC.Hash), filePerm); err != nil {
		return types.InternalErrorf("failed to write resolv.conf hash file when setting up dns for sandbox %s: %v", sb.ID(), err)
	}

	return nil
}

func (sb *sandbox) updateDNS(ipv6Enabled bool) error {
	var (
		currHash string
		hashFile = sb.config.resolvConfHashFile
	)

	if len(sb.config.dnsList) > 0 || len(sb.config.dnsSearchList) > 0 || len(sb.config.dnsOptionsList) > 0 {
		return nil
	}

	currRC, err := resolvconf.GetSpecific(sb.config.resolvConfPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		h, err := ioutil.ReadFile(hashFile)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			currHash = string(h)
		}
	}

	if currHash != "" && currHash != currRC.Hash {
		// Seems the user has changed the container resolv.conf since the last time
		// we checked so return without doing anything.
		log.Infof("Skipping update of resolv.conf file with ipv6Enabled: %t because file was touched by user", ipv6Enabled)
		return nil
	}

	// replace any localhost/127.* and remove IPv6 nameservers if IPv6 disabled.
	newRC, err := resolvconf.FilterResolvDNS(currRC.Content, ipv6Enabled)
	if err != nil {
		return err
	}

	// for atomic updates to these files, use temporary files with os.Rename:
	dir := path.Dir(sb.config.resolvConfPath)
	tmpHashFile, err := ioutil.TempFile(dir, "hash")
	if err != nil {
		return err
	}
	tmpResolvFile, err := ioutil.TempFile(dir, "resolv")
	if err != nil {
		return err
	}

	// Change the perms to filePerm (0644) since ioutil.TempFile creates it by default as 0600
	if err := os.Chmod(tmpResolvFile.Name(), filePerm); err != nil {
		return err
	}

	// write the updates to the temp files
	if err = ioutil.WriteFile(tmpHashFile.Name(), []byte(newRC.Hash), filePerm); err != nil {
		return err
	}
	if err = ioutil.WriteFile(tmpResolvFile.Name(), newRC.Content, filePerm); err != nil {
		return err
	}

	// rename the temp files for atomic replace
	if err = os.Rename(tmpHashFile.Name(), hashFile); err != nil {
		return err
	}
	return os.Rename(tmpResolvFile.Name(), sb.config.resolvConfPath)
}

// joinLeaveStart waits to ensure there are no joins or leaves in progress and
// marks this join/leave in progress without race
func (sb *sandbox) joinLeaveStart() {
	sb.Lock()
	defer sb.Unlock()

	for sb.joinLeaveDone != nil {
		joinLeaveDone := sb.joinLeaveDone
		sb.Unlock()

		select {
		case <-joinLeaveDone:
		}

		sb.Lock()
	}

	sb.joinLeaveDone = make(chan struct{})
}

// joinLeaveEnd marks the end of this join/leave operation and
// signals the same without race to other join and leave waiters
func (sb *sandbox) joinLeaveEnd() {
	sb.Lock()
	defer sb.Unlock()

	if sb.joinLeaveDone != nil {
		close(sb.joinLeaveDone)
		sb.joinLeaveDone = nil
	}
}

// OptionHostname function returns an option setter for hostname option to
// be passed to NewSandbox method.
func OptionHostname(name string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.hostName = name
	}
}

// OptionDomainname function returns an option setter for domainname option to
// be passed to NewSandbox method.
func OptionDomainname(name string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.domainName = name
	}
}

// OptionHostsPath function returns an option setter for hostspath option to
// be passed to NewSandbox method.
func OptionHostsPath(path string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.hostsPath = path
	}
}

// OptionOriginHostsPath function returns an option setter for origin hosts file path
// tbeo  passed to NewSandbox method.
func OptionOriginHostsPath(path string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.originHostsPath = path
	}
}

// OptionExtraHost function returns an option setter for extra /etc/hosts options
// which is a name and IP as strings.
func OptionExtraHost(name string, IP string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.extraHosts = append(sb.config.extraHosts, extraHost{name: name, IP: IP})
	}
}

// OptionParentUpdate function returns an option setter for parent container
// which needs to update the IP address for the linked container.
func OptionParentUpdate(cid string, name, ip string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.parentUpdates = append(sb.config.parentUpdates, parentUpdate{cid: cid, name: name, ip: ip})
	}
}

// OptionResolvConfPath function returns an option setter for resolvconfpath option to
// be passed to net container methods.
func OptionResolvConfPath(path string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.resolvConfPath = path
	}
}

// OptionOriginResolvConfPath function returns an option setter to set the path to the
// origin resolv.conf file to be passed to net container methods.
func OptionOriginResolvConfPath(path string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.originResolvConfPath = path
	}
}

// OptionDNS function returns an option setter for dns entry option to
// be passed to container Create method.
func OptionDNS(dns string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.dnsList = append(sb.config.dnsList, dns)
	}
}

// OptionDNSSearch function returns an option setter for dns search entry option to
// be passed to container Create method.
func OptionDNSSearch(search string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.dnsSearchList = append(sb.config.dnsSearchList, search)
	}
}

// OptionDNSOptions function returns an option setter for dns options entry option to
// be passed to container Create method.
func OptionDNSOptions(options string) SandboxOption {
	return func(sb *sandbox) {
		sb.config.dnsOptionsList = append(sb.config.dnsOptionsList, options)
	}
}

// OptionUseDefaultSandbox function returns an option setter for using default sandbox to
// be passed to container Create method.
func OptionUseDefaultSandbox() SandboxOption {
	return func(sb *sandbox) {
		sb.config.useDefaultSandBox = true
	}
}

// OptionUseExternalKey function returns an option setter for using provided namespace
// instead of creating one.
func OptionUseExternalKey() SandboxOption {
	return func(sb *sandbox) {
		sb.config.useExternalKey = true
	}
}

// OptionGeneric function returns an option setter for Generic configuration
// that is not managed by libNetwork but can be used by the Drivers during the call to
// net container creation method. Container Labels are a good example.
func OptionGeneric(generic map[string]interface{}) SandboxOption {
	return func(sb *sandbox) {
		sb.config.generic = generic
	}
}

func (eh epHeap) Len() int { return len(eh) }

func (eh epHeap) Less(i, j int) bool {
	ci, _ := eh[i].getSandbox()
	cj, _ := eh[j].getSandbox()

	epi := eh[i]
	epj := eh[j]

	if epi.endpointInGWNetwork() {
		return false
	}

	if epj.endpointInGWNetwork() {
		return true
	}

	cip, ok := ci.epPriority[eh[i].ID()]
	if !ok {
		cip = 0
	}
	cjp, ok := cj.epPriority[eh[j].ID()]
	if !ok {
		cjp = 0
	}
	if cip == cjp {
		return eh[i].getNetwork().Name() < eh[j].getNetwork().Name()
	}

	return cip > cjp
}

func (eh epHeap) Swap(i, j int) { eh[i], eh[j] = eh[j], eh[i] }

func (eh *epHeap) Push(x interface{}) {
	*eh = append(*eh, x.(*endpoint))
}

func (eh *epHeap) Pop() interface{} {
	old := *eh
	n := len(old)
	x := old[n-1]
	*eh = old[0 : n-1]
	return x
}

func createBasePath(dir string) error {
	return os.MkdirAll(dir, dirPerm)
}

func createFile(path string) error {
	var f *os.File

	dir, _ := filepath.Split(path)
	err := createBasePath(dir)
	if err != nil {
		return err
	}

	f, err = os.Create(path)
	if err == nil {
		f.Close()
	}

	return err
}

func copyFile(src, dst string) error {
	sBytes, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, sBytes, filePerm)
}
