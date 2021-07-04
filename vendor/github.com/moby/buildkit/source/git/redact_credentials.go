// +build go1.15

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

	return u.Redacted()
}
