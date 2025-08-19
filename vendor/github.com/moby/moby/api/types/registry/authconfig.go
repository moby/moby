package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
)

// AuthHeader is the name of the header used to send encoded registry
// authorization credentials for registry operations (push/pull).
const AuthHeader = "X-Registry-Auth"

// RequestAuthConfig is a function interface that clients can supply
// to retry operations after getting an authorization error.
//
// The function must return the [AuthHeader] value ([AuthConfig]), encoded
// in base64url format ([RFC4648, section 5]).
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

	// Email is an optional value associated with the username.
	// This field is deprecated and will be removed in a later
	// version of docker.
	Email string `json:"email,omitempty"`

	ServerAddress string `json:"serveraddress,omitempty"`

	// IdentityToken is used to authenticate the user and get
	// an access token for the registry.
	IdentityToken string `json:"identitytoken,omitempty"`

	// RegistryToken is a bearer token to be sent to a registry
	RegistryToken string `json:"registrytoken,omitempty"`
}

// EncodeAuthConfig serializes the auth configuration as a base64url encoded
// ([RFC4648, section 5]) JSON string for sending through the X-Registry-Auth header.
//
// [RFC4648, section 5]: https://tools.ietf.org/html/rfc4648#section-5
func EncodeAuthConfig(authConfig AuthConfig) (string, error) {
	// Older daemons (or registries) may not handle an empty string,
	// which resulted in an "io.EOF" when unmarshaling or decoding.
	//
	// FIXME(thaJeztah): find exactly what code-paths are impacted by this.
	// if authConfig == (AuthConfig{}) { return "", nil }
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", errInvalidParameter{err}
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

type errInvalidParameter struct{ error }

func (errInvalidParameter) InvalidParameter() {}

func (e errInvalidParameter) Cause() error { return e.error }

func (e errInvalidParameter) Unwrap() error { return e.error }
