//go:build !linux

package daemon // import "github.com/docker/docker/daemon"

// modifyRootKeyLimit is a noop on unsupported platforms.
func modifyRootKeyLimit() error {
	return nil
}
