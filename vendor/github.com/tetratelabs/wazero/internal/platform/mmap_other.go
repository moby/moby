// Separated from linux which has support for huge pages.

//go:build unix && !linux

package platform

import "golang.org/x/sys/unix"

func mmapCodeSegment(size int) ([]byte, error) {
	return unix.Mmap(
		-1,
		0,
		size,
		unix.PROT_READ|unix.PROT_WRITE,
		// Anonymous as this is not an actual file, but a memory,
		// Private as this is in-process memory region.
		unix.MAP_ANON|unix.MAP_PRIVATE,
	)
}
