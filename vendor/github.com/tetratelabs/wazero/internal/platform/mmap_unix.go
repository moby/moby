//go:build unix

package platform

import "golang.org/x/sys/unix"

func munmapCodeSegment(code []byte) error {
	return unix.Munmap(code)
}

// MprotectCodeSegment is like unix.Mprotect with RX permission.
func MprotectCodeSegment(b []byte) (err error) {
	return unix.Mprotect(b, unix.PROT_READ|unix.PROT_EXEC)
}
