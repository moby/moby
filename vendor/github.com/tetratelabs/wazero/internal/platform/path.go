//go:build !windows

package platform

// ToPosixPath returns the input, as only windows might return backslashes.
func ToPosixPath(in string) string { return in }
