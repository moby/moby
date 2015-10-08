package client

import (
	"errors"
	"net/url"
	"os"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdSave saves one or more images to a tar archive.
//
// The tar archive is written to STDOUT by default, or written to a file.
//
// Usage: docker save [OPTIONS] IMAGE [IMAGE...]
func (cli *DockerCli) CmdSave(args ...string) error {
	cmd := Cli.Subcmd("save", []string{"IMAGE [IMAGE...]"}, Cli.DockerCommands["save"].Description+" (streamed to STDOUT by default)", true)
	outfile := cmd.String([]string{"o", "-output"}, "", "Write to a file, instead of STDOUT")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var (
		output = cli.out
		err    error
	)

	if *outfile == "" && cli.isTerminalOut {
		return errors.New("Cowardly refusing to save to a terminal. Use the -o flag or redirect.")
	}
	if *outfile != "" {
		if output, err = os.Create(*outfile); err != nil {
			return err
		}
	}

	sopts := &streamOpts{
		rawTerminal: true,
		out:         output,
	}

	v := url.Values{}
	for _, arg := range cmd.Args() {
		v.Add("names", arg)
	}
	if _, err := cli.stream("GET", "/images/get?"+v.Encode(), sopts); err != nil {
		return err
	}

	return nil
}
