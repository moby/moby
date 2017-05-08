package service

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newRemoveCommand(dockerCli *client.DockerCli) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "rm [OPTIONS] SERVICE [SERVICE...]",
		Aliases: []string{"remove"},
		Short:   "Remove one or more services",
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(dockerCli, args)
		},
	}
	cmd.Flags()

	return cmd
}

func runRemove(dockerCli *client.DockerCli, sids []string) error {
	client := dockerCli.Client()

	ctx := context.Background()

	var errs []string
	for _, sid := range sids {
		err := client.ServiceRemove(ctx, sid)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		fmt.Fprintf(dockerCli.Out(), "%s\n", sid)
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "\n"))
	}
	return nil
}
