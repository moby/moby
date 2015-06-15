package client

import (
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdCp copies files/folders from a path on the container to a directory on the host running the command.
//
// If HOSTDIR is '-', the data is written as a tar file to STDOUT.
//
// Usage: docker cp CONTAINER:PATH HOSTDIR
func (cli *DockerCli) CmdCp(args ...string) error {
	cmd := cli.Subcmd("cp", "CONTAINER:PATH HOSTDIR|-", "Copy files/folders from a PATH on the container to a HOSTDIR on the host\nrunning the command. Use '-' to write the data as a tar file to STDOUT.", true)
	cmd.Require(flag.Exact, 2)

	cmd.ParseFlags(args, true)

	// deal with path name with `:`
	info := strings.SplitN(cmd.Arg(0), ":", 2)

	if len(info) != 2 {
		return fmt.Errorf("Error: Path not specified")
	}

	cfg := &types.CopyConfig{
		Resource: info[1],
	}
	stream, _, statusCode, err := cli.call("POST", "/containers/"+info[0]+"/copy", cfg, nil)
	if stream != nil {
		defer stream.Close()
	}
	if statusCode == 404 {
		return fmt.Errorf("No such container: %v", info[0])
	}
	if err != nil {
		return err
	}

	hostPath := cmd.Arg(1)
	if statusCode == 200 {
		if hostPath == "-" {
			_, err = io.Copy(cli.out, stream)
		} else {
			err = archive.Untar(stream, hostPath, &archive.TarOptions{NoLchown: true})
		}
		if err != nil {
			return err
		}
	}
	return nil
}
