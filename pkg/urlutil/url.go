package urlutil

import "strings"

var validUrlPrefixes = []string{
	"http://",
	"https://",
}

// IsURL returns true if the provided str is a valid URL by doing
// a simple change for the transport of the url.
func IsURL(str string) bool {
	for _, prefix := range validUrlPrefixes {
		if strings.HasPrefix(str, prefix) {
			return true
		}
	}
	return false
}
