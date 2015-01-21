package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/units"
)

// CmdPs outputs a list of Docker containers.
//
// Usage: docker ps [OPTIONS]
func (cli *DockerCli) CmdPs(args ...string) error {
	var (
		err error

		psFilterArgs = filters.Args{}
		v            = url.Values{}

		cmd      = cli.Subcmd("ps", "", "List containers", true)
		quiet    = cmd.Bool([]string{"q", "-quiet"}, false, "Only display numeric IDs")
		size     = cmd.Bool([]string{"s", "-size"}, false, "Display total file sizes")
		all      = cmd.Bool([]string{"a", "-all"}, false, "Show all containers (default shows just running)")
		noTrunc  = cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
		nLatest  = cmd.Bool([]string{"l", "-latest"}, false, "Show the latest created container, include non-running")
		since    = cmd.String([]string{"#sinceId", "#-since-id", "-since"}, "", "Show created since Id or Name, include non-running")
		before   = cmd.String([]string{"#beforeId", "#-before-id", "-before"}, "", "Show only container created before Id or Name")
		last     = cmd.Int([]string{"n"}, -1, "Show n last created containers, include non-running")
		fields   = cmd.String([]string{"-fields"}, "cimtspn", "Choose fields to print, and order (c,i,m,t,s,p,n,z)")
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

	rdr, _, err := cli.call("GET", "/containers/json?"+v.Encode(), nil, nil)
	if err != nil {
		return err
	}

	containers := []types.Container{}
	if err := json.NewDecoder(rdr).Decode(&containers); err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if *quiet {
		*fields = "c"
	}

	if *size {
		*fields = *fields + "z"
	}

	if !*quiet {
		headermap := map[rune]string{
			'c': "CONTAINER ID",
			'i': "IMAGE",
			'm': "COMMAND",
			's': "STATUS",
			't': "CREATED",
			'p': "PORTS",
			'n': "NAMES",
			'z': "SIZE",
		}

		headers := make([]string, 0)
		for _, v := range *fields {
			if title, ok := headermap[v]; ok {
				headers = append(headers, title)
			}
		}

		if len(headers) > 0 {
			fmt.Fprint(w, strings.Join(headers, "\t")+"\n")
		}
	}

	stripNamePrefix := func(ss []string) []string {
		for i, s := range ss {
			ss[i] = s[1:]
		}

		return ss
	}

	type containerMeta struct {
		c string
		i string
		m string
		t string
		s string
		p string
		n string
		z string
	}

	outp := make([]containerMeta, 0)
	for _, container := range containers {
		next := containerMeta{
			c: container.ID,
			n: "",
			m: strconv.Quote(container.Command),
			i: container.Image,
			t: units.HumanDuration(time.Now().UTC().Sub(time.Unix(int64(container.Created), 0))) + " ago",
			s: container.Status,
			p: api.DisplayablePorts(container.Ports),
			z: fmt.Sprintf("%s", units.HumanSize(float64(container.SizeRw))),
		}

		// handle truncation
		outNames := stripNamePrefix(container.Names)
		if !*noTrunc {
			next.c = stringid.TruncateID(next.c)
			next.m = stringutils.Truncate(next.m, 20)
			// only display the default name for the container with notrunc is passed
			for _, name := range outNames {
				if len(strings.Split(name, "/")) == 1 {
					outNames = []string{name}
					break
				}
			}
		}
		next.n = strings.Join(outNames, ",")

		if next.i == "" {
			next.i = "<no image>"
		}

		// handle rootfs sizing
		if container.SizeRootFs > 0 {
			next.z = next.z + fmt.Sprintf(" (virtual %s)", units.HumanSize(float64(container.SizeRootFs)))
		}

		outp = append(outp, next)
	}

	for _, out := range outp {
		of := make([]string, 0)
		for _, v := range *fields {
			switch v {
			case 'c':
				of = append(of, out.c)
			case 'i':
				of = append(of, out.i)
			case 'm':
				of = append(of, out.m)
			case 't':
				of = append(of, out.t)
			case 's':
				of = append(of, out.s)
			case 'p':
				of = append(of, out.p)
			case 'n':
				of = append(of, out.n)
			case 'z':
				of = append(of, out.z)

			}
		}
		if len(of) > 0 {
			fmt.Fprintf(w, "%s\n", strings.Join(of, "\t"))
		}
	}

	if !*quiet {
		w.Flush()
	}

	return nil
}
