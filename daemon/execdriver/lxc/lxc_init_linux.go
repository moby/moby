// +build amd64

package lxc

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/docker/libcontainer/namespaces"
	"github.com/docker/libcontainer/security/capabilities"
	"github.com/docker/libcontainer/utils"
	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/daemon/execdriver/native/template"
	"github.com/dotcloud/docker/pkg/system"
	utils2 "github.com/dotcloud/docker/utils"
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
		if err := capabilities.DropBoundingSet(container.Capabilities); err != nil {
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

		var caps []string
		for _, cap := range container.Capabilities {
			if !utils2.StringsContains(strings.Split(args.CapDrop, " "), cap) {
				caps = append(caps, cap)
			}
		}

		for _, cap := range strings.Split(args.CapAdd, " ") {
			if !utils2.StringsContains(caps, cap) {
				caps = append(caps, cap)
			}
		}

		// drop all other capabilities
		if err := capabilities.DropCapabilities(caps); err != nil {
			return fmt.Errorf("drop capabilities %s", err)
		}
	}

	if err := setupWorkingDirectory(args); err != nil {
		return err
	}

	return nil
}
