//go:build solaris && !tinygo

package platform

const noopMprotectRX = true

func MprotectRX(b []byte) error {
	// Assume we already called mmap with at least RX.
	return nil
}
