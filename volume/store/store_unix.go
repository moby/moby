// +build linux freebsd

package store

// normalizeVolumeName is a platform specific function to normalize the name
// of a volume. This is a no-op on Unix-like platforms
func normalizeVolumeName(name string) string {
	return name
}
