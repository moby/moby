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
	cmd := cli.Subcmd("version", nil, "Show the Docker version information.", true)
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	fmt.Println("Client:")
	if dockerversion.VERSION != "" {
		fmt.Fprintf(cli.out, " Version:      %s\n", dockerversion.VERSION)
	}
	fmt.Fprintf(cli.out, " API version:  %s\n", api.Version)
	fmt.Fprintf(cli.out, " Go version:   %s\n", runtime.Version())
	if dockerversion.GITCOMMIT != "" {
		fmt.Fprintf(cli.out, " Git commit:   %s\n", dockerversion.GITCOMMIT)
	}
	if dockerversion.BUILDTIME != "" {
		fmt.Fprintf(cli.out, " Built:        %s\n", dockerversion.BUILDTIME)
	}
	fmt.Fprintf(cli.out, " OS/Arch:      %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if utils.ExperimentalBuild() {
		fmt.Fprintf(cli.out, " Experimental: true\n")
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

	fmt.Println("\nServer:")
	fmt.Fprintf(cli.out, " Version:      %s\n", v.Version)
	if v.ApiVersion != "" {
		fmt.Fprintf(cli.out, " API version:  %s\n", v.ApiVersion)
	}
	fmt.Fprintf(cli.out, " Go version:   %s\n", v.GoVersion)
	fmt.Fprintf(cli.out, " Git commit:   %s\n", v.GitCommit)
	if len(v.BuildTime) > 0 {
		fmt.Fprintf(cli.out, " Built:        %s\n", v.BuildTime)
	}
	fmt.Fprintf(cli.out, " OS/Arch:      %s/%s\n", v.Os, v.Arch)
	if v.Experimental {
		fmt.Fprintf(cli.out, " Experimental: true\n")
	}
	fmt.Fprintf(cli.out, "\n")
	return nil
}
