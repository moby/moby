package client

import (
	"io"
	"os"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/jsonmessage"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/engine-api/types"
)

// CmdImport creates an empty filesystem image, imports the contents of the tarball into the image, and optionally tags the image.
//
// The URL argument is the address of a tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz) file or a path to local file relative to docker client. If the URL is '-', then the tar file is read from STDIN.
//
// Usage: docker import [OPTIONS] file|URL|- [REPOSITORY[:TAG]]
func (cli *DockerCli) CmdImport(args ...string) error {
	cmd := Cli.Subcmd("import", []string{"file|URL|- [REPOSITORY[:TAG]]"}, Cli.DockerCommands["import"].Description, true)
	flChanges := opts.NewListOpts(nil)
	cmd.Var(&flChanges, []string{"c", "-change"}, "Apply Dockerfile instruction to the created image")
	message := cmd.String([]string{"m", "-message"}, "", "Set commit message for imported image")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var (
		in      io.Reader
		src     = cmd.Arg(0)
		srcName = src
		ref     = cmd.Arg(1)
		changes = flChanges.GetAll()
	)

	if src == "-" {
		in = cli.in
	} else if !urlutil.IsURL(src) {
		srcName = "-"
		file, err := os.Open(src)
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
		Message: *message,
		Changes: changes,
	}

	responseBody, err := cli.client.ImageImport(context.Background(), source, ref, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, cli.out, cli.outFd, cli.isTerminalOut, nil)
}
