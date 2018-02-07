// +build !linux,!windows

package daemon // import "github.com/docker/docker/daemon"

func secretsSupported() bool {
	return false
}
