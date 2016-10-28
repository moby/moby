package network

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type bandwidthOptions struct {
	Set           int32
	Remove        bool
	Container     string
	EgressMin     string
	EgressMax     string
	IngressMin    string
	IngressMax    string
	SpeedTypeIn   string
	InterfaceName string
}

func newBandwidthCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts bandwidthOptions

	cmd := &cobra.Command{
		Use:     "bandwidth CONTAINER [OPTIONS]",
		Aliases: []string{"bw"},
		Short:   "Set/Remove Network Bandwidth Management rules",
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Container = args[0]
			}
			return runBandwidth(dockerCli, &opts)
		},
	}
	flags := cmd.Flags()
	flags.Int32VarP(&opts.Set, "Set", "s", 0, "Set Container Network Bandwidth RATE ")
	flags.StringVarP(&opts.SpeedTypeIn, "SpeedTypeIn", "t", "", "SpeedType [kbps|mbps|gbps] bits per second")
	flags.StringVarP(&opts.InterfaceName, "InterfaceName", "i", "", "Docker Host Physical Interface Name(optional when docker0 bridge ")
	flags.BoolVarP(&opts.Remove, "Remove", "r", false, "Remove Container Network Bandwidth")

	return cmd
}

//shiva
func runBandwidth(dockerCli *command.DockerCli, opts *bandwidthOptions) error {

	client := dockerCli.Client()

	ctx := context.Background()
	_, err := dockerCli.Client().ContainerInspect(ctx, opts.Container)
	if err != nil {
		fmt.Println("Specify a Valid running Container Id")
		return err
	}
	if opts.Remove == true {
		goto REMOVE
	}
	if opts.Set <= 0 {
		fmt.Println("Cannot set bandwidth less than or equal to zero")
		return err
	}
	if opts.Set != 0 && opts.Remove != false {
		fmt.Println("Cannot do set and remove bandwidth for a container")
		return err
	}

	if !(opts.Set != 0 && opts.SpeedTypeIn != "") {
		fmt.Println("options --set and --speedTypeIn must be specified")
		return err
	}

	if opts.SpeedTypeIn != "kbps" && opts.SpeedTypeIn != "mbps" && opts.SpeedTypeIn != "gbps" {
		fmt.Println("SpeedTypeIn either in kbps or mbps or gbps only")
		return err
	}
	//Allow speed only in bits per seconds
	if opts.SpeedTypeIn == "kbps" {
		opts.SpeedTypeIn = "kbit"
	} else if opts.SpeedTypeIn == "mbps" {
		opts.SpeedTypeIn = "mbit"
	} else if opts.SpeedTypeIn == "gbps" {
		opts.SpeedTypeIn = "gbit"
	}

REMOVE:
	// Construct network create request body
	bc := types.BandwidthCreateRequest{
		Driver:        "bandwidth_drv",
		Container:     opts.Container,
		EgressMin:     opts.Set,
		EgressMax:     opts.Set,
		IngressMin:    opts.Set,
		IngressMax:    opts.Set,
		SpeedTypeIn:   opts.SpeedTypeIn,
		InterfaceName: opts.InterfaceName,
		Remove:        opts.Remove,
	}

	resp, err := client.BandwidthCreateRequest(context.Background(), opts.Container, bc)
	if err != nil {
		return err
	}
	fmt.Fprintln(dockerCli.Out(), resp.ID, resp.Warning)

	return nil
}
