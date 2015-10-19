package client

import (
	"github.com/docker/distribution/digest"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"io"
)

// CmdManifest outputs an image manifest or a digest reference for the input IMAGE argument
//
// Usage: docker manifest [--digest] IMAGE
func (cli *DockerCli) CmdManifest(args ...string) (err error) {
	cmd := Cli.Subcmd("manifest", nil, "Output manifest JSON or digest reference for the input IMAGE", true)
	outdigest := cmd.Bool([]string{"-digest", "d"}, false, "Computes manifest digest instead of returning the manifest")
	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	if serverResp, err := cli.call("GET", "/images/"+cmd.Arg(0)+"/manifest", nil, nil); err == nil {
		defer serverResp.body.Close()
		if *outdigest {
			digester := digest.Canonical.New()
			if _, err := io.Copy(digester.Hash(), serverResp.body); err != nil {
				cli.out.Write([]byte("Error writing response to stdout: " + err.Error() + "\n"))
			} else {
				cli.out.Write([]byte(digester.Digest().String() + "\n"))
			}
		} else {
			if _, err := io.Copy(cli.out, serverResp.body); err != nil {
				cli.out.Write([]byte("Error writing response to stdout: " + err.Error() + "\n"))
			}
		}
	} else {
		cli.out.Write([]byte(err.Error() + "\n"))
	}

	return
}
