package networkallocator

import (
	"fmt"

	"github.com/docker/libnetwork/idm"
	"github.com/docker/swarmkit/api"
)

const (
	// Start of the dynamic port range from which node ports will
	// be allocated when the user did not specify a port.
	dynamicPortStart = 30000

	// End of the dynamic port range from which node ports will be
	// allocated when the user did not specify a port.
	dynamicPortEnd = 32767

	// The start of master port range which will hold all the
	// allocation state of ports allocated so far regerdless of
	// whether it was user defined or not.
	masterPortStart = 1

	// The end of master port range which will hold all the
	// allocation state of ports allocated so far regerdless of
	// whether it was user defined or not.
	masterPortEnd = 65535
)

type portAllocator struct {
	// portspace definition per protocol
	portSpaces map[api.PortConfig_Protocol]*portSpace
}

type portSpace struct {
	protocol         api.PortConfig_Protocol
	masterPortSpace  *idm.Idm
	dynamicPortSpace *idm.Idm
}

func newPortAllocator() (*portAllocator, error) {
	portSpaces := make(map[api.PortConfig_Protocol]*portSpace)
	for _, protocol := range []api.PortConfig_Protocol{api.ProtocolTCP, api.ProtocolUDP} {
		ps, err := newPortSpace(protocol)
		if err != nil {
			return nil, err
		}

		portSpaces[protocol] = ps
	}

	return &portAllocator{portSpaces: portSpaces}, nil
}

func newPortSpace(protocol api.PortConfig_Protocol) (*portSpace, error) {
	masterName := fmt.Sprintf("%s-master-ports", protocol)
	dynamicName := fmt.Sprintf("%s-dynamic-ports", protocol)

	master, err := idm.New(nil, masterName, masterPortStart, masterPortEnd)
	if err != nil {
		return nil, err
	}

	dynamic, err := idm.New(nil, dynamicName, dynamicPortStart, dynamicPortEnd)
	if err != nil {
		return nil, err
	}

	return &portSpace{
		protocol:         protocol,
		masterPortSpace:  master,
		dynamicPortSpace: dynamic,
	}, nil
}

func reconcilePortConfigs(s *api.Service) []*api.PortConfig {
	// If runtime state hasn't been created or if port config has
	// changed from port state return the port config from Spec.
	if s.Endpoint == nil || len(s.Spec.Endpoint.Ports) != len(s.Endpoint.Ports) {
		return s.Spec.Endpoint.Ports
	}

	var portConfigs []*api.PortConfig
	for i, portConfig := range s.Spec.Endpoint.Ports {
		portState := s.Endpoint.Ports[i]

		// If the portConfig is exactly the same as portState
		// except if SwarmPort is not user-define then prefer
		// portState to ensure sticky allocation of the same
		// port that was allocated before.
		if portConfig.Name == portState.Name &&
			portConfig.TargetPort == portState.TargetPort &&
			portConfig.Protocol == portState.Protocol &&
			portConfig.PublishedPort == 0 {
			portConfigs = append(portConfigs, portState)
			continue
		}

		// For all other cases prefer the portConfig
		portConfigs = append(portConfigs, portConfig)
	}

	return portConfigs
}

func (pa *portAllocator) serviceAllocatePorts(s *api.Service) (err error) {
	if s.Spec.Endpoint == nil {
		return nil
	}

	// We might have previous allocations which we want to stick
	// to if possible. So instead of strictly going by port
	// configs in the Spec reconcile the list of port configs from
	// both the Spec and runtime state.
	portConfigs := reconcilePortConfigs(s)

	// Port configuration might have changed. Cleanup all old allocations first.
	pa.serviceDeallocatePorts(s)

	defer func() {
		if err != nil {
			// Free all the ports allocated so far which
			// should be present in s.Endpoints.ExposedPorts
			pa.serviceDeallocatePorts(s)
		}
	}()

	for _, portConfig := range portConfigs {
		// Make a copy of port config to create runtime state
		portState := portConfig.Copy()
		if err = pa.portSpaces[portState.Protocol].allocate(portState); err != nil {
			return
		}

		if s.Endpoint == nil {
			s.Endpoint = &api.Endpoint{}
		}

		s.Endpoint.Ports = append(s.Endpoint.Ports, portState)
	}

	return nil
}

func (pa *portAllocator) serviceDeallocatePorts(s *api.Service) {
	if s.Endpoint == nil {
		return
	}

	for _, portState := range s.Endpoint.Ports {
		pa.portSpaces[portState.Protocol].free(portState)
	}

	s.Endpoint.Ports = nil
}

func (pa *portAllocator) isPortsAllocated(s *api.Service) bool {
	if s.Endpoint == nil {
		return false
	}

	// If we don't have same number of port states as port configs
	// we assume it is not allocated.
	if len(s.Spec.Endpoint.Ports) != len(s.Endpoint.Ports) {
		return false
	}

	for i, portConfig := range s.Spec.Endpoint.Ports {
		// The port configuration slice and port state slice
		// are expected to be in the same order.
		portState := s.Endpoint.Ports[i]

		// If name, port, protocol values don't match then we
		// are not allocated.
		if portConfig.Name != portState.Name ||
			portConfig.TargetPort != portState.TargetPort ||
			portConfig.Protocol != portState.Protocol {
			return false
		}

		// If SwarmPort was user defined but the port state
		// SwarmPort doesn't match we are not allocated.
		if portConfig.PublishedPort != portState.PublishedPort &&
			portConfig.PublishedPort != 0 {
			return false
		}

		// If SwarmPort was not defined by user and port state
		// is not initialized with a valid SwarmPort value then
		// we are not allocated.
		if portConfig.PublishedPort == 0 && portState.PublishedPort == 0 {
			return false
		}
	}

	return true
}

func (ps *portSpace) allocate(p *api.PortConfig) (err error) {
	if p.PublishedPort != 0 {
		// If it falls in the dynamic port range check out
		// from dynamic port space first.
		if p.PublishedPort >= dynamicPortStart && p.PublishedPort <= dynamicPortEnd {
			if err = ps.dynamicPortSpace.GetSpecificID(uint64(p.PublishedPort)); err != nil {
				return err
			}

			defer func() {
				if err != nil {
					ps.dynamicPortSpace.Release(uint64(p.PublishedPort))
				}
			}()
		}

		return ps.masterPortSpace.GetSpecificID(uint64(p.PublishedPort))
	}

	// Check out an arbitrary port from dynamic port space.
	swarmPort, err := ps.dynamicPortSpace.GetID()
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			ps.dynamicPortSpace.Release(uint64(swarmPort))
		}
	}()

	// Make sure we allocate the same port from the master space.
	if err = ps.masterPortSpace.GetSpecificID(uint64(swarmPort)); err != nil {
		return
	}

	p.PublishedPort = uint32(swarmPort)
	return nil
}

func (ps *portSpace) free(p *api.PortConfig) {
	if p.PublishedPort >= dynamicPortStart && p.PublishedPort <= dynamicPortEnd {
		ps.dynamicPortSpace.Release(uint64(p.PublishedPort))
	}

	ps.masterPortSpace.Release(uint64(p.PublishedPort))
}
