// +build !linux,!windows

package daemon // import "github.com/docker/docker/daemon"

func configsSupported() bool {
	return false
}
