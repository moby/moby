package image

import (
	"io"
	"os"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type importOptions struct {
	source    string
	reference string
	changes   []string
	message   string
}

// NewImportCommand creates a new `docker import` command
func NewImportCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts importOptions

	cmd := &cobra.Command{
		Use:   "import [OPTIONS] file|URL|- [REPOSITORY[:TAG]]",
		Short: "Import the contents from a tarball to create a filesystem image",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.source = args[0]
			if len(args) > 1 {
				opts.reference = args[1]
			}
			return runImport(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringSliceVarP(&opts.changes, "change", "c", []string{}, "Apply Dockerfile instruction to the created image")
	flags.StringVarP(&opts.message, "message", "m", "", "Set commit message for imported image")

	return cmd
}

func runImport(dockerCli *client.DockerCli, opts importOptions) error {
	var (
		in      io.Reader
		srcName = opts.source
	)

	if opts.source == "-" {
		in = dockerCli.In()
	} else if !urlutil.IsURL(opts.source) {
		srcName = "-"
		file, err := os.Open(opts.source)
		if err != nil {
			return err
		}
		defer file.Close()
		in = file
	}

	source := types.ImageImportSource{
		Source:     in,
		SourceName: srcName,
	}

	options := types.ImageImportOptions{
		Message: opts.message,
		Changes: opts.changes,
	}

	clnt := dockerCli.Client()

	responseBody, err := clnt.ImageImport(context.Background(), source, opts.reference, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, dockerCli.Out(), dockerCli.OutFd(), dockerCli.IsTerminalOut(), nil)
}
