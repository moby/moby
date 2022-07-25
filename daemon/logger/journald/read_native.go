//go:build linux && cgo && !static_build && journald
// +build linux,cgo,!static_build,journald

package journald // import "github.com/docker/docker/daemon/logger/journald"

// #cgo pkg-config: libsystemd
import "C"
