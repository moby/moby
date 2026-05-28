//go:build !linux

package daemon

// modifyRootKeyLimit is a noop on unsupported platforms.
func modifyRootKeyLimit() error {
	return nil
}
