package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdRmi removes all images with the specified name(s).
//
// Usage: docker rmi [OPTIONS] IMAGE [IMAGE...]
func (cli *DockerCli) CmdRmi(args ...string) error {
	cmd := Cli.Subcmd("rmi", []string{"IMAGE [IMAGE...]"}, "Remove one or more images", true)
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

	nameArgs := []string{}
	images := []types.Image{}
	wild, _ := regexp.Compile(`([a-zA-Z0-9.-_:]*)(\*(.*))?`)
	for _, arg := range cmd.Args() {
		if strings.Contains(arg, "*") {
			if len(images) == 0 {
				serverResp, err := cli.call("GET", "/images/json?"+v.Encode(), nil, nil)
				if err != nil {
					return err
				}
				defer serverResp.body.Close()
				images = []types.Image{}
				if err := json.NewDecoder(serverResp.body).Decode(&images); err != nil {
					return err
				}
			}
			parts := wild.FindStringSubmatch(arg)
			filterPat, _ := regexp.Compile("^" + regexp.QuoteMeta(parts[1]) + ".*" + regexp.QuoteMeta(parts[3]) + "$")
			for _, img := range images {
				found := false
				for _, t := range img.RepoTags {
					if filterPat.MatchString(t) {
						found = true
						break
					}
				}
				if found {
					nameArgs = append(nameArgs, img.ID)
				}
			}
		} else {
			nameArgs = append(nameArgs, arg)
		}
	}

	var errNames []string
	for _, name := range nameArgs {
		serverResp, err := cli.call("DELETE", "/images/"+name+"?"+v.Encode(), nil, nil)
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			errNames = append(errNames, name)
		} else {
			defer serverResp.body.Close()

			dels := []types.ImageDelete{}
			if err := json.NewDecoder(serverResp.body).Decode(&dels); err != nil {
				fmt.Fprintf(cli.err, "%s\n", err)
				errNames = append(errNames, name)
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
	if len(errNames) > 0 {
		return fmt.Errorf("Error: failed to remove images: %v", errNames)
	}
	return nil
}
