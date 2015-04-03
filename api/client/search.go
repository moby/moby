package client

import (
	"fmt"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

// CmdSearch searches the Docker Hub for images.
//
// Usage: docker search [OPTIONS] TERM
func (cli *DockerCli) CmdSearch(args ...string) error {
	cmd := cli.Subcmd("search", "TERM", "Search the Docker Hub for images", true)
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Don't truncate output")
	trusted := cmd.Bool([]string{"#t", "#trusted", "#-trusted"}, false, "Only show trusted builds")
	automated := cmd.Bool([]string{"-automated"}, false, "Only show automated builds")
	stars := cmd.Uint([]string{"s", "#stars", "-stars"}, 0, "Only displays with at least x stars")
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	name := cmd.Arg(0)
	v := url.Values{}
	v.Set("term", name)

	// Resolve the Repository name from fqn to hostname + name
	taglessRemote, _ := parsers.ParseRepositoryTag(name)
	repoInfo, err := registry.ParseRepositoryInfo(taglessRemote)
	if err != nil {
		return err
	}

	cli.LoadConfigFile()

	body, statusCode, errReq := cli.clientRequestAttemptLogin("GET", "/images/search?"+v.Encode(), nil, nil, repoInfo.Index, "search")
	rawBody, _, err := readBody(body, statusCode, errReq)
	if err != nil {
		return err
	}

	outs := engine.NewTable("star_count", 0)
	if _, err := outs.ReadListFrom(rawBody); err != nil {
		return err
	}
	outs.ReverseSort()
	w := tabwriter.NewWriter(cli.out, 10, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tSTARS\tOFFICIAL\tAUTOMATED\n")
	for _, out := range outs.Data {
		if ((*automated || *trusted) && (!out.GetBool("is_trusted") && !out.GetBool("is_automated"))) || (*stars > uint(out.GetInt("star_count"))) {
			continue
		}
		desc := strings.Replace(out.Get("description"), "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !*noTrunc && len(desc) > 45 {
			desc = utils.Trunc(desc, 42) + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t", out.Get("name"), desc, uint(out.GetInt("star_count")))
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
