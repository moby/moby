package network // import "github.com/docker/docker/integration/network"

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/internal/test/daemon"
	"gotest.tools/assert"
	"gotest.tools/icmd"
	"gotest.tools/poll"
	"gotest.tools/skip"
)

// delInterface removes given network interface
func delInterface(t *testing.T, ifName string) {
	icmd.RunCommand("ip", "link", "delete", ifName).Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "-t", "nat", "--flush").Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "--flush").Assert(t, icmd.Success)
}

func TestDaemonRestartWithLiveRestore(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.38"), "skip test from new feature")
	d := daemon.New(t)
	defer d.Stop(t)
	d.Start(t)
	d.Restart(t,
		"--live-restore=true",
		"--default-address-pool", "base=175.30.0.0/16,size=16",
		"--default-address-pool", "base=175.33.0.0/16,size=24",
	)

	// Verify bridge network's subnet
	c := d.NewClientT(t)
	defer c.Close()
	out, err := c.NetworkInspect(context.Background(), "bridge", types.NetworkInspectOptions{})
	assert.NilError(t, err)
	// Make sure docker0 doesn't get override with new IP in live restore case
	assert.Equal(t, out.IPAM.Config[0].Subnet, "172.18.0.0/16")
}

func TestDaemonDefaultNetworkPools(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	// Remove docker0 bridge and the start daemon defining the predefined address pools
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.38"), "skip test from new feature")
	defaultNetworkBridge := "docker0"
	delInterface(t, defaultNetworkBridge)
	d := daemon.New(t)
	defer d.Stop(t)
	d.Start(t,
		"--default-address-pool", "base=175.30.0.0/16,size=16",
		"--default-address-pool", "base=175.33.0.0/16,size=24",
	)

	c := d.NewClientT(t)
	defer c.Close()

	// Verify bridge network's subnet
	out, err := c.NetworkInspect(context.Background(), "bridge", types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, out.IPAM.Config[0].Subnet, "175.30.0.0/16")

	// Create a bridge network and verify its subnet is the second default pool
	name := "elango" + t.Name()
	network.CreateNoError(context.Background(), t, c, name,
		network.WithDriver("bridge"),
	)
	out, err = c.NetworkInspect(context.Background(), name, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, out.IPAM.Config[0].Subnet, "175.33.0.0/24")

	// Create a bridge network and verify its subnet is the third default pool
	name = "saanvi" + t.Name()
	network.CreateNoError(context.Background(), t, c, name,
		network.WithDriver("bridge"),
	)
	out, err = c.NetworkInspect(context.Background(), name, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, out.IPAM.Config[0].Subnet, "175.33.1.0/24")
	delInterface(t, defaultNetworkBridge)

}

func TestDaemonRestartWithExistingNetwork(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.38"), "skip test from new feature")
	defaultNetworkBridge := "docker0"
	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	// Create a bridge network
	name := "elango" + t.Name()
	network.CreateNoError(context.Background(), t, c, name,
		network.WithDriver("bridge"),
	)

	// Verify bridge network's subnet
	out, err := c.NetworkInspect(context.Background(), name, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	networkip := out.IPAM.Config[0].Subnet

	// Restart daemon with default address pool option
	d.Restart(t,
		"--default-address-pool", "base=175.30.0.0/16,size=16",
		"--default-address-pool", "base=175.33.0.0/16,size=24")

	out1, err := c.NetworkInspect(context.Background(), name, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, out1.IPAM.Config[0].Subnet, networkip)
	delInterface(t, defaultNetworkBridge)
}

func TestDaemonRestartWithExistingNetworkWithDefaultPoolRange(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.38"), "skip test from new feature")
	defaultNetworkBridge := "docker0"
	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	// Create a bridge network
	name := "elango" + t.Name()
	network.CreateNoError(context.Background(), t, c, name,
		network.WithDriver("bridge"),
	)

	// Verify bridge network's subnet
	out, err := c.NetworkInspect(context.Background(), name, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	networkip := out.IPAM.Config[0].Subnet

	// Create a bridge network
	name = "sthira" + t.Name()
	network.CreateNoError(context.Background(), t, c, name,
		network.WithDriver("bridge"),
	)
	out, err = c.NetworkInspect(context.Background(), name, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	networkip2 := out.IPAM.Config[0].Subnet

	// Restart daemon with default address pool option
	d.Restart(t,
		"--default-address-pool", "base=175.18.0.0/16,size=16",
		"--default-address-pool", "base=175.19.0.0/16,size=24",
	)

	// Create a bridge network
	name = "saanvi" + t.Name()
	network.CreateNoError(context.Background(), t, c, name,
		network.WithDriver("bridge"),
	)
	out1, err := c.NetworkInspect(context.Background(), name, types.NetworkInspectOptions{})
	assert.NilError(t, err)

	assert.Check(t, out1.IPAM.Config[0].Subnet != networkip)
	assert.Check(t, out1.IPAM.Config[0].Subnet != networkip2)
	delInterface(t, defaultNetworkBridge)
}

func TestDaemonWithBipAndDefaultNetworkPool(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.38"), "skip test from new feature")
	defaultNetworkBridge := "docker0"
	d := daemon.New(t)
	defer d.Stop(t)
	d.Start(t,
		"--bip=172.60.0.1/16",
		"--default-address-pool", "base=175.30.0.0/16,size=16",
		"--default-address-pool", "base=175.33.0.0/16,size=24",
	)

	c := d.NewClientT(t)
	defer c.Close()

	// Verify bridge network's subnet
	out, err := c.NetworkInspect(context.Background(), "bridge", types.NetworkInspectOptions{})
	assert.NilError(t, err)
	// Make sure BIP IP doesn't get override with new default address pool .
	assert.Equal(t, out.IPAM.Config[0].Subnet, "172.60.0.1/16")
	delInterface(t, defaultNetworkBridge)
}

func TestServiceWithPredefinedNetwork(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	hostName := "host"
	var instances uint64 = 1
	serviceName := "TestService" + t.Name()

	serviceID := swarm.CreateService(t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithNetwork(hostName),
	)

	poll.WaitOn(t, serviceRunningCount(c, serviceID, instances), swarm.ServicePoll)

	_, _, err := c.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)

	err = c.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)
}

const ingressNet = "ingress"

func TestServiceRemoveKeepsIngressNetwork(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	poll.WaitOn(t, swarmIngressReady(c), swarm.NetworkPoll)

	var instances uint64 = 1

	serviceID := swarm.CreateService(t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(t.Name()+"-service"),
		swarm.ServiceWithEndpoint(&swarmtypes.EndpointSpec{
			Ports: []swarmtypes.PortConfig{
				{
					Protocol:    swarmtypes.PortConfigProtocolTCP,
					TargetPort:  80,
					PublishMode: swarmtypes.PortConfigPublishModeIngress,
				},
			},
		}),
	)

	poll.WaitOn(t, serviceRunningCount(c, serviceID, instances), swarm.ServicePoll)

	ctx := context.Background()
	_, _, err := c.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)

	err = c.ServiceRemove(ctx, serviceID)
	assert.NilError(t, err)

	poll.WaitOn(t, noServices(ctx, c), swarm.ServicePoll)
	poll.WaitOn(t, swarm.NoTasks(ctx, c), swarm.ServicePoll)

	// Ensure that "ingress" is not removed or corrupted
	time.Sleep(10 * time.Second)
	netInfo, err := c.NetworkInspect(ctx, ingressNet, types.NetworkInspectOptions{
		Verbose: true,
		Scope:   "swarm",
	})
	assert.NilError(t, err, "Ingress network was removed after removing service!")
	assert.Assert(t, len(netInfo.Containers) != 0, "No load balancing endpoints in ingress network")
	assert.Assert(t, len(netInfo.Peers) != 0, "No peers (including self) in ingress network")
	_, ok := netInfo.Containers["ingress-sbox"]
	assert.Assert(t, ok, "ingress-sbox not present in ingress network")
}

func serviceRunningCount(client client.ServiceAPIClient, serviceID string, instances uint64) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		services, err := client.ServiceList(context.Background(), types.ServiceListOptions{})
		if err != nil {
			return poll.Error(err)
		}

		if len(services) != int(instances) {
			return poll.Continue("Service count at %d waiting for %d", len(services), instances)
		}
		return poll.Success()
	}
}

func swarmIngressReady(client client.NetworkAPIClient) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		netInfo, err := client.NetworkInspect(context.Background(), ingressNet, types.NetworkInspectOptions{
			Verbose: true,
			Scope:   "swarm",
		})
		if err != nil {
			return poll.Error(err)
		}
		np := len(netInfo.Peers)
		nc := len(netInfo.Containers)
		if np == 0 || nc == 0 {
			return poll.Continue("ingress not ready: %d peers and %d containers", nc, np)
		}
		_, ok := netInfo.Containers["ingress-sbox"]
		if !ok {
			return poll.Continue("ingress not ready: does not contain the ingress-sbox")
		}
		return poll.Success()
	}
}

func noServices(ctx context.Context, client client.ServiceAPIClient) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		services, err := client.ServiceList(ctx, types.ServiceListOptions{})
		switch {
		case err != nil:
			return poll.Error(err)
		case len(services) == 0:
			return poll.Success()
		default:
			return poll.Continue("waiting for all services to be removed: service count at %d", len(services))
		}
	}
}

func TestServiceWithDataPathPortInit(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.40"), "DataPathPort was added in API v1.40")
	defer setupTest(t)()
	var ops = []func(*daemon.Daemon){}
	var datapathPort uint32 = 7777
	ops = append(ops, daemon.WithSwarmDataPathPort(datapathPort))
	d := swarm.NewSwarm(t, testEnv, ops...)

	c := d.NewClientT(t)
	defer c.Close()

	// Create a overlay network
	name := "saanvisthira" + t.Name()
	network.CreateNoError(context.Background(), t, c, name,
		network.WithDriver("overlay"))

	var instances uint64 = 1
	serviceID := swarm.CreateService(t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithNetwork(name),
	)

	poll.WaitOn(t, serviceRunningCount(c, serviceID, instances), swarm.ServicePoll)

	info := d.Info(t)
	assert.Equal(t, info.Swarm.Cluster.DataPathPort, datapathPort)
	err := c.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)
	d.SwarmLeave(t, true)
	d.Stop(t)

	// Clean up , set it back to original one to make sure other tests don't fail
	// call without datapath port option.
	ops = []func(*daemon.Daemon){}
	d = swarm.NewSwarm(t, testEnv, ops...)
	c = d.NewClientT(t)

	// Create a overlay network
	name = "saanvisthira" + t.Name()
	network.CreateNoError(context.Background(), t, c, name,
		network.WithDriver("overlay"))

	serviceID = swarm.CreateService(t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithNetwork(name),
	)

	poll.WaitOn(t, serviceRunningCount(c, serviceID, instances), swarm.ServicePoll)

	info = d.Info(t)
	var defaultDataPathPort uint32 = 4789
	assert.Equal(t, info.Swarm.Cluster.DataPathPort, defaultDataPathPort)
	err = c.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)
	d.SwarmLeave(t, true)
	defer d.Stop(t)
}

func TestServiceWithDefaultAddressPoolInit(t *testing.T) {
	skip.If(t, testEnv.OSType == "windows")
	defer setupTest(t)()
	var ops = []func(*daemon.Daemon){}
	ipAddr := []string{"20.20.0.0/16"}
	ops = append(ops, daemon.WithSwarmDefaultAddrPool(ipAddr))
	ops = append(ops, daemon.WithSwarmDefaultAddrPoolSubnetSize(24))
	d := swarm.NewSwarm(t, testEnv, ops...)

	cli := d.NewClientT(t)
	defer cli.Close()

	// Create a overlay network
	name := "saanvisthira" + t.Name()
	network.CreateNoError(context.Background(), t, cli, name,
		network.WithDriver("overlay"))

	var instances uint64 = 1
	serviceName := "TestService" + t.Name()
	serviceID := swarm.CreateService(t, d,
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithNetwork(name),
	)

	poll.WaitOn(t, serviceRunningCount(cli, serviceID, instances), swarm.ServicePoll)

	_, _, err := cli.ServiceInspectWithRaw(context.Background(), serviceID, types.ServiceInspectOptions{})
	assert.NilError(t, err)

	out, err := cli.NetworkInspect(context.Background(), name, types.NetworkInspectOptions{})
	assert.NilError(t, err)
	t.Logf("%s: NetworkInspect: %+v", t.Name(), out)
	assert.Assert(t, len(out.IPAM.Config) > 0)
	assert.Equal(t, out.IPAM.Config[0].Subnet, "20.20.0.0/24")

	err = cli.ServiceRemove(context.Background(), serviceID)
	assert.NilError(t, err)
	d.SwarmLeave(t, true)
	d.Stop(t)

	// Clean up , set it back to original one to make sure other tests don't fail
	ipAddr = []string{"10.0.0.0/8"}
	ops = append(ops, daemon.WithSwarmDefaultAddrPool(ipAddr))
	ops = append(ops, daemon.WithSwarmDefaultAddrPoolSubnetSize(24))
	d = swarm.NewSwarm(t, testEnv, ops...)
	d.SwarmLeave(t, true)
	defer d.Stop(t)
}
