package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	flag "github.com/docker/docker/pkg/mflag"
)

const (
	nullNetType = "null"
)

type command struct {
	name        string
	description string
}

var (
	networkCommands = []command{
		{"create", "Create a network"},
		{"rm", "Remove a network"},
		{"ls", "List all networks"},
		{"info", "Display information of a network"},
		{"service create", "Create a service endpoint"},
		{"service rm", "Remove a service endpoint"},
		{"service join", "Join a container to a service endpoint"},
		{"service leave", "Leave a container from a service endpoint"},
		{"service ls", "Lists all service endpoints on a network"},
		{"service info", "Display information of a service endpoint"},
	}
)

// CmdNetwork handles the root Network UI
func (cli *NetworkCli) CmdNetwork(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "network", "COMMAND [OPTIONS] [arg...]", networkUsage(chain), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err == nil {
		cmd.Usage()
		return fmt.Errorf("invalid command : %v", args)
	}
	return err
}

// CmdNetworkCreate handles Network Create UI
func (cli *NetworkCli) CmdNetworkCreate(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "create", "NETWORK-NAME", "Creates a new network with a name specified by the user", false)
	flDriver := cmd.String([]string{"d", "-driver"}, "null", "Driver to manage the Network")
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	if *flDriver == "" {
		*flDriver = nullNetType
	}

	nc := networkCreate{Name: cmd.Arg(0), NetworkType: *flDriver}

	obj, _, err := readBody(cli.call("POST", "/networks", nc, nil))
	if err != nil {
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

// CmdNetworkRm handles Network Delete UI
func (cli *NetworkCli) CmdNetworkRm(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "rm", "NETWORK", "Deletes a network", false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	id, err := lookupNetworkID(cli, cmd.Arg(0))
	if err != nil {
		return err
	}
	obj, _, err := readBody(cli.call("DELETE", "/networks/"+id, nil, nil))
	if err != nil {
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

// CmdNetworkLs handles Network List UI
func (cli *NetworkCli) CmdNetworkLs(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "ls", "", "Lists all the networks created by the user", false)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	obj, _, err := readBody(cli.call("GET", "/networks", nil, nil))
	if err != nil {
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

// CmdNetworkInfo handles Network Info UI
func (cli *NetworkCli) CmdNetworkInfo(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "info", "NETWORK", "Displays detailed information on a network", false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	id, err := lookupNetworkID(cli, cmd.Arg(0))
	if err != nil {
		return err
	}

	obj, _, err := readBody(cli.call("GET", "/networks/"+id, nil, nil))
	if err != nil {
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

// Helper function to predict if a string is a name or id or partial-id
// This provides a best-effort mechanism to identify a id with the help of GET Filter APIs
// Being a UI, its most likely that name will be used by the user, which is used to lookup
// the corresponding ID. If ID is not found, this function will assume that the passed string
// is an ID by itself.

func lookupNetworkID(cli *NetworkCli, nameID string) (string, error) {
	obj, statusCode, err := readBody(cli.call("GET", "/networks?name="+nameID, nil, nil))
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
	obj, statusCode, err = readBody(cli.call("GET", "/networks?partial-id="+nameID, nil, nil))
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
		return "", fmt.Errorf("multiple Networks matching the partial identifier (%s). Please use full identifier", nameID)
	}
	return list[0].ID, nil
}

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
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
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

	obj, _, err := readBody(cli.call("DELETE", "/networks/"+networkID+"/endpoints/"+serviceID, nil, nil))
	if err != nil {
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

// CmdNetworkServiceLs handles service list UI
func (cli *NetworkCli) CmdNetworkServiceLs(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "ls", "NETWORK", "Lists all the services on a network", false)
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
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
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
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
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

	obj, _, err := readBody(cli.call("POST", "/networks/"+networkID+"/endpoints/"+serviceID+"/containers", nc, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
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

	obj, _, err := readBody(cli.call("DELETE", "/networks/"+networkID+"/endpoints/"+serviceID+"/containers/"+containerID, nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

func networkUsage(chain string) string {
	help := "Commands:\n"

	for _, cmd := range networkCommands {
		help += fmt.Sprintf("    %-25.25s%s\n", cmd.name, cmd.description)
	}

	help += fmt.Sprintf("\nRun '%s network COMMAND --help' for more information on a command.", chain)
	return help
}

func serviceUsage(chain string) string {
	help := "Commands:\n"

	for _, cmd := range networkCommands {
		if strings.HasPrefix(cmd.name, "service ") {
			command := strings.SplitAfter(cmd.name, "service ")
			help += fmt.Sprintf("    %-10.10s%s\n", command[1], cmd.description)
		}
	}

	help += fmt.Sprintf("\nRun '%s service COMMAND --help' for more information on a command.", chain)
	return help
}
