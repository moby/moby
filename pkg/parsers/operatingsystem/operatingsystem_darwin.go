package operatingsystem

// GetOperatingSystem gets the name of the current operating system.
func GetOperatingSystem() (string, error) {
	return "Darwin", nil
}

// IsContainerized returns true if we are running inside a container.
// No-op on Darwin, always returns false.
func IsContainerized() (bool, error) {
	return false, nil
}
