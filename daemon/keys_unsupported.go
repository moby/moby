// +build !linux

package daemon

// ModifyRootKeyLimit is an noop on unsupported platforms.
func ModifyRootKeyLimit() error {
	return nil
}
