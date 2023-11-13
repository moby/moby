package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// NoArgs validates args and returns an error if there are any args
func NoArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}

	if cmd.HasSubCommands() {
		return fmt.Errorf("\n" + strings.TrimRight(cmd.UsageString(), "\n"))
	}

	return fmt.Errorf(
		"\"%s\" accepts no argument(s).\nSee '%s --help'.\n\nUsage:  %s\n\n%s",
		cmd.CommandPath(),
		cmd.CommandPath(),
		cmd.UseLine(),
		cmd.Short,
	)
}
