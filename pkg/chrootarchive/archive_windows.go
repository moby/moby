package chrootarchive

// chroot is not supported by Windows
func chroot(path string) error {
	return nil
}
