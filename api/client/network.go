package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
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
	flDriver := cmd.String([]string{"d", "-driver"}, "", "Driver to manage the Network")
	cmd.Require(flag.Exact, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Construct network create request body
	nc := types.NetworkCreate{Name: cmd.Arg(0), Driver: *flDriver, CheckDuplicate: true}
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

// CmdNetworkRm deletes a network
//
// Usage: docker network rm <NETWORK-NAME | NETWORK-ID>
func (cli *DockerCli) CmdNetworkRm(args ...string) error {
	cmd := Cli.Subcmd("network rm", []string{"NETWORK"}, "Deletes a network", false)
	cmd.Require(flag.Exact, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	_, _, err = readBody(cli.call("DELETE", "/networks/"+cmd.Arg(0), nil, nil))
	if err != nil {
		return err
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
	cmd := Cli.Subcmd("network ls", []string{""}, "Lists all the networks created by the user", false)
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only display numeric IDs")
	noTrunc := cmd.Bool([]string{"", "-no-trunc"}, false, "Do not truncate the output")
	nLatest := cmd.Bool([]string{"l", "-latest"}, false, "Show the latest network created")
	last := cmd.Int([]string{"n"}, -1, "Show n last created networks")
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	obj, _, err := readBody(cli.call("GET", "/networks", nil, nil))
	if err != nil {
		return err
	}
	if *last == -1 && *nLatest {
		*last = 1
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
// Usage: docker network inspect <NETWORK>
// CmdNetworkInspect handles Network inspect UI
func (cli *DockerCli) CmdNetworkInspect(args ...string) error {
	cmd := Cli.Subcmd("network inspect", []string{"NETWORK"}, "Displays detailed information on a network", false)
	cmd.Require(flag.Exact, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	obj, _, err := readBody(cli.call("GET", "/networks/"+cmd.Arg(0), nil, nil))
	if err != nil {
		return err
	}
	networkResource := &types.NetworkResource{}
	if err := json.NewDecoder(bytes.NewReader(obj)).Decode(networkResource); err != nil {
		return err
	}

	indented := new(bytes.Buffer)
	if err := json.Indent(indented, obj, "", "    "); err != nil {
		return err
	}
	if _, err := io.Copy(cli.out, indented); err != nil {
		return err
	}
	return nil
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
