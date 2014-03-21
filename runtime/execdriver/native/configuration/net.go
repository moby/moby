package configuration

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"os/exec"
	"path/filepath"
	"strings"
)

// i.e: net join <name>
func parseNetOpt(container *libcontainer.Container, running map[string]*exec.Cmd, opts []string) error {
	opt := strings.TrimSpace(opts[1])
	switch opt {
	case "join":
		var (
			id  = strings.TrimSpace(opts[2])
			cmd = running[id]
		)

		if cmd == nil || cmd.Process == nil {
			return fmt.Errorf("%s is not a valid running container to join", id)
		}
		nspath := filepath.Join("/proc", fmt.Sprint(cmd.Process.Pid), "ns", "net")
		container.Networks = append(container.Networks, &libcontainer.Network{
			Type: "netns",
			Context: libcontainer.Context{
				"nspath": nspath,
			},
		})
	default:
		return fmt.Errorf("%s is not a valid network option", opt)
	}
	return nil
}
