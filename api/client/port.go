package client

import (
	"fmt"
	"strings"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/nat"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
)

// CmdPort lists port mappings for a container.
// If a private port is specified, it also shows the public-facing port that is NATed to the private port.
//
// Usage: docker port CONTAINER [PRIVATE_PORT[/PROTO]]
func (cli *DockerCli) CmdPort(args ...string) error {
	cmd := cli.Subcmd("port", "CONTAINER [PRIVATE_PORT[/PROTO]]", "List port mappings for the CONTAINER, or lookup the public-facing port that\nis NAT-ed to the PRIVATE_PORT", true)
	cmd.Require(flag.Min, 1)
	utils.ParseFlags(cmd, args, true)

	stream, _, err := cli.call("GET", "/containers/"+cmd.Arg(0)+"/json", nil, nil)
	if err != nil {
		return err
	}

	env := engine.Env{}
	if err := env.Decode(stream); err != nil {
		return err
	}
	ports := nat.PortMap{}
	if err := env.GetSubEnv("NetworkSettings").GetJson("Ports", &ports); err != nil {
		return err
	}

	if cmd.NArg() == 2 {
		var (
			port  = cmd.Arg(1)
			proto = "tcp"
			parts = strings.SplitN(port, "/", 2)
		)

		if len(parts) == 2 && len(parts[1]) != 0 {
			port = parts[0]
			proto = parts[1]
		}
		natPort := port + "/" + proto
		if frontends, exists := ports[nat.Port(port+"/"+proto)]; exists && frontends != nil {
			for _, frontend := range frontends {
				fmt.Fprintf(cli.out, "%s:%s\n", frontend.HostIp, frontend.HostPort)
			}
			return nil
		}
		return fmt.Errorf("Error: No public port '%s' published for %s", natPort, cmd.Arg(0))
	}

	for from, frontends := range ports {
		for _, frontend := range frontends {
			fmt.Fprintf(cli.out, "%s -> %s:%s\n", from, frontend.HostIp, frontend.HostPort)
		}
	}

	return nil
}
