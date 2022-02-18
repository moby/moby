//go:build !linux && !windows
// +build !linux,!windows

package daemon // import "github.com/moby/moby/daemon"

func configsSupported() bool {
	return false
}
