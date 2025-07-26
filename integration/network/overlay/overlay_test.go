//go:build !windows

package overlay

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	networktypes "github.com/moby/moby/api/types/network"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/integration/internal/network"
	"github.com/moby/moby/v2/integration/internal/swarm"
	"github.com/moby/moby/v2/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestEndpointWithCustomIfname(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support overlay networks")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartAndSwarmInit(ctx, t)
	defer d.Stop(t)
	defer d.SwarmLeave(ctx, t, true)

	apiClient := d.NewClientT(t)

	// create a network specifying the desired sub-interface name
	const netName = "overlay-custom-ifname"
	network.CreateNoError(ctx, t, apiClient, netName,
		network.WithDriver("overlay"),
		network.WithAttachable())

	ctrID := container.Run(ctx, t, apiClient,
		container.WithCmd("ip", "-o", "link", "show", "foobar"),
		container.WithEndpointSettings(netName, &networktypes.EndpointSettings{
			DriverOpts: map[string]string{
				netlabel.Ifname: "foobar",
			},
		}))
	defer container.Remove(ctx, t, apiClient, ctrID, containertypes.RemoveOptions{Force: true})

	out, err := container.Output(ctx, apiClient, ctrID)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(out.Stdout, ": foobar@if"), "expected ': foobar@if' in 'ip link show':\n%s", out.Stdout)
}

// TestHostPortMappings checks that, when a Swarm task has ports mappings in
// host mode, the ContainerList API endpoint reports the right number of ports.
//
// Regression test for https://github.com/moby/moby/issues/49719
func TestHostPortMappings(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "rootless mode doesn't support overlay networks")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartNodeWithBusybox(ctx, t)
	defer d.Stop(t)

	d.SwarmInit(ctx, t, swarmtypes.InitRequest{AdvertiseAddr: "127.0.0.1:2377"})
	defer d.SwarmLeave(ctx, t, true)

	apiClient := d.NewClientT(t)

	const netName = "testnet1"
	network.CreateNoError(ctx, t, apiClient, netName,
		network.WithDriver("overlay"),
		network.WithAttachable())

	svcID := swarm.CreateService(ctx, t, d,
		swarm.ServiceWithNetwork(netName),
		swarm.ServiceWithEndpoint(&swarmtypes.EndpointSpec{
			Ports: []swarmtypes.PortConfig{
				{Protocol: swarmtypes.PortConfigProtocolTCP, TargetPort: 80, PublishedPort: 80, PublishMode: swarmtypes.PortConfigPublishModeHost},
			},
		}))
	defer apiClient.ServiceRemove(ctx, svcID)

	poll.WaitOn(t, swarm.RunningTasksCount(ctx, apiClient, svcID, 1), swarm.ServicePoll)

	ctrs, err := apiClient.ContainerList(ctx, containertypes.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 1, len(ctrs))

	var addrs []string
	for _, port := range ctrs[0].Ports {
		addrs = append(addrs, fmt.Sprintf("%s:%d/%s", net.JoinHostPort(port.IP, strconv.Itoa(int(port.PublicPort))), port.PrivatePort, port.Type))
	}

	assert.Check(t, len(addrs) >= 1 && len(addrs) <= 2)

	exp := []string{"0.0.0.0:80:80/tcp"}
	if len(addrs) > 1 {
		exp = append(exp, "[::]:80:80/tcp")
	}

	slices.Sort(addrs)
	assert.DeepEqual(t, exp, addrs)
}
