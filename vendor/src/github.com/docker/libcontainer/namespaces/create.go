package namespaces

import (
	"os"
	"os/exec"

	"github.com/docker/libcontainer"
)

type CreateCommand func(container *libcontainer.Container, console, rootfs, dataPath, init string, childPipe *os.File, args []string) *exec.Cmd
