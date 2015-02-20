package libnetwork

import (
	"fmt"
	"os"
	"os/exec"

	dre "github.com/docker/docker/pkg/reexec"
)

type reexecCommand int

const (
	reexecCreateNamespace reexecCommand = iota
	reexecMoveInterface
)

var reexecCommands = map[reexecCommand]struct {
	Key        string
	Entrypoint func()
}{
	reexecCreateNamespace: {"netns-create", createNetworkNamespace},
	reexecMoveInterface:   {"netns-moveif", namespaceMoveInterface},
}

func init() {
	for _, reexecCmd := range reexecCommands {
		dre.Register(reexecCmd.Key, reexecCmd.Entrypoint)
	}
}

func reexec(command reexecCommand, params ...string) error {
	reexecCommand, ok := reexecCommands[command]
	if !ok {
		return fmt.Errorf("unknown reexec command %q", command)
	}

	cmd := &exec.Cmd{
		Path:   dre.Self(),
		Args:   append([]string{reexecCommand.Key}, params...),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return cmd.Run()
}
