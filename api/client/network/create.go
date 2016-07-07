package network

import (
	"fmt"
	"net"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/network"
	"github.com/spf13/cobra"
)

type createOptions struct {
	name       string
	driver     string
	driverOpts opts.MapOpts
	labels     []string
	internal   bool
	ipv6       bool

	ipamDriver  string
	ipamSubnet  []string
	ipamIPRange []string
	ipamGateway []string
	ipamAux     opts.MapOpts
	ipamOpt     opts.MapOpts
}

func newCreateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := createOptions{
		driverOpts: *opts.NewMapOpts(nil, nil),
		ipamAux:    *opts.NewMapOpts(nil, nil),
		ipamOpt:    *opts.NewMapOpts(nil, nil),
	}

	cmd := &cobra.Command{
		Use:   "create [OPTIONS] NETWORK",
		Short: "Create a network",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]
			return runCreate(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.driver, "driver", "d", "bridge", "Driver to manage the Network")
	flags.VarP(&opts.driverOpts, "opt", "o", "Set driver specific options")
	flags.StringSliceVar(&opts.labels, "label", []string{}, "Set metadata on a network")
	flags.BoolVar(&opts.internal, "internal", false, "restricts external access to the network")
	flags.BoolVar(&opts.ipv6, "ipv6", false, "enable IPv6 networking")

	flags.StringVar(&opts.ipamDriver, "ipam-driver", "default", "IP Address Management Driver")
	flags.StringSliceVar(&opts.ipamSubnet, "subnet", []string{}, "subnet in CIDR format that represents a network segment")
	flags.StringSliceVar(&opts.ipamIPRange, "ip-range", []string{}, "allocate container ip from a sub-range")
	flags.StringSliceVar(&opts.ipamGateway, "gateway", []string{}, "ipv4 or ipv6 Gateway for the master subnet")

	flags.Var(&opts.ipamAux, "aux-address", "auxiliary ipv4 or ipv6 addresses used by Network driver")
	flags.Var(&opts.ipamOpt, "ipam-opt", "set IPAM driver specific options")

	return cmd
}

func runCreate(dockerCli *client.DockerCli, opts createOptions) error {
	client := dockerCli.Client()

	ipamCfg, err := consolidateIpam(opts.ipamSubnet, opts.ipamIPRange, opts.ipamGateway, opts.ipamAux.GetAll())
	if err != nil {
		return err
	}

	// Construct network create request body
	nc := types.NetworkCreate{
		Driver:  opts.driver,
		Options: opts.driverOpts.GetAll(),
		IPAM: network.IPAM{
			Driver:  opts.ipamDriver,
			Config:  ipamCfg,
			Options: opts.ipamOpt.GetAll(),
		},
		CheckDuplicate: true,
		Internal:       opts.internal,
		EnableIPv6:     opts.ipv6,
		Labels:         runconfigopts.ConvertKVStringsToMap(opts.labels),
	}

	resp, err := client.NetworkCreate(context.Background(), opts.name, nc)
	if err != nil {
		return err
	}
	fmt.Fprintf(dockerCli.Out(), "%s\n", resp.ID)
	return nil
}

// Consolidates the ipam configuration as a group from different related configurations
// user can configure network with multiple non-overlapping subnets and hence it is
// possible to correlate the various related parameters and consolidate them.
// consoidateIpam consolidates subnets, ip-ranges, gateways and auxiliary addresses into
// structured ipam data.
func consolidateIpam(subnets, ranges, gateways []string, auxaddrs map[string]string) ([]network.IPAMConfig, error) {
	if len(subnets) < len(ranges) || len(subnets) < len(gateways) {
		return nil, fmt.Errorf("every ip-range or gateway must have a corresponding subnet")
	}
	iData := map[string]*network.IPAMConfig{}

	// Populate non-overlapping subnets into consolidation map
	for _, s := range subnets {
		for k := range iData {
			ok1, err := subnetMatches(s, k)
			if err != nil {
				return nil, err
			}
			ok2, err := subnetMatches(k, s)
			if err != nil {
				return nil, err
			}
			if ok1 || ok2 {
				return nil, fmt.Errorf("multiple overlapping subnet configuration is not supported")
			}
		}
		iData[s] = &network.IPAMConfig{Subnet: s, AuxAddress: map[string]string{}}
	}

	// Validate and add valid ip ranges
	for _, r := range ranges {
		match := false
		for _, s := range subnets {
			ok, err := subnetMatches(s, r)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if iData[s].IPRange != "" {
				return nil, fmt.Errorf("cannot configure multiple ranges (%s, %s) on the same subnet (%s)", r, iData[s].IPRange, s)
			}
			d := iData[s]
			d.IPRange = r
			match = true
		}
		if !match {
			return nil, fmt.Errorf("no matching subnet for range %s", r)
		}
	}

	// Validate and add valid gateways
	for _, g := range gateways {
		match := false
		for _, s := range subnets {
			ok, err := subnetMatches(s, g)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if iData[s].Gateway != "" {
				return nil, fmt.Errorf("cannot configure multiple gateways (%s, %s) for the same subnet (%s)", g, iData[s].Gateway, s)
			}
			d := iData[s]
			d.Gateway = g
			match = true
		}
		if !match {
			return nil, fmt.Errorf("no matching subnet for gateway %s", g)
		}
	}

	// Validate and add aux-addresses
	for key, aa := range auxaddrs {
		match := false
		for _, s := range subnets {
			ok, err := subnetMatches(s, aa)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			iData[s].AuxAddress[key] = aa
			match = true
		}
		if !match {
			return nil, fmt.Errorf("no matching subnet for aux-address %s", aa)
		}
	}

	idl := []network.IPAMConfig{}
	for _, v := range iData {
		idl = append(idl, *v)
	}
	return idl, nil
}

func subnetMatches(subnet, data string) (bool, error) {
	var (
		ip net.IP
	)

	_, s, err := net.ParseCIDR(subnet)
	if err != nil {
		return false, fmt.Errorf("Invalid subnet %s : %v", s, err)
	}

	if strings.Contains(data, "/") {
		ip, _, err = net.ParseCIDR(data)
		if err != nil {
			return false, fmt.Errorf("Invalid cidr %s : %v", data, err)
		}
	} else {
		ip = net.ParseIP(data)
	}

	return s.Contains(ip), nil
}
