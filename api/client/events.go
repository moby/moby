package client

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	eventtypes "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/jsonlog"
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

	return streamEvents(responseBody, cli.out)
}

// streamEvents decodes prints the incoming events in the provided output.
func streamEvents(input io.Reader, output io.Writer) error {
	return decodeEvents(input, func(event eventtypes.Message, err error) error {
		if err != nil {
			return err
		}
		printOutput(event, output)
		return nil
	})
}

type eventProcessor func(event eventtypes.Message, err error) error

func decodeEvents(input io.Reader, ep eventProcessor) error {
	dec := json.NewDecoder(input)
	for {
		var event eventtypes.Message
		err := dec.Decode(&event)
		if err != nil && err == io.EOF {
			break
		}

		if procErr := ep(event, err); procErr != nil {
			return procErr
		}
	}
	return nil
}

// printOutput prints all types of event information.
// Each output includes the event type, actor id, name and action.
// Actor attributes are printed at the end if the actor has any.
func printOutput(event eventtypes.Message, output io.Writer) {
	if event.TimeNano != 0 {
		fmt.Fprintf(output, "%s ", time.Unix(0, event.TimeNano).Format(jsonlog.RFC3339NanoFixed))
	} else if event.Time != 0 {
		fmt.Fprintf(output, "%s ", time.Unix(event.Time, 0).Format(jsonlog.RFC3339NanoFixed))
	}

	fmt.Fprintf(output, "%s %s %s", event.Type, event.Action, event.Actor.ID)

	if len(event.Actor.Attributes) > 0 {
		var attrs []string
		for k, v := range event.Actor.Attributes {
			attrs = append(attrs, fmt.Sprintf("%s=%s", k, v))
		}
		fmt.Fprintf(output, " (%s)", strings.Join(attrs, ", "))
	}
	fmt.Fprint(output, "\n")
}
