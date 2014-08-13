package mflag

import (
	"fmt"
	"sort"
	"strings"
)

// StringSet is a set of unique strings which implements the
// Value interface for command-line parsing.
type StringSet map[string]struct{}

func (s StringSet) Set(val string) error {
	s[val] = struct{}{}
	return nil
}

func (s StringSet) String() string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprintf("%s", strings.Join(keys, ","))
}
