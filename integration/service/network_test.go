package service // import "github.com/docker/docker/integration/service"

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	net "github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/docker/docker/libnetwork/scope"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDockerNetworkConnectAliasPreV144(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t, client.WithVersion("1.43"))
	defer client.Close()

	name := t.Name() + "test-alias"
	net.CreateNoError(ctx, t, client, name,
		net.WithDriver("overlay"),
		net.WithAttachable(),
	)

	cID1 := container.Create(ctx, t, client, func(c *container.TestContainerConfig) {
		c.NetworkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				name: {},
			},
		}
	})

	err := client.NetworkConnect(ctx, name, cID1, &network.EndpointSettings{
		Aliases: []string{
			"aaa",
		},
	})
	assert.NilError(t, err)

	err = client.ContainerStart(ctx, cID1, containertypes.StartOptions{})
	assert.NilError(t, err)

	ng1, err := client.ContainerInspect(ctx, cID1)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(ng1.NetworkSettings.Networks[name].Aliases), 2))
	assert.Check(t, is.Equal(ng1.NetworkSettings.Networks[name].Aliases[0], "aaa"))

	cID2 := container.Create(ctx, t, client, func(c *container.TestContainerConfig) {
		c.NetworkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				name: {},
			},
		}
	})

	err = client.NetworkConnect(ctx, name, cID2, &network.EndpointSettings{
		Aliases: []string{
			"bbb",
		},
	})
	assert.NilError(t, err)

	err = client.ContainerStart(ctx, cID2, containertypes.StartOptions{})
	assert.NilError(t, err)

	ng2, err := client.ContainerInspect(ctx, cID2)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(ng2.NetworkSettings.Networks[name].Aliases), 2))
	assert.Check(t, is.Equal(ng2.NetworkSettings.Networks[name].Aliases[0], "bbb"))
}

func TestDockerNetworkReConnect(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	name := t.Name() + "dummyNet"
	net.CreateNoError(ctx, t, client, name,
		net.WithDriver("overlay"),
		net.WithAttachable(),
	)

	c1 := container.Create(ctx, t, client, func(c *container.TestContainerConfig) {
		c.NetworkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				name: {},
			},
		}
	})

	err := client.NetworkConnect(ctx, name, c1, &network.EndpointSettings{})
	assert.NilError(t, err)

	err = client.ContainerStart(ctx, c1, containertypes.StartOptions{})
	assert.NilError(t, err)

	n1, err := client.ContainerInspect(ctx, c1)
	assert.NilError(t, err)

	err = client.NetworkConnect(ctx, name, c1, &network.EndpointSettings{})
	assert.ErrorContains(t, err, "is already attached to network")

	n2, err := client.ContainerInspect(ctx, c1)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(n1, n2))
}

// Check that a swarm-scoped network can't have EnableIPv4=false.
func TestSwarmNoDisableIPv4(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	_, err := net.Create(ctx, client, "overlay-v6-only",
		net.WithDriver("overlay"),
		net.WithAttachable(),
		net.WithIPv4(false),
	)
	assert.Check(t, is.ErrorContains(err, "IPv4 cannot be disabled in a Swarm scoped network"))
}

// Regression test for https://github.com/docker/cli/issues/5857
func TestSwarmScopedNetFromConfig(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	const configNetName = "config-net"
	_ = net.CreateNoError(ctx, t, c, configNetName,
		net.WithDriver("bridge"),
		net.WithConfigOnly(true),
	)
	const swarmNetName = "swarm-net"
	_, err := net.Create(ctx, c, swarmNetName,
		net.WithDriver("bridge"),
		net.WithConfigFrom(configNetName),
		net.WithAttachable(),
		net.WithScope(scope.Swarm),
	)
	assert.NilError(t, err)

	serviceID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithName("test-ssnfc"),
		swarm.ServiceWithNetwork(swarmNetName),
	)
	defer func() {
		err := c.ServiceRemove(ctx, serviceID)
		assert.NilError(t, err)
	}()

	poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, serviceID, 1), swarm.ServicePoll)
}
