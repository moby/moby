// Package urlutil provides helper function to check urls kind.
// It supports http urls, git urls and transport url (tcp://, …)
package urlutil // import "github.com/docker/docker/pkg/urlutil"

import (
	"regexp"
	"strings"
)

var (
	validPrefixes = map[string][]string{
		"url": {"http://", "https://"},

		// The github.com/ prefix is a special case used to treat context-paths
		// starting with `github.com` as a git URL if the given path does not
		// exist locally. The "github.com/" prefix is kept for backward compatibility,
		// and is a legacy feature.
		//
		// Going forward, no additional prefixes should be added, and users should
		// be encouraged to use explicit URLs (https://github.com/user/repo.git) instead.
		"git":       {"git://", "github.com/", "git@"},
		"transport": {"tcp://", "tcp+tls://", "udp://", "unix://", "unixgram://"},
	}
	urlPathWithFragmentSuffix = regexp.MustCompile(".git(?:#.+)?$")
)

// IsURL returns true if the provided str is an HTTP(S) URL.
func IsURL(str string) bool {
	return checkURL(str, "url")
}

// IsGitURL returns true if the provided str is a git repository URL.
func IsGitURL(str string) bool {
	if IsURL(str) && urlPathWithFragmentSuffix.MatchString(str) {
		return true
	}
	return checkURL(str, "git")
}

// IsTransportURL returns true if the provided str is a transport (tcp, tcp+tls, udp, unix) URL.
func IsTransportURL(str string) bool {
	return checkURL(str, "transport")
}

func checkURL(str, kind string) bool {
	for _, prefix := range validPrefixes[kind] {
		if strings.HasPrefix(str, prefix) {
			return true
		}
	}
	return false
}
