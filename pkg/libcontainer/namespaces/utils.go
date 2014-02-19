package namespaces

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

// print and error to stderr and exit(1)
func writeError(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
	os.Exit(1)
}

// getNsFds inspects the container's namespace configuration and opens the fds to
// each of the namespaces.
func getNsFds(container *libcontainer.Container) ([]uintptr, error) {
	var (
		namespaces = []string{}
		fds        = []uintptr{}
	)

	for _, ns := range container.Namespaces {
		namespaces = append(namespaces, namespaceFileMap[ns])
	}

	for _, ns := range namespaces {
		fd, err := getNsFd(container.NsPid, ns)
		if err != nil {
			for _, fd = range fds {
				syscall.Close(int(fd))
			}
			return nil, err
		}
		fds = append(fds, fd)
	}
	return fds, nil
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

func setupUser(container *libcontainer.Container) error {
	// TODO: honor user passed on container
	if err := setgroups(nil); err != nil {
		return err
	}
	if err := setresgid(0, 0, 0); err != nil {
		return err
	}
	if err := setresuid(0, 0, 0); err != nil {
		return err
	}
	return nil
}

func getMasterAndConsole(container *libcontainer.Container) (string, *os.File, error) {
	master, err := openpmtx()
	if err != nil {
		return "", nil, err
	}

	console, err := ptsname(master)
	if err != nil {
		master.Close()
		return "", nil, err
	}
	return console, master, nil
}
