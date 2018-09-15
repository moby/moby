package client // import "github.com/docker/docker/client"

import (
	"net/url"
	"strings"
)

// FilterURL filters url to avoid invalid req
func FilterURL(name string) string {
	bldot := false
	if strings.HasSuffix(name, "/..") {
		bldot = true
	}
	if strings.HasSuffix(name, "/.") {
		bldot = true
	}
	if strings.HasPrefix(name, "../") {
		bldot = true
	}
	if strings.HasPrefix(name, "./") {
		bldot = true
	}
	if strings.Contains(name, "/../") {
		bldot = true
	}
	if strings.Contains(name, "/./") {
		bldot = true
	}
	if bldot {
		if strings.HasPrefix(name, "/") {
			return "/" + url.PathEscape(strings.TrimPrefix(name, "/"))
		}
		return url.PathEscape(name)
	}
	prefix := ""
	if strings.HasPrefix(name, "/") {
		prefix = "/"
		name = strings.TrimPrefix(name, "/")
	}
	nameParts := strings.Split(name, "/")
	for idx, str := range nameParts {
		nameParts[idx] = url.PathEscape(str)
	}
	return prefix + strings.Join(nameParts, "/")
}
