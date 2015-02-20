package libnetwork

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/docker/docker/pkg/reexec"
)

type ReexecCommand int

const (
	ReexecCreateNamespace ReexecCommand = iota
	ReexecMoveInterface
)

var ReexecCommands = map[ReexecCommand]struct {
	Key        string
	Entrypoint func()
}{
	ReexecCreateNamespace: {"netns-create", createNetworkNamespace},
	ReexecMoveInterface:   {"netns-moveif", namespaceMoveInterface},
}

func init() {
	for _, reexecCmd := range ReexecCommands {
		reexec.Register(reexecCmd.Key, reexecCmd.Entrypoint)
	}
}

func Reexec(command ReexecCommand, params ...string) error {
	reexecCommand, ok := ReexecCommands[command]
	if !ok {
		return fmt.Errorf("unknown reexec command %q", command)
	}

	cmd := &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{reexecCommand.Key}, params...),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return cmd.Run()
}
