package client

import (
	"fmt"
	"net/url"

	"github.com/docker/docker/api/client/lib"
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

	var errNames []string
	for _, name := range cmd.Args() {
		options := lib.ImageRemoveOptions{
			ImageID:       name,
			Force:         *force,
			PruneChildren: !*noprune,
		}

		dels, err := cli.client.ImageRemove(options)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			errNames = append(errNames, name)
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
	if len(errNames) > 0 {
		return fmt.Errorf("Error: failed to remove images: %v", errNames)
	}
	return nil
}
