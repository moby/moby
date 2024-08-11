package daemon // import "github.com/docker/docker/integration/daemon"

import (
	"testing"

	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/icmd"
)

func TestDaemonDefaultBridgeWithFixedCidrButNoBip(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)

	bridgeName := "ext-bridge1"
	d := daemon.New(t, daemon.WithEnvVars("DOCKER_TEST_CREATE_DEFAULT_BRIDGE="+bridgeName))
	defer func() {
		d.Stop(t)
		d.Cleanup(t)
	}()

	defer func() {
		// No need to clean up when running this test in rootless mode, as the
		// interface is deleted when the daemon is stopped and the netns
		// reclaimed by the kernel.
		if !testEnv.IsRootless() {
			deleteInterface(t, bridgeName)
		}
	}()
	d.StartWithBusybox(ctx, t, "--bridge", bridgeName, "--fixed-cidr", "192.168.130.0/24")
}

func deleteInterface(t *testing.T, ifName string) {
	icmd.RunCommand("ip", "link", "delete", ifName).Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "-t", "nat", "--flush").Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "--flush").Assert(t, icmd.Success)
}
