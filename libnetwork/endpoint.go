package libnetwork

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/pkg/etchosts"
	"github.com/docker/libnetwork/pkg/netlabel"
	"github.com/docker/libnetwork/pkg/resolvconf"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
)

// Endpoint represents a logical connection between a network and a sandbox.
type Endpoint interface {
	// A system generated id for this endpoint.
	ID() string

	// Name returns the name of this endpoint.
	Name() string

	// Network returns the name of the network to which this endpoint is attached.
	Network() string

	// Join creates a new sandbox for the given container ID and populates the
	// network resources allocated for the endpoint and joins the sandbox to
	// the endpoint. It returns the sandbox key to the caller
	Join(containerID string, options ...EndpointOption) (*ContainerData, error)

	// Leave removes the sandbox associated with  container ID and detaches
	// the network resources populated in the sandbox
	Leave(containerID string, options ...EndpointOption) error

	// SandboxInfo returns the sandbox information for this endpoint.
	SandboxInfo() *sandbox.Info

	// Info returns a collection of operational data related to this endpoint retrieved from the driver
	Info() (map[string]interface{}, error)

	// Delete and detaches this endpoint from the network.
	Delete() error
}

// EndpointOption is a option setter function type used to pass varios options to Network
// and Endpoint interfaces methods. The various setter functions of type EndpointOption are
// provided by libnetwork, they look like <Create|Join|Leave>Option[...](...)
type EndpointOption func(ep *endpoint)

// ContainerData is a set of data returned when a container joins an endpoint.
type ContainerData struct {
	SandboxKey string
}

// These are the container configs used to customize container /etc/hosts file.
type hostsPathConfig struct {
	hostName      string
	domainName    string
	hostsPath     string
	extraHosts    []extraHost
	parentUpdates []parentUpdate
}

// These are the container configs used to customize container /etc/resolv.conf file.
type resolvConfPathConfig struct {
	resolvConfPath string
	dnsList        []string
	dnsSearchList  []string
}

type containerConfig struct {
	hostsPathConfig
	resolvConfPathConfig
	generic           map[string]interface{}
	useDefaultSandBox bool
}

type extraHost struct {
	name string
	IP   string
}

type parentUpdate struct {
	eid  string
	name string
	ip   string
}

type containerInfo struct {
	id     string
	config containerConfig
	data   ContainerData
}

type endpoint struct {
	name          string
	id            types.UUID
	network       *network
	sandboxInfo   *sandbox.Info
	sandBox       sandbox.Sandbox
	joinInfo      *driverapi.JoinInfo
	container     *containerInfo
	exposedPorts  []netutils.TransportPort
	generic       map[string]interface{}
	context       map[string]interface{}
	joinLeaveDone chan struct{}
	sync.Mutex
}

const defaultPrefix = "/var/lib/docker/network/files"

func (ep *endpoint) ID() string {
	ep.Lock()
	defer ep.Unlock()

	return string(ep.id)
}

func (ep *endpoint) Name() string {
	ep.Lock()
	defer ep.Unlock()

	return ep.name
}

func (ep *endpoint) Network() string {
	ep.Lock()
	defer ep.Unlock()

	return ep.network.name
}

func (ep *endpoint) SandboxInfo() *sandbox.Info {
	ep.Lock()
	defer ep.Unlock()

	if ep.sandboxInfo == nil {
		return nil
	}
	return ep.sandboxInfo.GetCopy()
}

func (ep *endpoint) Info() (map[string]interface{}, error) {
	ep.Lock()
	network := ep.network
	epid := ep.id
	ep.Unlock()

	network.Lock()
	driver := network.driver
	nid := network.id
	network.Unlock()

	return driver.EndpointInfo(nid, epid)
}

func (ep *endpoint) processOptions(options ...EndpointOption) {
	ep.Lock()
	defer ep.Unlock()

	for _, opt := range options {
		if opt != nil {
			opt(ep)
		}
	}
}

func createBasePath(dir string) error {
	err := os.MkdirAll(dir, 0644)
	if err != nil && !os.IsExist(err) {
		return err
	}

	return nil
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

// joinLeaveStart waits to ensure there are no joins or leaves in progress and
// marks this join/leave in progress without race
func (ep *endpoint) joinLeaveStart() {
	ep.Lock()
	defer ep.Unlock()

	for ep.joinLeaveDone != nil {
		joinLeaveDone := ep.joinLeaveDone
		ep.Unlock()

		select {
		case <-joinLeaveDone:
		}

		ep.Lock()
	}

	ep.joinLeaveDone = make(chan struct{})
}

// joinLeaveEnd marks the end of this join/leave operation and
// signals the same without race to other join and leave waiters
func (ep *endpoint) joinLeaveEnd() {
	ep.Lock()
	defer ep.Unlock()

	if ep.joinLeaveDone != nil {
		close(ep.joinLeaveDone)
		ep.joinLeaveDone = nil
	}
}

func (ep *endpoint) Join(containerID string, options ...EndpointOption) (*ContainerData, error) {
	var err error

	if containerID == "" {
		return nil, InvalidContainerIDError(containerID)
	}

	ep.joinLeaveStart()
	defer ep.joinLeaveEnd()

	ep.Lock()
	if ep.container != nil {
		ep.Unlock()
		return nil, ErrInvalidJoin
	}

	ep.container = &containerInfo{
		id: containerID,
		config: containerConfig{
			hostsPathConfig: hostsPathConfig{
				extraHosts:    []extraHost{},
				parentUpdates: []parentUpdate{},
			},
		}}

	container := ep.container
	network := ep.network
	epid := ep.id

	ep.Unlock()
	defer func() {
		ep.Lock()
		if err != nil {
			ep.container = nil
		}
		ep.Unlock()
	}()

	network.Lock()
	driver := network.driver
	nid := network.id
	ctrlr := network.ctrlr
	network.Unlock()

	ep.processOptions(options...)

	sboxKey := sandbox.GenerateKey(containerID)
	if container.config.useDefaultSandBox {
		sboxKey = sandbox.GenerateKey("default")
	}

	joinInfo, err := driver.Join(nid, epid, sboxKey, container.config.generic)
	if err != nil {
		return nil, err
	}

	ep.Lock()
	ep.joinInfo = joinInfo
	ep.Unlock()

	err = ep.buildHostsFiles()
	if err != nil {
		return nil, err
	}

	err = ep.updateParentHosts()
	if err != nil {
		return nil, err
	}

	err = ep.setupDNS()
	if err != nil {
		return nil, err
	}

	sb, err := ctrlr.sandboxAdd(sboxKey, !container.config.useDefaultSandBox)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			ctrlr.sandboxRm(sboxKey)
		}
	}()

	sinfo := ep.SandboxInfo()
	if sinfo != nil {
		for _, i := range sinfo.Interfaces {
			err = sb.AddInterface(i)
			if err != nil {
				return nil, err
			}
		}

		err = sb.SetGateway(sinfo.Gateway)
		if err != nil {
			return nil, err
		}

		err = sb.SetGatewayIPv6(sinfo.GatewayIPv6)
		if err != nil {
			return nil, err
		}
	}

	container.data.SandboxKey = sb.Key()
	cData := container.data

	return &cData, nil
}

func (ep *endpoint) Leave(containerID string, options ...EndpointOption) error {
	var err error

	ep.joinLeaveStart()
	defer ep.joinLeaveEnd()

	ep.processOptions(options...)

	ep.Lock()
	container := ep.container
	n := ep.network
	context := ep.context

	if container == nil || container.id == "" ||
		containerID == "" || container.id != containerID {
		if container == nil {
			err = ErrNoContainer
		} else {
			err = InvalidContainerIDError(containerID)
		}

		ep.Unlock()
		return err
	}
	ep.container = nil
	ep.context = nil
	ep.Unlock()

	n.Lock()
	driver := n.driver
	ctrlr := n.ctrlr
	n.Unlock()

	err = driver.Leave(n.id, ep.id, context)

	sinfo := ep.SandboxInfo()
	if sinfo != nil {
		sb := ctrlr.sandboxGet(container.data.SandboxKey)
		for _, i := range sinfo.Interfaces {
			err = sb.RemoveInterface(i)
			if err != nil {
				logrus.Debugf("Remove interface failed: %v", err)
			}
		}
	}

	ctrlr.sandboxRm(container.data.SandboxKey)

	return err
}

func (ep *endpoint) Delete() error {
	var err error

	ep.Lock()
	epid := ep.id
	name := ep.name
	if ep.container != nil {
		ep.Unlock()
		return &ActiveContainerError{name: name, id: string(epid)}
	}

	n := ep.network
	ep.Unlock()

	n.Lock()
	_, ok := n.endpoints[epid]
	if !ok {
		n.Unlock()
		return &UnknownEndpointError{name: name, id: string(epid)}
	}

	nid := n.id
	driver := n.driver
	delete(n.endpoints, epid)
	n.Unlock()
	defer func() {
		if err != nil {
			n.Lock()
			n.endpoints[epid] = ep
			n.Unlock()
		}
	}()

	err = driver.DeleteEndpoint(nid, epid)
	return err
}

func (ep *endpoint) buildHostsFiles() error {
	var extraContent []etchosts.Record

	ep.Lock()
	container := ep.container
	joinInfo := ep.joinInfo
	ep.Unlock()

	if container == nil {
		return ErrNoContainer
	}

	if container.config.hostsPath == "" {
		container.config.hostsPath = defaultPrefix + "/" + container.id + "/hosts"
	}

	dir, _ := filepath.Split(container.config.hostsPath)
	err := createBasePath(dir)
	if err != nil {
		return err
	}

	if joinInfo != nil && joinInfo.HostsPath != "" {
		content, err := ioutil.ReadFile(joinInfo.HostsPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if err == nil {
			return ioutil.WriteFile(container.config.hostsPath, content, 0644)
		}
	}

	name := container.config.hostName
	if container.config.domainName != "" {
		name = name + "." + container.config.domainName
	}

	for _, extraHost := range container.config.extraHosts {
		extraContent = append(extraContent,
			etchosts.Record{Hosts: extraHost.name, IP: extraHost.IP})
	}

	IP := ""
	sinfo := ep.SandboxInfo()
	if sinfo != nil && sinfo.Interfaces[0] != nil &&
		sinfo.Interfaces[0].Address != nil {
		IP = sinfo.Interfaces[0].Address.IP.String()
	}

	return etchosts.Build(container.config.hostsPath, IP, container.config.hostName,
		container.config.domainName, extraContent)
}

func (ep *endpoint) updateParentHosts() error {
	ep.Lock()
	container := ep.container
	network := ep.network
	ep.Unlock()

	if container == nil {
		return ErrNoContainer
	}

	for _, update := range container.config.parentUpdates {
		network.Lock()
		pep, ok := network.endpoints[types.UUID(update.eid)]
		if !ok {
			network.Unlock()
			continue
		}
		network.Unlock()

		pep.Lock()
		pContainer := pep.container
		pep.Unlock()

		if pContainer != nil {
			if err := etchosts.Update(pContainer.config.hostsPath, update.ip, update.name); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ep *endpoint) setupDNS() error {
	ep.Lock()
	container := ep.container
	network := ep.network
	ep.Unlock()

	if container == nil {
		return ErrNoContainer
	}

	if container.config.resolvConfPath == "" {
		container.config.resolvConfPath = defaultPrefix + "/" + container.id + "/resolv.conf"
	}

	dir, _ := filepath.Split(container.config.resolvConfPath)
	err := createBasePath(dir)
	if err != nil {
		return err
	}

	resolvConf, err := resolvconf.Get()
	if err != nil {
		return err
	}

	if len(container.config.dnsList) > 0 ||
		len(container.config.dnsSearchList) > 0 {
		var (
			dnsList       = resolvconf.GetNameservers(resolvConf)
			dnsSearchList = resolvconf.GetSearchDomains(resolvConf)
		)

		if len(container.config.dnsList) > 0 {
			dnsList = container.config.dnsList
		}

		if len(container.config.dnsSearchList) > 0 {
			dnsSearchList = container.config.dnsSearchList
		}

		return resolvconf.Build(container.config.resolvConfPath, dnsList, dnsSearchList)
	}

	// replace any localhost/127.* but always discard IPv6 entries for now.
	resolvConf, _ = resolvconf.FilterResolvDNS(resolvConf, network.enableIPv6)
	return ioutil.WriteFile(ep.container.config.resolvConfPath, resolvConf, 0644)
}

// EndpointOptionGeneric function returns an option setter for a Generic option defined
// in a Dictionary of Key-Value pair
func EndpointOptionGeneric(generic map[string]interface{}) EndpointOption {
	return func(ep *endpoint) {
		for k, v := range generic {
			ep.generic[k] = v
		}
	}
}

// JoinOptionHostname function returns an option setter for hostname option to
// be passed to endpoint Join method.
func JoinOptionHostname(name string) EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.hostName = name
	}
}

// JoinOptionDomainname function returns an option setter for domainname option to
// be passed to endpoint Join method.
func JoinOptionDomainname(name string) EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.domainName = name
	}
}

// JoinOptionHostsPath function returns an option setter for hostspath option to
// be passed to endpoint Join method.
func JoinOptionHostsPath(path string) EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.hostsPath = path
	}
}

// JoinOptionExtraHost function returns an option setter for extra /etc/hosts options
// which is a name and IP as strings.
func JoinOptionExtraHost(name string, IP string) EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.extraHosts = append(ep.container.config.extraHosts, extraHost{name: name, IP: IP})
	}
}

// JoinOptionParentUpdate function returns an option setter for parent container
// which needs to update the IP address for the linked container.
func JoinOptionParentUpdate(eid string, name, ip string) EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.parentUpdates = append(ep.container.config.parentUpdates, parentUpdate{eid: eid, name: name, ip: ip})
	}
}

// JoinOptionResolvConfPath function returns an option setter for resolvconfpath option to
// be passed to endpoint Join method.
func JoinOptionResolvConfPath(path string) EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.resolvConfPath = path
	}
}

// JoinOptionDNS function returns an option setter for dns entry option to
// be passed to endpoint Join method.
func JoinOptionDNS(dns string) EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.dnsList = append(ep.container.config.dnsList, dns)
	}
}

// JoinOptionDNSSearch function returns an option setter for dns search entry option to
// be passed to endpoint Join method.
func JoinOptionDNSSearch(search string) EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.dnsSearchList = append(ep.container.config.dnsSearchList, search)
	}
}

// JoinOptionUseDefaultSandbox function returns an option setter for using default sandbox to
// be passed to endpoint Join method.
func JoinOptionUseDefaultSandbox() EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.useDefaultSandBox = true
	}
}

// CreateOptionExposedPorts function returns an option setter for the container exposed
// ports option to be passed to network.CreateEndpoint() method.
func CreateOptionExposedPorts(exposedPorts []netutils.TransportPort) EndpointOption {
	return func(ep *endpoint) {
		// Defensive copy
		eps := make([]netutils.TransportPort, len(exposedPorts))
		copy(eps, exposedPorts)
		// Store endpoint label and in generic because driver needs it
		ep.exposedPorts = eps
		ep.generic[netlabel.ExposedPorts] = eps
	}
}

// CreateOptionPortMapping function returns an option setter for the mapping
// ports option to be passed to network.CreateEndpoint() method.
func CreateOptionPortMapping(portBindings []netutils.PortBinding) EndpointOption {
	return func(ep *endpoint) {
		// Store a copy of the bindings as generic data to pass to the driver
		pbs := make([]netutils.PortBinding, len(portBindings))
		copy(pbs, portBindings)
		ep.generic[netlabel.PortMap] = pbs
	}
}

// JoinOptionGeneric function returns an option setter for Generic configuration
// that is not managed by libNetwork but can be used by the Drivers during the call to
// endpoint join method. Container Labels are a good example.
func JoinOptionGeneric(generic map[string]interface{}) EndpointOption {
	return func(ep *endpoint) {
		ep.container.config.generic = generic
	}
}

// LeaveOptionGeneric function returns an option setter for Generic configuration
// that is not managed by libNetwork but can be used by the Drivers during the call to
// endpoint leave method. Container Labels are a good example.
func LeaveOptionGeneric(context map[string]interface{}) EndpointOption {
	return func(ep *endpoint) {
		ep.context = context
	}
}
