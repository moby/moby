package swarm

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type leaveOptions struct {
	force bool
}

func newLeaveCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := leaveOptions{}

	cmd := &cobra.Command{
		Use:   "leave [OPTIONS]",
		Short: "Leave a swarm",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLeave(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.force, "force", false, "Force leave ignoring warnings.")
	return cmd
}

func runLeave(dockerCli *client.DockerCli, opts leaveOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	if err := client.SwarmLeave(ctx, opts.force); err != nil {
		return err
	}

	fmt.Fprintln(dockerCli.Out(), "Node left the swarm.")
	return nil
}
