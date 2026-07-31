package defaults

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/moby/swarmkit/v2/api"
)

// Service is a ServiceSpec object with all fields filled in using default
// values.
//
// It is a pointer because protobuf messages must not be copied. Take care not
// to hand any of its sub-messages out without copying them first, or callers
// end up mutating the defaults.
var Service = &api.ServiceSpec{
	Task: &api.TaskSpec{
		Runtime: &api.TaskSpec_Container{
			Container: &api.ContainerSpec{
				StopGracePeriod: durationpb.New(10 * time.Second),
				PullOptions:     &api.ContainerSpec_PullOptions{},
				DnsConfig:       &api.ContainerSpec_DNSConfig{},
			},
		},
		Resources: &api.ResourceRequirements{},
		Restart: &api.RestartPolicy{
			Condition: api.RestartPolicy_ANY,
			Delay:     durationpb.New(5 * time.Second),
		},
		Placement: &api.Placement{},
	},
	Update: &api.UpdateConfig{
		FailureAction: api.UpdateConfig_PAUSE,
		Monitor:       durationpb.New(5 * time.Second),
		Parallelism:   1,
		Order:         api.UpdateConfig_STOP_FIRST,
	},
	Rollback: &api.UpdateConfig{
		FailureAction: api.UpdateConfig_PAUSE,
		Monitor:       durationpb.New(5 * time.Second),
		Parallelism:   1,
		Order:         api.UpdateConfig_STOP_FIRST,
	},
}

// InterpolateService returns a ServiceSpec based on the provided spec, which
// has all unspecified values filled in with default values.
func InterpolateService(origSpec *api.ServiceSpec) *api.ServiceSpec {
	spec := origSpec.Copy()

	container := spec.Task.GetContainer()
	defaultContainer := Service.Task.GetContainer()
	if container != nil {
		if container.StopGracePeriod == nil {
			container.StopGracePeriod = durationpb.New(defaultContainer.StopGracePeriod.AsDuration())
		}
		if container.PullOptions == nil {
			container.PullOptions = defaultContainer.PullOptions.Copy()
		}
		if container.DnsConfig == nil {
			container.DnsConfig = defaultContainer.DnsConfig.Copy()
		}
	}

	if spec.Task.Resources == nil {
		spec.Task.Resources = Service.Task.Resources.Copy()
	}

	if spec.Task.Restart == nil {
		spec.Task.Restart = Service.Task.Restart.Copy()
	} else {
		if spec.Task.Restart.Delay == nil {
			spec.Task.Restart.Delay = durationpb.New(Service.Task.Restart.Delay.AsDuration())
		}
	}

	if spec.Task.Placement == nil {
		spec.Task.Placement = Service.Task.Placement.Copy()
	}

	if spec.Update == nil {
		spec.Update = Service.Update.Copy()
	} else {
		if spec.Update.Monitor == nil {
			spec.Update.Monitor = durationpb.New(Service.Update.Monitor.AsDuration())
		}
	}

	if spec.Rollback == nil {
		spec.Rollback = Service.Rollback.Copy()
	} else {
		if spec.Rollback.Monitor == nil {
			spec.Rollback.Monitor = durationpb.New(Service.Rollback.Monitor.AsDuration())
		}
	}

	return spec
}
