package swarm

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/environment"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// ServicePoll tweaks the pollSettings for `service`
func ServicePoll(config *poll.Settings) {
	// Override the default pollSettings for `service` resource here ...
	config.Timeout = 15 * time.Second
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

// NewSwarm creates a swarm daemon for testing
func NewSwarm(ctx context.Context, t *testing.T, testEnv *environment.Execution, ops ...daemon.Option) *daemon.Daemon {
	t.Helper()
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	if testEnv.DaemonInfo.ExperimentalBuild {
		ops = append(ops, daemon.WithExperimental())
	}
	d := daemon.New(t, ops...)
	d.StartAndSwarmInit(ctx, t)
	return d
}

// ServiceSpecOpt is used with `CreateService` to pass in service spec modifiers
type ServiceSpecOpt func(*swarmtypes.ServiceSpec)

// CreateService creates a service on the passed in swarm daemon.
func CreateService(ctx context.Context, t *testing.T, d *daemon.Daemon, opts ...ServiceSpecOpt) string {
	t.Helper()

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	spec := CreateServiceSpec(t, opts...)
	resp, err := apiClient.ServiceCreate(ctx, spec, types.ServiceCreateOptions{})
	assert.NilError(t, err, "error creating service")
	return resp.ID
}

// CreateServiceSpec creates a default service-spec, and applies the provided options
func CreateServiceSpec(t *testing.T, opts ...ServiceSpecOpt) swarmtypes.ServiceSpec {
	t.Helper()
	var spec swarmtypes.ServiceSpec
	ServiceWithImage("busybox:latest")(&spec)
	ServiceWithCommand([]string{"/bin/top"})(&spec)
	ServiceWithReplicas(1)(&spec)

	for _, o := range opts {
		o(&spec)
	}
	return spec
}

// ServiceWithMode sets the mode of the service to the provided mode.
func ServiceWithMode(mode swarmtypes.ServiceMode) func(*swarmtypes.ServiceSpec) {
	return func(spec *swarmtypes.ServiceSpec) {
		spec.Mode = mode
	}
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

// ServiceWithMaxReplicas sets the max replicas for the service
func ServiceWithMaxReplicas(n uint64) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		ensurePlacement(spec)
		spec.TaskTemplate.Placement.MaxReplicas = n
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

// ServiceWithSysctls sets the Sysctls option of the service's ContainerSpec.
func ServiceWithSysctls(sysctls map[string]string) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		ensureContainerSpec(spec)
		spec.TaskTemplate.ContainerSpec.Sysctls = sysctls
	}
}

// ServiceWithCapabilities sets the Capabilities option of the service's ContainerSpec.
func ServiceWithCapabilities(add []string, drop []string) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		ensureContainerSpec(spec)
		spec.TaskTemplate.ContainerSpec.CapabilityAdd = add
		spec.TaskTemplate.ContainerSpec.CapabilityDrop = drop
	}
}

// ServiceWithPidsLimit sets the PidsLimit option of the service's Resources.Limits.
func ServiceWithPidsLimit(limit int64) ServiceSpecOpt {
	return func(spec *swarmtypes.ServiceSpec) {
		ensureResources(spec)
		spec.TaskTemplate.Resources.Limits.Pids = limit
	}
}

// GetRunningTasks gets the list of running tasks for a service
func GetRunningTasks(ctx context.Context, t *testing.T, c client.ServiceAPIClient, serviceID string) []swarmtypes.Task {
	t.Helper()

	tasks, err := c.TaskList(ctx, types.TaskListOptions{
		Filters: filters.NewArgs(
			filters.Arg("service", serviceID),
			filters.Arg("desired-state", "running"),
		),
	})

	assert.NilError(t, err)
	return tasks
}

// ExecTask runs the passed in exec config on the given task
func ExecTask(ctx context.Context, t *testing.T, d *daemon.Daemon, task swarmtypes.Task, options container.ExecOptions) types.HijackedResponse {
	t.Helper()
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	resp, err := apiClient.ContainerExecCreate(ctx, task.Status.ContainerStatus.ContainerID, options)
	assert.NilError(t, err, "error creating exec")

	attach, err := apiClient.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{})
	assert.NilError(t, err, "error attaching to exec")
	return attach
}

func ensureResources(spec *swarmtypes.ServiceSpec) {
	if spec.TaskTemplate.Resources == nil {
		spec.TaskTemplate.Resources = &swarmtypes.ResourceRequirements{}
	}
	if spec.TaskTemplate.Resources.Limits == nil {
		spec.TaskTemplate.Resources.Limits = &swarmtypes.Limit{}
	}
	if spec.TaskTemplate.Resources.Reservations == nil {
		spec.TaskTemplate.Resources.Reservations = &swarmtypes.Resources{}
	}
}

func ensureContainerSpec(spec *swarmtypes.ServiceSpec) {
	if spec.TaskTemplate.ContainerSpec == nil {
		spec.TaskTemplate.ContainerSpec = &swarmtypes.ContainerSpec{}
	}
}

func ensurePlacement(spec *swarmtypes.ServiceSpec) {
	if spec.TaskTemplate.Placement == nil {
		spec.TaskTemplate.Placement = &swarmtypes.Placement{}
	}
}
