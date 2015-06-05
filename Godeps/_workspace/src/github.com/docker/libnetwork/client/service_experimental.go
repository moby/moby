// +build experimental

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"text/tabwriter"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/stringid"
)

var (
	serviceCommands = []command{
		{"create", "Create a service endpoint"},
		{"rm", "Remove a service endpoint"},
		{"join", "Join a container to a service endpoint"},
		{"leave", "Leave a container from a service endpoint"},
		{"ls", "Lists all service endpoints on a network"},
		{"info", "Display information of a service endpoint"},
	}
)

func lookupServiceID(cli *NetworkCli, networkID string, nameID string) (string, error) {
	obj, statusCode, err := readBody(cli.call("GET", fmt.Sprintf("/networks/%s/endpoints?name=%s", networkID, nameID), nil, nil))
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("name query failed for %s due to : statuscode(%d) %v", nameID, statusCode, string(obj))
	}

	var list []*networkResource
	err = json.Unmarshal(obj, &list)
	if err != nil {
		return "", err
	}
	if len(list) > 0 {
		// name query filter will always return a single-element collection
		return list[0].ID, nil
	}

	// Check for Partial-id
	obj, statusCode, err = readBody(cli.call("GET", fmt.Sprintf("/networks/%s/endpoints?partial-id=%s", networkID, nameID), nil, nil))
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("partial-id match query failed for %s due to : statuscode(%d) %v", nameID, statusCode, string(obj))
	}

	err = json.Unmarshal(obj, &list)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("resource not found %s", nameID)
	}
	if len(list) > 1 {
		return "", fmt.Errorf("multiple services matching the partial identifier (%s). Please use full identifier", nameID)
	}
	return list[0].ID, nil
}

func lookupContainerID(cli *NetworkCli, nameID string) (string, error) {
	// TODO : containerID to sandbox-key ?
	return nameID, nil
}

// CmdNetworkService handles the network service UI
func (cli *NetworkCli) CmdNetworkService(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "service", "COMMAND [OPTIONS] [arg...]", serviceUsage(chain), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err == nil {
		cmd.Usage()
		return fmt.Errorf("Invalid command : %v", args)
	}
	return err
}

// CmdNetworkServiceCreate handles service create UI
func (cli *NetworkCli) CmdNetworkServiceCreate(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "create", "SERVICE NETWORK", "Creates a new service on a network", false)
	cmd.Require(flag.Min, 2)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	networkID, err := lookupNetworkID(cli, cmd.Arg(1))
	if err != nil {
		return err
	}

	ec := endpointCreate{Name: cmd.Arg(0), NetworkID: networkID}

	obj, _, err := readBody(cli.call("POST", "/networks/"+networkID+"/endpoints", ec, nil))
	if err != nil {
		return err
	}

	var replyID string
	err = json.Unmarshal(obj, &replyID)
	if err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "%s\n", replyID)
	return nil
}

// CmdNetworkServiceRm handles service delete UI
func (cli *NetworkCli) CmdNetworkServiceRm(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "rm", "SERVICE NETWORK", "Deletes a service", false)
	cmd.Require(flag.Min, 2)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	networkID, err := lookupNetworkID(cli, cmd.Arg(1))
	if err != nil {
		return err
	}

	serviceID, err := lookupServiceID(cli, networkID, cmd.Arg(0))
	if err != nil {
		return err
	}

	_, _, err = readBody(cli.call("DELETE", "/networks/"+networkID+"/endpoints/"+serviceID, nil, nil))
	if err != nil {
		return err
	}
	return nil
}

// CmdNetworkServiceLs handles service list UI
func (cli *NetworkCli) CmdNetworkServiceLs(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "ls", "NETWORK", "Lists all the services on a network", false)
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only display numeric IDs")
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Do not truncate the output")
	nLatest := cmd.Bool([]string{"l", "-latest"}, false, "Show the latest network created")
	last := cmd.Int([]string{"n"}, -1, "Show n last created networks")
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	cmd.Require(flag.Min, 1)

	networkID, err := lookupNetworkID(cli, cmd.Arg(0))
	if err != nil {
		return err
	}

	obj, _, err := readBody(cli.call("GET", "/networks/"+networkID+"/endpoints", nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	if *last == -1 && *nLatest {
		*last = 1
	}

	var endpointResources []endpointResource
	err = json.Unmarshal(obj, &endpointResources)
	if err != nil {
		return err
	}

	wr := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	// unless quiet (-q) is specified, print field titles
	if !*quiet {
		fmt.Fprintln(wr, "NETWORK SERVICE ID\tNAME\tNETWORK")
	}

	for _, networkResource := range endpointResources {
		ID := networkResource.ID
		netName := networkResource.Name
		if !*noTrunc {
			ID = stringid.TruncateID(ID)
		}
		if *quiet {
			fmt.Fprintln(wr, ID)
			continue
		}
		network := networkResource.Network
		fmt.Fprintf(wr, "%s\t%s\t%s",
			ID,
			netName,
			network)
		fmt.Fprint(wr, "\n")
	}
	wr.Flush()

	return nil
}

// CmdNetworkServiceInfo handles service info UI
func (cli *NetworkCli) CmdNetworkServiceInfo(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "info", "SERVICE NETWORK", "Displays detailed information on a service", false)
	cmd.Require(flag.Min, 2)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	networkID, err := lookupNetworkID(cli, cmd.Arg(1))
	if err != nil {
		return err
	}

	serviceID, err := lookupServiceID(cli, networkID, cmd.Arg(0))
	if err != nil {
		return err
	}

	obj, _, err := readBody(cli.call("GET", "/networks/"+networkID+"/endpoints/"+serviceID, nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}

	endpointResource := &endpointResource{}
	if err := json.NewDecoder(bytes.NewReader(obj)).Decode(endpointResource); err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "Service Id: %s\n", endpointResource.ID)
	fmt.Fprintf(cli.out, "\tName: %s\n", endpointResource.Name)
	fmt.Fprintf(cli.out, "\tNetwork: %s\n", endpointResource.Network)

	return nil
}

// CmdNetworkServiceJoin handles service join UI
func (cli *NetworkCli) CmdNetworkServiceJoin(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "join", "CONTAINER SERVICE NETWORK", "Sets a container as a service backend", false)
	cmd.Require(flag.Min, 3)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	containerID, err := lookupContainerID(cli, cmd.Arg(0))
	if err != nil {
		return err
	}

	networkID, err := lookupNetworkID(cli, cmd.Arg(2))
	if err != nil {
		return err
	}

	serviceID, err := lookupServiceID(cli, networkID, cmd.Arg(1))
	if err != nil {
		return err
	}

	nc := endpointJoin{ContainerID: containerID}

	_, _, err = readBody(cli.call("POST", "/networks/"+networkID+"/endpoints/"+serviceID+"/containers", nc, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	return nil
}

// CmdNetworkServiceLeave handles service leave UI
func (cli *NetworkCli) CmdNetworkServiceLeave(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "leave", "CONTAINER SERVICE NETWORK", "Removes a container from service backend", false)
	cmd.Require(flag.Min, 3)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	containerID, err := lookupContainerID(cli, cmd.Arg(0))
	if err != nil {
		return err
	}

	networkID, err := lookupNetworkID(cli, cmd.Arg(2))
	if err != nil {
		return err
	}

	serviceID, err := lookupServiceID(cli, networkID, cmd.Arg(1))
	if err != nil {
		return err
	}

	_, _, err = readBody(cli.call("DELETE", "/networks/"+networkID+"/endpoints/"+serviceID+"/containers/"+containerID, nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	return nil
}

func serviceUsage(chain string) string {
	help := "Commands:\n"

	for _, cmd := range serviceCommands {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd, cmd.description)
	}

	help += fmt.Sprintf("\nRun '%s service COMMAND --help' for more information on a command.", chain)
	return help
}
