package client

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdRmi removes all images with the specified name(s).
//
// Usage: docker rmi [OPTIONS] IMAGE [IMAGE...]
func (cli *DockerCli) CmdRmi(args ...string) error {
	cmd := Cli.Subcmd("rmi", []string{"IMAGE [IMAGE...]"}, Cli.DockerCommands["rmi"].Description, true)
	force := cmd.Bool([]string{"f", "-force"}, false, "Force removal of the image")
	noprune := cmd.Bool([]string{"-no-prune"}, false, "Do not delete untagged parents")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	v := url.Values{}
	if *force {
		v.Set("force", "1")
	}
	if *noprune {
		v.Set("noprune", "1")
	}

	var errs []string
	for _, name := range cmd.Args() {
		options := types.ImageRemoveOptions{
			ImageID:       name,
			Force:         *force,
			PruneChildren: !*noprune,
		}

		dels, err := cli.client.ImageRemove(options)
		if err != nil {
			errs = append(errs, fmt.Sprintf("Failed to remove image (%s): %s", name, err))
		} else {
			for _, del := range dels {
				if del.Deleted != "" {
					fmt.Fprintf(cli.out, "Deleted: %s\n", del.Deleted)
				} else {
					fmt.Fprintf(cli.out, "Untagged: %s\n", del.Untagged)
				}
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
