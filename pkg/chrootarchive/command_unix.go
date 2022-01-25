//go:build !windows
// +build !windows

package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import (
	"os/exec"
)

const archiveCmdName = "docker-chrootarchive"

func command(args ...string) *exec.Cmd {
	c := exec.Command(archiveCmdName)
	c.Args = args
	c.SysProcAttr = sysProcAttr()
	return c
}
