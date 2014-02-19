package namespaces

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func addEnvIfNotSet(container *libcontainer.Container, key, value string) {
	jv := fmt.Sprintf("%s=%s", key, value)
	if len(container.Command.Env) == 0 {
		container.Command.Env = []string{jv}
		return
	}

	for _, v := range container.Command.Env {
		parts := strings.Split(v, "=")
		if parts[0] == key {
			return
		}
	}
	container.Command.Env = append(container.Command.Env, jv)
}

// getNsFd returns the fd for a specific pid and namespace option
func getNsFd(pid int, ns string) (uintptr, error) {
	nspath := filepath.Join("/proc", strconv.Itoa(pid), "ns", ns)
	// OpenFile adds closOnExec
	f, err := os.OpenFile(nspath, os.O_RDONLY, 0666)
	if err != nil {
		return 0, err
	}
	return f.Fd(), nil
}

// setupEnvironment adds additional environment variables to the container's
// Command such as USER, LOGNAME, container, and TERM
func setupEnvironment(container *libcontainer.Container) {
	addEnvIfNotSet(container, "container", "docker")
	// TODO: check if pty
	addEnvIfNotSet(container, "TERM", "xterm")
	// TODO: get username from container
	addEnvIfNotSet(container, "USER", "root")
	addEnvIfNotSet(container, "LOGNAME", "root")
}
