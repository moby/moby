package client

import (
	"io"
	"os"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

func (cli *DockerCli) CmdLoad(args ...string) error {
	cmd := cli.Subcmd("load", "", "Load an image from a tar archive on STDIN", true)
	infile := cmd.String([]string{"i", "-input"}, "", "Read from a tar archive file, instead of STDIN")
	cmd.Require(flag.Exact, 0)

	utils.ParseFlags(cmd, args, true)

	var (
		input io.Reader = cli.in
		err   error
	)
	if *infile != "" {
		input, err = os.Open(*infile)
		if err != nil {
			return err
		}
	}
	if err := cli.stream("POST", "/images/load", input, cli.out, nil); err != nil {
		return err
	}
	return nil
}
