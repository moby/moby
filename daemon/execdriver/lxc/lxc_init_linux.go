package lxc

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/daemon/execdriver/native/template"
	"github.com/docker/libcontainer/namespaces"
	"github.com/docker/libcontainer/security/capabilities"
	"github.com/docker/libcontainer/system"
	"github.com/docker/libcontainer/utils"
)

func setHostname(hostname string) error {
	return syscall.Sethostname([]byte(hostname))
}

func finalizeNamespace(args *InitArgs) error {
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

		var (
			adds  []string
			drops []string
		)

		if args.CapAdd != "" {
			adds = strings.Split(args.CapAdd, ":")
		}
		if args.CapDrop != "" {
			drops = strings.Split(args.CapDrop, ":")
		}

		caps, err := execdriver.TweakCapabilities(container.Capabilities, adds, drops)
		if err != nil {
			return err
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
