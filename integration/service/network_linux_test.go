package service

import (
	"context"
	stdnet "net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/moby/api/types/network"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
	"github.com/moby/moby/v2/integration/internal/container"
	net "github.com/moby/moby/v2/integration/internal/network"
	"github.com/moby/moby/v2/integration/internal/swarm"
	"github.com/moby/moby/v2/integration/internal/testutils/networking"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDockerNetworkConnectAliasPreV144(t *testing.T) {
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv, daemon.WithEnvVars("DOCKER_MIN_API_VERSION=1.43"))
	defer d.Stop(t)
	apiClient := d.NewClientT(t, client.WithAPIVersion("1.43"))
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

	_, err := apiClient.NetworkConnect(ctx, name, client.NetworkConnectOptions{
		Container: cID1,
		EndpointConfig: &network.EndpointSettings{
			Aliases: []string{
				"aaa",
			},
		},
	})
	assert.NilError(t, err)

	_, err = apiClient.ContainerStart(ctx, cID1, client.ContainerStartOptions{})
	assert.NilError(t, err)

	ng1, err := apiClient.ContainerInspect(ctx, cID1, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(ng1.Container.NetworkSettings.Networks[name].Aliases), 2))
	assert.Check(t, is.Equal(ng1.Container.NetworkSettings.Networks[name].Aliases[0], "aaa"))

	cID2 := container.Create(ctx, t, apiClient, func(c *container.TestContainerConfig) {
		c.NetworkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				name: {},
			},
		}
	})

	_, err = apiClient.NetworkConnect(ctx, name, client.NetworkConnectOptions{
		Container: cID2,
		EndpointConfig: &network.EndpointSettings{
			Aliases: []string{
				"bbb",
			},
		},
	})
	assert.NilError(t, err)

	_, err = apiClient.ContainerStart(ctx, cID2, client.ContainerStartOptions{})
	assert.NilError(t, err)

	ng2, err := apiClient.ContainerInspect(ctx, cID2, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(ng2.Container.NetworkSettings.Networks[name].Aliases), 2))
	assert.Check(t, is.Equal(ng2.Container.NetworkSettings.Networks[name].Aliases[0], "bbb"))
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

	_, err := apiClient.NetworkConnect(ctx, name, client.NetworkConnectOptions{
		Container:      c1,
		EndpointConfig: &network.EndpointSettings{},
	})
	assert.NilError(t, err)

	_, err = apiClient.ContainerStart(ctx, c1, client.ContainerStartOptions{})
	assert.NilError(t, err)

	n1, err := apiClient.ContainerInspect(ctx, c1, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	_, err = apiClient.NetworkConnect(ctx, name, client.NetworkConnectOptions{
		Container:      c1,
		EndpointConfig: &network.EndpointSettings{},
	})
	assert.ErrorContains(t, err, "is already attached to network")

	n2, err := apiClient.ContainerInspect(ctx, c1, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(n1.Container, n2.Container, cmpopts.EquateComparable(netip.Addr{}, netip.Prefix{})))
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
		_, err := c.ServiceRemove(ctx, serviceID, client.ServiceRemoveOptions{})
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
			_, err := c.ServiceRemove(ctx, serviceID, client.ServiceRemoveOptions{})
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
		_, err := c.ServiceRemove(ctx, serviceID, client.ServiceRemoveOptions{})
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

// TestServiceVIPAcrossRollingUpdate checks that a service's VIP keeps carrying
// connections while the task behind it is replaced. Start-first ordering makes
// the replacement task run before the one it replaces stops, which is what
// leaves the load balancer holding both backends and having to keep the VIP
// serving through the handover.
func TestServiceVIPAcrossRollingUpdate(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	skip.If(t, strings.HasPrefix(testEnv.FirewallBackendDriver(), "nftables"), "swarm cannot be used with nftables")
	ctx := setupTest(t)

	d := swarm.NewSwarm(ctx, t, testEnv, daemon.WithSwarmIptables(true))
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	const netName = "svipu-net"
	netID := net.CreateNoError(ctx, t, c, netName,
		net.WithDriver("overlay"),
		// Attachable so a plain container can join and address the VIP.
		net.WithAttachable(),
	)

	serviceID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithName("test-svipu"),
		swarm.ServiceWithCommand([]string{"httpd", "-f"}),
		swarm.ServiceWithNetwork(netName),
		func(spec *swarmtypes.ServiceSpec) {
			// Start the replacement task before stopping the one it replaces, so the
			// load balancer holds both backends at once and has to keep the VIP
			// serving through the handover. Under the default stop-first order the
			// old backend is gone before the new one arrives, and the VIP is
			// briefly backed by nothing at all - a weaker thing to assert.
			spec.UpdateConfig = &swarmtypes.UpdateConfig{Order: swarmtypes.UpdateOrderStartFirst}
		},
	)
	defer func() {
		_, err := c.ServiceRemove(ctx, serviceID, client.ServiceRemoveOptions{})
		assert.NilError(t, err)
	}()
	t.Log("Waiting for the service to start")
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, serviceID, 1), swarm.ServicePoll)

	svc := getService(ctx, t, c, serviceID)
	var vip string
	for _, v := range svc.Endpoint.VirtualIPs {
		if v.NetworkID == netID {
			vip = v.Addr.Addr().String()
		}
	}
	assert.Assert(t, vip != "", "no VIP allocated on %s: %+v", netName, svc.Endpoint.VirtualIPs)
	t.Log("Service VIP is", vip)

	clientID := container.Run(ctx, t, c, container.WithNetworkMode(netName))
	defer c.ContainerRemove(ctx, clientID, client.ContainerRemoveOptions{Force: true})

	// The tasks run httpd with nothing to serve, so a "404 Not Found" is the proof
	// that the request reached one through the VIP - anything short of a task
	// answering is a connection error instead.
	checkVIP := func(_ poll.LogT) poll.Result {
		res, err := container.Exec(ctx, c, clientID,
			[]string{"wget", "-T1", "-t1", "-O-", "http://" + vip + "/"})
		if err != nil {
			return poll.Error(err)
		}
		if !strings.Contains(res.Combined(), "404 Not Found") {
			return poll.Continue("404 Not Found not found in: %s", res.Combined())
		}
		return poll.Success()
	}

	t.Log("Checking VIP access before the update")
	poll.WaitOn(t, checkVIP, poll.WithTimeout(30*time.Second))

	// Roll the task without changing anything else about the service. Nothing here
	// depends on what triggered the rollout, only that one task replaces another
	// under the VIP.
	t.Log("Forcing a rolling update")
	svc.Spec.TaskTemplate.ForceUpdate++
	_, err := c.ServiceUpdate(ctx, serviceID, client.ServiceUpdateOptions{
		Version: svc.Version,
		Spec:    svc.Spec,
	})
	assert.NilError(t, err)
	poll.WaitOn(t, serviceIsUpdated(ctx, c, serviceID), swarm.ServicePoll)
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, serviceID, 1), swarm.ServicePoll)

	// Back to one running task, and it is not the one that answered above. The VIP
	// has to resolve to the replacement.
	t.Log("Checking VIP access after the update")
	poll.WaitOn(t, checkVIP, poll.WithTimeout(30*time.Second))
}

// checkIngressPort returns a poll check that a service's published port is
// accessible from host. The tasks run httpd with nothing to serve, so a "404 Not
// Found" is the proof that the request reached one - anything short of a task
// answering is a connection error instead.
func checkIngressPort(t *testing.T, host networking.Host, hostAddr, port string) func(poll.LogT) poll.Result {
	return func(_ poll.LogT) poll.Result {
		var res *icmd.Result
		// This is called from inside a "Do()" thread in the docker host's netns, but it
		// uses poll.WaitOn - which runs the command in a different goroutine.
		host.Do(t, func() {
			res = icmd.RunCommand("wget", "-T1", "-t1", "-O-",
				"http://"+stdnet.JoinHostPort(hostAddr, port))
		})
		if !strings.Contains(res.Stderr(), "404 Not Found") {
			return poll.Continue("404 Not Found not found in: %s", res.Stderr())
		}
		return poll.Success()
	}
}

// ingressPortSpec is an endpoint spec publishing each of published on the ingress
// network, every one of them mapped to target port 80.
func ingressPortSpec(published ...uint32) *swarmtypes.EndpointSpec {
	spec := &swarmtypes.EndpointSpec{}
	for _, p := range published {
		spec.Ports = append(spec.Ports, swarmtypes.PortConfig{
			Protocol:      "tcp",
			TargetPort:    80,
			PublishedPort: p,
			PublishMode:   swarmtypes.PortConfigPublishModeIngress,
		})
	}
	return spec
}

// createIngressService creates a service publishing endpoint's ports, and waits
// for replicas of its tasks to be running.
func createIngressService(ctx context.Context, t *testing.T, d *daemon.Daemon, c *client.Client, name string, replicas uint64, endpoint *swarmtypes.EndpointSpec) string {
	t.Helper()
	id := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithName(name),
		swarm.ServiceWithCommand([]string{"httpd", "-f"}),
		swarm.ServiceWithEndpoint(endpoint),
		swarm.ServiceWithReplicas(replicas),
		func(spec *swarmtypes.ServiceSpec) {
			// Start the replacement task before stopping the one it replaces, so that an
			// update to the published-port set really does have both generations of the
			// service - and so both claims on a port they share - bound at once. Under
			// the default stop-first order the old binding can be gone before the new
			// one arrives, leaving nothing shared to get wrong.
			spec.UpdateConfig = &swarmtypes.UpdateConfig{Order: swarmtypes.UpdateOrderStartFirst}
		},
	)
	t.Log("Waiting for service", name, "to start")
	poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, id, replicas), swarm.ServicePoll)
	return id
}

// TestIngressPortsAcrossServiceUpdate follows a service's host-published ingress
// ports through the two events that change which host-port reservations it holds:
// a published-port change, which leaves two generations of the service alive at
// once, and the service's removal, which has to release the reservations for good.
//
// Those reservations are ref-counted, and both ways of getting the count wrong are
// silent. Releasing too eagerly takes down a port the surviving generation is
// still serving; not releasing at all leaves the count non-zero, and the next
// service to publish that port is told there is nothing to plumb and never becomes
// reachable. Neither shows up as an error from any API call, which is why this
// drives the ports over the wire rather than inspecting the daemon's bookkeeping.
func TestIngressPortsAcrossServiceUpdate(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	skip.If(t, testEnv.FirewallBackendDriver() == "nftables")
	skip.If(t, networking.FirewalldRunning(), "can't use firewalld in host netns to add rules in L3Segment")
	ctx := setupTest(t)

	// Run the test in its own netns, to avoid interfering with iptables on the test
	// host - and so that the host ports it publishes are its own.
	const hostAddr = "192.168.112.222"
	const l3SegHost = "ipsu"
	l3 := networking.NewL3Segment(t, "test-"+l3SegHost)
	defer l3.Destroy(t)
	l3.AddHost(t, l3SegHost, "ns-"+l3SegHost, "eth0", netip.MustParsePrefix(hostAddr+"/24"))

	checkHTTP := func(port string) func(poll.LogT) poll.Result {
		return checkIngressPort(t, l3.Hosts[l3SegHost], hostAddr, port)
	}

	l3.Hosts[l3SegHost].Do(t, func() {
		d := swarm.NewSwarm(ctx, t, testEnv, daemon.WithSwarmIptables(true))
		defer d.Stop(t)
		c := d.NewClientT(t)
		defer c.Close()

		createService := func(name string, endpoint *swarmtypes.EndpointSpec) string {
			return createIngressService(ctx, t, d, c, name, 1, endpoint)
		}

		serviceID := createService("test-"+l3SegHost, ingressPortSpec(8080))
		t.Log("Checking http access on the initial published port")
		poll.WaitOn(t, checkHTTP("8080"), poll.WithTimeout(30*time.Second))

		// Publish a second port. The service's port set is part of its identity in
		// libnetwork, so two services are bound on the ingress network, both
		// publishing 8080, for as long as the rollout takes. Only one of them may
		// hold that reservation, and the old one draining must not release it.
		t.Log("Adding a published port")
		svc := getService(ctx, t, c, serviceID)
		svc.Spec.EndpointSpec = ingressPortSpec(8080, 9090)
		_, err := c.ServiceUpdate(ctx, serviceID, client.ServiceUpdateOptions{
			Version: svc.Version,
			Spec:    svc.Spec,
		})
		assert.NilError(t, err)
		poll.WaitOn(t, serviceIsUpdated(ctx, c, serviceID), swarm.ServicePoll)
		poll.WaitOn(t, swarm.RunningTasksCount(ctx, c, serviceID, 1), swarm.ServicePoll)

		t.Log("Checking http access on the added published port")
		poll.WaitOn(t, checkHTTP("9090"), poll.WithTimeout(30*time.Second))
		t.Log("Checking http access on the port that spanned the update")
		poll.WaitOn(t, checkHTTP("8080"), poll.WithTimeout(30*time.Second))

		// Remove the service, then have a fresh one publish the same two ports. It
		// can only reach them if the first service released both its reservation and
		// the underlying host-port binding, so this stands in for the unpublishing
		// that has no observable output of its own.
		t.Log("Removing the service")
		_, err = c.ServiceRemove(ctx, serviceID, client.ServiceRemoveOptions{})
		assert.NilError(t, err)
		poll.WaitOn(t, swarm.NoTasksForService(ctx, c, serviceID), swarm.ServicePoll)

		successor := createService("test-"+l3SegHost+"-2", ingressPortSpec(8080, 9090))
		defer func() {
			_, err := c.ServiceRemove(ctx, successor, client.ServiceRemoveOptions{})
			assert.NilError(t, err)
		}()
		t.Log("Checking http access to the successor service's published ports")
		poll.WaitOn(t, checkHTTP("8080"), poll.WithTimeout(30*time.Second))
		poll.WaitOn(t, checkHTTP("9090"), poll.WithTimeout(30*time.Second))
	})
}
