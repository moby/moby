// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package registry

import (
	"io"

	"github.com/moby/moby/api/pkg/authconfig"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
)

// AuthHeader is the name of the header used to send encoded registry
// authorization credentials for registry operations (push/pull).
const AuthHeader = registry.AuthHeader

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
type RequestAuthConfig = registry.RequestAuthConfig

// AuthConfig contains authorization information for connecting to a Registry.
type AuthConfig = registry.AuthConfig

// EncodeAuthConfig serializes the auth configuration as a base64url encoded
// ([RFC4648, section 5]) JSON string for sending through the X-Registry-Auth header.
//
// [RFC4648, section 5]: https://tools.ietf.org/html/rfc4648#section-5
func EncodeAuthConfig(authConfig registry.AuthConfig) (string, error) {
	return authconfig.Encode(authConfig)
}

// DecodeAuthConfig decodes base64url encoded ([RFC4648, section 5]) JSON
// authentication information as sent through the X-Registry-Auth header.
//
// This function always returns an [AuthConfig], even if an error occurs. It is up
// to the caller to decide if authentication is required, and if the error can
// be ignored.
//
// [RFC4648, section 5]: https://tools.ietf.org/html/rfc4648#section-5
func DecodeAuthConfig(authEncoded string) (*registry.AuthConfig, error) {
	return authconfig.Decode(authEncoded)
}

// DecodeAuthConfigBody decodes authentication information as sent as JSON in the
// body of a request. This function is to provide backward compatibility with old
// clients and API versions. Current clients and API versions expect authentication
// to be provided through the X-Registry-Auth header.
//
// Like [DecodeAuthConfig], this function always returns an [AuthConfig], even if an
// error occurs. It is up to the caller to decide if authentication is required,
// and if the error can be ignored.
func DecodeAuthConfigBody(rdr io.ReadCloser) (*registry.AuthConfig, error) {
	return authconfig.DecodeRequestBody(rdr)
}

// AuthenticateOKBody authenticate o k body
type AuthenticateOKBody = registry.AuthenticateOKBody

// ServiceConfig stores daemon registry services configuration.
type ServiceConfig = registry.ServiceConfig

// NetIPNet is the net.IPNet type, which can be marshalled and
// unmarshalled to JSON
type NetIPNet = registry.NetIPNet

// IndexInfo contains information about a registry
type IndexInfo = registry.IndexInfo

// DistributionInspect describes the result obtained from contacting the
// registry to retrieve image metadata
type DistributionInspect = registry.DistributionInspect

// SearchOptions holds parameters to search images with.
type SearchOptions = client.ImageSearchOptions

// SearchResult describes a search result returned from a registry
type SearchResult = registry.SearchResult

// SearchResults lists a collection search results returned from a registry
type SearchResults = registry.SearchResults
