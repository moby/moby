// +build experimental

package stack

import (
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/bundlefile"
	"github.com/spf13/cobra"
)

type configOptions struct {
	bundlefile string
	namespace  string
}

func newConfigCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts configOptions

	cmd := &cobra.Command{
		Use:   "config [OPTIONS] STACK",
		Short: "Print the stack configuration",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.namespace = args[0]
			return runConfig(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	addBundlefileFlag(&opts.bundlefile, flags)
	return cmd
}

func runConfig(dockerCli *command.DockerCli, opts configOptions) error {
	bundle, err := loadBundlefile(dockerCli.Err(), opts.namespace, opts.bundlefile)
	if err != nil {
		return err
	}
	return bundlefile.Print(dockerCli.Out(), bundle)
}
