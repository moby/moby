package client

import (
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/docker/docker/api/client/ps"
	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers/filters"
)

// CmdPs outputs a list of Docker containers.
//
// Usage: docker ps [OPTIONS]
func (cli *DockerCli) CmdPs(args ...string) error {
	var (
		err error

		psFilterArgs = filters.Args{}
		v            = url.Values{}

		cmd      = Cli.Subcmd("ps", nil, Cli.DockerCommands["ps"].Description, true)
		quiet    = cmd.Bool([]string{"q", "-quiet"}, false, "Only display numeric IDs")
		size     = cmd.Bool([]string{"s", "-size"}, false, "Display total file sizes")
		all      = cmd.Bool([]string{"a", "-all"}, false, "Show all containers (default shows just running)")
		noTrunc  = cmd.Bool([]string{"-no-trunc"}, false, "Don't truncate output")
		nLatest  = cmd.Bool([]string{"l", "-latest"}, false, "Show the latest created container (includes all states)")
		since    = cmd.String([]string{"#-since"}, "", "Show containers created since Id or Name (includes all states)")
		before   = cmd.String([]string{"#-before"}, "", "Only show containers created before Id or Name")
		last     = cmd.Int([]string{"n"}, -1, "Show n last created containers (includes all states)")
		format   = cmd.String([]string{"-format"}, "", "Pretty-print containers using a Go template")
		flFilter = opts.NewListOpts(nil)
	)
	cmd.Require(flag.Exact, 0)

	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")

	cmd.ParseFlags(args, true)
	if *last == -1 && *nLatest {
		*last = 1
	}

	if *all {
		v.Set("all", "1")
	}

	if *last != -1 {
		v.Set("limit", strconv.Itoa(*last))
	}

	if *since != "" {
		v.Set("since", *since)
	}

	if *before != "" {
		v.Set("before", *before)
	}

	if *size {
		v.Set("size", "1")
	}

	// Consolidate all filter flags, and sanity check them.
	// They'll get processed in the daemon/server.
	for _, f := range flFilter.GetAll() {
		if psFilterArgs, err = filters.ParseFlag(f, psFilterArgs); err != nil {
			return err
		}
	}

	if len(psFilterArgs) > 0 {
		filterJSON, err := filters.ToParam(psFilterArgs)
		if err != nil {
			return err
		}

		v.Set("filters", filterJSON)
	}

	serverResp, err := cli.call("GET", "/containers/json?"+v.Encode(), nil, nil)
	if err != nil {
		return err
	}

	defer serverResp.body.Close()

	containers := []types.Container{}
	if err := json.NewDecoder(serverResp.body).Decode(&containers); err != nil {
		return err
	}

	f := *format
	if len(f) == 0 {
		if len(cli.PsFormat()) > 0 && !*quiet {
			f = cli.PsFormat()
		} else {
			f = "table"
		}
	}

	psCtx := ps.Context{
		Output: cli.out,
		Format: f,
		Quiet:  *quiet,
		Size:   *size,
		Trunc:  !*noTrunc,
	}

	ps.Format(psCtx, containers)

	return nil
}
