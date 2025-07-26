package service

import (
	stdnet "net"
	"net/netip"
	"strings"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
	"github.com/moby/moby/v2/integration/internal/container"
	net "github.com/moby/moby/v2/integration/internal/network"
	"github.com/moby/moby/v2/integration/internal/swarm"
	"github.com/moby/moby/v2/integration/internal/testutils/networking"
	"github.com/moby/moby/v2/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDockerNetworkConnectAliasPreV144(t *testing.T) {
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	apiClient := d.NewClientT(t, client.WithVersion("1.43"))
	defer apiClient.Close()

	name := t.Name() + "test-alias"
	net.CreateNoError(ctx, t, apiClient, name,
		net.WithDriver("overlay"),
		net.WithAttachable(),
	)

	cID1 := container.Create(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.NetworkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				name: {},
			},
		}
	})

	err := apiClient.NetworkConnect(ctx, name, cID1, &network.EndpointSettings{
		Aliases: []string{
			"aaa",
		},
	})
	assert.NilError(t, err)

	err = apiClient.ContainerStart(ctx, cID1, containertypes.StartOptions{})
	assert.NilError(t, err)

	ng1, err := apiClient.ContainerInspect(ctx, cID1)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(ng1.NetworkSettings.Networks[name].Aliases), 2))
	assert.Check(t, is.Equal(ng1.NetworkSettings.Networks[name].Aliases[0], "aaa"))

	cID2 := container.Create(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.NetworkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				name: {},
			},
		}
	})

	err = apiClient.NetworkConnect(ctx, name, cID2, &network.EndpointSettings{
		Aliases: []string{
			"bbb",
		},
	})
	assert.NilError(t, err)

	err = apiClient.ContainerStart(ctx, cID2, containertypes.StartOptions{})
	assert.NilError(t, err)

	ng2, err := apiClient.ContainerInspect(ctx, cID2)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(ng2.NetworkSettings.Networks[name].Aliases), 2))
	assert.Check(t, is.Equal(ng2.NetworkSettings.Networks[name].Aliases[0], "bbb"))
}

func TestDockerNetworkReConnect(t *testing.T) {
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	name := t.Name() + "dummyNet"
	net.CreateNoError(ctx, t, apiClient, name,
		net.WithDriver("overlay"),
		net.WithAttachable(),
	)

	c1 := container.Create(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.NetworkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				name: {},
			},
		}
	})

	err := apiClient.NetworkConnect(ctx, name, c1, &network.EndpointSettings{})
	assert.NilError(t, err)

	err = apiClient.ContainerStart(ctx, c1, containertypes.StartOptions{})
	assert.NilError(t, err)

	n1, err := apiClient.ContainerInspect(ctx, c1)
	assert.NilError(t, err)

	err = apiClient.NetworkConnect(ctx, name, c1, &network.EndpointSettings{})
	assert.ErrorContains(t, err, "is already attached to network")

	n2, err := apiClient.ContainerInspect(ctx, c1)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(n1, n2))
}

// Check that a swarm-scoped network can't have EnableIPv4=false.
func TestSwarmNoDisableIPv4(t *testing.T) {
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	_, err := net.Create(ctx, apiClient, "overlay-v6-only",
		net.WithDriver("overlay"),
		net.WithAttachable(),
		net.WithIPv4(false),
	)
	assert.Check(t, is.ErrorContains(err, "IPv4 cannot be disabled in a Swarm scoped network"))
}

// Regression test for https://github.com/docker/cli/issues/5857
func TestSwarmScopedNetFromConfig(t *testing.T) {
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

// Check that, when swarm has ports mapped to the host, the iptables jump to
// DOCKER-INGRESS remains in place after a daemon restart.
// Regression test for https://github.com/moby/moby/pull/49538
func TestDockerIngressChainPosition(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	skip.If(t, testEnv.FirewallBackendDriver() == "nftables")
	skip.If(t, networking.FirewalldRunning(), "can't use firewalld in host netns to add rules in L3Segment")
	ctx := setupTest(t)

	// Run the test in its own netns, to avoid interfering with iptables on the test host.
	const hostAddr = "192.168.111.222"
	const l3SegHost = "dicp"
	l3 := networking.NewL3Segment(t, "test-"+l3SegHost)
	defer l3.Destroy(t)
	l3.AddHost(t, l3SegHost, "ns-"+l3SegHost, "eth0", netip.MustParsePrefix(hostAddr+"/24"))

	// Check the published port is accessible.
	checkHTTP := func(_ poll.LogT) poll.Result {
		var res *icmd.Result
		// This is called from inside a "Do()" thread in the docker host's netns, but it
		// uses poll.WaitOn - which runs the command in a different goroutine.
		l3.Hosts[l3SegHost].Do(t, func() {
			res = icmd.RunCommand("wget", "-T1", "-t1", "-O-",
				"http://"+stdnet.JoinHostPort(hostAddr, "8080"))
		})
		// A "404 Not Found" means the server responded, but it's got nothing to serve.
		if !strings.Contains(res.Stderr(), "404 Not Found") {
			return poll.Continue("404 Not Found not found in: %s", res.Stderr())
		}
		return poll.Success()
	}

	// Check the jump to DOCKER-INGRESS is at the top of the DOCKER-FORWARD chain.
	checkChain := func() {
		t.Helper()
		res := icmd.RunCommand("iptables", "-S", "DOCKER-FORWARD")
		assert.NilError(t, res.Error, "stderr: %s", res.Stderr())
		// Only compare the first (fixed) part of the chain - per-bridge rules may be
		// re-ordered when the daemon restarts.
		out := strings.SplitAfter(res.Stdout(), "\n")
		if len(out) > 5 {
			out = out[:5]
		}
		golden.Assert(t, strings.Join(out, ""), t.Name()+"_docker_forward.golden")
	}

	l3.Hosts[l3SegHost].Do(t, func() {
		d := swarm.NewSwarm(ctx, t, testEnv, daemon.WithSwarmIptables(true))
		defer d.Stop(t)
		c := d.NewClientT(t)
		defer c.Close()

		serviceID := swarm.CreateService(ctx, t, d,
			swarm.ServiceWithName("test-dicp"),
			swarm.ServiceWithCommand([]string{"httpd", "-f"}),
			swarm.ServiceWithEndpoint(&swarmtypes.EndpointSpec{
				Ports: []swarmtypes.PortConfig{
					{
						Protocol:      "tcp",
						TargetPort:    80,
						PublishedPort: 8080,
						PublishMode:   swarmtypes.PortConfigPublishModeIngress,
					},
				},
			}),
		)
		defer func() {
			err := c.ServiceRemove(ctx, serviceID)
			assert.NilError(t, err)
		}()

		t.Log("Waiting for the service to start")
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, serviceID, 1), swarm.ServicePoll)
		t.Log("Checking http access to the service")
		poll.WaitOn(t, checkHTTP, poll.WithTimeout(30*time.Second))
		checkChain()

		t.Log("Restarting the daemon")
		d.Restart(t)

		t.Log("Waiting for the service to start")
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, serviceID, 1), swarm.ServicePoll)
		t.Log("Checking http access to the service")
		// It takes a while before this works ...
		poll.WaitOn(t, checkHTTP, poll.WithTimeout(30*time.Second))
		checkChain()
	})
}

func TestRestoreIngressRulesOnFirewalldReload(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	skip.If(t, testEnv.FirewallBackendDriver() != "iptables+firewalld", "nftables backend doesn't support Swarm-mode")
	skip.If(t, !networking.FirewalldRunning(), "Need firewalld to test restoration ingress rules")
	ctx := setupTest(t)

	// Check the published port is accessible.
	checkHTTP := func(_ poll.LogT) poll.Result {
		res := icmd.RunCommand("curl", "-v", "-o", "/dev/null", "-w", "%{http_code}\n",
			"http://"+stdnet.JoinHostPort("localhost", "8080"))
		// A "404 Not Found" means the server responded, but it's got nothing to serve.
		if !strings.Contains(res.Stdout(), "404") {
			return poll.Continue("404 - not found in: %s, %+v", res.Stdout(), res)
		}
		return poll.Success()
	}

	d := swarm.NewSwarm(ctx, t, testEnv, daemon.WithSwarmIptables(true))
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	serviceID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithName("test-ingress-on-firewalld-reload"),
		swarm.ServiceWithCommand([]string{"httpd", "-f"}),
		swarm.ServiceWithEndpoint(&swarmtypes.EndpointSpec{
			Ports: []swarmtypes.PortConfig{
				{
					Protocol:      "tcp",
					TargetPort:    80,
					PublishedPort: 8080,
					PublishMode:   swarmtypes.PortConfigPublishModeIngress,
				},
			},
		}),
	)
	defer func() {
		err := c.ServiceRemove(ctx, serviceID)
		assert.NilError(t, err)
	}()

	t.Log("Waiting for the service to start")
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, serviceID, 1), swarm.ServicePoll)
	t.Log("Checking http access to the service")
	poll.WaitOn(t, checkHTTP, poll.WithTimeout(30*time.Second))

	t.Log("Firewalld reload")
	networking.FirewalldReload(t, d)

	t.Log("Checking http access to the service")
	// It takes a while before this works ...
	poll.WaitOn(t, checkHTTP, poll.WithTimeout(30*time.Second))
}
