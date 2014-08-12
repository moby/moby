package opts

import (
	"fmt"
	"sort"
	"strings"
)

// Set is a set of unique strings which implements the
// flag.Value interface for command-line parsing.
type Set map[string]struct{}

func (s Set) Set(val string) error {
	s[val] = struct{}{}
	return nil
}

func (s Set) String() string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprintf("%s", strings.Join(keys, ","))
}
