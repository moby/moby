package system

import (
	"strings"

	"golang.org/x/sys/windows"
)

// EscapeArgs makes a Windows-style escaped command line from a set of arguments
//
// Deprecated: this function is no longer used and will be removed in the next release.
func EscapeArgs(args []string) string {
	escapedArgs := make([]string, len(args))
	for i, a := range args {
		escapedArgs[i] = windows.EscapeArg(a)
	}
	return strings.Join(escapedArgs, " ")
}
