package shell

import "strings"

// EqualEnvKeys compare two strings and returns true if they are equal.
// On Unix this comparison is case-sensitive.
// On Windows this comparison is case-insensitive.
func EqualEnvKeys(from, to string) bool {
	return strings.EqualFold(from, to)
}

// NormalizeEnvKey returns the key in a normalized form that can be used
// for comparison. On Unix this is a no-op. On Windows this converts the
// key to uppercase.
func NormalizeEnvKey(key string) string {
	return strings.ToUpper(key)
}
