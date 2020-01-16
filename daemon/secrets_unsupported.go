// +build !linux,!windows

package daemon // import "github.com/moby/moby/daemon"

func secretsSupported() bool {
	return false
}
