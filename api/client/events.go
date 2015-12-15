package client

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/jsonmessage"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdEvents prints a live stream of real time events from the server.
//
// Usage: docker events [OPTIONS]
func (cli *DockerCli) CmdEvents(args ...string) error {
	cmd := Cli.Subcmd("events", nil, Cli.DockerCommands["events"].Description, true)
	since := cmd.String([]string{"-since"}, "", "Show all events created since timestamp")
	until := cmd.String([]string{"-until"}, "", "Stream events until this timestamp")
	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	eventFilterArgs := filters.NewArgs()

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process in the daemon/server.
	for _, f := range flFilter.GetAll() {
		var err error
		eventFilterArgs, err = filters.ParseFlag(f, eventFilterArgs)
		if err != nil {
			return err
		}
	}

	options := types.EventsOptions{
		Since:   *since,
		Until:   *until,
		Filters: eventFilterArgs,
	}

	responseBody, err := cli.client.Events(options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, cli.out, cli.outFd, cli.isTerminalOut)
}
