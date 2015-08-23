// +build experimental

package client

import (
	"os"

	nwclient "github.com/docker/libnetwork/client"
)

// CmdService is used to manage network services.
// service command is user to publish, attach and list a service from a container.
func (cli *DockerCli) CmdService(args ...string) error {
	nCli := nwclient.NewNetworkCli(cli.out, cli.err, nwclient.CallFunc(cli.callWrapper))
	args = append([]string{"service"}, args...)
	return nCli.Cmd(os.Args[0], args...)
}
