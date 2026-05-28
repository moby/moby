package resolver

import (
	"fmt"
	"net/url"
	"strings"
)

func extractMirrorHostAndPath(mirror string) (string, string) {
	var path string
	host := mirror

	u, err := url.Parse(mirror)
	if err != nil || u.Host == "" {
		u, err = url.Parse(fmt.Sprintf("//%s", mirror))
	}
	if err != nil || u.Host == "" {
		return host, path
	}

	return u.Host, strings.TrimRight(u.Path, "/")
}
