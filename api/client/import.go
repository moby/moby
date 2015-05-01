package client

import (
	"fmt"
	"io"
	"net/url"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/registry"
)

// CmdImport creates an empty filesystem image, imports the contents of the tarball into the image, and optionally tags the image.
//
// The URL argument is the address of a tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz) file. If the URL is '-', then the tar file is read from STDIN.
//
// Usage: docker import [OPTIONS] URL [REPOSITORY[:TAG]]
func (cli *DockerCli) CmdImport(args ...string) error {
	cmd := cli.Subcmd("import", "URL|- [REPOSITORY[:TAG]]", "Create an empty filesystem image and import the contents of the\ntarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz) into it, then\noptionally tag it.", true)
	flChanges := opts.NewListOpts(nil)
	cmd.Var(&flChanges, []string{"c", "-change"}, "Apply Dockerfile instruction to the created image")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var (
		v          = url.Values{}
		src        = cmd.Arg(0)
		repository = cmd.Arg(1)
	)

	v.Set("fromSrc", src)
	v.Set("repo", repository)
	for _, change := range flChanges.GetAll() {
		v.Add("changes", change)
	}
	if cmd.NArg() == 3 {
		fmt.Fprintf(cli.err, "[DEPRECATED] The format 'URL|- [REPOSITORY [TAG]]' has been deprecated. Please use URL|- [REPOSITORY[:TAG]]\n")
		v.Set("tag", cmd.Arg(2))
	}

	if repository != "" {
		//Check if the given image name can be resolved
		repo, _ := parsers.ParseRepositoryTag(repository)
		if err := registry.ValidateRepositoryName(repo); err != nil {
			return err
		}
	}

	var in io.Reader

	if src == "-" {
		in = cli.in
	}

	sopts := &streamOpts{
		rawTerminal: true,
		in:          in,
		out:         cli.out,
	}

	return cli.stream("POST", "/images/create?"+v.Encode(), sopts)
}
