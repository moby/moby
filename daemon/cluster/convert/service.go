package convert

import (
	"fmt"
	"strings"

	"github.com/docker/docker/pkg/namesgenerator"
	types "github.com/docker/engine-api/types/swarm"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/protobuf/ptypes"
)

// ServiceFromGRPC converts a grpc Service to a Service.
func ServiceFromGRPC(s swarmapi.Service) types.Service {
	spec := s.Spec
	containerConfig := spec.Task.Runtime.(*swarmapi.TaskSpec_Container).Container

	networks := make([]types.NetworkAttachmentConfig, 0, len(spec.Networks))
	for _, n := range spec.Networks {
		networks = append(networks, types.NetworkAttachmentConfig{Target: n.Target})
	}
	service := types.Service{
		ID: s.ID,

		Spec: types.ServiceSpec{
			TaskSpec: types.TaskSpec{
				ContainerSpec: containerSpecFromGRPC(containerConfig),
				Resources:     resourcesFromGRPC(s.Spec.Task.Resources),
				RestartPolicy: restartPolicyFromGRPC(s.Spec.Task.Restart),
				Placement:     placementFromGRPC(s.Spec.Task.Placement),
			},

			Networks:     networks,
			EndpointSpec: endpointSpecFromGRPC(s.Spec.Endpoint),
		},
		Endpoint: endpointFromGRPC(s.Endpoint),
	}

	// Meta
	service.Version.Index = s.Meta.Version.Index
	service.CreatedAt, _ = ptypes.Timestamp(s.Meta.CreatedAt)
	service.UpdatedAt, _ = ptypes.Timestamp(s.Meta.UpdatedAt)

	// Annotations
	service.Spec.Name = s.Spec.Annotations.Name
	service.Spec.Labels = s.Spec.Annotations.Labels

	// UpdateConfig
	if s.Spec.Update != nil {
		service.Spec.UpdateConfig = &types.UpdateConfig{
			Parallelism: s.Spec.Update.Parallelism,
		}

		service.Spec.UpdateConfig.Delay, _ = ptypes.Duration(&s.Spec.Update.Delay)
	}

	//Mode
	switch t := s.Spec.GetMode().(type) {
	case *swarmapi.ServiceSpec_Global:
		service.Spec.Mode.Global = &types.GlobalService{}
	case *swarmapi.ServiceSpec_Replicated:
		service.Spec.Mode.Replicated = &types.ReplicatedService{
			Instances: &t.Replicated.Instances,
		}
	}

	return service
}

// ServiceSpecToGRPC converts a ServiceSpec to a grpc ServiceSpec.
func ServiceSpecToGRPC(s types.ServiceSpec) (swarmapi.ServiceSpec, error) {
	name := s.Name
	if name == "" {
		name = namesgenerator.GetRandomName(0)
	}

	networks := make([]*swarmapi.ServiceSpec_NetworkAttachmentConfig, 0, len(s.Networks))
	for _, n := range s.Networks {
		networks = append(networks, &swarmapi.ServiceSpec_NetworkAttachmentConfig{Target: n.Target})
	}

	spec := swarmapi.ServiceSpec{
		Annotations: swarmapi.Annotations{
			Name:   name,
			Labels: s.Labels,
		},
		Task: swarmapi.TaskSpec{
			Resources: resourcesToGRPC(s.TaskSpec.Resources),
		},
		Networks: networks,
	}

	containerSpec, err := containerToGRPC(s.TaskSpec.ContainerSpec)
	if err != nil {
		return swarmapi.ServiceSpec{}, err
	}
	spec.Task.Runtime = &swarmapi.TaskSpec_Container{Container: containerSpec}

	restartPolicy, err := restartPolicyToGRPC(s.TaskSpec.RestartPolicy)
	if err != nil {
		return swarmapi.ServiceSpec{}, err
	}
	spec.Task.Restart = restartPolicy

	if s.TaskSpec.Placement != nil {
		spec.Task.Placement = &swarmapi.Placement{
			Constraints: s.TaskSpec.Placement.Constraints,
		}
	}

	if s.UpdateConfig != nil {
		spec.Update = &swarmapi.UpdateConfig{
			Parallelism: s.UpdateConfig.Parallelism,
			Delay:       *ptypes.DurationProto(s.UpdateConfig.Delay),
		}
	}

	if s.EndpointSpec != nil {
		spec.Endpoint = &swarmapi.EndpointSpec{}
		for _, portConfig := range s.EndpointSpec.ExposedPorts {
			spec.Endpoint.ExposedPorts = append(spec.Endpoint.ExposedPorts, &swarmapi.PortConfig{
				Name:      portConfig.Name,
				Protocol:  swarmapi.PortConfig_Protocol(swarmapi.PortConfig_Protocol_value[strings.ToUpper(string(portConfig.Protocol))]),
				Port:      portConfig.Port,
				SwarmPort: portConfig.SwarmPort,
			})
		}
	}

	//Mode
	if s.Mode.Global != nil {
		spec.Mode = &swarmapi.ServiceSpec_Global{
			Global: &swarmapi.GlobalService{},
		}
	} else if s.Mode.Replicated != nil && s.Mode.Replicated.Instances != nil {
		spec.Mode = &swarmapi.ServiceSpec_Replicated{
			Replicated: &swarmapi.ReplicatedService{Instances: *s.Mode.Replicated.Instances},
		}
	} else {
		spec.Mode = &swarmapi.ServiceSpec_Replicated{
			Replicated: &swarmapi.ReplicatedService{Instances: 1},
		}
	}

	return spec, nil
}

func resourcesFromGRPC(res *swarmapi.ResourceRequirements) *types.ResourceRequirements {
	var resources *types.ResourceRequirements
	if res != nil {
		resources = &types.ResourceRequirements{}
		if res.Limits != nil {
			resources.Limits = &types.Resources{
				NanoCPUs:    res.Limits.NanoCPUs,
				MemoryBytes: res.Limits.MemoryBytes,
			}
		}
		if res.Reservations != nil {
			resources.Reservations = &types.Resources{
				NanoCPUs:    res.Reservations.NanoCPUs,
				MemoryBytes: res.Reservations.MemoryBytes,
			}
		}
	}

	return resources
}

func resourcesToGRPC(res *types.ResourceRequirements) *swarmapi.ResourceRequirements {
	var reqs *swarmapi.ResourceRequirements
	if res != nil {
		reqs = &swarmapi.ResourceRequirements{}
		if res.Limits != nil {
			reqs.Limits = &swarmapi.Resources{
				NanoCPUs:    res.Limits.NanoCPUs,
				MemoryBytes: res.Limits.MemoryBytes,
			}
		}
		if res.Reservations != nil {
			reqs.Reservations = &swarmapi.Resources{
				NanoCPUs:    res.Reservations.NanoCPUs,
				MemoryBytes: res.Reservations.MemoryBytes,
			}

		}
	}
	return reqs
}

func restartPolicyFromGRPC(p *swarmapi.RestartPolicy) *types.RestartPolicy {
	var rp *types.RestartPolicy
	if p != nil {
		rp = &types.RestartPolicy{}
		rp.Condition = types.RestartPolicyCondition(p.Condition.String())
		if p.Delay != nil {
			delay, _ := ptypes.Duration(p.Delay)
			rp.Delay = &delay
		}
		if p.Window != nil {
			window, _ := ptypes.Duration(p.Window)
			rp.Window = &window
		}

		rp.MaxAttempts = &p.MaxAttempts
	}
	return rp
}

func restartPolicyToGRPC(p *types.RestartPolicy) (*swarmapi.RestartPolicy, error) {
	var rp *swarmapi.RestartPolicy
	if p != nil {
		rp = &swarmapi.RestartPolicy{}
		if condition, ok := swarmapi.RestartPolicy_RestartCondition_value[string(p.Condition)]; ok {
			rp.Condition = swarmapi.RestartPolicy_RestartCondition(condition)
		} else if string(p.Condition) == "" {
			rp.Condition = swarmapi.RestartOnAny
		} else {
			return nil, fmt.Errorf("invalid RestartCondition: %q", p.Condition)
		}

		if p.Delay != nil {
			rp.Delay = ptypes.DurationProto(*p.Delay)
		}
		if p.Window != nil {
			rp.Window = ptypes.DurationProto(*p.Window)
		}
		if p.MaxAttempts != nil {
			rp.MaxAttempts = *p.MaxAttempts

		}
	}
	return rp, nil
}

func placementFromGRPC(p *swarmapi.Placement) *types.Placement {
	var r *types.Placement
	if p != nil {
		r = &types.Placement{}
		r.Constraints = p.Constraints
	}

	return r
}
