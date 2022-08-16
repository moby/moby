//go:build !linux
// +build !linux

package caps // import "github.com/docker/docker/oci/caps"

func initCaps() {
	// no capabilities on Windows
}
