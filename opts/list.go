package opts

import (
	"fmt"

	flag "github.com/docker/docker/pkg/mflag"
)

// ListVar defines a "list of strings" flag with specified name, default value, and
// usage string. The argument p points to a string variable in which to
// store the value of the flag.
func ListVar(values *[]string, names []string, usage string) {
	flag.Var((*List)(values), names, usage)
}

// Set is a list of strings which implements the
// flag.Value interface for command-line parsing.
type List []string

func (l *List) Set(val string) error {
	(*l) = append(*l, val)
	return nil
}

func (l *List) String() string {
	return fmt.Sprintf("%v", *l)
}
