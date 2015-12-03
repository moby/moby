package client

import (
	"io"
	"os"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdLoad loads an image from a tar archive.
//
// The tar archive is read from STDIN by default, or from a tar archive file.
//
// Usage: docker load [OPTIONS]
func (cli *DockerCli) CmdLoad(args ...string) error {
	cmd := Cli.Subcmd("load", nil, Cli.DockerCommands["load"].Description, true)
	infile := cmd.String([]string{"i", "-input"}, "", "Read from a tar archive file, instead of STDIN")
	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	var input io.Reader = cli.in
	if *infile != "" {
		file, err := os.Open(*infile)
		if err != nil {
			return err
		}
		defer file.Close()
		input = file
	}

	responseBody, err := cli.client.ImageLoad(input)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	_, err = io.Copy(cli.out, responseBody)
	return err
}
