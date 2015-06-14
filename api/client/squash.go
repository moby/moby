package client

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/graph/tags"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/registry"
)

// CmdSquash merges filesystem layers of an image into a new image.
//
// Usage: docker squash IMAGE [ANCESTOR]
func (cli *DockerCli) CmdSquash(args ...string) error {
	cmd := cli.Subcmd("squash", []string{"IMAGE [ANCESTOR]"}, "Merge filesystem layers of an image into a new image", true)
	cmd.Require(flag.Min, 1)
	cmd.Require(flag.Max, 2)

	repoTag := cmd.String([]string{"t", "-tag"}, "", "Repository name (and optionally a tag) for the image")
	noTrunc := cmd.Bool([]string{"-no-trunc"}, false, "Don't truncate output")

	cmd.ParseFlags(args, true)

	// Set request parameters.
	query := make(url.Values)

	if cmd.NArg() > 1 {
		query.Set("ancestor", cmd.Arg(1))
	}

	//Check if the given image name/tag can be resolved
	if *repoTag != "" {
		repository, tag := parsers.ParseRepositoryTag(*repoTag)
		if err := registry.ValidateRepositoryName(repository); err != nil {
			return err
		}
		if tag != "" {
			if err := tags.ValidateTagName(tag); err != nil {
				return err
			}
		}
		query.Set("tag", *repoTag)
	}

	urlStr := fmt.Sprintf("/images/%s/squash?%s", cmd.Arg(0), query.Encode())

	responseJSON, _, err := readBody(cli.call("POST", urlStr, nil, nil))
	if err != nil {
		return err
	}

	var response types.ImageSquashResponse
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return fmt.Errorf("unable to decode response: %s", err)
	}

	ID := response.ID
	if !*noTrunc {
		ID = stringid.TruncateID(ID)
	}

	fmt.Fprintln(cli.out, ID)

	return nil
}
