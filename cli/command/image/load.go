package image

import (
	"io"
	"os"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/spf13/cobra"
)

type loadOptions struct {
	input string
	quiet bool
}

// NewLoadCommand creates a new `docker load` command
func NewLoadCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts loadOptions

	cmd := &cobra.Command{
		Use:   "load [OPTIONS]",
		Short: "Load an image from a tar archive or STDIN",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoad(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.input, "input", "i", "", "Read from tar archive file, instead of STDIN")
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Suppress the load output")

	return cmd
}

func runLoad(dockerCli *command.DockerCli, opts loadOptions) error {

	var input io.Reader = dockerCli.In()
	if opts.input != "" {
		file, err := os.Open(opts.input)
		if err != nil {
			return err
		}
		defer file.Close()
		input = file
	}
	if !dockerCli.Out().IsTerminal() {
		opts.quiet = true
	}
	response, err := dockerCli.Client().ImageLoad(context.Background(), input, opts.quiet)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.Body != nil && response.JSON {
		return jsonmessage.DisplayJSONMessagesToStream(response.Body, dockerCli.Out(), nil)
	}

	_, err = io.Copy(dockerCli.Out(), response.Body)
	return err
}
