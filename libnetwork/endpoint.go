package libnetwork

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/etchosts"
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
	Join(containerID string, options ...JoinOption) (*ContainerData, error)

	// Leave removes the sandbox associated with  container ID and detaches
	// the network resources populated in the sandbox
	Leave(containerID string) error

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

type containerConfig struct {
	Hostname   string
	Domainname string
}

type containerInfo struct {
	ID     string
	Config containerConfig
	Data   ContainerData
}

type endpoint struct {
	name        string
	id          types.UUID
	network     *network
	sandboxInfo *sandbox.Info
	sandBox     sandbox.Sandbox
	container   *containerInfo
	generic     options.Generic
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

	ep.container.Data.HostsPath = prefix + "/" + containerID + "/hosts"
	err = createHostsFile(ep.container.Data.HostsPath)
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

	ep.container.ID = containerID
	ep.container.Data.SandboxKey = sb.Key()

	cData := ep.container.Data
	return &cData, nil
}

func (ep *endpoint) Leave(containerID string) error {
	if ep.container == nil || ep.container.ID == "" ||
		containerID == "" || ep.container.ID != containerID {
		return InvalidContainerIDError(containerID)
	}

	ep.network.ctrlr.sandboxRm(sandbox.GenerateKey(containerID))
	ep.container = nil
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

	name := ep.container.Config.Hostname
	if ep.container.Config.Domainname != "" {
		name = name + "." + ep.container.Config.Domainname
	}

	IP := ""
	if ep.sandboxInfo != nil && ep.sandboxInfo.Interfaces[0] != nil &&
		ep.sandboxInfo.Interfaces[0].Address != nil {
		IP = ep.sandboxInfo.Interfaces[0].Address.IP.String()
	}

	return etchosts.Build(ep.container.Data.HostsPath, IP, ep.container.Config.Hostname,
		ep.container.Config.Domainname, extraContent)
}

// JoinOptionHostname function returns an option setter for hostname option to
// be passed to endpoint Join method.
func JoinOptionHostname(name string) JoinOption {
	return func(ep *endpoint) {
		ep.container.Config.Hostname = name
	}
}

// JoinOptionDomainname function returns an option setter for domainname option to
// be passed to endpoint Join method.
func JoinOptionDomainname(name string) JoinOption {
	return func(ep *endpoint) {
		ep.container.Config.Domainname = name
	}
}

func (ep *endpoint) processJoinOptions(options ...JoinOption) {
	for _, opt := range options {
		if opt != nil {
			opt(ep)
		}
	}
}
