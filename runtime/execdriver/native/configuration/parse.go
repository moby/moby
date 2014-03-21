package configuration

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"os/exec"
	"strings"
)

// configureCustomOptions takes string commands from the user and allows modification of the
// container's default configuration.
//
// format: <key> <...value>
// i.e: cgroup devices.allow *:*
func ParseConfiguration(container *libcontainer.Container, running map[string]*exec.Cmd, opts []string) error {
	for _, opt := range opts {
		var (
			err   error
			parts = strings.Split(strings.TrimSpace(opt), " ")
		)

		switch parts[0] {
		case "cap":
			err = parseCapOpt(container, parts[1:])
		case "ns":
			err = parseNsOpt(container, parts[1:])
		case "net":
			err = parseNetOpt(container, running, parts[1:])
		default:
			return fmt.Errorf("%s is not a valid configuration option for the native driver", parts[0])
		}
		if err != nil {
			return err
		}
	}
	return nil
}
