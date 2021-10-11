//go:build linux && cgo && !static_build
// +build linux,cgo,!static_build

package devicemapper // import "github.com/docker/docker/pkg/devicemapper"

// #cgo pkg-config: devmapper
import "C"
