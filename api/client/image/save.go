package image

import (
	"errors"
	"io"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type saveOptions struct {
	images []string
	output string
}

// NewSaveCommand creates a new `docker save` command
func NewSaveCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts saveOptions

	cmd := &cobra.Command{
		Use:   "save [OPTIONS] IMAGE [IMAGE...]",
		Short: "Save one or more images to a tar archive (streamed to STDOUT by default)",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.images = args
			return runSave(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.output, "output", "o", "", "Write to a file, instead of STDOUT")

	return cmd
}

func runSave(dockerCli *client.DockerCli, opts saveOptions) error {
	if opts.output == "" && dockerCli.IsTerminalOut() {
		return errors.New("Cowardly refusing to save to a terminal. Use the -o flag or redirect.")
	}

	responseBody, err := dockerCli.Client().ImageSave(context.Background(), opts.images)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if opts.output == "" {
		_, err := io.Copy(dockerCli.Out(), responseBody)
		return err
	}

	return client.CopyToFile(opts.output, responseBody)
}
