package client

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"text/tabwriter"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
)

// CmdSearch searches the Docker Hub for images.
//
// Usage: docker search [OPTIONS] TERM
func (cli *DockerCli) CmdSearch(args ...string) error {
	cmd := Cli.Subcmd("search", []string{"TERM"}, Cli.DockerCommands["search"].Description, true)
	noTrunc := cmd.Bool([]string{"-no-trunc"}, false, "Don't truncate output")
	noIndex := cmd.Bool([]string{"#noindex", "-no-index"}, false, "Don't prepend index to output")
	automated := cmd.Bool([]string{"-automated"}, false, "Only show automated builds")
	stars := cmd.Uint([]string{"s", "-stars"}, 0, "Only displays with at least x stars")
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	name := cmd.Arg(0)

	indexInfo, err := registry.ParseSearchIndexInfo(name)
	if err != nil {
		return err
	}

	requestPrivilege := cli.registryAuthenticationPrivilegedFunc(indexInfo, "search", false)

	encodedAuth, err := encodeAuthToBase64(cli.configFile.AuthConfigs)
	if err != nil {
		return err
	}

	options := types.ImageSearchOptions{
		Term:         name,
		RegistryAuth: encodedAuth,
		NoIndex:      *noIndex,
	}

	results, err := cli.client.ImageSearch(options, requestPrivilege)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 10, 1, 3, ' ', 0)
	if *noIndex {
		fmt.Fprintf(w, "NAME\tDESCRIPTION\tSTARS\tOFFICIAL\tAUTOMATED\n")
	} else {
		fmt.Fprintf(w, "INDEX\tNAME\tDESCRIPTION\tSTARS\tOFFICIAL\tAUTOMATED\n")
	}
	for _, res := range results {
		if (*automated && !res.IsAutomated) || (int(*stars) > res.StarCount) {
			continue
		}
		row := []string{}
		if !*noIndex {
			indexName := res.IndexName
			if !*noTrunc {
				// Shorten index name to DOMAIN.TLD unless --no-trunc is given.
				if host, _, err := net.SplitHostPort(indexName); err == nil {
					indexName = host
				}
				// do not shorten ip address
				if net.ParseIP(indexName) == nil {
					// shorten index name just to the last 2 components (`DOMAIN.TLD`)
					indexNameSubStrings := strings.Split(indexName, ".")
					if len(indexNameSubStrings) > 2 {
						indexName = strings.Join(indexNameSubStrings[len(indexNameSubStrings)-2:], ".")
					}
				}
			}
			row = append(row, indexName)
		}

		desc := strings.Replace(res.Description, "\n", " ", -1)
		desc = strings.Replace(desc, "\r", " ", -1)
		if !*noTrunc && len(desc) > 45 {
			desc = stringutils.Truncate(desc, 42) + "..."
		}
		row = append(row, res.RegistryName+"/"+res.Name, desc, strconv.Itoa(res.StarCount), "", "")
		if res.IsOfficial {
			row[len(row)-2] = "[OK]"
		}
		if res.IsAutomated || res.IsTrusted {
			row[len(row)-1] = "[OK]"
		}
		fmt.Fprintf(w, "%s\n", strings.Join(row, "\t"))
	}
	w.Flush()
	return nil
}
