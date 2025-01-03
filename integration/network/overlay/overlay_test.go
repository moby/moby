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
)

func TestEndpointWithCustomIfname(t *testing.T) {
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartAndSwarmInit(ctx, t)

	apiClient := d.NewClientT(t)

	// create a network specifying the desired sub-interface name
	netName := "overlay-custom-ifname"
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
