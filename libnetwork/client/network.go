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

// CmdNetwork handles the root Network UI
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

	obj, _, err := readBody(cli.call("POST", "/networks/name/"+cmd.Arg(0), nc, nil))
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
	obj, _, err := readBody(cli.call("DELETE", "/networks/name/"+cmd.Arg(0), nil, nil))
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
	obj, _, err := readBody(cli.call("GET", "/networks/name/"+cmd.Arg(0), nil, nil))
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
		help += fmt.Sprintf("    %-10.10s%s\n", cmd.name, cmd.description)
	}

	help += fmt.Sprintf("\nRun '%s network COMMAND --help' for more information on a command.", chain)
	return help
}
