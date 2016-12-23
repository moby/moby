package network

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/opts"
	"github.com/spf13/cobra"
)

type connectOptions struct {
	network      string
	container    string
	ipaddress    string
	ipv6address  string
	links        opts.ListOpts
	aliases      []string
	linklocalips []string
}

func newConnectCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := connectOptions{
		links: opts.NewListOpts(opts.ValidateLink),
	}

	cmd := &cobra.Command{
		Use:   "connect [OPTIONS] NETWORK CONTAINER",
		Short: "Connect a container to a network",
		Args:  cli.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.network = args[0]
			opts.container = args[1]
			return runConnect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.ipaddress, "ip", "", "IP Address")
	flags.StringVar(&opts.ipv6address, "ip6", "", "IPv6 Address")
	flags.Var(&opts.links, "link", "Add link to another container")
	flags.StringSliceVar(&opts.aliases, "alias", []string{}, "Add network-scoped alias for the container")
	flags.StringSliceVar(&opts.linklocalips, "link-local-ip", []string{}, "Add a link-local address for the container")

	return cmd
}

func runConnect(dockerCli *command.DockerCli, opts connectOptions) error {
	client := dockerCli.Client()

	epConfig := &network.EndpointSettings{
		IPAMConfig: &network.EndpointIPAMConfig{
			IPv4Address:  opts.ipaddress,
			IPv6Address:  opts.ipv6address,
			LinkLocalIPs: opts.linklocalips,
		},
		Links:   opts.links.GetAll(),
		Aliases: opts.aliases,
	}

	return client.NetworkConnect(context.Background(), opts.network, opts.container, epConfig)
}
