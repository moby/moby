package client

import (
	"bytes"
	"fmt"
	"io"

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
	}
)

func (cli *NetworkCli) CmdNetwork(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "network", "COMMAND [OPTIONS] [arg...]", networkUsage(chain), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err == nil {
		cmd.Usage()
		return fmt.Errorf("Invalid command : %v", args)
	}
	return err
}

func networkUsage(chain string) string {
	help := "Commands:\n"

	for _, cmd := range networkCommands {
		help += fmt.Sprintf("    %-10.10s%s\n", cmd.name, cmd.description)
	}

	help += fmt.Sprintf("\nRun '%s network COMMAND --help' for more information on a command.", chain)
	return help
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
	// TODO : Proper Backend handling
	obj, _, err := readBody(cli.call("POST", "/networks/"+args[0], nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

// CmdNetworkRm handles Network Delete UI
func (cli *NetworkCli) CmdNetworkRm(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "rm", "NETWORK-NAME", "Deletes a network", false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	// TODO : Proper Backend handling
	obj, _, err := readBody(cli.call("DELETE", "/networks/"+args[0], nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
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
	// TODO : Proper Backend handling
	obj, _, err := readBody(cli.call("GET", "/networks", nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

// CmdNetworkInfo handles Network Info UI
func (cli *NetworkCli) CmdNetworkInfo(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "info", "NETWORK-NAME", "Displays detailed information on a network", false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	// TODO : Proper Backend handling
	obj, _, err := readBody(cli.call("GET", "/networks/"+args[0], nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

// CmdNetworkJoin handles the UI to let a Container join a Network via an endpoint
// Sample UI : <chain> network join <container-name/id> <network-name/id> [<endpoint-name>]
func (cli *NetworkCli) CmdNetworkJoin(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "join", "CONTAINER-NAME/ID NETWORK-NAME/ID [ENDPOINT-NAME]",
		chain+" join", false)
	cmd.Require(flag.Min, 2)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	// TODO : Proper Backend handling
	obj, _, err := readBody(cli.call("POST", "/endpoints/", nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}

// CmdNetworkLeave handles the UI to let a Container disconnect from a Network
// Sample UI : <chain> network leave <container-name/id> <network-name/id>
func (cli *NetworkCli) CmdNetworkLeave(chain string, args ...string) error {
	cmd := cli.Subcmd(chain, "leave", "CONTAINER-NAME/ID NETWORK-NAME/ID",
		chain+" leave", false)
	cmd.Require(flag.Min, 2)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	// TODO : Proper Backend handling
	obj, _, err := readBody(cli.call("PUT", "/endpoints/", nil, nil))
	if err != nil {
		fmt.Fprintf(cli.err, "%s", err.Error())
		return err
	}
	if _, err := io.Copy(cli.out, bytes.NewReader(obj)); err != nil {
		return err
	}
	return nil
}
