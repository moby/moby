package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// NoArgs validate args and returns an error if there are any args
func NoArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}

	if cmd.HasSubCommands() {
		return fmt.Errorf("\n" + strings.TrimRight(cmd.UsageString(), "\n"))
	}

	return fmt.Errorf(
		"\"%s\" accepts no argument(s).\n\nUsage:  %s\n\n%s",
		cmd.CommandPath(),
		cmd.UseLine(),
		cmd.Short,
	)
}

// RequiresMinArgs returns an error if there is not at least min args
func RequiresMinArgs(min int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) >= min {
			return nil
		}
		return fmt.Errorf(
			"\"%s\" requires at least %d argument(s).\n\nUsage:  %s\n\n%s",
			cmd.CommandPath(),
			min,
			cmd.UseLine(),
			cmd.Short,
		)
	}
}
