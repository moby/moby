package client

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/docker/docker/api/types"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdRmi removes all images with the specified name(s).
//
// Usage: docker rmi [OPTIONS] IMAGE [IMAGE...]
func (cli *DockerCli) CmdRmi(args ...string) error {
	var (
		cmd     = cli.Subcmd("rmi", "IMAGE [IMAGE...]", "Remove one or more images", true)
		force   = cmd.Bool([]string{"f", "-force"}, false, "Force removal of the image")
		noprune = cmd.Bool([]string{"-no-prune"}, false, "Do not delete untagged parents")
	)
	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	v := url.Values{}
	if *force {
		v.Set("force", "1")
	}
	if *noprune {
		v.Set("noprune", "1")
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		rdr, _, err := cli.call("DELETE", "/images/"+name+"?"+v.Encode(), nil, nil)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to remove one or more images")
		} else {
			dels := []types.ImageDelete{}
			err = json.NewDecoder(rdr).Decode(&dels)
			if err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				encounteredError = fmt.Errorf("Error: failed to remove one or more images")
				continue
			}

			for _, del := range dels {
				if del.Deleted != "" {
					fmt.Fprintf(cli.out, "Deleted: %s\n", del.Deleted)
				} else {
					fmt.Fprintf(cli.out, "Untagged: %s\n", del.Untagged)
				}
			}
		}
	}
	return encounteredError
}
