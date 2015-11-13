package daemon

// checkIfPathIsInAVolume checks if the path is in a volume. If it is, it
// cannot be in a read-only volume. If it  is not in a volume, the container
// cannot be configured with a read-only rootfs.
//
// This is a no-op on Windows which does not support read-only volumes, or
// extracting to a mount point inside a volume. TODO Windows: FIXME Post-TP4
func checkIfPathIsInAVolume(container *Container, absPath string) (bool, error) {
	return false, nil
}
