package registry

import "context"

// AuthHeader is the name of the header used to send encoded registry
// authorization credentials for registry operations (push/pull).
const AuthHeader = "X-Registry-Auth"

// RequestAuthConfig is a function interface that clients can supply
// to retry operations after getting an authorization error.
//
// The function must return the [AuthHeader] value ([AuthConfig]), encoded
// in base64url format ([RFC4648, section 5]), which can be decoded by
// [DecodeAuthConfig].
//
// It must return an error if the privilege request fails.
//
// [RFC4648, section 5]: https://tools.ietf.org/html/rfc4648#section-5
type RequestAuthConfig func(context.Context) (string, error)

// AuthConfig contains authorization information for connecting to a Registry.
type AuthConfig struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`

	ServerAddress string `json:"serveraddress,omitempty"`

	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`

	// RegistryToken is a bearer token to be sent to a registry
	RegistryToken string `json:"registrytoken,omitempty"`
}
