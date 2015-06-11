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
		{"publish", "Publish a service"},
		{"unpublish", "Remove a service"},
		{"attach", "Attach a backend (container) to the service"},
		{"detach", "Detach the backend from the service"},
		{"ls", "Lists all services"},
		{"info", "Display information about a service"},
	}
)

func lookupServiceID(cli *NetworkCli, nwName, svNameID string) (string, error) {
	// Sanity Check
	obj, _, err := readBody(cli.call("GET", fmt.Sprintf("/networks?name=%s", nwName), nil, nil))
	if err != nil {
		return "", err
	}
	var nwList []networkResource
	if err = json.Unmarshal(obj, &nwList); err != nil {
		return "", err
	}
	if len(nwList) == 0 {
		return "", fmt.Errorf("Network %s does not exist", nwName)
	}

	// Query service by name
	obj, statusCode, err := readBody(cli.call("GET", fmt.Sprintf("/services?name=%s", svNameID), nil, nil))
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("name query failed for %s due to: (%d) %s", svNameID, statusCode, string(obj))
	}

	var list []*serviceResource
	if err = json.Unmarshal(obj, &list); err != nil {
		return "", err
	}
	for _, sr := range list {
		if sr.Network == nwName {
			return sr.ID, nil
		}
	}

	// Query service by Partial-id (this covers full id as well)
	obj, statusCode, err = readBody(cli.call("GET", fmt.Sprintf("/services?partial-id=%s", svNameID), nil, nil))
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("partial-id match query failed for %s due to: (%d) %s", svNameID, statusCode, string(obj))
	}

	if err = json.Unmarshal(obj, &list); err != nil {
		return "", err
	}
	for _, sr := range list {
		if sr.Network == nwName {
			return sr.ID, nil
		}
	}

	return "", fmt.Errorf("Service %s not found on network %s", svNameID, nwName)
}

func lookupContainerID(cli *NetworkCli, cnNameID string) (string, error) {
	// TODO : containerID to sandbox-key ?
	return cnNameID, nil
}

// CmdService handles the service UI
func (cli *NetworkCli) CmdService(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "service", "COMMAND [OPTIONS] [arg...]", serviceUsage(chain), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err == nil {
		cmd.Usage()
		return fmt.Errorf("Invalid command : %v", args)
	}
	return err
}

// CmdServicePublish handles service create UI
func (cli *NetworkCli) CmdServicePublish(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "publish", "SERVICE", "Publish a new service on a network", false)
	flNetwork := cmd.String([]string{"net", "-network"}, "", "Network where to publish the service")
	cmd.Require(flag.Min, 1)

	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Default network changes will come later
	nw := "docker0"
	if *flNetwork != "" {
		nw = *flNetwork
	}

	sc := serviceCreate{Name: cmd.Arg(0), Network: nw}
	obj, _, err := readBody(cli.call("POST", "/services", sc, nil))
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

// CmdServiceUnpublish handles service delete UI
func (cli *NetworkCli) CmdServiceUnpublish(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "unpublish", "SERVICE", "Removes a service", false)
	flNetwork := cmd.String([]string{"net", "-network"}, "", "Network where to publish the service")
	cmd.Require(flag.Min, 1)

	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Default network changes will come later
	nw := "docker0"
	if *flNetwork != "" {
		nw = *flNetwork
	}

	serviceID, err := lookupServiceID(cli, nw, cmd.Arg(0))
	if err != nil {
		return err
	}

	_, _, err = readBody(cli.call("DELETE", "/services/"+serviceID, nil, nil))

	return err
}

// CmdServiceLs handles service list UI
func (cli *NetworkCli) CmdServiceLs(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "ls", "SERVICE", "Lists all the services on a network", false)
	flNetwork := cmd.String([]string{"net", "-network"}, "", "Only show the services that are published on the specified network")
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only display numeric IDs")
	noTrunc := cmd.Bool([]string{"#notrunc", "-no-trunc"}, false, "Do not truncate the output")

	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	cmd.Require(flag.Min, 1)

	var obj []byte
	if *flNetwork == "" {
		obj, _, err = readBody(cli.call("GET", "/services", nil, nil))
	} else {
		obj, _, err = readBody(cli.call("GET", "/services?network="+*flNetwork, nil, nil))
	}
	if err != nil {
		return err
	}

	var serviceResources []serviceResource
	err = json.Unmarshal(obj, &serviceResources)
	if err != nil {
		fmt.Println(err)
		return err
	}

	wr := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	// unless quiet (-q) is specified, print field titles
	if !*quiet {
		fmt.Fprintln(wr, "SERVICE ID\tNAME\tNETWORK\tCONTAINER")
	}

	for _, sr := range serviceResources {
		ID := sr.ID
		bkID, err := getBackendID(cli, ID)
		if err != nil {
			return err
		}
		if !*noTrunc {
			ID = stringid.TruncateID(ID)
			bkID = stringid.TruncateID(bkID)
		}
		if !*quiet {
			fmt.Fprintf(wr, "%s\t%s\t%s\t%s\n", ID, sr.Name, sr.Network, bkID)
		} else {
			fmt.Fprintln(wr, ID)
		}
	}
	wr.Flush()

	return nil
}

func getBackendID(cli *NetworkCli, servID string) (string, error) {
	var (
		obj []byte
		err error
		bk  string
	)

	if obj, _, err = readBody(cli.call("GET", "/services/"+servID+"/backend", nil, nil)); err == nil {
		var bkl []backendResource
		if err := json.NewDecoder(bytes.NewReader(obj)).Decode(&bkl); err == nil {
			if len(bkl) > 0 {
				bk = bkl[0].ID
			}
		} else {
			// Only print a message, don't make the caller cli fail for this
			fmt.Fprintf(cli.out, "Failed to retrieve backend list for service %s (%v)", servID, err)
		}
	}

	return bk, err
}

// CmdServiceInfo handles service info UI
func (cli *NetworkCli) CmdServiceInfo(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "info", "SERVICE", "Displays detailed information about a service", false)
	flNetwork := cmd.String([]string{"net", "-network"}, "", "Network where to publish the service")
	cmd.Require(flag.Min, 1)

	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Default network changes will come later
	nw := "docker0"
	if *flNetwork != "" {
		nw = *flNetwork
	}

	serviceID, err := lookupServiceID(cli, nw, cmd.Arg(0))
	if err != nil {
		return err
	}

	obj, _, err := readBody(cli.call("GET", "/services/"+serviceID, nil, nil))
	if err != nil {
		return err
	}

	sr := &serviceResource{}
	if err := json.NewDecoder(bytes.NewReader(obj)).Decode(sr); err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "Service Id: %s\n", sr.ID)
	fmt.Fprintf(cli.out, "\tName: %s\n", sr.Name)
	fmt.Fprintf(cli.out, "\tNetwork: %s\n", sr.Network)

	return nil
}

// CmdServiceAttach handles service attach UI
func (cli *NetworkCli) CmdServiceAttach(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "attach", "CONTAINER SERVICE", "Sets a container as a service backend", false)
	flNetwork := cmd.String([]string{"net", "-network"}, "", "Network where to publish the service")
	cmd.Require(flag.Min, 2)

	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Default network changes will come later
	nw := "docker0"
	if *flNetwork != "" {
		nw = *flNetwork
	}

	containerID, err := lookupContainerID(cli, cmd.Arg(0))
	if err != nil {
		return err
	}

	serviceID, err := lookupServiceID(cli, nw, cmd.Arg(1))
	if err != nil {
		return err
	}

	nc := serviceAttach{ContainerID: containerID}

	_, _, err = readBody(cli.call("POST", "/services/"+serviceID+"/backend", nc, nil))

	return err
}

// CmdServiceDetach handles service detach UI
func (cli *NetworkCli) CmdServiceDetach(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "detach", "CONTAINER SERVICE", "Removes a container from service backend", false)
	flNetwork := cmd.String([]string{"net", "-network"}, "", "Network where to publish the service")
	cmd.Require(flag.Min, 2)

	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Default network changes will come later
	nw := "docker0"
	if *flNetwork != "" {
		nw = *flNetwork
	}

	containerID, err := lookupContainerID(cli, cmd.Arg(0))
	if err != nil {
		return err
	}

	serviceID, err := lookupServiceID(cli, nw, cmd.Arg(1))
	if err != nil {
		return err
	}

	_, _, err = readBody(cli.call("DELETE", "/services/"+serviceID+"/backend/"+containerID, nil, nil))
	if err != nil {
		return err
	}
	return nil
}

func serviceUsage(chain string) string {
	help := "Commands:\n"

	for _, cmd := range serviceCommands {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd.name, cmd.description)
	}

	help += fmt.Sprintf("\nRun '%s service COMMAND --help' for more information on a command.", chain)
	return help
}
