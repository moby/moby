//go:build !no_embedded_containerd

package embedded

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"github.com/containerd/ttrpc"

	// Windows-specific containerd plugin registrations. Cross-platform plugins
	// are registered in server.go.
	_ "github.com/containerd/containerd/v2/plugins/diff/lcow"
	_ "github.com/containerd/containerd/v2/plugins/diff/windows"
	_ "github.com/containerd/containerd/v2/plugins/snapshots/windows"
)

func defaultAddress(stateDir string) string {
	return `\\.\pipe\docker-containerd-embedded-` + stateDirID(stateDir)
}

// stateDirID returns a stable identifier for the daemon state directory.
func stateDirID(stateDir string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(stateDir)))
	return hex.EncodeToString(sum[:])
}

// namedPipePermissions grants full access only to built-in Administrators and
// LocalSystem.
const namedPipePermissions = "D:P(A;;GA;;;BA)(A;;GA;;;SY)"

func listen(address string) (net.Listener, error) {
	return winio.ListenPipe(address, &winio.PipeConfig{
		SecurityDescriptor: namedPipePermissions,
	})
}

func newTTRPCServer() (*ttrpc.Server, error) {
	return ttrpc.NewServer()
}
