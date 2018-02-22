package cnmallocator

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
	// allocation state of ports allocated so far regardless of
	// whether it was user defined or not.
	masterPortStart = 1

	// The end of master port range which will hold all the
	// allocation state of ports allocated so far regardless of
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

type allocatedPorts map[api.PortConfig]map[uint32]*api.PortConfig

// addState add the state of an allocated port to the collection.
// `allocatedPorts` is a map of portKey:publishedPort:portState.
// In case the value of the portKey is missing, the map
// publishedPort:portState is created automatically
func (ps allocatedPorts) addState(p *api.PortConfig) {
	portKey := getPortConfigKey(p)
	if _, ok := ps[portKey]; !ok {
		ps[portKey] = make(map[uint32]*api.PortConfig)
	}
	ps[portKey][p.PublishedPort] = p
}

// delState delete the state of an allocated port from the collection.
// `allocatedPorts` is a map of portKey:publishedPort:portState.
//
// If publishedPort is non-zero, then it is user defined. We will try to
// remove the portState from `allocatedPorts` directly and return
// the portState (or nil if no portState exists)
//
// If publishedPort is zero, then it is dynamically allocated. We will try
// to remove the portState from `allocatedPorts`, as long as there is
// a portState associated with a non-zero publishedPort.
// Note multiple dynamically allocated ports might exists. In this case,
// we will remove only at a time so both allocated ports are tracked.
//
// Note because of the potential co-existence of user-defined and dynamically
// allocated ports, delState has to be called for user-defined port first.
// dynamically allocated ports should be removed later.
func (ps allocatedPorts) delState(p *api.PortConfig) *api.PortConfig {
	portKey := getPortConfigKey(p)

	portStateMap, ok := ps[portKey]

	// If name, port, protocol values don't match then we
	// are not allocated.
	if !ok {
		return nil
	}

	if p.PublishedPort != 0 {
		// If SwarmPort was user defined but the port state
		// SwarmPort doesn't match we are not allocated.
		v := portStateMap[p.PublishedPort]

		// Delete state from allocatedPorts
		delete(portStateMap, p.PublishedPort)

		return v
	}

	// If PublishedPort == 0 and we don't have non-zero port
	// then we are not allocated
	for publishedPort, v := range portStateMap {
		if publishedPort != 0 {
			// Delete state from allocatedPorts
			delete(portStateMap, publishedPort)
			return v
		}
	}

	return nil
}

func newPortAllocator() (*portAllocator, error) {
	portSpaces := make(map[api.PortConfig_Protocol]*portSpace)
	for _, protocol := range []api.PortConfig_Protocol{api.ProtocolTCP, api.ProtocolUDP, api.ProtocolSCTP} {
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

// getPortConfigKey returns a map key for doing set operations with
// ports. The key consists of name, protocol and target port which
// uniquely identifies a port within a single Endpoint.
func getPortConfigKey(p *api.PortConfig) api.PortConfig {
	return api.PortConfig{
		Name:       p.Name,
		Protocol:   p.Protocol,
		TargetPort: p.TargetPort,
	}
}

func reconcilePortConfigs(s *api.Service) []*api.PortConfig {
	// If runtime state hasn't been created or if port config has
	// changed from port state return the port config from Spec.
	if s.Endpoint == nil || len(s.Spec.Endpoint.Ports) != len(s.Endpoint.Ports) {
		return s.Spec.Endpoint.Ports
	}

	portStates := allocatedPorts{}
	for _, portState := range s.Endpoint.Ports {
		if portState.PublishMode == api.PublishModeIngress {
			portStates.addState(portState)
		}
	}

	var portConfigs []*api.PortConfig

	// Process the portConfig with portConfig.PublishMode != api.PublishModeIngress
	// and PublishedPort != 0 (high priority)
	for _, portConfig := range s.Spec.Endpoint.Ports {
		if portConfig.PublishMode != api.PublishModeIngress {
			// If the PublishMode is not Ingress simply pick up the port config.
			portConfigs = append(portConfigs, portConfig)
		} else if portConfig.PublishedPort != 0 {
			// Otherwise we only process PublishedPort != 0 in this round

			// Remove record from portState
			portStates.delState(portConfig)

			// For PublishedPort != 0 prefer the portConfig
			portConfigs = append(portConfigs, portConfig)
		}
	}

	// Iterate portConfigs with PublishedPort == 0 (low priority)
	for _, portConfig := range s.Spec.Endpoint.Ports {
		// Ignore ports which are not PublishModeIngress (already processed)
		// And we only process PublishedPort == 0 in this round
		// So the following:
		//  `portConfig.PublishMode == api.PublishModeIngress && portConfig.PublishedPort == 0`
		if portConfig.PublishMode == api.PublishModeIngress && portConfig.PublishedPort == 0 {
			// If the portConfig is exactly the same as portState
			// except if SwarmPort is not user-define then prefer
			// portState to ensure sticky allocation of the same
			// port that was allocated before.

			// Remove record from portState
			if portState := portStates.delState(portConfig); portState != nil {
				portConfigs = append(portConfigs, portState)
				continue
			}

			// For all other cases prefer the portConfig
			portConfigs = append(portConfigs, portConfig)
		}
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

		// Do an actual allocation only if the PublishMode is Ingress
		if portConfig.PublishMode == api.PublishModeIngress {
			if err = pa.portSpaces[portState.Protocol].allocate(portState); err != nil {
				return
			}
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
		// Do an actual free only if the PublishMode is
		// Ingress
		if portState.PublishMode != api.PublishModeIngress {
			continue
		}

		pa.portSpaces[portState.Protocol].free(portState)
	}

	s.Endpoint.Ports = nil
}

func (pa *portAllocator) hostPublishPortsNeedUpdate(s *api.Service) bool {
	if s.Endpoint == nil && s.Spec.Endpoint == nil {
		return false
	}

	portStates := allocatedPorts{}
	if s.Endpoint != nil {
		for _, portState := range s.Endpoint.Ports {
			if portState.PublishMode == api.PublishModeHost {
				portStates.addState(portState)
			}
		}
	}

	if s.Spec.Endpoint != nil {
		for _, portConfig := range s.Spec.Endpoint.Ports {
			if portConfig.PublishMode == api.PublishModeHost &&
				portConfig.PublishedPort != 0 {
				if portStates.delState(portConfig) == nil {
					return true
				}
			}
		}
	}

	return false
}

func (pa *portAllocator) isPortsAllocated(s *api.Service) bool {
	return pa.isPortsAllocatedOnInit(s, false)
}

func (pa *portAllocator) isPortsAllocatedOnInit(s *api.Service, onInit bool) bool {
	// If service has no user-defined endpoint and allocated endpoint,
	// we assume it is allocated and return true.
	if s.Endpoint == nil && s.Spec.Endpoint == nil {
		return true
	}

	// If service has allocated endpoint while has no user-defined endpoint,
	// we assume allocated endpoints are redundant, and they need deallocated.
	// If service has no allocated endpoint while has user-defined endpoint,
	// we assume it is not allocated.
	if (s.Endpoint != nil && s.Spec.Endpoint == nil) ||
		(s.Endpoint == nil && s.Spec.Endpoint != nil) {
		return false
	}

	// If we don't have same number of port states as port configs
	// we assume it is not allocated.
	if len(s.Spec.Endpoint.Ports) != len(s.Endpoint.Ports) {
		return false
	}

	portStates := allocatedPorts{}
	hostTargetPorts := map[uint32]struct{}{}
	for _, portState := range s.Endpoint.Ports {
		switch portState.PublishMode {
		case api.PublishModeIngress:
			portStates.addState(portState)
		case api.PublishModeHost:
			// build a map of host mode ports we've seen. if in the spec we get
			// a host port that's not in the service, then we need to do
			// allocation. if we get the same target port but something else
			// has changed, then HostPublishPortsNeedUpdate will cover that
			// case. see docker/swarmkit#2376
			hostTargetPorts[portState.TargetPort] = struct{}{}
		}
	}

	// Iterate portConfigs with PublishedPort != 0 (high priority)
	for _, portConfig := range s.Spec.Endpoint.Ports {
		// Ignore ports which are not PublishModeIngress
		if portConfig.PublishMode != api.PublishModeIngress {
			continue
		}
		if portConfig.PublishedPort != 0 && portStates.delState(portConfig) == nil {
			return false
		}
	}

	// Iterate portConfigs with PublishedPort == 0 (low priority)
	for _, portConfig := range s.Spec.Endpoint.Ports {
		// Ignore ports which are not PublishModeIngress
		switch portConfig.PublishMode {
		case api.PublishModeIngress:
			if portConfig.PublishedPort == 0 && portStates.delState(portConfig) == nil {
				return false
			}

			// If SwarmPort was not defined by user and the func
			// is called during allocator initialization state then
			// we are not allocated.
			if portConfig.PublishedPort == 0 && onInit {
				return false
			}
		case api.PublishModeHost:
			// check if the target port is already in the port config. if it
			// isn't, then it's our problem.
			if _, ok := hostTargetPorts[portConfig.TargetPort]; !ok {
				return false
			}
			// NOTE(dperny) there could be a further case where we check if
			// there are host ports in the config that aren't in the spec, but
			// that's only possible if there's a mismatch in the number of
			// ports, which is handled by a length check earlier in the code
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
	swarmPort, err := ps.dynamicPortSpace.GetID(true)
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
