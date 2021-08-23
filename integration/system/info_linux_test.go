//go:build !windows
// +build !windows

package system // import "github.com/docker/docker/integration/system"

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/docker/docker/testutil/daemon"
	req "github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestInfoBinaryCommits(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()

	info, err := client.Info(context.Background())
	assert.NilError(t, err)

	assert.Check(t, "N/A" != info.ContainerdCommit.ID)
	assert.Check(t, is.Equal(info.ContainerdCommit.Expected, info.ContainerdCommit.ID))

	assert.Check(t, "N/A" != info.InitCommit.ID)
	assert.Check(t, is.Equal(info.InitCommit.Expected, info.InitCommit.ID))

	assert.Check(t, "N/A" != info.RuncCommit.ID)
	assert.Check(t, is.Equal(info.RuncCommit.Expected, info.RuncCommit.ID))
}

func TestInfoAPIVersioned(t *testing.T) {
	// Windows only supports 1.25 or later

	res, body, err := req.Get("/v1.20/info")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusOK))

	b, err := req.ReadBody(body)
	assert.NilError(t, err)

	out := string(b)
	assert.Check(t, is.Contains(out, "ExecutionDriver"))
	assert.Check(t, is.Contains(out, "not supported"))
}

// TestInfoDiscoveryBackend verifies that a daemon run with `--cluster-advertise` and
// `--cluster-store` properly returns the backend's endpoint in info output.
func TestInfoDiscoveryBackend(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")

	const (
		discoveryBackend   = "consul://consuladdr:consulport/some/path"
		discoveryAdvertise = "1.1.1.1:2375"
	)

	d := daemon.New(t)
	d.Start(t, "--cluster-store="+discoveryBackend, "--cluster-advertise="+discoveryAdvertise)
	defer d.Stop(t)

	info := d.Info(t)
	assert.Equal(t, info.ClusterStore, discoveryBackend)
	assert.Equal(t, info.ClusterAdvertise, discoveryAdvertise)
}

// TestInfoDiscoveryInvalidAdvertise verifies that a daemon run with
// an invalid `--cluster-advertise` configuration
func TestInfoDiscoveryInvalidAdvertise(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	d := daemon.New(t)

	// --cluster-advertise with an invalid string is an error
	err := d.StartWithError("--cluster-store=consul://consuladdr:consulport/some/path", "--cluster-advertise=invalid")
	if err == nil {
		d.Stop(t)
	}
	assert.ErrorContains(t, err, "", "expected error when starting daemon")

	// --cluster-advertise without --cluster-store is also an error
	err = d.StartWithError("--cluster-advertise=1.1.1.1:2375")
	if err == nil {
		d.Stop(t)
	}
	assert.ErrorContains(t, err, "", "expected error when starting daemon")
}

// TestInfoDiscoveryAdvertiseInterfaceName verifies that a daemon run with `--cluster-advertise`
// configured with interface name properly show the advertise ip-address in info output.
func TestInfoDiscoveryAdvertiseInterfaceName(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")
	// TODO should we check for networking availability (integration-cli suite checks for networking through `Network()`)

	d := daemon.New(t)
	const (
		discoveryStore     = "consul://consuladdr:consulport/some/path"
		discoveryInterface = "eth0"
	)

	d.Start(t, "--cluster-store="+discoveryStore, fmt.Sprintf("--cluster-advertise=%s:2375", discoveryInterface))
	defer d.Stop(t)

	iface, err := net.InterfaceByName(discoveryInterface)
	assert.NilError(t, err)
	addrs, err := iface.Addrs()
	assert.NilError(t, err)
	assert.Assert(t, len(addrs) > 0)
	ip, _, err := net.ParseCIDR(addrs[0].String())
	assert.NilError(t, err)

	info := d.Info(t)
	assert.Equal(t, info.ClusterStore, discoveryStore)
	assert.Equal(t, info.ClusterAdvertise, ip.String()+":2375")
}
