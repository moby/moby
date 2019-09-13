// +build !linux

package daemon // import "github.com/docker/docker/daemon"

import (
	"io"

	"github.com/pkg/errors"
)

func ensureDefaultAppArmorProfile() error {
	return nil
}

// PrintDefaultAppArmorProfile dumps the default profile to out
func PrintDefaultAppArmorProfile(out io.Writer) error {
	return errors.New("apparmor is unsupported in this build")
}
