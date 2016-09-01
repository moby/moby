package image

import (
	"errors"
	"io"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/spf13/cobra"
)

type saveOptions struct {
	images []string
	output string
	format string
	refs   []string
}

// NewSaveCommand creates a new `docker save` command
func NewSaveCommand(dockerCli *command.DockerCli) *cobra.Command {
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
	flags.StringVarP(&opts.format, "format", "f", "", "Specify the format of the output tar archive")
	flags.StringSliceVar(&opts.refs, "ref", []string{}, "References to use when loading an OCI image layout tar archive")

	return cmd
}

func runSave(dockerCli *command.DockerCli, opts saveOptions) error {
	if opts.output == "" && dockerCli.Out().IsTerminal() {
		return errors.New("Cowardly refusing to save to a terminal. Use the -o flag or redirect.")
	}

	imageSaveOpts := types.ImageSaveOptions{
		Format: opts.format,
		Refs:   runconfigopts.ConvertKVStringsToMap(opts.refs),
	}

	responseBody, err := dockerCli.Client().ImageSave(context.Background(), opts.images, imageSaveOpts)
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
