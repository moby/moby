// +build windows

package daemon

// Not supported on Windows
func copyOwnership(source, destination string) error {
	return nil
}
