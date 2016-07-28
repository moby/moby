package service

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type scaleOptions struct {
	args             []string
	sendRegistryAuth bool
}

func newScaleCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := scaleOptions{}

	cmd := &cobra.Command{
		Use:   "scale [OPTIONS] SERVICE=REPLICAS [SERVICE=REPLICAS...]",
		Short: "Scale one or multiple services",
		Args:  scaleArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.args = args
			return runScale(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.sendRegistryAuth, flagRegistryAuth, false, "Send registry authentication details to Swarm agents")
	return cmd
}

func scaleArgs(cmd *cobra.Command, args []string) error {
	if err := cli.RequiresMinArgs(1)(cmd, args); err != nil {
		return err
	}
	for _, arg := range args {
		if parts := strings.SplitN(arg, "=", 2); len(parts) != 2 {
			return fmt.Errorf(
				"Invalid scale specifier '%s'.\nSee '%s --help'.\n\nUsage:  %s\n\n%s",
				arg,
				cmd.CommandPath(),
				cmd.UseLine(),
				cmd.Short,
			)
		}
	}
	return nil
}

func runScale(dockerCli *client.DockerCli, opts scaleOptions) error {
	var errors []string
	for _, arg := range opts.args {
		parts := strings.SplitN(arg, "=", 2)
		serviceID, scale := parts[0], parts[1]
		if err := runServiceScale(dockerCli, serviceID, scale, opts.sendRegistryAuth); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", serviceID, err.Error()))
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf(strings.Join(errors, "\n"))
}

func runServiceScale(dockerCli *client.DockerCli, serviceID string, scale string, sendAuth bool) error {
	client := dockerCli.Client()
	ctx := context.Background()
	updateOpts := types.ServiceUpdateOptions{}

	service, _, err := client.ServiceInspectWithRaw(ctx, serviceID)

	if err != nil {
		return err
	}

	serviceMode := &service.Spec.Mode
	if serviceMode.Replicated == nil {
		return fmt.Errorf("scale can only be used with replicated mode")
	}
	uintScale, err := strconv.ParseUint(scale, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid replicas value %s: %s", scale, err.Error())
	}
	serviceMode.Replicated.Replicas = &uintScale

	if sendAuth {
		// Retrieve encoded auth token from the image reference
		image := service.Spec.TaskTemplate.ContainerSpec.Image
		encodedAuth, err := dockerCli.RetrieveAuthTokenFromImage(ctx, image)
		if err != nil {
			return err
		}
		updateOpts.EncodedRegistryAuth = encodedAuth
	}

	err = client.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, updateOpts)
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s scaled to %s\n", serviceID, scale)
	return nil
}
