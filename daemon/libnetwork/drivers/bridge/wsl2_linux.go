package bridge

import (
	"context"
	"os"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

// Path to the executable installed in Linux under WSL2 that reports on
// WSL config. https://github.com/microsoft/WSL/releases/tag/2.0.4
// Can be modified by tests.
var wslinfoPath = "/usr/bin/wslinfo"

// isRunningUnderWSL2MirroredMode returns true if the host Linux appears to be
// running under Windows WSL2 with mirrored mode networking. If a loopback0
// device exists, and there's an executable at /usr/bin/wslinfo, infer that
// this is WSL2 with mirrored networking. ("wslinfo --networking-mode" reports
// "mirrored", but applying the workaround for WSL2's loopback device when it's
// not needed is low risk, compared with executing wslinfo with dockerd's
// elevated permissions.)
func isRunningUnderWSL2MirroredMode(ctx context.Context) bool {
	if _, err := nlwrap.LinkByName("loopback0"); err != nil {
		if !errors.As(err, &netlink.LinkNotFoundError{}) {
			log.G(ctx).WithError(err).Warn("Failed to check for WSL interface")
		}
		return false
	}
	stat, err := os.Stat(wslinfoPath)
	if err != nil {
		return false
	}
	return stat.Mode().IsRegular() && (stat.Mode().Perm()&0o111) != 0
}
