package image

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
)

type imagesOptions struct {
	matchName string

	quiet       bool
	all         bool
	noTrunc     bool
	showDigests bool
	format      string
	filter      opts.FilterOpt
}

// NewImagesCommand creates a new `docker images` command
func NewImagesCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := imagesOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "images [OPTIONS] [REPOSITORY[:TAG]]",
		Short: "List images",
		Args:  cli.RequiresMaxArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.matchName = args[0]
			}
			return runImages(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only show numeric IDs")
	flags.BoolVarP(&opts.all, "all", "a", false, "Show all images (default hides intermediate images)")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Don't truncate output")
	flags.BoolVar(&opts.showDigests, "digests", false, "Show digests")
	flags.StringVar(&opts.format, "format", "", "Pretty-print images using a Go template")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runImages(dockerCli *command.DockerCli, opts imagesOptions) error {
	ctx := context.Background()

	options := types.ImageListOptions{
		MatchName: opts.matchName,
		All:       opts.all,
		Filters:   opts.filter.Value(),
	}

	images, err := dockerCli.Client().ImageList(ctx, options)
	if err != nil {
		return err
	}

	f := opts.format
	if len(f) == 0 {
		if len(dockerCli.ConfigFile().ImagesFormat) > 0 && !opts.quiet {
			f = dockerCli.ConfigFile().ImagesFormat
		} else {
			f = "table"
		}
	}

	imagesCtx := formatter.ImageContext{
		Context: formatter.Context{
			Output: dockerCli.Out(),
			Format: f,
			Quiet:  opts.quiet,
			Trunc:  !opts.noTrunc,
		},
		Digest: opts.showDigests,
		Images: images,
	}

	imagesCtx.Write()

	return nil
}
