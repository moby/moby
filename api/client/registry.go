package client

import (
	"fmt"
	"net/url"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdRegistryAdd is the add registry in the runtime
//
// Usage: docker registry add [OPTIONS] [REGISTRIES]
func (cli *DockerCli) CmdRegistryAdd(args ...string) (err error) {
	cmd := Cli.Subcmd("registry add", nil, "Add registries.", true)
	cmd.Require(flag.Exact, 0)
	flInsecureRegistries := opts.NewListOpts(nil)
	cmd.Var(&flInsecureRegistries, []string{"-insecure-registry"}, "Enable insecure registry communication")

	cmd.ParseFlags(args, true)

	v := url.Values{}
	for _, registry := range flInsecureRegistries.GetAll() {
		v.Add("registry", registry)
	}

	if _, _, err := readBody(cli.call("POST", fmt.Sprintf("/registry/add?%s", v.Encode()), nil, nil)); err != nil {
		fmt.Fprintf(cli.err, "%s\n", err)
		return fmt.Errorf("Error: failed to add insecure registry.")
	}

	return nil
}
