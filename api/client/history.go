package client

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/units"
)

// CmdHistory shows the history of an image.
//
// Usage: docker history [OPTIONS] IMAGE
func (cli *DockerCli) CmdHistory(args ...string) error {
	cmd := Cli.Subcmd("history", []string{"IMAGE"}, Cli.DockerCommands["history"].Description, true)
	human := cmd.Bool([]string{"H", "-human"}, true, "Print sizes and dates in human readable format")
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only show numeric IDs")
	noTrunc := cmd.Bool([]string{"-no-trunc"}, false, "Don't truncate output")
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	serverResp, err := cli.call("GET", "/images/"+cmd.Arg(0)+"/history", nil, nil)
	if err != nil {
		return err
	}

	defer serverResp.body.Close()

	history := []types.ImageHistory{}
	if err := json.NewDecoder(serverResp.body).Decode(&history); err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)

	if *quiet {
		for _, entry := range history {
			if *noTrunc {
				fmt.Fprintf(w, "%s\n", entry.ID)
			} else {
				fmt.Fprintf(w, "%s\n", stringid.TruncateID(entry.ID))
			}
		}
		w.Flush()
		return nil
	}

	var imageID string
	var createdBy string
	var created string
	var size string

	fmt.Fprintln(w, "IMAGE\tCREATED\tCREATED BY\tSIZE\tCOMMENT")
	for _, entry := range history {
		imageID = entry.ID
		createdBy = strings.Replace(entry.CreatedBy, "\t", " ", -1)
		if *noTrunc == false {
			createdBy = stringutils.Truncate(createdBy, 45)
			imageID = stringid.TruncateID(entry.ID)
		}

		if *human {
			created = units.HumanDuration(time.Now().UTC().Sub(time.Unix(entry.Created, 0))) + " ago"
			size = units.HumanSize(float64(entry.Size))
		} else {
			created = time.Unix(entry.Created, 0).Format(time.RFC3339)
			size = strconv.FormatInt(entry.Size, 10)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", imageID, created, createdBy, size, entry.Comment)
	}
	w.Flush()
	return nil
}
