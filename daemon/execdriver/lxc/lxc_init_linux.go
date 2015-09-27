// +build linux

package lxc

import (
	derr "github.com/docker/docker/errors"
	"github.com/opencontainers/runc/libcontainer/utils"
)

func finalizeNamespace(args *InitArgs) error {
	if err := utils.CloseExecFrom(3); err != nil {
		return err
	}
	if err := setupUser(args.User); err != nil {
		return derr.ErrorCodeErrSetupUser.WithArgs(err)
	}
	if err := setupWorkingDirectory(args); err != nil {
		return err
	}
	return nil
}
