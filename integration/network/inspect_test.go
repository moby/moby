package network

import (
	"net/netip"
	"strconv"
	"testing"

	networktypes "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/network"
	"github.com/moby/moby/v2/integration/internal/swarm"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestInspectNetwork(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support Swarm-mode")
	ctx := setupTest(t)

	var mgr [3]*daemon.Daemon
	mgr[0] = swarm.NewSwarm(ctx, t, testEnv, daemon.WithSwarmListenAddr("127.0.0.2"))
	defer mgr[0].Stop(t)

	for i := range mgr {
		if i != 0 {
			mgr[i] = daemon.New(t, daemon.WithSwarmListenAddr("127.0.0."+strconv.Itoa(i+2)))
			mgr[i].StartAndSwarmJoin(ctx, t, mgr[0], true)
			defer mgr[i].Stop(t)
		}
		t.Logf("Daemon %s is Swarm Node %s", mgr[i].ID(), mgr[i].NodeID())
	}

	c1 := mgr[0].NewClientT(t)
	defer c1.Close()

	worker1 := daemon.New(t, daemon.WithSwarmListenAddr("127.0.0."+strconv.Itoa(len(mgr)+2)))
	worker1.StartAndSwarmJoin(ctx, t, mgr[0], false)
	defer worker1.Stop(t)
	t.Logf("Daemon %s is Swarm Node %s", worker1.ID(), worker1.NodeID())

	w1 := worker1.NewClientT(t)
	defer w1.Close()

	networkName := "Overlay" + t.Name()
	cidrv4 := netip.MustParsePrefix("192.168.0.0/24")
	ipv4Range := netip.MustParsePrefix("192.168.0.0/25")

	overlayID := network.CreateNoError(ctx, t, c1, networkName,
		network.WithDriver("overlay"),
		network.WithIPAMConfig(networktypes.IPAMConfig{
			Subnet:  cidrv4,
			IPRange: ipv4Range,
		}),
	)
	// Other tests fail unless the network is removed, even though they run
	// on a new daemon. This is due to the vxlan link (the netlink kernel
	// object) leaking, which prevents other daemons on the same kernel from
	// creating a new vxlan link with the same VNI.
	defer func() {
		assert.NilError(t, c1.NetworkRemove(ctx, overlayID))
		poll.WaitOn(t, network.IsRemoved(ctx, w1, overlayID), swarm.NetworkPoll)
	}()

	const instances = 2
	serviceName := "TestService" + t.Name()

	serviceID := swarm.CreateService(ctx, t, mgr[0],
		swarm.ServiceWithReplicas(instances),
		swarm.ServiceWithPlacementConstraints("node.role == worker"),
		swarm.ServiceWithName(serviceName),
		swarm.ServiceWithNetwork(networkName),
	)
	defer func() {
		assert.NilError(t, c1.ServiceRemove(ctx, serviceID))
		poll.WaitOn(t, swarm.NoTasksForService(ctx, c1, serviceID), swarm.ServicePoll)
	}()

	poll.WaitOn(t, swarm.RunningTasksCount(ctx, c1, serviceID, instances), swarm.ServicePoll)

	tests := []struct {
		name    string
		network string
		opts    client.NetworkInspectOptions
	}{
		{
			name:    "full network id",
			network: overlayID,
			opts: client.NetworkInspectOptions{
				Verbose: true,
			},
		},
		{
			name:    "partial network id",
			network: overlayID[0:11],
			opts: client.NetworkInspectOptions{
				Verbose: true,
			},
		},
		{
			name:    "network name",
			network: networkName,
			opts: client.NetworkInspectOptions{
				Verbose: true,
			},
		},
		{
			name:    "network name and swarm scope",
			network: networkName,
			opts: client.NetworkInspectOptions{
				Verbose: true,
				Scope:   "swarm",
			},
		},
	}
	checkNetworkInspect := func(t *testing.T) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)

				for _, d := range append([]*daemon.Daemon{worker1}, mgr[:]...) {
					t.Logf("--- Node %s (%s) ---", d.ID(), d.NodeID())
					c := d.NewClientT(t)
					nw, err := c.NetworkInspect(ctx, tc.network, tc.opts)
					if !assert.Check(t, err) {
						continue
					}

					assert.Check(t, nw.IPAM.Config != nil)
					for _, cfg := range nw.IPAM.Config {
						assert.Assert(t, cfg.Gateway.IsValid())
						assert.Assert(t, cfg.Subnet.IsValid())
					}

					if d.CachedInfo.Swarm.ControlAvailable {
						// The global view of the network status is only available from manager nodes.
						if assert.Check(t, nw.Status != nil) {
							wantSubnetStatus := map[netip.Prefix]networktypes.SubnetStatus{
								cidrv4: {
									IPsInUse:            uint64(1 + instances + len(mgr) + 1),
									DynamicIPsAvailable: uint64(128 - (instances + len(mgr) + 1)),
								},
							}
							assert.Check(t, is.DeepEqual(wantSubnetStatus, nw.Status.IPAM.Subnets))
						}
					} else {
						// Services are only inspectable on nodes that have the network instantiated in
						// libnetwork, i.e. nodes with tasks attached to the network. In this test, only
						// the one worker node has tasks assigned.
						if assert.Check(t, is.Contains(nw.Services, serviceName)) {
							assert.Check(t, is.Len(nw.Services[serviceName].Tasks, instances))
						}
					}
					c.Close()
				}
			})
		}
	}

	t.Run("BeforeLeaderChange", checkNetworkInspect)

	leaderID := func() string {
		ls, err := c1.NodeList(ctx, client.NodeListOptions{
			Filters: make(client.Filters).Add("role", "manager"),
		})
		assert.NilError(t, err)
		for _, node := range ls {
			if node.ManagerStatus != nil && node.ManagerStatus.Leader {
				return node.ID
			}
		}
		t.Fatal("could not find current leader")
		return ""
	}

	t.Run("AfterLeaderChange", func(t *testing.T) {
		oldLeader := leaderID()
		var leader *daemon.Daemon
		for _, d := range mgr {
			if d.NodeID() == oldLeader {
				leader = d
				break
			}
		}
		assert.Assert(t, leader != nil)
		// Force a leader change
		for range 3 {
			leader.RestartNode(t)
			poll.WaitOn(t, swarm.HasLeader(ctx, c1), swarm.NetworkPoll)
			if leaderID() != oldLeader {
				break
			}
			t.Log("Restarting the node did not trigger a leader change")
		}
		assert.Assert(t, leaderID() != oldLeader, "leader did not change")

		checkNetworkInspect(t)
	})
}
