package libnetwork

import (
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
	Join(containerID string) (string, error)

	// Leave removes the sandbox associated with  container ID and detaches
	// the network resources populated in the sandbox
	Leave(containerID string) error

	// SandboxInfo returns the sandbox information for this endpoint.
	SandboxInfo() *sandbox.Info

	// Delete and detaches this endpoint from the network.
	Delete() error
}

type endpoint struct {
	name        string
	id          types.UUID
	network     *network
	sandboxInfo *sandbox.Info
	sandBox     sandbox.Sandbox
	containerID string
}

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

func (ep *endpoint) Join(containerID string) (string, error) {
	if containerID == "" {
		return "", InvalidContainerIDError(containerID)
	}

	if ep.containerID != "" {
		return "", ErrInvalidJoin
	}

	sboxKey := sandbox.GenerateKey(containerID)
	sb, err := ep.network.ctrlr.sandboxAdd(sboxKey)
	if err != nil {
		return "", err
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
				return "", err
			}
		}

		err = sb.SetGateway(sinfo.Gateway)
		if err != nil {
			return "", err
		}

		err = sb.SetGatewayIPv6(sinfo.GatewayIPv6)
		if err != nil {
			return "", err
		}
	}

	ep.containerID = containerID
	return sb.Key(), nil
}

func (ep *endpoint) Leave(containerID string) error {
	if ep.containerID == "" || containerID == "" || ep.containerID != containerID {
		return InvalidContainerIDError(containerID)
	}

	ep.network.ctrlr.sandboxRm(sandbox.GenerateKey(containerID))
	ep.containerID = ""
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
