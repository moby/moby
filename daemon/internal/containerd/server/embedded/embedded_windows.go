package embedded

import (
	"net"

	"github.com/Microsoft/go-winio"

	// Windows-specific containerd plugin registrations: the Windows and LCOW
	// snapshotters and differs. Cross-platform plugins are registered in
	// server.go.
	_ "github.com/containerd/containerd/v2/plugins/diff/lcow"
	_ "github.com/containerd/containerd/v2/plugins/diff/windows"
	_ "github.com/containerd/containerd/v2/plugins/snapshots/lcow"
	_ "github.com/containerd/containerd/v2/plugins/snapshots/windows"
)

func defaultAddress(string) string {
	return `\\.\pipe\docker-containerd-embedded`
}

func listen(address string) (net.Listener, error) {
	return winio.ListenPipe(address, nil)
}
