// +build experimental

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	flag "github.com/docker/docker/pkg/mflag"
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

func serviceUsage(chain string) string {
	help := "Commands:\n"

	for _, cmd := range serviceCommands {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd, cmd.description)
	}

	help += fmt.Sprintf("\nRun '%s service COMMAND --help' for more information on a command.", chain)
	return help
}
