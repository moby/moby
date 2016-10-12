package authentication

import (
	"net/http"
)

// PluginImplements is the type of subsystem plugin that we look for.
const (
	PluginImplements          = "Authentication"
	AuthenticationRequestName = PluginImplements + ".Authenticate"
	SetOptionsRequestName     = PluginImplements + ".SetOptions"
)

// User represents an authenticated remote user.  We know at least one of the
// user's name (if not "") and UID (if HaveUID is true), and possibly both.
type User struct {
	Name    string    `json:",omitempty"`
	HaveUID bool      `json:",omitempty"`
	UID     uint32    `json:",omitempty"`
	Groups  *[]string `json:",omitempty"`
	Scheme  string    `json:",omitempty"`
}

// AuthnPluginAuthenticateRequest is the structure that we pass to an
// authentication plugin to have it authenticate a request.  It contains the
// incoming request's method, the Scheme, Path, Fragment, RawQuery, and RawPath
// fields of the request's net.http.URL, stored in a map, the hostname, all of
// the request's headers, and the peer's certificate if the connection provided
// a verified client certificate.
type AuthnPluginAuthenticateRequest struct {
	Method      string      `json:",omitempty"`
	URL         string      `json:",omitempty"`
	Host        string      `json:",omitempty"`
	Header      http.Header `json:",omitempty"`
	Certificate []byte      `json:",omitempty"`
}

// AuthnPluginAuthenticateResponse is the structure that we get back from an
// authentication request.  If authentication suceeded, it contains information
// about the authenticated user.  If authentication succeeded, only header
// values returned by the plugin which succeeded will be included in the
// response which is sent to the client.  If authentication fails, all headers
// returned by all called plugins will be included in the response.
type AuthnPluginAuthenticateResponse struct {
	AuthedUser User        `json:",omitempty"`
	Header     http.Header `json:",omitempty"`
}

// AuthnPluginSetOptionsRequest is the structure that we use to pass
// authentication options to a plugin.
type AuthnPluginSetOptionsRequest struct {
	Options map[string]string `json:",omitempty"`
}

// AuthnPluginSetOptionsResponse is the structure that we get back from a
// set-options request.
type AuthnPluginSetOptionsResponse struct {
}
