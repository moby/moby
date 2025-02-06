//go:build !windows

package overlay // import "github.com/docker/docker/integration/network/overlay"

import (
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/container"
	net "github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
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
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithDriver("overlay"),
		net.WithAttachable())

	ctrID := container.Run(ctx, t, apiClient,
		container.WithCmd("ip", "-o", "link", "show", "foobar"),
		container.WithEndpointSettings(netName, &network.EndpointSettings{
			DriverOpts: map[string]string{
				netlabel.Ifname: "foobar",
			},
		}))
	defer container.Remove(ctx, t, apiClient, ctrID, containertypes.RemoveOptions{Force: true})

	out, err := container.Output(ctx, apiClient, ctrID)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(out.Stdout, ": foobar@if"), "expected ': foobar@if' in 'ip link show':\n%s", out.Stdout)
}
