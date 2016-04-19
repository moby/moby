package client

import (
	"encoding/json"
	"fmt"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
)

// CmdCommit creates a new image from a container's changes.
//
// Usage: docker commit [OPTIONS] CONTAINER [REPOSITORY[:TAG]]
func (cli *DockerCli) CmdCommit(args ...string) error {
	cmd := Cli.Subcmd("commit", []string{"CONTAINER [REPOSITORY[:TAG]]"}, Cli.DockerCommands["commit"].Description, true)
	flPause := cmd.Bool([]string{"p", "-pause"}, true, "Pause container during commit")
	flComment := cmd.String([]string{"m", "-message"}, "", "Commit message")
	flAuthor := cmd.String([]string{"a", "-author"}, "", "Author (e.g., \"John Hannibal Smith <hannibal@a-team.com>\")")
	flChanges := opts.NewListOpts(nil)
	cmd.Var(&flChanges, []string{"c", "-change"}, "Apply Dockerfile instruction to the created image")
	// FIXME: --run is deprecated, it will be replaced with inline Dockerfile commands.
	flConfig := cmd.String([]string{"#-run"}, "", "This option is deprecated and will be removed in a future version in favor of inline Dockerfile-compatible commands")
	cmd.Require(flag.Max, 2)
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var (
		name      = cmd.Arg(0)
		reference = cmd.Arg(1)
	)

	var config *container.Config
	if *flConfig != "" {
		config = &container.Config{}
		if err := json.Unmarshal([]byte(*flConfig), config); err != nil {
			return err
		}
	}

	options := types.ContainerCommitOptions{
		Reference: reference,
		Comment:   *flComment,
		Author:    *flAuthor,
		Changes:   flChanges.GetAll(),
		Pause:     *flPause,
		Config:    config,
	}

	response, err := cli.client.ContainerCommit(context.Background(), name, options)
	if err != nil {
		return err
	}

	fmt.Fprintln(cli.out, response.ID)
	return nil
}
