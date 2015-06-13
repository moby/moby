package client

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/autogen/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

// CmdVersion shows Docker version information.
//
// Available version information is shown for: client Docker version, client API version, client Go version, client Git commit, client OS/Arch, server Docker version, server API version, server Go version, server Git commit, and server OS/Arch.
//
// Usage: docker version
func (cli *DockerCli) CmdVersion(args ...string) error {
	cmd := cli.Subcmd("version", "", "Show the Docker version information.", true)
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	if dockerversion.VERSION != "" {
		fmt.Fprintf(cli.out, "Client version: %s\n", dockerversion.VERSION)
	}
	fmt.Fprintf(cli.out, "Client API version: %s\n", api.APIVERSION)
	fmt.Fprintf(cli.out, "Go version (client): %s\n", runtime.Version())
	if dockerversion.GITCOMMIT != "" {
		fmt.Fprintf(cli.out, "Git commit (client): %s\n", dockerversion.GITCOMMIT)
	}
	fmt.Fprintf(cli.out, "OS/Arch (client): %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if utils.ExperimentalBuild() {
		fmt.Fprintf(cli.out, "Experimental (client): true\n")
	}

	stream, _, _, err := cli.call("GET", "/version", nil, nil)
	if err != nil {
		return err
	}

	var v types.Version
	if err := json.NewDecoder(stream).Decode(&v); err != nil {
		fmt.Fprintf(cli.err, "Error reading remote version: %s\n", err)
		return err
	}

	fmt.Fprintf(cli.out, "Server version: %s\n", v.Version)
	if v.ApiVersion != "" {
		fmt.Fprintf(cli.out, "Server API version: %s\n", v.ApiVersion)
	}
	fmt.Fprintf(cli.out, "Go version (server): %s\n", v.GoVersion)
	fmt.Fprintf(cli.out, "Git commit (server): %s\n", v.GitCommit)
	fmt.Fprintf(cli.out, "OS/Arch (server): %s/%s\n", v.Os, v.Arch)
	if v.Experimental {
		fmt.Fprintf(cli.out, "Experimental (server): true\n")
	}
	return nil
}
