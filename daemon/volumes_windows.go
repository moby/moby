// +build windows

package daemon

// Not supported on Windows
func copyOwnership(source, destination string) error {
	return nil
}

func (container *Container) prepareVolumes() error {
	return nil
}

func (container *Container) setupMounts() error {
	return nil
}
