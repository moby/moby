package client

import (
	"errors"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/registry"
)

// CmdTag tags an image into a repository.
//
// Usage: docker tag [OPTIONS] IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]
func (cli *DockerCli) CmdTag(args ...string) error {
	cmd := Cli.Subcmd("tag", []string{"IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]"}, Cli.DockerCommands["tag"].Description, true)
	force := cmd.Bool([]string{"#f", "#-force"}, false, "Force the tagging even if there's a conflict")
	cmd.Require(flag.Exact, 2)

	cmd.ParseFlags(args, true)

	ref, err := reference.ParseNamed(cmd.Arg(1))
	if err != nil {
		return err
	}

	_, isDigested := ref.(reference.Digested)
	if isDigested {
		return errors.New("refusing to create a tag with a digest reference")
	}

	tag := ""
	tagged, isTagged := ref.(reference.Tagged)
	if isTagged {
		tag = tagged.Tag()
	}

	//Check if the given image name can be resolved
	if err := registry.ValidateRepositoryName(ref); err != nil {
		return err
	}

	options := types.ImageTagOptions{
		ImageID:        cmd.Arg(0),
		RepositoryName: ref.Name(),
		Tag:            tag,
		Force:          *force,
	}

	return cli.client.ImageTag(options)
}
