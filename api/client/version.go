package client

import (
	"fmt"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/engine"
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

	utils.ParseFlags(cmd, args, false)

	if dockerversion.VERSION != "" {
		fmt.Fprintf(cli.out, "Client version: %s\n", dockerversion.VERSION)
	}
	fmt.Fprintf(cli.out, "Client API version: %s\n", api.APIVERSION)
	fmt.Fprintf(cli.out, "Go version (client): %s\n", runtime.Version())
	if dockerversion.GITCOMMIT != "" {
		fmt.Fprintf(cli.out, "Git commit (client): %s\n", dockerversion.GITCOMMIT)
	}
	fmt.Fprintf(cli.out, "OS/Arch (client): %s/%s\n", runtime.GOOS, runtime.GOARCH)

	body, _, err := readBody(cli.call("GET", "/version", nil, nil))
	if err != nil {
		return err
	}

	out := engine.NewOutput()
	remoteVersion, err := out.AddEnv()
	if err != nil {
		log.Errorf("Error reading remote version: %s", err)
		return err
	}
	if _, err := out.Write(body); err != nil {
		log.Errorf("Error reading remote version: %s", err)
		return err
	}
	out.Close()
	fmt.Fprintf(cli.out, "Server version: %s\n", remoteVersion.Get("Version"))
	if apiVersion := remoteVersion.Get("ApiVersion"); apiVersion != "" {
		fmt.Fprintf(cli.out, "Server API version: %s\n", apiVersion)
	}
	fmt.Fprintf(cli.out, "Go version (server): %s\n", remoteVersion.Get("GoVersion"))
	fmt.Fprintf(cli.out, "Git commit (server): %s\n", remoteVersion.Get("GitCommit"))
	fmt.Fprintf(cli.out, "OS/Arch (server): %s/%s\n", remoteVersion.Get("Os"), remoteVersion.Get("Arch"))
	return nil
}
