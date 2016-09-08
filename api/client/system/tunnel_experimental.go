// +build experimental

package system

import (
	"fmt"
	"net"
	"strconv"

	"golang.org/x/net/context"
	"golang.org/x/net/websocket"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/spf13/cobra"
)

type tunnelOptions struct {
	local  int
	remote int
}

// NewTunnelCommand creates a new cobra.Command for `docker tunnel`
func NewTunnelCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts tunnelOptions

	cmd := &cobra.Command{
		Use:   "tunnel REMOTE_TCP_PORT",
		Short: "Connect to the published port over a secure tunnel",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			opts.remote, err = strconv.Atoi(args[0])
			if err != nil {
				return err
			}
			if opts.local == 0 {
				opts.local = opts.remote
			}
			return runTunnel(dockerCli, &opts)
		},
	}
	flags := cmd.Flags()
	flags.IntVarP(&opts.local, "local", "l", 0, "Local TCP port (default: the port identical to the repote)")
	return cmd
}

func runTunnel(dockerCli *client.DockerCli, opts *tunnelOptions) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", opts.local))
	if err != nil {
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintf(dockerCli.Err(),
				"Error while accepting a connection: %v\n", err)
			continue
		}
		wsConn, err := dockerCli.Client().Tunnel(context.Background(), opts.remote)
		if err != nil {
			fmt.Fprintf(dockerCli.Err(),
				"Error while connecting to websocket: %v\n", err)
			conn.Close()
			continue
		}
		go handleTunnelConnection(dockerCli, conn, wsConn)
	}
}

func handleTunnelConnection(dockerCli *client.DockerCli, tcpConn net.Conn, wsConn *websocket.Conn) {
	ioutils.BidirectionalCopy(tcpConn, wsConn,
		func(err error, direction ioutils.Direction) {
			fmt.Fprintf(dockerCli.Err(),
				"error while copying (%v): %v\n",
				direction, err)
		})
}
