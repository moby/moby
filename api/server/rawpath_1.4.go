// +build !go1.5

package server

import "net/url"

func getRawPath(url *url.URL) string {
	return ""
}
