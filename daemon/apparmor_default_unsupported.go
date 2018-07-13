// +build !linux

package daemon // import "github.com/docker/docker/daemon"

func ensureDefaultAppArmorProfile() error {
	return nil
}
