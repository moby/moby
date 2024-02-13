package shell

import "strings"

// EqualEnvKeys compare two strings and returns true if they are equal.
// On Unix this comparison is case-sensitive.
// On Windows this comparison is case-insensitive.
func EqualEnvKeys(from, to string) bool {
	return strings.EqualFold(from, to)
}
