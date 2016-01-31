package client

import (
	"github.com/docker/docker/api/client/formatter"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
)

// CmdImages lists the images in a specified repository, or all top-level images if no repository is specified.
//
// Usage: docker images [OPTIONS] [REPOSITORY]
func (cli *DockerCli) CmdImages(args ...string) error {
	cmd := Cli.Subcmd("images", []string{"[REPOSITORY[:TAG]]"}, Cli.DockerCommands["images"].Description, true)
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only show numeric IDs")
	all := cmd.Bool([]string{"a", "-all"}, false, "Show all images (default hides intermediate images)")
	noTrunc := cmd.Bool([]string{"-no-trunc"}, false, "Don't truncate output")
	showDigests := cmd.Bool([]string{"-digests"}, false, "Show digests")
	format := cmd.String([]string{"-format"}, "", "Pretty-print images using a Go template")

	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")
	cmd.Require(flag.Max, 1)

	cmd.ParseFlags(args, true)

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process in the daemon/server.
	imageFilterArgs := filters.NewArgs()
	for _, f := range flFilter.GetAll() {
		var err error
		imageFilterArgs, err = filters.ParseFlag(f, imageFilterArgs)
		if err != nil {
			return err
		}
	}

	var matchName string
	if cmd.NArg() == 1 {
		matchName = cmd.Arg(0)
	}

	options := types.ImageListOptions{
		MatchName: matchName,
		All:       *all,
		Filters:   imageFilterArgs,
	}

	images, err := cli.client.ImageList(options)
	if err != nil {
		return err
	}

	f := *format
	if len(f) == 0 {
		if len(cli.ImagesFormat()) > 0 && !*quiet {
			f = cli.ImagesFormat()
		} else {
			f = "table"
		}
	}

	imagesCtx := formatter.ImageContext{
		Context: formatter.Context{
			Output: cli.out,
			Format: f,
			Quiet:  *quiet,
			Trunc:  !*noTrunc,
		},
		Digest: *showDigests,
		Images: images,
	}

	imagesCtx.Write()

	return nil
}
