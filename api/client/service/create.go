package service

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newCreateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := newServiceOptions()

	cmd := &cobra.Command{
		Use:   "create [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short: "Create a new service",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.image = args[0]
			if len(args) > 1 {
				opts.command = args[1:]
			}
			return runCreate(dockerCli, opts)
		},
	}
	addServiceFlags(cmd, opts)
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func runCreate(dockerCli *client.DockerCli, opts *serviceOptions) error {
	client := dockerCli.Client()
	response, err := client.ServiceCreate(context.Background(), opts.ToService())
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s\n", response.ID)
	return nil
}
