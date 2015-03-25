package client

import (
	"fmt"
	"net/url"

	"github.com/docker/docker/engine"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
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

	utils.ParseFlags(cmd, args, true)

	v := url.Values{}
	if *force {
		v.Set("force", "1")
	}
	if *noprune {
		v.Set("noprune", "1")
	}

	var encounteredError error
	for _, name := range cmd.Args() {
		body, _, err := readBody(cli.call("DELETE", "/images/"+name+"?"+v.Encode(), nil, false))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			encounteredError = fmt.Errorf("Error: failed to remove one or more images")
		} else {
			outs := engine.NewTable("Created", 0)
			if _, err := outs.ReadListFrom(body); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				encounteredError = fmt.Errorf("Error: failed to remove one or more images")
				continue
			}
			for _, out := range outs.Data {
				if out.Get("Deleted") != "" {
					fmt.Fprintf(cli.out, "Deleted: %s\n", out.Get("Deleted"))
				} else {
					fmt.Fprintf(cli.out, "Untagged: %s\n", out.Get("Untagged"))
				}
			}
		}
	}
	return encounteredError
}
