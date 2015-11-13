package daemon

// platformSpecificRm is a platform-specific helper function for rm/delete.
// It is a no-op on Windows
func platformSpecificRm(container *Container) {
}
