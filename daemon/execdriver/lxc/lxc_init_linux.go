package lxc

import (
	"fmt"
	"github.com/docker/libcontainer/namespaces"
	"github.com/docker/libcontainer/utils"
)

func finalizeNamespace(args *InitArgs) error {
	if err := utils.CloseExecFrom(3); err != nil {
		return err
	}

	if err := namespaces.SetupUser(args.User); err != nil {
		return fmt.Errorf("setup user %s", err)
	}

	if err := setupWorkingDirectory(args); err != nil {
		return err
	}

	return nil
}
