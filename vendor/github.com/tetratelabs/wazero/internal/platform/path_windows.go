package platform

import "strings"

// ToPosixPath returns the input, converting any backslashes to forward ones.
func ToPosixPath(in string) string {
	// strings.Map only allocates on change, which is good enough especially as
	// path.Join uses forward slash even on windows.
	return strings.Map(windowsToPosixSeparator, in)
}

func windowsToPosixSeparator(r rune) rune {
	if r == '\\' {
		return '/'
	}
	return r
}
