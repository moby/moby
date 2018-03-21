package swarm

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/internal/test/environment"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/gotestyourself/gotestyourself/skip"
)

const (
	dockerdBinary    = "dockerd"
	defaultSwarmPort = 2477
)

// NewSwarm creates a swarm daemon for testing
func NewSwarm(t *testing.T, testEnv *environment.Execution) *daemon.Swarm {
	skip.IfCondition(t, testEnv.IsRemoteDaemon())
	d := &daemon.Swarm{
		Daemon: daemon.New(t, "", dockerdBinary, daemon.Config{
			Experimental: testEnv.DaemonInfo.ExperimentalBuild,
		}),
		// TODO: better method of finding an unused port
		Port: defaultSwarmPort,
	}
	// TODO: move to a NewSwarm constructor
	d.ListenAddr = fmt.Sprintf("0.0.0.0:%d", d.Port)

	// avoid networking conflicts
	args := []string{"--iptables=false", "--swarm-default-advertise-addr=lo"}
	d.StartWithBusybox(t, args...)

	assert.NilError(t, d.Init(swarmtypes.InitRequest{}))
	return d
}

// ServiceSpecOpt is used with `CreateService` to pass in service spec modifiers
type ServiceSpecOpt func(*swarmtypes.ServiceSpec)

// CreateService creates a service on the passed in swarm daemon.
func CreateService(t *testing.T, d *daemon.Swarm, opts ...ServiceSpecOpt) string {
	spec := defaultServiceSpec()
	for _, o := range opts {
		o(&spec)
	}

	client := GetClient(t, d)

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

// GetRunningTasks gets the list of running tasks for a service
func GetRunningTasks(t *testing.T, d *daemon.Swarm, serviceID string) []swarmtypes.Task {
	client := GetClient(t, d)

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
func ExecTask(t *testing.T, d *daemon.Swarm, task swarmtypes.Task, config types.ExecConfig) types.HijackedResponse {
	client := GetClient(t, d)

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

// GetClient creates a new client for the passed in swarm daemon.
func GetClient(t *testing.T, d *daemon.Swarm) client.APIClient {
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	assert.NilError(t, err)
	return client
}
