//go:build linux && cgo && !static_build && journald && journald_compat
// +build linux,cgo,!static_build,journald,journald_compat

package journald // import "github.com/moby/moby/daemon/logger/journald"

// #cgo pkg-config: libsystemd-journal
import "C"
