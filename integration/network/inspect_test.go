package network // import "github.com/docker/docker/integration/network"

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

const defaultSwarmPort = 2477
const dockerdBinary = "dockerd"

func TestInspectNetwork(t *testing.T) {
	defer setupTest(t)()
	d := newSwarm(t)
	defer d.Stop(t)
	client, err := client.NewClientWithOpts(client.WithHost((d.Sock())))
	require.NoError(t, err)

	overlayName := "overlay1"
	networkCreate := types.NetworkCreate{
		CheckDuplicate: true,
		Driver:         "overlay",
	}

	netResp, err := client.NetworkCreate(context.Background(), overlayName, networkCreate)
	require.NoError(t, err)
	overlayID := netResp.ID

	var instances uint64 = 4
	serviceName := "TestService"
	serviceSpec := swarmServiceSpec(serviceName, instances)
	serviceSpec.TaskTemplate.Networks = append(serviceSpec.TaskTemplate.Networks, swarm.NetworkAttachmentConfig{Target: overlayName})

	serviceResp, err := client.ServiceCreate(context.Background(), serviceSpec, types.ServiceCreateOptions{
		QueryRegistry: false,
	})
	require.NoError(t, err)

	pollSettings := func(config *poll.Settings) {
		if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
			config.Timeout = 30 * time.Second
			config.Delay = 100 * time.Millisecond
		}
	}

	serviceID := serviceResp.ID
	poll.WaitOn(t, serviceRunningTasksCount(client, serviceID, instances), pollSettings)

	_, _, err = client.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	require.NoError(t, err)

	// Test inspect verbose with full NetworkID
	networkVerbose, err := client.NetworkInspect(context.Background(), overlayID, types.NetworkInspectOptions{
		Verbose: true,
	})
	require.NoError(t, err)
	require.True(t, validNetworkVerbose(networkVerbose, serviceName, instances))

	// Test inspect verbose with partial NetworkID
	networkVerbose, err = client.NetworkInspect(context.Background(), overlayID[0:11], types.NetworkInspectOptions{
		Verbose: true,
	})
	require.NoError(t, err)
	require.True(t, validNetworkVerbose(networkVerbose, serviceName, instances))

	// Test inspect verbose with Network name and swarm scope
	networkVerbose, err = client.NetworkInspect(context.Background(), overlayName, types.NetworkInspectOptions{
		Verbose: true,
		Scope:   "swarm",
	})
	require.NoError(t, err)
	require.True(t, validNetworkVerbose(networkVerbose, serviceName, instances))

	err = client.ServiceRemove(context.Background(), serviceID)
	require.NoError(t, err)

	poll.WaitOn(t, serviceIsRemoved(client, serviceID), pollSettings)
	poll.WaitOn(t, noTasks(client), pollSettings)

	serviceResp, err = client.ServiceCreate(context.Background(), serviceSpec, types.ServiceCreateOptions{
		QueryRegistry: false,
	})
	require.NoError(t, err)

	serviceID2 := serviceResp.ID
	poll.WaitOn(t, serviceRunningTasksCount(client, serviceID2, instances), pollSettings)

	err = client.ServiceRemove(context.Background(), serviceID2)
	require.NoError(t, err)

	poll.WaitOn(t, serviceIsRemoved(client, serviceID2), pollSettings)
	poll.WaitOn(t, noTasks(client), pollSettings)

	err = client.NetworkRemove(context.Background(), overlayID)
	require.NoError(t, err)

	poll.WaitOn(t, networkIsRemoved(client, overlayID), poll.WithTimeout(1*time.Minute), poll.WithDelay(10*time.Second))
}

func newSwarm(t *testing.T) *daemon.Swarm {
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

	require.NoError(t, d.Init(swarm.InitRequest{}))
	return d
}

func swarmServiceSpec(name string, replicas uint64) swarm.ServiceSpec {
	return swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: name,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image:   "busybox:latest",
				Command: []string{"/bin/top"},
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
		},
	}
}

func serviceRunningTasksCount(client client.ServiceAPIClient, serviceID string, instances uint64) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		filter := filters.NewArgs()
		filter.Add("service", serviceID)
		tasks, err := client.TaskList(context.Background(), types.TaskListOptions{
			Filters: filter,
		})
		switch {
		case err != nil:
			return poll.Error(err)
		case len(tasks) == int(instances):
			for _, task := range tasks {
				if task.Status.State != swarm.TaskStateRunning {
					return poll.Continue("waiting for tasks to enter run state")
				}
			}
			return poll.Success()
		default:
			return poll.Continue("task count at %d waiting for %d", len(tasks), instances)
		}
	}
}

func networkIsRemoved(client client.NetworkAPIClient, networkID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		_, err := client.NetworkInspect(context.Background(), networkID, types.NetworkInspectOptions{})
		if err == nil {
			return poll.Continue("waiting for network %s to be removed", networkID)
		}
		return poll.Success()
	}
}

func serviceIsRemoved(client client.ServiceAPIClient, serviceID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		filter := filters.NewArgs()
		filter.Add("service", serviceID)
		_, err := client.TaskList(context.Background(), types.TaskListOptions{
			Filters: filter,
		})
		if err == nil {
			return poll.Continue("waiting for service %s to be deleted", serviceID)
		}
		return poll.Success()
	}
}

func noTasks(client client.ServiceAPIClient) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		filter := filters.NewArgs()
		tasks, err := client.TaskList(context.Background(), types.TaskListOptions{
			Filters: filter,
		})
		switch {
		case err != nil:
			return poll.Error(err)
		case len(tasks) == 0:
			return poll.Success()
		default:
			return poll.Continue("task count at %d waiting for 0", len(tasks))
		}
	}
}

// Check to see if Service and Tasks info are part of the inspect verbose response
func validNetworkVerbose(network types.NetworkResource, service string, instances uint64) bool {
	if service, ok := network.Services[service]; ok {
		if len(service.Tasks) == int(instances) {
			return true
		}
	}
	return false
}
