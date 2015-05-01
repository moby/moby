package libnetwork

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/etchosts"
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
	Join(containerID string, options ...JoinOption) (*ContainerData, error)

	// Leave removes the sandbox associated with  container ID and detaches
	// the network resources populated in the sandbox
	Leave(containerID string, options ...LeaveOption) error

	// SandboxInfo returns the sandbox information for this endpoint.
	SandboxInfo() *sandbox.Info

	// Delete and detaches this endpoint from the network.
	Delete() error
}

// ContainerData is a set of data returned when a container joins an endpoint.
type ContainerData struct {
	SandboxKey string
	HostsPath  string
}

// JoinOption is a option setter function type used to pass varios options to
// endpoint Join method. The various setter functions of type JoinOption are
// provided by libnetwork, they look like JoinOption[...](...)
type JoinOption func(ep *endpoint)

// LeaveOption is a option setter function type used to pass varios options to
// endpoint Leave method. The various setter functions of type LeaveOption are
// provided by libnetwork, they look like LeaveOptionXXXX(...)
type LeaveOption func(ep *endpoint)

type containerConfig struct {
	hostName   string
	domainName string
	generic    map[string]interface{}
}

type containerInfo struct {
	id     string
	config containerConfig
	data   ContainerData
}

type endpoint struct {
	name        string
	id          types.UUID
	network     *network
	sandboxInfo *sandbox.Info
	sandBox     sandbox.Sandbox
	container   *containerInfo
	generic     map[string]interface{}
	context     map[string]interface{}
}

const prefix = "/var/lib/docker/network/files"

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

// EndpointOption is a option setter function type used to pass various options to
// CreateEndpoint method. The various setter functions of type EndpointOption are
// provided by libnetwork, they look like EndpointOptionXXXX(...)
type EndpointOption func(ep *endpoint)

// EndpointOptionGeneric function returns an option setter for a Generic option defined
// in a Dictionary of Key-Value pair
func EndpointOptionGeneric(generic map[string]interface{}) EndpointOption {
	return func(ep *endpoint) {
		ep.generic = generic
	}
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

func createHostsFile(path string) error {
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

func (ep *endpoint) Join(containerID string, options ...JoinOption) (*ContainerData, error) {
	var err error

	if containerID == "" {
		return nil, InvalidContainerIDError(containerID)
	}

	if ep.container != nil {
		return nil, ErrInvalidJoin
	}

	ep.container = &containerInfo{}
	defer func() {
		if err != nil {
			ep.container = nil
		}
	}()

	ep.processJoinOptions(options...)

	ep.container.data.HostsPath = prefix + "/" + containerID + "/hosts"
	err = createHostsFile(ep.container.data.HostsPath)
	if err != nil {
		return nil, err
	}

	err = ep.buildHostsFiles()
	if err != nil {
		return nil, err
	}

	sboxKey := sandbox.GenerateKey(containerID)
	sb, err := ep.network.ctrlr.sandboxAdd(sboxKey)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			ep.network.ctrlr.sandboxRm(sboxKey)
		}
	}()

	n := ep.network
	err = n.driver.Join(n.id, ep.id, sboxKey, ep.container.Config.generic)
	if err != nil {
		return nil, err
	}

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

func (ep *endpoint) Leave(containerID string, options ...LeaveOption) error {
	if ep.container == nil || ep.container.id == "" ||
		containerID == "" || ep.container.id != containerID {
		return InvalidContainerIDError(containerID)
	}

	ep.processLeaveOptions(options...)

	n := ep.network
	err := n.driver.Leave(n.id, ep.id, ep.context)
	if err != nil {
		return err
	}

	ep.network.ctrlr.sandboxRm(ep.container.data.SandboxKey)
	ep.container = nil
	ep.context = nil
	return nil
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

	name := ep.container.config.hostName
	if ep.container.config.domainName != "" {
		name = name + "." + ep.container.config.domainName
	}

	IP := ""
	if ep.sandboxInfo != nil && ep.sandboxInfo.Interfaces[0] != nil &&
		ep.sandboxInfo.Interfaces[0].Address != nil {
		IP = ep.sandboxInfo.Interfaces[0].Address.IP.String()
	}

	return etchosts.Build(ep.container.data.HostsPath, IP, ep.container.config.hostName,
		ep.container.config.domainName, extraContent)
}

// JoinOptionHostname function returns an option setter for hostname option to
// be passed to endpoint Join method.
func JoinOptionHostname(name string) JoinOption {
	return func(ep *endpoint) {
		ep.container.config.hostName = name
	}
}

// JoinOptionDomainname function returns an option setter for domainname option to
// be passed to endpoint Join method.
func JoinOptionDomainname(name string) JoinOption {
	return func(ep *endpoint) {
		ep.container.config.domainName = name
	}
}

// JoinOptionGeneric function returns an option setter for Generic configuration
// that is not managed by libNetwork but can be used by the Drivers during the call to
// endpoint join method. Container Labels are a good example.
func JoinOptionGeneric(generic map[string]interface{}) JoinOption {
	return func(ep *endpoint) {
		ep.container.Config.generic = generic
	}
}

func (ep *endpoint) processJoinOptions(options ...JoinOption) {
	for _, opt := range options {
		if opt != nil {
			opt(ep)
		}
	}
}

// LeaveOptionGeneric function returns an option setter for Generic configuration
// that is not managed by libNetwork but can be used by the Drivers during the call to
// endpoint leave method. Container Labels are a good example.
func LeaveOptionGeneric(context map[string]interface{}) JoinOption {
	return func(ep *endpoint) {
		ep.context = context
	}
}

func (ep *endpoint) processLeaveOptions(options ...LeaveOption) {
	for _, opt := range options {
		if opt != nil {
			opt(ep)
		}
	}
}
