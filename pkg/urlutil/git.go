package urlutil

import "strings"

var (
	validPrefixes = []string{
		"git://",
		"github.com/",
		"git@",
	}
)

// IsGitURL returns true if the provided str is a git repository URL.
func IsGitURL(str string) bool {
	if IsURL(str) && strings.HasSuffix(str, ".git") {
		return true
	}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(str, prefix) {
			return true
		}
	}
	return false
}

// IsGitTransport returns true if the provided str is a git transport by inspecting
// the prefix of the string for known protocols used in git.
func IsGitTransport(str string) bool {
	return IsURL(str) || strings.HasPrefix(str, "git://") || strings.HasPrefix(str, "git@")
}
