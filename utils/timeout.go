package utils

import (
	"net"
	"net/url"
)

// IsTimeout takes an error returned from (generally) the http package and determines if it is a timeout error.
func IsTimeout(err error) bool {
	switch e := err.(type) {
	case net.Error:
		return e.Timeout()
	case *url.Error:
		if t, ok := e.Err.(net.Error); ok {
			return t.Timeout()
		}
		return false
	default:
		return false
	}
}
