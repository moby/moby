package client

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdDiffi shows changes on an images's filesystem.
//
// Each changed file is printed on a separate line, prefixed with a single
// character that indicates the status of the file: C (modified), A (added),
// or D (deleted).
//
// Usage: docker diffi IMAGE
func (cli *DockerCli) CmdDiffi(args ...string) error {
	cmd := cli.Subcmd("diffi", "IMAGE", "Inspect changes on a image's filesystem", true)
	cmd.Require(flag.Exact, 1)
	cmd.ParseFlags(args, true)

	if cmd.Arg(0) == "" {
		return fmt.Errorf("Image name cannot be empty")
	}

	rdr, _, err := cli.call("GET", "/images/"+cmd.Arg(0)+"/changes", nil, nil)
	if err != nil {
		return err
	}

	changes := []types.ContainerChange{}
	if err := json.NewDecoder(rdr).Decode(&changes); err != nil {
		return err
	}

	for _, change := range changes {
		var kind string
		switch change.Kind {
		case archive.ChangeModify:
			kind = "C"
		case archive.ChangeAdd:
			kind = "A"
		case archive.ChangeDelete:
			kind = "D"
		}
		fmt.Fprintf(cli.out, "%s %s\n", kind, change.Path)
	}

	return nil
}
