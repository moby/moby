package swarm

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"golang.org/x/net/context"
)

func newUnlockCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock swarm",
		Args:  cli.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := dockerCli.Client()
			ctx := context.Background()

			key, err := readKey(dockerCli.In(), "Please enter unlock key: ")
			if err != nil {
				return err
			}
			req := swarm.UnlockRequest{
				UnlockKey: key,
			}

			return client.SwarmUnlock(ctx, req)
		},
	}

	return cmd
}

func readKey(in *command.InStream, prompt string) (string, error) {
	if in.IsTerminal() {
		fmt.Print(prompt)
		dt, err := terminal.ReadPassword(int(in.FD()))
		fmt.Println()
		return string(dt), err
	}
	key, err := bufio.NewReader(in).ReadString('\n')
	if err == io.EOF {
		err = nil
	}
	return strings.TrimSpace(key), err
}
