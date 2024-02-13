package aws

import (
	"net/http"

	smithybearer "github.com/aws/smithy-go/auth/bearer"
	"github.com/aws/smithy-go/logging"
	"github.com/aws/smithy-go/middleware"
)

// HTTPClient provides the interface to provide custom HTTPClients. Generally
// *http.Client is sufficient for most use cases. The HTTPClient should not
// follow 301 or 302 redirects.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// A Config provides service configuration for service clients.
type Config struct {
	// The region to send requests to. This parameter is required and must
	// be configured globally or on a per-client basis unless otherwise
	// noted. A full list of regions is found in the "Regions and Endpoints"
	// document.
	//
	// See http://docs.aws.amazon.com/general/latest/gr/rande.html for
	// information on AWS regions.
	Region string

	// The credentials object to use when signing requests.
	// Use the LoadDefaultConfig to load configuration from all the SDK's supported
	// sources, and resolve credentials using the SDK's default credential chain.
	Credentials CredentialsProvider

	// The Bearer Authentication token provider to use for authenticating API
	// operation calls with a Bearer Authentication token. The API clients and
	// operation must support Bearer Authentication scheme in order for the
	// token provider to be used. API clients created with NewFromConfig will
	// automatically be configured with this option, if the API client support
	// Bearer Authentication.
	//
	// The SDK's config.LoadDefaultConfig can automatically populate this
	// option for external configuration options such as SSO session.
	// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sso.html
	BearerAuthTokenProvider smithybearer.TokenProvider

	// The HTTP Client the SDK's API clients will use to invoke HTTP requests.
	// The SDK defaults to a BuildableClient allowing API clients to create
	// copies of the HTTP Client for service specific customizations.
	//
	// Use a (*http.Client) for custom behavior. Using a custom http.Client
	// will prevent the SDK from modifying the HTTP client.
	HTTPClient HTTPClient

	// An endpoint resolver that can be used to provide or override an endpoint
	// for the given service and region.
	//
	// See the `aws.EndpointResolver` documentation for additional usage
	// information.
	//
	// Deprecated: See Config.EndpointResolverWithOptions
	EndpointResolver EndpointResolver

	// An endpoint resolver that can be used to provide or override an endpoint
	// for the given service and region.
	//
	// When EndpointResolverWithOptions is specified, it will be used by a
	// service client rather than using EndpointResolver if also specified.
	//
	// See the `aws.EndpointResolverWithOptions` documentation for additional
	// usage information.
	EndpointResolverWithOptions EndpointResolverWithOptions

	// RetryMaxAttempts specifies the maximum number attempts an API client
	// will call an operation that fails with a retryable error.
	//
	// API Clients will only use this value to construct a retryer if the
	// Config.Retryer member is not nil. This value will be ignored if
	// Retryer is not nil.
	RetryMaxAttempts int

	// RetryMode specifies the retry model the API client will be created with.
	//
	// API Clients will only use this value to construct a retryer if the
	// Config.Retryer member is not nil. This value will be ignored if
	// Retryer is not nil.
	RetryMode RetryMode

	// Retryer is a function that provides a Retryer implementation. A Retryer
	// guides how HTTP requests should be retried in case of recoverable
	// failures. When nil the API client will use a default retryer.
	//
	// In general, the provider function should return a new instance of a
	// Retryer if you are attempting to provide a consistent Retryer
	// configuration across all clients. This will ensure that each client will
	// be provided a new instance of the Retryer implementation, and will avoid
	// issues such as sharing the same retry token bucket across services.
	//
	// If not nil, RetryMaxAttempts, and RetryMode will be ignored by API
	// clients.
	Retryer func() Retryer

	// ConfigSources are the sources that were used to construct the Config.
	// Allows for additional configuration to be loaded by clients.
	ConfigSources []interface{}

	// APIOptions provides the set of middleware mutations modify how the API
	// client requests will be handled. This is useful for adding additional
	// tracing data to a request, or changing behavior of the SDK's client.
	APIOptions []func(*middleware.Stack) error

	// The logger writer interface to write logging messages to. Defaults to
	// standard error.
	Logger logging.Logger

	// Configures the events that will be sent to the configured logger. This
	// can be used to configure the logging of signing, retries, request, and
	// responses of the SDK clients.
	//
	// See the ClientLogMode type documentation for the complete set of logging
	// modes and available configuration.
	ClientLogMode ClientLogMode

	// The configured DefaultsMode. If not specified, service clients will
	// default to legacy.
	//
	// Supported modes are: auto, cross-region, in-region, legacy, mobile,
	// standard
	DefaultsMode DefaultsMode

	// The RuntimeEnvironment configuration, only populated if the DefaultsMode
	// is set to DefaultsModeAuto and is initialized by
	// `config.LoadDefaultConfig`. You should not populate this structure
	// programmatically, or rely on the values here within your applications.
	RuntimeEnvironment RuntimeEnvironment
}

// NewConfig returns a new Config pointer that can be chained with builder
// methods to set multiple configuration values inline without using pointers.
func NewConfig() *Config {
	return &Config{}
}

// Copy will return a shallow copy of the Config object. If any additional
// configurations are provided they will be merged into the new config returned.
func (c Config) Copy() Config {
	cp := c
	return cp
}

// EndpointDiscoveryEnableState indicates if endpoint discovery is
// enabled, disabled, auto or unset state.
//
// Default behavior (Auto or Unset) indicates operations that require endpoint
// discovery will use Endpoint Discovery by default. Operations that
// optionally use Endpoint Discovery will not use Endpoint Discovery
// unless EndpointDiscovery is explicitly enabled.
type EndpointDiscoveryEnableState uint

// Enumeration values for EndpointDiscoveryEnableState
const (
	// EndpointDiscoveryUnset represents EndpointDiscoveryEnableState is unset.
	// Users do not need to use this value explicitly. The behavior for unset
	// is the same as for EndpointDiscoveryAuto.
	EndpointDiscoveryUnset EndpointDiscoveryEnableState = iota

	// EndpointDiscoveryAuto represents an AUTO state that allows endpoint
	// discovery only when required by the api. This is the default
	// configuration resolved by the client if endpoint discovery is neither
	// enabled or disabled.
	EndpointDiscoveryAuto // default state

	// EndpointDiscoveryDisabled indicates client MUST not perform endpoint
	// discovery even when required.
	EndpointDiscoveryDisabled

	// EndpointDiscoveryEnabled indicates client MUST always perform endpoint
	// discovery if supported for the operation.
	EndpointDiscoveryEnabled
)
