//go:build linux && cgo && !static_build
// +build linux,cgo,!static_build

package devicemapper // import "github.com/moby/moby/pkg/devicemapper"

// #cgo pkg-config: devmapper
import "C"
