//go:build !linux
// +build !linux

package caps // import "github.com/moby/moby/oci/caps"

func initCaps() {
	// no capabilities on Windows
}
