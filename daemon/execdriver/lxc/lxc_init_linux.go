// +build amd64

package lxc

import (
	"fmt"
	"syscall"

	"github.com/docker/libcontainer/namespaces"
	"github.com/docker/libcontainer/security/capabilities"
	"github.com/docker/libcontainer/utils"
	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/daemon/execdriver/native/template"
	"github.com/dotcloud/docker/pkg/system"
)

func setHostname(hostname string) error {
	return syscall.Sethostname([]byte(hostname))
}

func finalizeNamespace(args *execdriver.InitArgs) error {
	if err := utils.CloseExecFrom(3); err != nil {
		return err
	}

	// We use the native drivers default template so that things like caps are consistent
	// across both drivers
	container := template.New()

	if !args.Privileged {
		// drop capabilities in bounding set before changing user
		if err := capabilities.DropBoundingSet(container); err != nil {
			return fmt.Errorf("drop bounding set %s", err)
		}

		// preserve existing capabilities while we change users
		if err := system.SetKeepCaps(); err != nil {
			return fmt.Errorf("set keep caps %s", err)
		}
	}

	if err := namespaces.SetupUser(args.User); err != nil {
		return fmt.Errorf("setup user %s", err)
	}

	if !args.Privileged {
		if err := system.ClearKeepCaps(); err != nil {
			return fmt.Errorf("clear keep caps %s", err)
		}

		// drop all other capabilities
		if err := capabilities.DropCapabilities(container); err != nil {
			return fmt.Errorf("drop capabilities %s", err)
		}
	}

	if err := setupWorkingDirectory(args); err != nil {
		return err
	}

	return nil
}
