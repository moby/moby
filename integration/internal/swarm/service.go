package swarm

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/internal/test/daemon"
	"github.com/docker/docker/internal/test/environment"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
)

// ServicePoll tweaks the pollSettings for `service`
func ServicePoll(config *poll.Settings) {
	// Override the default pollSettings for `service` resource here ...
	config.Timeout = 30 * time.Second
	config.Delay = 100 * time.Millisecond
	if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
		config.Timeout = 90 * time.Second
	}
}

// NetworkPoll tweaks the pollSettings for `network`
func NetworkPoll(config *poll.Settings) {
	// Override the default pollSettings for `network` resource here ...
	config.Timeout = 30 * time.Second
	config.Delay = 100 * time.Millisecond

	if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
		config.Timeout = 50 * time.Second
	}
}

// ContainerPoll tweaks the pollSettings for `container`
func ContainerPoll(config *poll.Settings) {
	// Override the default pollSettings for `container` resource here ...

	if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
		config.Timeout = 30 * time.Second
		config.Delay = 100 * time.Millisecond
	}
}

// NewSwarm creates a swarm daemon for testing
func NewSwarm(t *testing.T, testEnv *environment.Execution, ops ...func(*daemon.Daemon)) *daemon.Daemon {
	t.Helper()
	skip.If(t, testEnv.IsRemoteDaemon)
	if testEnv.DaemonInfo.ExperimentalBuild {
		ops = append(ops, daemon.WithExperimental)
	}
	d := daemon.New(t, ops...)
	d.StartAndSwarmInit(t)
	return d
}

// ServiceSpecOpt is used with `CreateService` to pass in service spec modifiers
type ServiceSpecOpt func(*swarmtypes.ServiceSpec)

// CreateService creates a service on the passed in swarm daemon.
func CreateService(t *testing.T, d *daemon.Daemon, opts ...ServiceSpecOpt) string {
	t.Helper()
	spec := defaultServiceSpec()
	for _, o := range opts {
		o(&spec)
	}

	client := d.NewClientT(t)
	defer client.Close()

	resp, err := client.ServiceCreate(context.Background(), spec, types.ServiceCreateOptions{})
	assert.NilError(t, err, "error creating service")
	return resp.ID
}

func defaultServiceSpec() swarmtypes.ServiceSpec {
	var spec swarmtypes.ServiceSpec
	ServiceWithImage("busybox:latest")(&spec)
	ServiceWithCommand([]string{"/bin/top"})(&spec)
	ServiceWithReplicas(1)(&spec)
	return spec
}

// ServiceWithInit sets whether the service should use init or not
func ServiceWithInit(b *bool) func(*swarmtypes.ServiceSpec) {
	return func(spec *swarmtypes.ServiceSpec) {
		ensureContainerSpec(spec)
		spec.TaskTemplate.ContainerSpec.Init = b
	}
}

// ServiceWithImage sets the image to use for the service
func ServiceWithImage(image string) func(*swarmtypes.ServiceSpec) {
	return func(spec *swarmtypes.ServiceSpec) {
		ensureContainerSpec(spec)
		spec.TaskTemplate.ContainerSpec.Image = image
	}
}

// ServiceWithCommand sets the command to use for the service
func ServiceWithCommand(cmd []string) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		ensureContainerSpec(spec)
		spec.TaskTemplate.ContainerSpec.Command = cmd
	}
}

// ServiceWithConfig adds the config reference to the service
func ServiceWithConfig(configRef *swarmtypes.ConfigReference) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		ensureContainerSpec(spec)
		spec.TaskTemplate.ContainerSpec.Configs = append(spec.TaskTemplate.ContainerSpec.Configs, configRef)
	}
}

// ServiceWithSecret adds the secret reference to the service
func ServiceWithSecret(secretRef *swarmtypes.SecretReference) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		ensureContainerSpec(spec)
		spec.TaskTemplate.ContainerSpec.Secrets = append(spec.TaskTemplate.ContainerSpec.Secrets, secretRef)
	}
}

// ServiceWithReplicas sets the replicas for the service
func ServiceWithReplicas(n uint64) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		spec.Mode = swarmtypes.ServiceMode{
			Replicated: &swarmtypes.ReplicatedService{
				Replicas: &n,
			},
		}
	}
}

// ServiceWithName sets the name of the service
func ServiceWithName(name string) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		spec.Annotations.Name = name
	}
}

// ServiceWithNetwork sets the network of the service
func ServiceWithNetwork(network string) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		spec.TaskTemplate.Networks = append(spec.TaskTemplate.Networks,
			swarmtypes.NetworkAttachmentConfig{Target: network})
	}
}

// ServiceWithEndpoint sets the Endpoint of the service
func ServiceWithEndpoint(endpoint *swarmtypes.EndpointSpec) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		spec.EndpointSpec = endpoint
	}
}

// GetRunningTasks gets the list of running tasks for a service
func GetRunningTasks(t *testing.T, d *daemon.Daemon, serviceID string) []swarmtypes.Task {
	t.Helper()
	client := d.NewClientT(t)
	defer client.Close()

	filterArgs := filters.NewArgs()
	filterArgs.Add("desired-state", "running")
	filterArgs.Add("service", serviceID)

	options := types.TaskListOptions{
		Filters: filterArgs,
	}
	tasks, err := client.TaskList(context.Background(), options)
	assert.NilError(t, err)
	return tasks
}

// ExecTask runs the passed in exec config on the given task
func ExecTask(t *testing.T, d *daemon.Daemon, task swarmtypes.Task, config types.ExecConfig) types.HijackedResponse {
	t.Helper()
	client := d.NewClientT(t)
	defer client.Close()

	ctx := context.Background()
	resp, err := client.ContainerExecCreate(ctx, task.Status.ContainerStatus.ContainerID, config)
	assert.NilError(t, err, "error creating exec")

	startCheck := types.ExecStartCheck{}
	attach, err := client.ContainerExecAttach(ctx, resp.ID, startCheck)
	assert.NilError(t, err, "error attaching to exec")
	return attach
}

func ensureContainerSpec(spec *swarmtypes.ServiceSpec) {
	if spec.TaskTemplate.ContainerSpec == nil {
		spec.TaskTemplate.ContainerSpec = &swarmtypes.ContainerSpec{}
	}
}
