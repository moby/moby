package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// MinRequiredArgs checks if the minimum number of args exists, and returns an
// error if they do not.
func MinRequiredArgs(args []string, min int, cmd *cobra.Command) error {
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
