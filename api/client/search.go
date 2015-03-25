package client

import (
	"fmt"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

func (cli *DockerCli) CmdSearch(args ...string) error {
	cmd := cli.Subcmd("search", "TERM", "Search the Docker Hub for images", true)
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
	trusted := cmd.Bool([]string{"#t", "#trusted", "#-trusted"}, false, "Only show trusted builds")
	automated := cmd.Bool([]string{"-automated"}, false, "Only show automated builds")
	stars := cmd.Int([]string{"s", "#stars", "-stars"}, 0, "Only displays with at least x stars")
	cmd.Require(flag.Exact, 1)

	utils.ParseFlags(cmd, args, true)

	v := url.Values{}
	v.Set("term", cmd.Arg(0))

	body, _, err := readBody(cli.call("GET", "/images/search?"+v.Encode(), nil, true))

	if err != nil {
		return err
	}
	outs := engine.NewTable("star_count", 0)
	if _, err := outs.ReadListFrom(body); err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 10, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tSTARS\tOFFICIAL\tAUTOMATED\n")
	for _, out := range outs.Data {
		if ((*automated || *trusted) && (!out.GetBool("is_trusted") && !out.GetBool("is_automated"))) || (*stars > out.GetInt("star_count")) {
			continue
		}
		desc := strings.Replace(out.Get("description"), "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !*noTrunc && len(desc) > 45 {
			desc = utils.Trunc(desc, 42) + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t", out.Get("name"), desc, out.GetInt("star_count"))
		if out.GetBool("is_official") {
			fmt.Fprint(w, "[OK]")

		}
		fmt.Fprint(w, "\t")
		if out.GetBool("is_automated") || out.GetBool("is_trusted") {
			fmt.Fprint(w, "[OK]")
		}
		fmt.Fprint(w, "\n")
	}
	w.Flush()
	return nil
}
