package client

import (
	"errors"
	"net/url"

	"github.com/docker/distribution/reference"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/registry"
)

// CmdTag tags an image into a repository.
//
// Usage: docker tag [OPTIONS] IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]
func (cli *DockerCli) CmdTag(args ...string) error {
	cmd := Cli.Subcmd("tag", []string{"IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]"}, Cli.DockerCommands["tag"].Description, true)
	force := cmd.Bool([]string{"f", "-force"}, false, "Force the tagging even if there's a conflict")
	cmd.Require(flag.Exact, 2)

	cmd.ParseFlags(args, true)

	v := url.Values{}
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
	v.Set("repo", ref.Name())
	v.Set("tag", tag)

	if *force {
		v.Set("force", "1")
	}

	if _, _, err := readBody(cli.call("POST", "/images/"+cmd.Arg(0)+"/tag?"+v.Encode(), nil, nil)); err != nil {
		return err
	}
	return nil
}
