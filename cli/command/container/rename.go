package container

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type renameOptions struct {
	oldName string
	newName string
}

// NewRenameCommand creates a new cobra.Command for `docker rename`
func NewRenameCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts renameOptions

	cmd := &cobra.Command{
		Use:   "rename CONTAINER NEW_NAME",
		Short: "Rename a container",
		Args:  cli.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.oldName = args[0]
			opts.newName = args[1]
			return runRename(dockerCli, &opts)
		},
	}
	return cmd
}

func runRename(dockerCli *command.DockerCli, opts *renameOptions) error {
	ctx := context.Background()

	oldName := strings.TrimSpace(opts.oldName)
	newName := strings.TrimSpace(opts.newName)

	if oldName == "" || newName == "" {
		return fmt.Errorf("Error: Neither old nor new names may be empty")
	}

	if err := dockerCli.Client().ContainerRename(ctx, oldName, newName); err != nil {
		fmt.Fprintf(dockerCli.Err(), "%s\n", err)
		return fmt.Errorf("Error: failed to rename container named %s", oldName)
	}
	return nil
}
