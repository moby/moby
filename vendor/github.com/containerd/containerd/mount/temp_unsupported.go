// +build windows

package mount

// SetTempMountLocation sets the temporary mount location
func SetTempMountLocation(root string) error {
	return nil
}

// CleanupTempMounts all temp mounts and remove the directories
func CleanupTempMounts(flags int) error {
	return nil
}
