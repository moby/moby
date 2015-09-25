package client

import (
	"fmt"
	Cli "github.com/docker/docker/cli"
	"sort"
	"strings"
)

func IsComplexAlias(aliasCmd string) bool {
	return strings.Index(aliasCmd, "!") == 0
}

func outputAlias(cli *DockerCli, key string, command []string) {
	fmt.Fprintf(cli.out, "alias %s=%s\n", key, strings.Join(command, " "))
}

func listAliases(cli *DockerCli, listOnly bool) {
	var keys []string
	for key := range cli.configFile.Aliases {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		command := cli.configFile.Aliases[key]
		if listOnly {
			fmt.Fprintf(cli.out, "%s\n", key)
		} else {
			outputAlias(cli, key, command)
		}

	}
}

func expandAlias(cli *DockerCli, key string, showOnlyCommand bool) error {
	if command, exists := cli.configFile.Aliases[key]; exists {
		if showOnlyCommand {
			fmt.Fprintf(cli.out, "%s\n", strings.Join(command, " "))
		} else {
			outputAlias(cli, key, command)
		}
	} else {
		fmt.Fprintf(cli.out, "Alias %s does not exist\n", key)
	}
	return nil
}

func deleteAlias(cli *DockerCli, key string) error {
	delete(cli.configFile.Aliases, key)
	if err := cli.configFile.Save(); err != nil {
		return fmt.Errorf("Error saving config file: %v", err)
	} else {
		fmt.Fprintf(cli.out, "Alias %s has been deleted\n", key)
	}
	return nil
}

func saveAlias(cli *DockerCli, alias string, command []string) error {
	if cli.configFile.Aliases == nil {
		cli.configFile.Aliases = map[string][]string{}
	}
	cli.configFile.Aliases[alias] = command
	if err := cli.configFile.Save(); err != nil {
		return fmt.Errorf("Error saving config file: %v", err)
	} else {
		fmt.Fprintf(cli.out, "Alias %v has been updated\n", alias)
	}
	return nil
}

// CmdAlias Manage the aliases.
//
// Usage: docker alias [OPTIONS] [ALIAS] [COMMANDS]
func (cli *DockerCli) CmdAlias(args ...string) error {
	cmd := Cli.Subcmd("alias", nil, "Manage the aliases", true)

	listOnly := cmd.Bool([]string{"l", "-list"}, false, "List all aliases")
	expand := cmd.String([]string{"e", "-expand"}, "", "Expand the specified alias")
	toDelete := cmd.String([]string{"d", "-delete"}, "", "Delete the specified alias")

	cmd.ParseFlags(args, true)

	if *toDelete != "" {
		return deleteAlias(cli, *toDelete)
	}

	if *expand != "" {
		return expandAlias(cli, *expand, true)
	}

	if cmd.NArg() == 1 {
		return expandAlias(cli, cmd.Arg(0), false)
	}

	if cmd.NArg() == 0 || *listOnly {
		listAliases(cli, *listOnly)
		return nil
	}

	return saveAlias(cli, cmd.Args()[0], cmd.Args()[1:])
}
