package network // import "github.com/docker/docker/integration/network"

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/poll"
	"gotest.tools/skip"
)

const defaultSwarmPort = 2477

func TestNetworkFilter(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	netName1 := "foo_" + t.Name()
	netName2 := "bar_" + t.Name()

	network.CreateNoError(t, context.Background(), client, netName1)
	defer client.NetworkRemove(context.Background(), netName1)
	network.CreateNoError(t, context.Background(), client, netName2)
	defer client.NetworkRemove(context.Background(), netName2)

	filter := filters.NewArgs()
	filter.Add("name", netName1)
	networkList, err := client.NetworkList(context.Background(), types.NetworkListOptions{
		Filters: filter,
	})
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(1, len(networkList)))

	netID := networkList[0].ID
	assert.Assert(t, 0 != len(netID))

	nr, err := client.NetworkInspect(context.Background(), netName1, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(nr.ID, netID))
}

func TestNetworkInspectBridge(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	// Inspect default bridge network
	nr, err := client.NetworkInspect(context.Background(), "bridge", types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal("bridge", nr.Name))

	// run a container and attach it to the default bridge network
	containerID := container.Run(t, context.Background(), client, container.WithName("test_"+t.Name()),
		container.WithCmd("top"),
	)
	containerIP := FindContainerIP(t, client, containerID, "bridge")

	// inspect default bridge network again and make sure the container is connected
	nr, err = client.NetworkInspect(context.Background(), nr.ID, types.NetworkInspectOptions{})
	assert.NilError(t, err)

	assert.Check(t, is.Equal("bridge", nr.Driver))
	assert.Check(t, is.Equal("local", nr.Scope))
	assert.Check(t, is.Equal(false, nr.Internal))
	assert.Check(t, is.Equal(false, nr.EnableIPv6))
	assert.Check(t, is.Equal("default", nr.IPAM.Driver))
	_, ok := nr.Containers[containerID]
	assert.Check(t, ok)

	ip, _, err := net.ParseCIDR(nr.Containers[containerID].IPv4Address)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(containerIP, ip.String()))
}

func TestNetworkInspectUserDefinedNetwork(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	netName := "br0_" + t.Name()
	id0 := network.CreateNoError(t, context.Background(), client, netName,
		network.WithDriver("bridge"),
		network.WithIPAM("default", "172.28.0.0/16", "172.28.5.254", "172.28.5.0/24"),
		network.WithOption("foo", "bar"),
		network.WithOption("opts", "dopts"),
	)

	assert.Check(t, IsNetworkAvailable(client, netName))

	nr, err := client.NetworkInspect(context.Background(), id0, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(1, len(nr.IPAM.Config)))
	assert.Check(t, is.Equal("172.28.0.0/16", nr.IPAM.Config[0].Subnet))
	assert.Check(t, is.Equal("172.28.5.0/24", nr.IPAM.Config[0].IPRange))
	assert.Check(t, is.Equal("172.28.5.254", nr.IPAM.Config[0].Gateway))
	assert.Check(t, is.Equal("bar", nr.Options["foo"]))
	assert.Check(t, is.Equal("dopts", nr.Options["opts"]))

	// delete the network and make sure it is deleted
	err = client.NetworkRemove(context.Background(), id0)
	assert.NilError(t, err)
	assert.Check(t, IsNetworkNotAvailable(client, netName))
}

func TestInspectNetwork(t *testing.T) {
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	overlayName := "overlay1"
	overlayID := network.CreateNoError(t, context.Background(), client, overlayName,
		network.WithDriver("overlay"),
		network.WithCheckDuplicate(),
	)

	var instances uint64 = 4
	serviceName := "TestService" + t.Name()

	serviceID := swarm.CreateService(t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithNetwork(overlayName),
	)

	poll.WaitOn(t, serviceRunningTasksCount(client, serviceID, instances), swarm.ServicePoll)

	_, _, err := client.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)

	// Test inspect verbose with full NetworkID
	networkVerbose, err := client.NetworkInspect(context.Background(), overlayID, types.NetworkInspectOptions{
		Verbose: true,
	})
	assert.NilError(t, err)
	assert.Assert(t, validNetworkVerbose(networkVerbose, serviceName, instances))

	// Test inspect verbose with partial NetworkID
	networkVerbose, err = client.NetworkInspect(context.Background(), overlayID[0:11], types.NetworkInspectOptions{
		Verbose: true,
	})
	assert.NilError(t, err)
	assert.Assert(t, validNetworkVerbose(networkVerbose, serviceName, instances))

	// Test inspect verbose with Network name and swarm scope
	networkVerbose, err = client.NetworkInspect(context.Background(), overlayName, types.NetworkInspectOptions{
		Verbose: true,
		Scope:   "swarm",
	})
	assert.NilError(t, err)
	assert.Assert(t, validNetworkVerbose(networkVerbose, serviceName, instances))

	err = client.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)

	poll.WaitOn(t, serviceIsRemoved(client, serviceID), swarm.ServicePoll)
	poll.WaitOn(t, noTasks(client), swarm.ServicePoll)

	serviceID2 := swarm.CreateService(t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithNetwork(overlayName),
	)

	poll.WaitOn(t, serviceRunningTasksCount(client, serviceID2, instances), swarm.ServicePoll)

	err = client.ServiceRemove(context.Background(), serviceID2)
	assert.NilError(t, err)

	poll.WaitOn(t, serviceIsRemoved(client, serviceID2), swarm.ServicePoll)
	poll.WaitOn(t, noTasks(client), swarm.ServicePoll)

	err = client.NetworkRemove(context.Background(), overlayID)
	assert.NilError(t, err)

	poll.WaitOn(t, networkIsRemoved(client, overlayID), poll.WithTimeout(1*time.Minute), poll.WithDelay(10*time.Second))
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
				if task.Status.State != swarmtypes.TaskStateRunning {
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
		if len(service.Tasks) != int(instances) {
			return false
		}
	}

	if network.IPAM.Config == nil {
		return false
	}

	for _, cfg := range network.IPAM.Config {
		if cfg.Gateway == "" || cfg.Subnet == "" {
			return false
		}
	}
	return true
}
