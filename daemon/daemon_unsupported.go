// +build !linux,!freebsd,!windows

package daemon // import "github.com/moby/moby/daemon"
import "github.com/moby/moby/daemon/config"

const platformSupported = false

func setupResolvConf(config *config.Config) {
}
