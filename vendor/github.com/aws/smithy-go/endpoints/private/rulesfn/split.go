package rulesfn

import "strings"

// Split splits the input string by the delimiter and returns the resulting
// parts. If limit is > 0, at most limit substrings are returned.
// Returns a slice with a single empty string if the input is empty.
func Split(input, delimiter string, limit int) []string {
	if len(input) == 0 {
		return []string{""}
	}
	if limit > 0 {
		return strings.SplitN(input, delimiter, limit)
	}
	return strings.Split(input, delimiter)
}
