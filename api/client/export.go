package client

import (
	"errors"
	"io"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdExport exports a filesystem as a tar archive.
//
// The tar archive is streamed to STDOUT by default or written to a file.
//
// Usage: docker export [OPTIONS] CONTAINER
func (cli *DockerCli) CmdExport(args ...string) error {
	cmd := Cli.Subcmd("export", []string{"CONTAINER"}, Cli.DockerCommands["export"].Description, true)
	outfile := cmd.String([]string{"o", "-output"}, "", "Write to a file, instead of STDOUT")
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	if *outfile == "" && cli.isTerminalOut {
		return errors.New("Cowardly refusing to save to a terminal. Use the -o flag or redirect.")
	}

	responseBody, err := cli.client.ContainerExport(cmd.Arg(0))
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if *outfile == "" {
		_, err := io.Copy(cli.out, responseBody)
		return err
	}

	return copyToFile(*outfile, responseBody)

}
