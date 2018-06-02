package shell

import "strings"

// EqualEnvKeys compare two strings and returns true if they are equal. On
// Windows this comparison is case insensitive.
func EqualEnvKeys(from, to string) bool {
	return strings.ToUpper(from) == strings.ToUpper(to)
}
