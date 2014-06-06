package namespaces

import (
	"os"
	"os/exec"

	"github.com/dotcloud/docker/pkg/libcontainer"
)

type CreateCommand func(container *libcontainer.Container, console, rootfs, dataPath, init string, childPipe *os.File, args []string) *exec.Cmd
