package convert

import (
	"fmt"
	"strings"

	types "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/namesgenerator"
	swarmapi "github.com/docker/swarmkit/api"
	gogotypes "github.com/gogo/protobuf/types"
)

// ServiceFromGRPC converts a grpc Service to a Service.
func ServiceFromGRPC(s swarmapi.Service) types.Service {
	service := types.Service{
		ID:           s.ID,
		Spec:         *serviceSpecFromGRPC(&s.Spec),
		PreviousSpec: serviceSpecFromGRPC(s.PreviousSpec),

		Endpoint: endpointFromGRPC(s.Endpoint),
	}

	// Meta
	service.Version.Index = s.Meta.Version.Index
	service.CreatedAt, _ = gogotypes.TimestampFromProto(s.Meta.CreatedAt)
	service.UpdatedAt, _ = gogotypes.TimestampFromProto(s.Meta.UpdatedAt)

	// UpdateStatus
	if s.UpdateStatus != nil {
		service.UpdateStatus = &types.UpdateStatus{}
		switch s.UpdateStatus.State {
		case swarmapi.UpdateStatus_UPDATING:
			service.UpdateStatus.State = types.UpdateStateUpdating
		case swarmapi.UpdateStatus_PAUSED:
			service.UpdateStatus.State = types.UpdateStatePaused
		case swarmapi.UpdateStatus_COMPLETED:
			service.UpdateStatus.State = types.UpdateStateCompleted
		}

		startedAt, _ := gogotypes.TimestampFromProto(s.UpdateStatus.StartedAt)
		if !startedAt.IsZero() {
			service.UpdateStatus.StartedAt = &startedAt
		}

		completedAt, _ := gogotypes.TimestampFromProto(s.UpdateStatus.CompletedAt)
		if !completedAt.IsZero() {
			service.UpdateStatus.CompletedAt = &completedAt
		}

		service.UpdateStatus.Message = s.UpdateStatus.Message
	}

	return service
}

func serviceSpecFromGRPC(spec *swarmapi.ServiceSpec) *types.ServiceSpec {
	if spec == nil {
		return nil
	}

	serviceNetworks := make([]types.NetworkAttachmentConfig, 0, len(spec.Networks))
	for _, n := range spec.Networks {
		serviceNetworks = append(serviceNetworks, types.NetworkAttachmentConfig{Target: n.Target, Aliases: n.Aliases})
	}

	taskNetworks := make([]types.NetworkAttachmentConfig, 0, len(spec.Task.Networks))
	for _, n := range spec.Task.Networks {
		taskNetworks = append(taskNetworks, types.NetworkAttachmentConfig{Target: n.Target, Aliases: n.Aliases})
	}

	containerConfig := spec.Task.Runtime.(*swarmapi.TaskSpec_Container).Container
	convertedSpec := &types.ServiceSpec{
		Annotations: types.Annotations{
			Name:   spec.Annotations.Name,
			Labels: spec.Annotations.Labels,
		},

		TaskTemplate: types.TaskSpec{
			ContainerSpec: containerSpecFromGRPC(containerConfig),
			Resources:     resourcesFromGRPC(spec.Task.Resources),
			RestartPolicy: restartPolicyFromGRPC(spec.Task.Restart),
			Placement:     placementFromGRPC(spec.Task.Placement),
			LogDriver:     driverFromGRPC(spec.Task.LogDriver),
			Networks:      taskNetworks,
			ForceUpdate:   spec.Task.ForceUpdate,
		},

		Networks:     serviceNetworks,
		EndpointSpec: endpointSpecFromGRPC(spec.Endpoint),
	}

	// UpdateConfig
	if spec.Update != nil {
		convertedSpec.UpdateConfig = &types.UpdateConfig{
			Parallelism:     spec.Update.Parallelism,
			MaxFailureRatio: spec.Update.MaxFailureRatio,
		}

		convertedSpec.UpdateConfig.Delay = spec.Update.Delay
		if spec.Update.Monitor != nil {
			convertedSpec.UpdateConfig.Monitor, _ = gogotypes.DurationFromProto(spec.Update.Monitor)
		}

		switch spec.Update.FailureAction {
		case swarmapi.UpdateConfig_PAUSE:
			convertedSpec.UpdateConfig.FailureAction = types.UpdateFailureActionPause
		case swarmapi.UpdateConfig_CONTINUE:
			convertedSpec.UpdateConfig.FailureAction = types.UpdateFailureActionContinue
		}
	}

	// Mode
	switch t := spec.GetMode().(type) {
	case *swarmapi.ServiceSpec_Global:
		convertedSpec.Mode.Global = &types.GlobalService{}
	case *swarmapi.ServiceSpec_Replicated:
		convertedSpec.Mode.Replicated = &types.ReplicatedService{
			Replicas: &t.Replicated.Replicas,
		}
	}

	return convertedSpec
}

// ServiceSpecToGRPC converts a ServiceSpec to a grpc ServiceSpec.
func ServiceSpecToGRPC(s types.ServiceSpec) (swarmapi.ServiceSpec, error) {
	name := s.Name
	if name == "" {
		name = namesgenerator.GetRandomName(0)
	}

	serviceNetworks := make([]*swarmapi.NetworkAttachmentConfig, 0, len(s.Networks))
	for _, n := range s.Networks {
		serviceNetworks = append(serviceNetworks, &swarmapi.NetworkAttachmentConfig{Target: n.Target, Aliases: n.Aliases})
	}

	taskNetworks := make([]*swarmapi.NetworkAttachmentConfig, 0, len(s.TaskTemplate.Networks))
	for _, n := range s.TaskTemplate.Networks {
		taskNetworks = append(taskNetworks, &swarmapi.NetworkAttachmentConfig{Target: n.Target, Aliases: n.Aliases})
	}

	spec := swarmapi.ServiceSpec{
		Annotations: swarmapi.Annotations{
			Name:   name,
			Labels: s.Labels,
		},
		Task: swarmapi.TaskSpec{
			Resources:   resourcesToGRPC(s.TaskTemplate.Resources),
			LogDriver:   driverToGRPC(s.TaskTemplate.LogDriver),
			Networks:    taskNetworks,
			ForceUpdate: s.TaskTemplate.ForceUpdate,
		},
		Networks: serviceNetworks,
	}

	containerSpec, err := containerToGRPC(s.TaskTemplate.ContainerSpec)
	if err != nil {
		return swarmapi.ServiceSpec{}, err
	}
	spec.Task.Runtime = &swarmapi.TaskSpec_Container{Container: containerSpec}

	restartPolicy, err := restartPolicyToGRPC(s.TaskTemplate.RestartPolicy)
	if err != nil {
		return swarmapi.ServiceSpec{}, err
	}
	spec.Task.Restart = restartPolicy

	if s.TaskTemplate.Placement != nil {
		spec.Task.Placement = &swarmapi.Placement{
			Constraints: s.TaskTemplate.Placement.Constraints,
		}
	}

	if s.UpdateConfig != nil {
		var failureAction swarmapi.UpdateConfig_FailureAction
		switch s.UpdateConfig.FailureAction {
		case types.UpdateFailureActionPause, "":
			failureAction = swarmapi.UpdateConfig_PAUSE
		case types.UpdateFailureActionContinue:
			failureAction = swarmapi.UpdateConfig_CONTINUE
		default:
			return swarmapi.ServiceSpec{}, fmt.Errorf("unrecongized update failure action %s", s.UpdateConfig.FailureAction)
		}
		spec.Update = &swarmapi.UpdateConfig{
			Parallelism:     s.UpdateConfig.Parallelism,
			Delay:           s.UpdateConfig.Delay,
			FailureAction:   failureAction,
			MaxFailureRatio: s.UpdateConfig.MaxFailureRatio,
		}
		if s.UpdateConfig.Monitor != 0 {
			spec.Update.Monitor = gogotypes.DurationProto(s.UpdateConfig.Monitor)
		}
	}

	if s.EndpointSpec != nil {
		if s.EndpointSpec.Mode != "" &&
			s.EndpointSpec.Mode != types.ResolutionModeVIP &&
			s.EndpointSpec.Mode != types.ResolutionModeDNSRR {
			return swarmapi.ServiceSpec{}, fmt.Errorf("invalid resolution mode: %q", s.EndpointSpec.Mode)
		}

		spec.Endpoint = &swarmapi.EndpointSpec{}

		spec.Endpoint.Mode = swarmapi.EndpointSpec_ResolutionMode(swarmapi.EndpointSpec_ResolutionMode_value[strings.ToUpper(string(s.EndpointSpec.Mode))])

		for _, portConfig := range s.EndpointSpec.Ports {
			spec.Endpoint.Ports = append(spec.Endpoint.Ports, &swarmapi.PortConfig{
				Name:          portConfig.Name,
				Protocol:      swarmapi.PortConfig_Protocol(swarmapi.PortConfig_Protocol_value[strings.ToUpper(string(portConfig.Protocol))]),
				PublishMode:   swarmapi.PortConfig_PublishMode(swarmapi.PortConfig_PublishMode_value[strings.ToUpper(string(portConfig.PublishMode))]),
				TargetPort:    portConfig.TargetPort,
				PublishedPort: portConfig.PublishedPort,
			})
		}
	}

	// Mode
	if s.Mode.Global != nil && s.Mode.Replicated != nil {
		return swarmapi.ServiceSpec{}, fmt.Errorf("cannot specify both replicated mode and global mode")
	}

	if s.Mode.Global != nil {
		spec.Mode = &swarmapi.ServiceSpec_Global{
			Global: &swarmapi.GlobalService{},
		}
	} else if s.Mode.Replicated != nil && s.Mode.Replicated.Replicas != nil {
		spec.Mode = &swarmapi.ServiceSpec_Replicated{
			Replicated: &swarmapi.ReplicatedService{Replicas: *s.Mode.Replicated.Replicas},
		}
	} else {
		spec.Mode = &swarmapi.ServiceSpec_Replicated{
			Replicated: &swarmapi.ReplicatedService{Replicas: 1},
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

		switch p.Condition {
		case swarmapi.RestartOnNone:
			rp.Condition = types.RestartPolicyConditionNone
		case swarmapi.RestartOnFailure:
			rp.Condition = types.RestartPolicyConditionOnFailure
		case swarmapi.RestartOnAny:
			rp.Condition = types.RestartPolicyConditionAny
		default:
			rp.Condition = types.RestartPolicyConditionAny
		}

		if p.Delay != nil {
			delay, _ := gogotypes.DurationFromProto(p.Delay)
			rp.Delay = &delay
		}
		if p.Window != nil {
			window, _ := gogotypes.DurationFromProto(p.Window)
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

		switch p.Condition {
		case types.RestartPolicyConditionNone:
			rp.Condition = swarmapi.RestartOnNone
		case types.RestartPolicyConditionOnFailure:
			rp.Condition = swarmapi.RestartOnFailure
		case types.RestartPolicyConditionAny:
			rp.Condition = swarmapi.RestartOnAny
		default:
			if string(p.Condition) != "" {
				return nil, fmt.Errorf("invalid RestartCondition: %q", p.Condition)
			}
			rp.Condition = swarmapi.RestartOnAny
		}

		if p.Delay != nil {
			rp.Delay = gogotypes.DurationProto(*p.Delay)
		}
		if p.Window != nil {
			rp.Window = gogotypes.DurationProto(*p.Window)
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

func driverFromGRPC(p *swarmapi.Driver) *types.Driver {
	if p == nil {
		return nil
	}

	return &types.Driver{
		Name:    p.Name,
		Options: p.Options,
	}
}

func driverToGRPC(p *types.Driver) *swarmapi.Driver {
	if p == nil {
		return nil
	}

	return &swarmapi.Driver{
		Name:    p.Name,
		Options: p.Options,
	}
}
