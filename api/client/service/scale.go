package service

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

func newScaleCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "scale SERVICE=<SCALE> [SERVICE=<SCALE>...]",
		Short: "Scale one or multiple services",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScale(dockerCli, args)
		},
	}
}

func runScale(dockerCli *client.DockerCli, args []string) error {
	var errors []string
	for _, arg := range args {
		if err := runServiceScale(dockerCli, arg); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf(strings.Join(errors, "\n"))
}

func runServiceScale(dockerCli *client.DockerCli, arg string) error {
	var parts []string
	if parts = strings.SplitN(arg, "=", 2); len(parts) != 2 {
		return fmt.Errorf("%s: invalid scale specifier (expected \"<service>=<value>\")", arg)
	}

	flags := newUpdateCommand(dockerCli).Flags()
	if err := flags.Set("scale", parts[1]); err != nil {
		return fmt.Errorf("%s: invalid value %q for scale", parts[0], parts[1])
	}
	return runUpdate(dockerCli, flags, parts[0])
}
