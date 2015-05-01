package client

import (
	"net/url"
	"strconv"
	"time"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/timeutils"
)

// CmdEvents prints a live stream of real time events from the server.
//
// Usage: docker events [OPTIONS]
func (cli *DockerCli) CmdEvents(args ...string) error {
	cmd := cli.Subcmd("events", "", "Get real time events from the server", true)
	since := cmd.String([]string{"#since", "-since"}, "", "Show all events created since timestamp")
	until := cmd.String([]string{"-until"}, "", "Stream events until this timestamp")
	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	var (
		v               = url.Values{}
		loc             = time.FixedZone(time.Now().Zone())
		eventFilterArgs = filters.Args{}
	)

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process in the daemon/server.
	for _, f := range flFilter.GetAll() {
		var err error
		eventFilterArgs, err = filters.ParseFlag(f, eventFilterArgs)
		if err != nil {
			return err
		}
	}
	var setTime = func(key, value string) {
		format := timeutils.RFC3339NanoFixed
		if len(value) < len(format) {
			format = format[:len(value)]
		}
		if t, err := time.ParseInLocation(format, value, loc); err == nil {
			v.Set(key, strconv.FormatInt(t.Unix(), 10))
		} else {
			v.Set(key, value)
		}
	}
	if *since != "" {
		setTime("since", *since)
	}
	if *until != "" {
		setTime("until", *until)
	}
	if len(eventFilterArgs) > 0 {
		filterJSON, err := filters.ToParam(eventFilterArgs)
		if err != nil {
			return err
		}
		v.Set("filters", filterJSON)
	}
	sopts := &streamOpts{
		rawTerminal: true,
		out:         cli.out,
	}
	if err := cli.stream("GET", "/events?"+v.Encode(), sopts); err != nil {
		return err
	}
	return nil
}
