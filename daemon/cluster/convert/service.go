package convert

import (
	"errors"
	"fmt"
	"strings"

	types "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/namesgenerator"
	swarmapi "github.com/docker/swarmkit/api"
	gogotypes "github.com/gogo/protobuf/types"
)

var (
	// ErrUnsupportedRuntime returns an error if the runtime is not supported by the daemon
	ErrUnsupportedRuntime = errors.New("unsupported runtime")
)

// ServiceFromGRPC converts a grpc Service to a Service.
func ServiceFromGRPC(s swarmapi.Service) (types.Service, error) {
	curSpec, err := serviceSpecFromGRPC(&s.Spec)
	if err != nil {
		return types.Service{}, err
	}
	prevSpec, err := serviceSpecFromGRPC(s.PreviousSpec)
	if err != nil {
		return types.Service{}, err
	}
	service := types.Service{
		ID:           s.ID,
		Spec:         *curSpec,
		PreviousSpec: prevSpec,

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
		case swarmapi.UpdateStatus_ROLLBACK_STARTED:
			service.UpdateStatus.State = types.UpdateStateRollbackStarted
		case swarmapi.UpdateStatus_ROLLBACK_PAUSED:
			service.UpdateStatus.State = types.UpdateStateRollbackPaused
		case swarmapi.UpdateStatus_ROLLBACK_COMPLETED:
			service.UpdateStatus.State = types.UpdateStateRollbackCompleted
		}

		startedAt, _ := gogotypes.TimestampFromProto(s.UpdateStatus.StartedAt)
		if !startedAt.IsZero() && startedAt.Unix() != 0 {
			service.UpdateStatus.StartedAt = &startedAt
		}

		completedAt, _ := gogotypes.TimestampFromProto(s.UpdateStatus.CompletedAt)
		if !completedAt.IsZero() && completedAt.Unix() != 0 {
			service.UpdateStatus.CompletedAt = &completedAt
		}

		service.UpdateStatus.Message = s.UpdateStatus.Message
	}

	return service, nil
}

func serviceSpecFromGRPC(spec *swarmapi.ServiceSpec) (*types.ServiceSpec, error) {
	if spec == nil {
		return nil, nil
	}

	serviceNetworks := make([]types.NetworkAttachmentConfig, 0, len(spec.Networks))
	for _, n := range spec.Networks {
		netConfig := types.NetworkAttachmentConfig{Target: n.Target, Aliases: n.Aliases, DriverOpts: n.DriverAttachmentOpts}
		serviceNetworks = append(serviceNetworks, netConfig)

	}

	taskTemplate := taskSpecFromGRPC(spec.Task)

	switch t := spec.Task.GetRuntime().(type) {
	case *swarmapi.TaskSpec_Container:
		containerConfig := t.Container
		taskTemplate.ContainerSpec = containerSpecFromGRPC(containerConfig)
		taskTemplate.Runtime = types.RuntimeContainer
	case *swarmapi.TaskSpec_Generic:
		switch t.Generic.Kind {
		case string(types.RuntimePlugin):
			taskTemplate.Runtime = types.RuntimePlugin
		default:
			return nil, fmt.Errorf("unknown task runtime type: %s", t.Generic.Payload.TypeUrl)
		}

	default:
		return nil, fmt.Errorf("error creating service; unsupported runtime %T", t)
	}

	convertedSpec := &types.ServiceSpec{
		Annotations:  annotationsFromGRPC(spec.Annotations),
		TaskTemplate: taskTemplate,
		Networks:     serviceNetworks,
		EndpointSpec: endpointSpecFromGRPC(spec.Endpoint),
	}

	// UpdateConfig
	convertedSpec.UpdateConfig = updateConfigFromGRPC(spec.Update)
	convertedSpec.RollbackConfig = updateConfigFromGRPC(spec.Rollback)

	// Mode
	switch t := spec.GetMode().(type) {
	case *swarmapi.ServiceSpec_Global:
		convertedSpec.Mode.Global = &types.GlobalService{}
	case *swarmapi.ServiceSpec_Replicated:
		convertedSpec.Mode.Replicated = &types.ReplicatedService{
			Replicas: &t.Replicated.Replicas,
		}
	}

	return convertedSpec, nil
}

// ServiceSpecToGRPC converts a ServiceSpec to a grpc ServiceSpec.
func ServiceSpecToGRPC(s types.ServiceSpec) (swarmapi.ServiceSpec, error) {
	name := s.Name
	if name == "" {
		name = namesgenerator.GetRandomName(0)
	}

	serviceNetworks := make([]*swarmapi.NetworkAttachmentConfig, 0, len(s.Networks))
	for _, n := range s.Networks {
		netConfig := &swarmapi.NetworkAttachmentConfig{Target: n.Target, Aliases: n.Aliases, DriverAttachmentOpts: n.DriverOpts}
		serviceNetworks = append(serviceNetworks, netConfig)
	}

	taskNetworks := make([]*swarmapi.NetworkAttachmentConfig, 0, len(s.TaskTemplate.Networks))
	for _, n := range s.TaskTemplate.Networks {
		netConfig := &swarmapi.NetworkAttachmentConfig{Target: n.Target, Aliases: n.Aliases, DriverAttachmentOpts: n.DriverOpts}
		taskNetworks = append(taskNetworks, netConfig)

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

	switch s.TaskTemplate.Runtime {
	case types.RuntimeContainer, "": // if empty runtime default to container
		containerSpec, err := containerToGRPC(s.TaskTemplate.ContainerSpec)
		if err != nil {
			return swarmapi.ServiceSpec{}, err
		}
		spec.Task.Runtime = &swarmapi.TaskSpec_Container{Container: containerSpec}
	case types.RuntimePlugin:
		spec.Task.Runtime = &swarmapi.TaskSpec_Generic{
			Generic: &swarmapi.GenericRuntimeSpec{
				Kind: string(types.RuntimePlugin),
				Payload: &gogotypes.Any{
					TypeUrl: string(types.RuntimeURLPlugin),
				},
			},
		}
	default:
		return swarmapi.ServiceSpec{}, ErrUnsupportedRuntime
	}

	restartPolicy, err := restartPolicyToGRPC(s.TaskTemplate.RestartPolicy)
	if err != nil {
		return swarmapi.ServiceSpec{}, err
	}
	spec.Task.Restart = restartPolicy

	if s.TaskTemplate.Placement != nil {
		var preferences []*swarmapi.PlacementPreference
		for _, pref := range s.TaskTemplate.Placement.Preferences {
			if pref.Spread != nil {
				preferences = append(preferences, &swarmapi.PlacementPreference{
					Preference: &swarmapi.PlacementPreference_Spread{
						Spread: &swarmapi.SpreadOver{
							SpreadDescriptor: pref.Spread.SpreadDescriptor,
						},
					},
				})
			}
		}
		var platforms []*swarmapi.Platform
		for _, plat := range s.TaskTemplate.Placement.Platforms {
			platforms = append(platforms, &swarmapi.Platform{
				Architecture: plat.Architecture,
				OS:           plat.OS,
			})
		}
		spec.Task.Placement = &swarmapi.Placement{
			Constraints: s.TaskTemplate.Placement.Constraints,
			Preferences: preferences,
			Platforms:   platforms,
		}
	}

	spec.Update, err = updateConfigToGRPC(s.UpdateConfig)
	if err != nil {
		return swarmapi.ServiceSpec{}, err
	}
	spec.Rollback, err = updateConfigToGRPC(s.RollbackConfig)
	if err != nil {
		return swarmapi.ServiceSpec{}, err
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

func annotationsFromGRPC(ann swarmapi.Annotations) types.Annotations {
	a := types.Annotations{
		Name:   ann.Name,
		Labels: ann.Labels,
	}

	if a.Labels == nil {
		a.Labels = make(map[string]string)
	}

	return a
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
	if p == nil {
		return nil
	}
	r := &types.Placement{
		Constraints: p.Constraints,
	}

	for _, pref := range p.Preferences {
		if spread := pref.GetSpread(); spread != nil {
			r.Preferences = append(r.Preferences, types.PlacementPreference{
				Spread: &types.SpreadOver{
					SpreadDescriptor: spread.SpreadDescriptor,
				},
			})
		}
	}

	for _, plat := range p.Platforms {
		r.Platforms = append(r.Platforms, types.Platform{
			Architecture: plat.Architecture,
			OS:           plat.OS,
		})
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

func updateConfigFromGRPC(updateConfig *swarmapi.UpdateConfig) *types.UpdateConfig {
	if updateConfig == nil {
		return nil
	}

	converted := &types.UpdateConfig{
		Parallelism:     updateConfig.Parallelism,
		MaxFailureRatio: updateConfig.MaxFailureRatio,
	}

	converted.Delay = updateConfig.Delay
	if updateConfig.Monitor != nil {
		converted.Monitor, _ = gogotypes.DurationFromProto(updateConfig.Monitor)
	}

	switch updateConfig.FailureAction {
	case swarmapi.UpdateConfig_PAUSE:
		converted.FailureAction = types.UpdateFailureActionPause
	case swarmapi.UpdateConfig_CONTINUE:
		converted.FailureAction = types.UpdateFailureActionContinue
	case swarmapi.UpdateConfig_ROLLBACK:
		converted.FailureAction = types.UpdateFailureActionRollback
	}

	switch updateConfig.Order {
	case swarmapi.UpdateConfig_STOP_FIRST:
		converted.Order = types.UpdateOrderStopFirst
	case swarmapi.UpdateConfig_START_FIRST:
		converted.Order = types.UpdateOrderStartFirst
	}

	return converted
}

func updateConfigToGRPC(updateConfig *types.UpdateConfig) (*swarmapi.UpdateConfig, error) {
	if updateConfig == nil {
		return nil, nil
	}

	converted := &swarmapi.UpdateConfig{
		Parallelism:     updateConfig.Parallelism,
		Delay:           updateConfig.Delay,
		MaxFailureRatio: updateConfig.MaxFailureRatio,
	}

	switch updateConfig.FailureAction {
	case types.UpdateFailureActionPause, "":
		converted.FailureAction = swarmapi.UpdateConfig_PAUSE
	case types.UpdateFailureActionContinue:
		converted.FailureAction = swarmapi.UpdateConfig_CONTINUE
	case types.UpdateFailureActionRollback:
		converted.FailureAction = swarmapi.UpdateConfig_ROLLBACK
	default:
		return nil, fmt.Errorf("unrecognized update failure action %s", updateConfig.FailureAction)
	}
	if updateConfig.Monitor != 0 {
		converted.Monitor = gogotypes.DurationProto(updateConfig.Monitor)
	}

	switch updateConfig.Order {
	case types.UpdateOrderStopFirst, "":
		converted.Order = swarmapi.UpdateConfig_STOP_FIRST
	case types.UpdateOrderStartFirst:
		converted.Order = swarmapi.UpdateConfig_START_FIRST
	default:
		return nil, fmt.Errorf("unrecognized update order %s", updateConfig.Order)
	}

	return converted, nil
}

func taskSpecFromGRPC(taskSpec swarmapi.TaskSpec) types.TaskSpec {
	taskNetworks := make([]types.NetworkAttachmentConfig, 0, len(taskSpec.Networks))
	for _, n := range taskSpec.Networks {
		netConfig := types.NetworkAttachmentConfig{Target: n.Target, Aliases: n.Aliases, DriverOpts: n.DriverAttachmentOpts}
		taskNetworks = append(taskNetworks, netConfig)
	}

	c := taskSpec.GetContainer()
	cSpec := types.ContainerSpec{}
	if c != nil {
		cSpec = containerSpecFromGRPC(c)
	}

	return types.TaskSpec{
		ContainerSpec: cSpec,
		Resources:     resourcesFromGRPC(taskSpec.Resources),
		RestartPolicy: restartPolicyFromGRPC(taskSpec.Restart),
		Placement:     placementFromGRPC(taskSpec.Placement),
		LogDriver:     driverFromGRPC(taskSpec.LogDriver),
		Networks:      taskNetworks,
		ForceUpdate:   taskSpec.ForceUpdate,
	}
}
