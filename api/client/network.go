package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/stringid"
)

// CmdNetwork is the parent subcommand for all network commands
//
// Usage: docker network <COMMAND> [OPTIONS]
func (cli *DockerCli) CmdNetwork(args ...string) error {
	cmd := Cli.Subcmd("network", []string{"COMMAND [OPTIONS]"}, networkUsage(), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	cmd.Usage()
	return err
}

// CmdNetworkCreate creates a new network with a given name
//
// Usage: docker network create [OPTIONS] <NETWORK-NAME>
func (cli *DockerCli) CmdNetworkCreate(args ...string) error {
	cmd := Cli.Subcmd("network create", []string{"NETWORK-NAME"}, "Creates a new network with a name specified by the user", false)
	flDriver := cmd.String([]string{"d", "-driver"}, "bridge", "Driver to manage the Network")
	flOpts := opts.NewMapOpts(nil, nil)

	flIpamDriver := cmd.String([]string{"-ipam-driver"}, "default", "IP Address Management Driver")
	flIpamSubnet := opts.NewListOpts(nil)
	flIpamIPRange := opts.NewListOpts(nil)
	flIpamGateway := opts.NewListOpts(nil)
	flIpamAux := opts.NewMapOpts(nil, nil)

	cmd.Var(&flIpamSubnet, []string{"-subnet"}, "subnet in CIDR format that represents a network segment")
	cmd.Var(&flIpamIPRange, []string{"-ip-range"}, "allocate container ip from a sub-range")
	cmd.Var(&flIpamGateway, []string{"-gateway"}, "ipv4 or ipv6 Gateway for the master subnet")
	cmd.Var(flIpamAux, []string{"-aux-address"}, "auxiliary ipv4 or ipv6 addresses used by Network driver")
	cmd.Var(flOpts, []string{"o", "-opt"}, "set driver specific options")

	cmd.Require(flag.Exact, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Set the default driver to "" if the user didn't set the value.
	// That way we can know whether it was user input or not.
	driver := *flDriver
	if !cmd.IsSet("-driver") && !cmd.IsSet("d") {
		driver = ""
	}

	ipamCfg, err := consolidateIpam(flIpamSubnet.GetAll(), flIpamIPRange.GetAll(), flIpamGateway.GetAll(), flIpamAux.GetAll())
	if err != nil {
		return err
	}

	// Construct network create request body
	nc := types.NetworkCreate{
		Name:           cmd.Arg(0),
		Driver:         driver,
		IPAM:           network.IPAM{Driver: *flIpamDriver, Config: ipamCfg},
		Options:        flOpts.GetAll(),
		CheckDuplicate: true,
	}
	obj, _, err := readBody(cli.call("POST", "/networks/create", nc, nil))
	if err != nil {
		return err
	}
	var resp types.NetworkCreateResponse
	err = json.Unmarshal(obj, &resp)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s\n", resp.ID)
	return nil
}

// CmdNetworkRm deletes one or more networks
//
// Usage: docker network rm NETWORK-NAME|NETWORK-ID [NETWORK-NAME|NETWORK-ID...]
func (cli *DockerCli) CmdNetworkRm(args ...string) error {
	cmd := Cli.Subcmd("network rm", []string{"NETWORK [NETWORK...]"}, "Deletes one or more networks", false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	status := 0
	for _, net := range cmd.Args() {
		_, _, err = readBody(cli.call("DELETE", "/networks/"+net, nil, nil))
		if err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			status = 1
			continue
		}
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

// CmdNetworkConnect connects a container to a network
//
// Usage: docker network connect <NETWORK> <CONTAINER>
func (cli *DockerCli) CmdNetworkConnect(args ...string) error {
	cmd := Cli.Subcmd("network connect", []string{"NETWORK CONTAINER"}, "Connects a container to a network", false)
	cmd.Require(flag.Exact, 2)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	nc := types.NetworkConnect{Container: cmd.Arg(1)}
	_, _, err = readBody(cli.call("POST", "/networks/"+cmd.Arg(0)+"/connect", nc, nil))
	return err
}

// CmdNetworkDisconnect disconnects a container from a network
//
// Usage: docker network disconnect <NETWORK> <CONTAINER>
func (cli *DockerCli) CmdNetworkDisconnect(args ...string) error {
	cmd := Cli.Subcmd("network disconnect", []string{"NETWORK CONTAINER"}, "Disconnects container from a network", false)
	cmd.Require(flag.Exact, 2)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	nc := types.NetworkConnect{Container: cmd.Arg(1)}
	_, _, err = readBody(cli.call("POST", "/networks/"+cmd.Arg(0)+"/disconnect", nc, nil))
	return err
}

// CmdNetworkLs lists all the netorks managed by docker daemon
//
// Usage: docker network ls [OPTIONS]
func (cli *DockerCli) CmdNetworkLs(args ...string) error {
	cmd := Cli.Subcmd("network ls", nil, "Lists networks", true)
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only display numeric IDs")
	noTrunc := cmd.Bool([]string{"-no-trunc"}, false, "Do not truncate the output")

	cmd.Require(flag.Exact, 0)
	err := cmd.ParseFlags(args, true)

	if err != nil {
		return err
	}
	obj, _, err := readBody(cli.call("GET", "/networks", nil, nil))
	if err != nil {
		return err
	}

	var networkResources []types.NetworkResource
	err = json.Unmarshal(obj, &networkResources)
	if err != nil {
		return err
	}

	wr := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)

	// unless quiet (-q) is specified, print field titles
	if !*quiet {
		fmt.Fprintln(wr, "NETWORK ID\tNAME\tDRIVER")
	}

	for _, networkResource := range networkResources {
		ID := networkResource.ID
		netName := networkResource.Name
		if !*noTrunc {
			ID = stringid.TruncateID(ID)
		}
		if *quiet {
			fmt.Fprintln(wr, ID)
			continue
		}
		driver := networkResource.Driver
		fmt.Fprintf(wr, "%s\t%s\t%s\t",
			ID,
			netName,
			driver)
		fmt.Fprint(wr, "\n")
	}
	wr.Flush()
	return nil
}

// CmdNetworkInspect inspects the network object for more details
//
// Usage: docker network inspect [OPTIONS] <NETWORK> [NETWORK...]
func (cli *DockerCli) CmdNetworkInspect(args ...string) error {
	cmd := Cli.Subcmd("network inspect", []string{"NETWORK [NETWORK...]"}, "Displays detailed information on one or more networks", false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	status := 0
	var networks []*types.NetworkResource
	for _, name := range cmd.Args() {
		obj, _, err := readBody(cli.call("GET", "/networks/"+name, nil, nil))
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				fmt.Fprintf(cli.err, "Error: No such network: %s\n", name)
			} else {
				fmt.Fprintf(cli.err, "%s\n", err)
			}
			status = 1
			continue
		}
		networkResource := types.NetworkResource{}
		if err := json.NewDecoder(bytes.NewReader(obj)).Decode(&networkResource); err != nil {
			return err
		}

		networks = append(networks, &networkResource)
	}

	b, err := json.MarshalIndent(networks, "", "    ")
	if err != nil {
		return err
	}

	if _, err := io.Copy(cli.out, bytes.NewReader(b)); err != nil {
		return err
	}
	io.WriteString(cli.out, "\n")

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

// Consolidates the ipam configuration as a group from different related configurations
// user can configure network with multiple non-overlapping subnets and hence it is
// possible to corelate the various related parameters and consolidate them.
// consoidateIpam consolidates subnets, ip-ranges, gateways and auxilary addresses into
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

func networkUsage() string {
	networkCommands := map[string]string{
		"create":     "Create a network",
		"connect":    "Connect container to a network",
		"disconnect": "Disconnect container from a network",
		"inspect":    "Display detailed network information",
		"ls":         "List all networks",
		"rm":         "Remove a network",
	}

	help := "Commands:\n"

	for cmd, description := range networkCommands {
		help += fmt.Sprintf("  %-25.25s%s\n", cmd, description)
	}

	help += fmt.Sprintf("\nRun 'docker network COMMAND --help' for more information on a command.")
	return help
}
