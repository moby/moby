// +build !linux

package daemon // import "github.com/moby/moby/daemon"

func ensureDefaultAppArmorProfile() error {
	return nil
}
