// +build !go1.15

package git

import "net/url"

// redactCredentials takes a URL and redacts a password from it.
// e.g. "https://user:password@github.com/user/private-repo-failure.git" will be changed to
// "https://user:xxxxx@github.com/user/private-repo-failure.git"
func redactCredentials(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s // string is not a URL, just return it
	}

	return urlRedacted(u)
}

// urlRedacted comes from go's url.Redacted() which isn't available on go < 1.15
func urlRedacted(u *url.URL) string {
	if u == nil {
		return ""
	}

	ru := *u
	if _, has := ru.User.Password(); has {
		ru.User = url.UserPassword(ru.User.Username(), "xxxxx")
	}
	return ru.String()
}
