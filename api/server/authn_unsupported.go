// +build !linux !cgo gccgo

package server

import (
	"net/http"
)

func getUserFromHTTPResponseWriter(w http.ResponseWriter, options map[string]string) User {
	return User{}
}
