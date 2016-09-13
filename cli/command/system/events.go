package system

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	eventtypes "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/spf13/cobra"
)

type eventsOptions struct {
	since  string
	until  string
	filter opts.FilterOpt
}

// NewEventsCommand creates a new cobra.Command for `docker events`
func NewEventsCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := eventsOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "events [OPTIONS]",
		Short: "Get real time events from the server",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvents(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.since, "since", "", "Show all events created since timestamp")
	flags.StringVar(&opts.until, "until", "", "Stream events until this timestamp")
	flags.VarP(&opts.filter, "filter", "f", "Filter output based on conditions provided")

	return cmd
}

func runEvents(dockerCli *command.DockerCli, opts *eventsOptions) error {
	options := types.EventsOptions{
		Since:   opts.since,
		Until:   opts.until,
		Filters: opts.filter.Value(),
	}

	responseBody, err := dockerCli.Client().Events(context.Background(), options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return streamEvents(responseBody, dockerCli.Out())
}

// streamEvents decodes prints the incoming events in the provided output.
func streamEvents(input io.Reader, output io.Writer) error {
	return DecodeEvents(input, func(event eventtypes.Message, err error) error {
		if err != nil {
			return err
		}
		printOutput(event, output)
		return nil
	})
}

type eventProcessor func(event eventtypes.Message, err error) error

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
		var keys []string
		for k := range event.Actor.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := event.Actor.Attributes[k]
			attrs = append(attrs, fmt.Sprintf("%s=%s", k, v))
		}
		fmt.Fprintf(output, " (%s)", strings.Join(attrs, ", "))
	}
	fmt.Fprint(output, "\n")
}
