package container

import (
	"errors"
	"io"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type exportOptions struct {
	container string
	output    string
}

// NewExportCommand creates a new `docker export` command
func NewExportCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts exportOptions

	cmd := &cobra.Command{
		Use:   "export [OPTIONS] CONTAINER",
		Short: "Export a container's filesystem as a tar archive",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			return runExport(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.output, "output", "o", "", "Write to a file, instead of STDOUT")

	return cmd
}

func runExport(dockerCli *command.DockerCli, opts exportOptions) error {
	if opts.output == "" && dockerCli.Out().IsTerminal() {
		return errors.New("Cowardly refusing to save to a terminal. Use the -o flag or redirect.")
	}

	clnt := dockerCli.Client()

	responseBody, err := clnt.ContainerExport(context.Background(), opts.container)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if opts.output == "" {
		_, err := io.Copy(dockerCli.Out(), responseBody)
		return err
	}

	return command.CopyToFile(opts.output, responseBody)
}
