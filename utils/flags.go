package utils

import (
	"fmt"
	"os"

	flag "github.com/docker/docker/pkg/mflag"
)

// ParseFlags is a utility function that adds a help flag if withHelp is true,
// calls cmd.Parse(args) and prints a relevant error message if there are incorrect number of arguments.
// TODO: move this to a better package than utils
func ParseFlags(cmd *flag.FlagSet, args []string, withHelp bool) error {
	var help *bool
	if withHelp {
		help = cmd.Bool([]string{"#help", "-help"}, false, "Print usage")
	}
	if err := cmd.Parse(args); err != nil {
		return err
	}
	if help != nil && *help {
		cmd.Usage()
		// just in case Usage does not exit
		os.Exit(0)
	}
	if str := cmd.CheckArgs(); str != "" {
		if withHelp {
			str += ". See 'docker " + cmd.Name() + " --help'"
		}
		fmt.Fprintf(cmd.Out(), "docker: %s.\n", str)
		os.Exit(1)
	}
	return nil
}
