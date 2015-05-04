package libnetwork

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/etchosts"
	"github.com/docker/docker/pkg/resolvconf"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/pkg/options"
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

type containerConfig struct {
	hostName       string
	domainName     string
	generic        map[string]interface{}
	hostsPath      string
	ExtraHosts     []extraHost
	parentUpdates  []parentUpdate
	resolvConfPath string
	dnsList        []string
	dnsSearchList  []string
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
	name         string
	id           types.UUID
	network      *network
	sandboxInfo  *sandbox.Info
	sandBox      sandbox.Sandbox
	joinInfo     *driverapi.JoinInfo
	container    *containerInfo
	exposedPorts []netutils.TransportPort
	generic      map[string]interface{}
	context      map[string]interface{}
}

const defaultPrefix = "/var/lib/docker/network/files"

func (ep *endpoint) ID() string {
	return string(ep.id)
}

func (ep *endpoint) Name() string {
	return ep.name
}

func (ep *endpoint) Network() string {
	return ep.network.name
}

func (ep *endpoint) SandboxInfo() *sandbox.Info {
	if ep.sandboxInfo == nil {
		return nil
	}
	return ep.sandboxInfo.GetCopy()
}

func (ep *endpoint) Info() (map[string]interface{}, error) {
	return ep.network.driver.EndpointInfo(ep.network.id, ep.id)
}

func (ep *endpoint) processOptions(options ...EndpointOption) {
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

func (ep *endpoint) Join(containerID string, options ...EndpointOption) (*ContainerData, error) {
	var err error

	if containerID == "" {
		return nil, InvalidContainerIDError(containerID)
	}

	if ep.container != nil {
		return nil, ErrInvalidJoin
	}

	ep.container = &containerInfo{
		config: containerConfig{
			ExtraHosts:    []extraHost{},
			parentUpdates: []parentUpdate{},
		}}
	defer func() {
		if err != nil {
			ep.container = nil
		}
	}()

	ep.processOptions(options...)

	if ep.container.config.hostsPath == "" {
		ep.container.config.hostsPath = defaultPrefix + "/" + containerID + "/hosts"
	}

	if ep.container.config.resolvConfPath == "" {
		ep.container.config.resolvConfPath = defaultPrefix + "/" + containerID + "/resolv.conf"
	}

	sboxKey := sandbox.GenerateKey(containerID)

	joinInfo, err := ep.network.driver.Join(ep.network.id, ep.id,
		sboxKey, ep.container.config.generic)
	if err != nil {
		return nil, err
	}
	ep.joinInfo = joinInfo

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

	create := true
	if joinInfo != nil {
		if joinInfo.SandboxKey != "" {
			sboxKey = joinInfo.SandboxKey
		}

		if joinInfo.NoSandboxCreate {
			create = false
		}
	}

	sb, err := ep.network.ctrlr.sandboxAdd(sboxKey, create)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			ep.network.ctrlr.sandboxRm(sboxKey)
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

	ep.container.id = containerID
	ep.container.data.SandboxKey = sb.Key()

	cData := ep.container.data
	return &cData, nil
}

func (ep *endpoint) Leave(containerID string, options ...EndpointOption) error {
	if ep.container == nil || ep.container.id == "" ||
		containerID == "" || ep.container.id != containerID {
		return InvalidContainerIDError(containerID)
	}

	ep.processOptions(options...)

	n := ep.network
	err := n.driver.Leave(n.id, ep.id, ep.context)
	ep.network.ctrlr.sandboxRm(ep.container.data.SandboxKey)
	ep.container = nil
	ep.context = nil
	return err
}

func (ep *endpoint) Delete() error {
	var err error

	n := ep.network
	n.Lock()
	_, ok := n.endpoints[ep.id]
	if !ok {
		n.Unlock()
		return &UnknownEndpointError{name: ep.name, id: string(ep.id)}
	}

	delete(n.endpoints, ep.id)
	n.Unlock()
	defer func() {
		if err != nil {
			n.Lock()
			n.endpoints[ep.id] = ep
			n.Unlock()
		}
	}()

	err = n.driver.DeleteEndpoint(n.id, ep.id)
	return err
}

func (ep *endpoint) buildHostsFiles() error {
	var extraContent []etchosts.Record

	dir, _ := filepath.Split(ep.container.config.hostsPath)
	err := createBasePath(dir)
	if err != nil {
		return err
	}

	if ep.joinInfo != nil && ep.joinInfo.HostsPath != "" {
		content, err := ioutil.ReadFile(ep.joinInfo.HostsPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if err == nil {
			return ioutil.WriteFile(ep.container.config.hostsPath, content, 0644)
		}
	}

	name := ep.container.config.hostName
	if ep.container.config.domainName != "" {
		name = name + "." + ep.container.config.domainName
	}

	for _, extraHost := range ep.container.config.ExtraHosts {
		extraContent = append(extraContent,
			etchosts.Record{Hosts: extraHost.name, IP: extraHost.IP})
	}

	IP := ""
	if ep.sandboxInfo != nil && ep.sandboxInfo.Interfaces[0] != nil &&
		ep.sandboxInfo.Interfaces[0].Address != nil {
		IP = ep.sandboxInfo.Interfaces[0].Address.IP.String()
	}

	return etchosts.Build(ep.container.config.hostsPath, IP, ep.container.config.hostName,
		ep.container.config.domainName, extraContent)
}

func (ep *endpoint) updateParentHosts() error {
	for _, update := range ep.container.config.parentUpdates {
		ep.network.Lock()
		pep, ok := ep.network.endpoints[types.UUID(update.eid)]
		if !ok {
			ep.network.Unlock()
			continue
		}
		ep.network.Unlock()

		if err := etchosts.Update(pep.container.config.hostsPath,
			update.ip, update.name); err != nil {
			return err
		}
	}

	return nil
}

func (ep *endpoint) setupDNS() error {
	dir, _ := filepath.Split(ep.container.config.resolvConfPath)
	err := createBasePath(dir)
	if err != nil {
		return err
	}

	resolvConf, err := resolvconf.Get()
	if err != nil {
		return err
	}

	if len(ep.container.config.dnsList) > 0 ||
		len(ep.container.config.dnsSearchList) > 0 {
		var (
			dnsList       = resolvconf.GetNameservers(resolvConf)
			dnsSearchList = resolvconf.GetSearchDomains(resolvConf)
		)

		if len(ep.container.config.dnsList) > 0 {
			dnsList = ep.container.config.dnsList
		}

		if len(ep.container.config.dnsSearchList) > 0 {
			dnsSearchList = ep.container.config.dnsSearchList
		}

		return resolvconf.Build(ep.container.config.resolvConfPath, dnsList, dnsSearchList)
	}

	// replace any localhost/127.* but always discard IPv6 entries for now.
	resolvConf, _ = resolvconf.FilterResolvDns(resolvConf, false)
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
		ep.container.config.ExtraHosts = append(ep.container.config.ExtraHosts, extraHost{name: name, IP: IP})
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

// CreateOptionPortMapping function returns an option setter for the container exposed
// ports option to be passed to network.CreateEndpoint() method.
func CreateOptionPortMapping(portBindings []netutils.PortBinding) EndpointOption {
	return func(ep *endpoint) {
		// Store endpoint label
		ep.generic[options.PortMap] = portBindings
		// Extract exposed ports as this is the only concern of libnetwork endpoint
		ep.exposedPorts = make([]netutils.TransportPort, 0, len(portBindings))
		for _, b := range portBindings {
			ep.exposedPorts = append(ep.exposedPorts, netutils.TransportPort{Proto: b.Proto, Port: b.Port})
		}
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
