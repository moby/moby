// +build windows

package daemon

// copyOwnership copies the permissions and uid:gid of the source file
// into the destination file
func copyOwnership(source, destination string) error {
	return nil
}
