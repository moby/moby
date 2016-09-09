package service

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newCreateCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := newServiceOptions()

	cmd := &cobra.Command{
		Use:   "create [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short: "Create a new service",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.image = args[0]
			if len(args) > 1 {
				opts.args = args[1:]
			}
			return runCreate(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.mode, flagMode, "replicated", "Service mode (replicated or global)")
	addServiceFlags(cmd, opts)

	flags.VarP(&opts.labels, flagLabel, "l", "Service labels")
	flags.Var(&opts.containerLabels, flagContainerLabel, "Container labels")
	flags.VarP(&opts.env, flagEnv, "e", "Set environment variables")
	flags.Var(&opts.mounts, flagMount, "Attach a mount to the service")
	flags.StringSliceVar(&opts.constraints, flagConstraint, []string{}, "Placement constraints")
	flags.StringSliceVar(&opts.networks, flagNetwork, []string{}, "Network attachments")
	flags.VarP(&opts.endpoint.ports, flagPublish, "p", "Publish a port as a node port")

	flags.SetInterspersed(false)
	return cmd
}

func runCreate(dockerCli *command.DockerCli, opts *serviceOptions) error {
	apiClient := dockerCli.Client()
	createOpts := types.ServiceCreateOptions{}

	service, err := opts.ToService()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// only send auth if flag was set
	if opts.registryAuth {
		// Retrieve encoded auth token from the image reference
		encodedAuth, err := command.RetrieveAuthTokenFromImage(ctx, dockerCli, opts.image)
		if err != nil {
			return err
		}
		createOpts.EncodedRegistryAuth = encodedAuth
	}

	response, err := apiClient.ServiceCreate(ctx, service, createOpts)
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s\n", response.ID)
	return nil
}
