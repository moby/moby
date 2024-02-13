package config

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/credentials/endpointcreds"
	"github.com/aws/aws-sdk-go-v2/credentials/processcreds"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	smithybearer "github.com/aws/smithy-go/auth/bearer"
	"github.com/aws/smithy-go/logging"
	"github.com/aws/smithy-go/middleware"
)

// LoadOptionsFunc is a type alias for LoadOptions functional option
type LoadOptionsFunc func(*LoadOptions) error

// LoadOptions are discrete set of options that are valid for loading the
// configuration
type LoadOptions struct {

	// Region is the region to send requests to.
	Region string

	// Credentials object to use when signing requests.
	Credentials aws.CredentialsProvider

	// Token provider for authentication operations with bearer authentication.
	BearerAuthTokenProvider smithybearer.TokenProvider

	// HTTPClient the SDK's API clients will use to invoke HTTP requests.
	HTTPClient HTTPClient

	// EndpointResolver that can be used to provide or override an endpoint for
	// the given service and region.
	//
	// See the `aws.EndpointResolver` documentation on usage.
	//
	// Deprecated: See EndpointResolverWithOptions
	EndpointResolver aws.EndpointResolver

	// EndpointResolverWithOptions that can be used to provide or override an
	// endpoint for the given service and region.
	//
	// See the `aws.EndpointResolverWithOptions` documentation on usage.
	EndpointResolverWithOptions aws.EndpointResolverWithOptions

	// RetryMaxAttempts specifies the maximum number attempts an API client
	// will call an operation that fails with a retryable error.
	//
	// This value will only be used if Retryer option is nil.
	RetryMaxAttempts int

	// RetryMode specifies the retry model the API client will be created with.
	//
	// This value will only be used if Retryer option is nil.
	RetryMode aws.RetryMode

	// Retryer is a function that provides a Retryer implementation. A Retryer
	// guides how HTTP requests should be retried in case of recoverable
	// failures.
	//
	// If not nil, RetryMaxAttempts, and RetryMode will be ignored.
	Retryer func() aws.Retryer

	// APIOptions provides the set of middleware mutations modify how the API
	// client requests will be handled. This is useful for adding additional
	// tracing data to a request, or changing behavior of the SDK's client.
	APIOptions []func(*middleware.Stack) error

	// Logger writer interface to write logging messages to.
	Logger logging.Logger

	// ClientLogMode is used to configure the events that will be sent to the
	// configured logger. This can be used to configure the logging of signing,
	// retries, request, and responses of the SDK clients.
	//
	// See the ClientLogMode type documentation for the complete set of logging
	// modes and available configuration.
	ClientLogMode *aws.ClientLogMode

	// SharedConfigProfile is the profile to be used when loading the SharedConfig
	SharedConfigProfile string

	// SharedConfigFiles is the slice of custom shared config files to use when
	// loading the SharedConfig. A non-default profile used within config file
	// must have name defined with prefix 'profile '. eg [profile xyz]
	// indicates a profile with name 'xyz'. To read more on the format of the
	// config file, please refer the documentation at
	// https://docs.aws.amazon.com/credref/latest/refdocs/file-format.html#file-format-config
	//
	// If duplicate profiles are provided within the same, or across multiple
	// shared config files, the next parsed profile will override only the
	// properties that conflict with the previously defined profile. Note that
	// if duplicate profiles are provided within the SharedCredentialsFiles and
	// SharedConfigFiles, the properties defined in shared credentials file
	// take precedence.
	SharedConfigFiles []string

	// SharedCredentialsFile is the slice of custom shared credentials files to
	// use when loading the SharedConfig. The profile name used within
	// credentials file must not prefix 'profile '. eg [xyz] indicates a
	// profile with name 'xyz'. Profile declared as [profile xyz] will be
	// ignored. To read more on the format of the credentials file, please
	// refer the documentation at
	// https://docs.aws.amazon.com/credref/latest/refdocs/file-format.html#file-format-creds
	//
	// If duplicate profiles are provided with a same, or across multiple
	// shared credentials files, the next parsed profile will override only
	// properties that conflict with the previously defined profile. Note that
	// if duplicate profiles are provided within the SharedCredentialsFiles and
	// SharedConfigFiles, the properties defined in shared credentials file
	// take precedence.
	SharedCredentialsFiles []string

	// CustomCABundle is CA bundle PEM bytes reader
	CustomCABundle io.Reader

	// DefaultRegion is the fall back region, used if a region was not resolved
	// from other sources
	DefaultRegion string

	// UseEC2IMDSRegion indicates if SDK should retrieve the region
	// from the EC2 Metadata service
	UseEC2IMDSRegion *UseEC2IMDSRegion

	// CredentialsCacheOptions is a function for setting the
	// aws.CredentialsCacheOptions
	CredentialsCacheOptions func(*aws.CredentialsCacheOptions)

	// BearerAuthTokenCacheOptions is a function for setting the smithy-go
	// auth/bearer#TokenCacheOptions
	BearerAuthTokenCacheOptions func(*smithybearer.TokenCacheOptions)

	// SSOTokenProviderOptions is a function for setting the
	// credentials/ssocreds.SSOTokenProviderOptions
	SSOTokenProviderOptions func(*ssocreds.SSOTokenProviderOptions)

	// ProcessCredentialOptions is a function for setting
	// the processcreds.Options
	ProcessCredentialOptions func(*processcreds.Options)

	// EC2RoleCredentialOptions is a function for setting
	// the ec2rolecreds.Options
	EC2RoleCredentialOptions func(*ec2rolecreds.Options)

	// EndpointCredentialOptions is a function for setting
	// the endpointcreds.Options
	EndpointCredentialOptions func(*endpointcreds.Options)

	// WebIdentityRoleCredentialOptions is a function for setting
	// the stscreds.WebIdentityRoleOptions
	WebIdentityRoleCredentialOptions func(*stscreds.WebIdentityRoleOptions)

	// AssumeRoleCredentialOptions is a function for setting the
	// stscreds.AssumeRoleOptions
	AssumeRoleCredentialOptions func(*stscreds.AssumeRoleOptions)

	// SSOProviderOptions is a function for setting
	// the ssocreds.Options
	SSOProviderOptions func(options *ssocreds.Options)

	// LogConfigurationWarnings when set to true, enables logging
	// configuration warnings
	LogConfigurationWarnings *bool

	// S3UseARNRegion specifies if the S3 service should allow ARNs to direct
	// the region, the client's requests are sent to.
	S3UseARNRegion *bool

	// EnableEndpointDiscovery specifies if endpoint discovery is enable for
	// the client.
	EnableEndpointDiscovery aws.EndpointDiscoveryEnableState

	// Specifies if the EC2 IMDS service client is enabled.
	//
	// AWS_EC2_METADATA_DISABLED=true
	EC2IMDSClientEnableState imds.ClientEnableState

	// Specifies the EC2 Instance Metadata Service default endpoint selection
	// mode (IPv4 or IPv6)
	EC2IMDSEndpointMode imds.EndpointModeState

	// Specifies the EC2 Instance Metadata Service endpoint to use. If
	// specified it overrides EC2IMDSEndpointMode.
	EC2IMDSEndpoint string

	// Specifies that SDK clients must resolve a dual-stack endpoint for
	// services.
	UseDualStackEndpoint aws.DualStackEndpointState

	// Specifies that SDK clients must resolve a FIPS endpoint for
	// services.
	UseFIPSEndpoint aws.FIPSEndpointState

	// Specifies the SDK configuration mode for defaults.
	DefaultsModeOptions DefaultsModeOptions
}

func (o LoadOptions) getDefaultsMode(ctx context.Context) (aws.DefaultsMode, bool, error) {
	if len(o.DefaultsModeOptions.Mode) == 0 {
		return "", false, nil
	}
	return o.DefaultsModeOptions.Mode, true, nil
}

// GetRetryMaxAttempts returns the RetryMaxAttempts if specified in the
// LoadOptions and not 0.
func (o LoadOptions) GetRetryMaxAttempts(ctx context.Context) (int, bool, error) {
	if o.RetryMaxAttempts == 0 {
		return 0, false, nil
	}
	return o.RetryMaxAttempts, true, nil
}

// GetRetryMode returns the RetryMode specified in the LoadOptions.
func (o LoadOptions) GetRetryMode(ctx context.Context) (aws.RetryMode, bool, error) {
	if len(o.RetryMode) == 0 {
		return "", false, nil
	}
	return o.RetryMode, true, nil
}

func (o LoadOptions) getDefaultsModeIMDSClient(ctx context.Context) (*imds.Client, bool, error) {
	if o.DefaultsModeOptions.IMDSClient == nil {
		return nil, false, nil
	}
	return o.DefaultsModeOptions.IMDSClient, true, nil
}

// getRegion returns Region from config's LoadOptions
func (o LoadOptions) getRegion(ctx context.Context) (string, bool, error) {
	if len(o.Region) == 0 {
		return "", false, nil
	}

	return o.Region, true, nil
}

// WithRegion is a helper function to construct functional options
// that sets Region on config's LoadOptions. Setting the region to
// an empty string, will result in the region value being ignored.
// If multiple WithRegion calls are made, the last call overrides
// the previous call values.
func WithRegion(v string) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.Region = v
		return nil
	}
}

// getDefaultRegion returns DefaultRegion from config's LoadOptions
func (o LoadOptions) getDefaultRegion(ctx context.Context) (string, bool, error) {
	if len(o.DefaultRegion) == 0 {
		return "", false, nil
	}

	return o.DefaultRegion, true, nil
}

// WithDefaultRegion is a helper function to construct functional options
// that sets a DefaultRegion on config's LoadOptions. Setting the default
// region to an empty string, will result in the default region value
// being ignored. If multiple WithDefaultRegion calls are made, the last
// call overrides the previous call values. Note that both WithRegion and
// WithEC2IMDSRegion call takes precedence over WithDefaultRegion call
// when resolving region.
func WithDefaultRegion(v string) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.DefaultRegion = v
		return nil
	}
}

// getSharedConfigProfile returns SharedConfigProfile from config's LoadOptions
func (o LoadOptions) getSharedConfigProfile(ctx context.Context) (string, bool, error) {
	if len(o.SharedConfigProfile) == 0 {
		return "", false, nil
	}

	return o.SharedConfigProfile, true, nil
}

// WithSharedConfigProfile is a helper function to construct functional options
// that sets SharedConfigProfile on config's LoadOptions. Setting the shared
// config profile to an empty string, will result in the shared config profile
// value being ignored.
// If multiple WithSharedConfigProfile calls are made, the last call overrides
// the previous call values.
func WithSharedConfigProfile(v string) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.SharedConfigProfile = v
		return nil
	}
}

// getSharedConfigFiles returns SharedConfigFiles set on config's LoadOptions
func (o LoadOptions) getSharedConfigFiles(ctx context.Context) ([]string, bool, error) {
	if o.SharedConfigFiles == nil {
		return nil, false, nil
	}

	return o.SharedConfigFiles, true, nil
}

// WithSharedConfigFiles is a helper function to construct functional options
// that sets slice of SharedConfigFiles on config's LoadOptions.
// Setting the shared config files to an nil string slice, will result in the
// shared config files value being ignored.
// If multiple WithSharedConfigFiles calls are made, the last call overrides
// the previous call values.
func WithSharedConfigFiles(v []string) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.SharedConfigFiles = v
		return nil
	}
}

// getSharedCredentialsFiles returns SharedCredentialsFiles set on config's LoadOptions
func (o LoadOptions) getSharedCredentialsFiles(ctx context.Context) ([]string, bool, error) {
	if o.SharedCredentialsFiles == nil {
		return nil, false, nil
	}

	return o.SharedCredentialsFiles, true, nil
}

// WithSharedCredentialsFiles is a helper function to construct functional options
// that sets slice of SharedCredentialsFiles on config's LoadOptions.
// Setting the shared credentials files to an nil string slice, will result in the
// shared credentials files value being ignored.
// If multiple WithSharedCredentialsFiles calls are made, the last call overrides
// the previous call values.
func WithSharedCredentialsFiles(v []string) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.SharedCredentialsFiles = v
		return nil
	}
}

// getCustomCABundle returns CustomCABundle from LoadOptions
func (o LoadOptions) getCustomCABundle(ctx context.Context) (io.Reader, bool, error) {
	if o.CustomCABundle == nil {
		return nil, false, nil
	}

	return o.CustomCABundle, true, nil
}

// WithCustomCABundle is a helper function to construct functional options
// that sets CustomCABundle on config's LoadOptions. Setting the custom CA Bundle
// to nil will result in custom CA Bundle value being ignored.
// If multiple WithCustomCABundle calls are made, the last call overrides the
// previous call values.
func WithCustomCABundle(v io.Reader) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.CustomCABundle = v
		return nil
	}
}

// UseEC2IMDSRegion provides a regionProvider that retrieves the region
// from the EC2 Metadata service.
type UseEC2IMDSRegion struct {
	// If unset will default to generic EC2 IMDS client.
	Client *imds.Client
}

// getRegion attempts to retrieve the region from EC2 Metadata service.
func (p *UseEC2IMDSRegion) getRegion(ctx context.Context) (string, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	client := p.Client
	if client == nil {
		client = imds.New(imds.Options{})
	}

	result, err := client.GetRegion(ctx, nil)
	if err != nil {
		return "", false, err
	}
	if len(result.Region) != 0 {
		return result.Region, true, nil
	}
	return "", false, nil
}

// getEC2IMDSRegion returns the value of EC2 IMDS region.
func (o LoadOptions) getEC2IMDSRegion(ctx context.Context) (string, bool, error) {
	if o.UseEC2IMDSRegion == nil {
		return "", false, nil
	}

	return o.UseEC2IMDSRegion.getRegion(ctx)
}

// WithEC2IMDSRegion is a helper function to construct functional options
// that enables resolving EC2IMDS region. The function takes
// in a UseEC2IMDSRegion functional option, and can be used to set the
// EC2IMDS client which will be used to resolve EC2IMDSRegion.
// If no functional option is provided, an EC2IMDS client is built and used
// by the resolver. If multiple WithEC2IMDSRegion calls are made, the last
// call overrides the previous call values. Note that the WithRegion calls takes
// precedence over WithEC2IMDSRegion when resolving region.
func WithEC2IMDSRegion(fnOpts ...func(o *UseEC2IMDSRegion)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.UseEC2IMDSRegion = &UseEC2IMDSRegion{}

		for _, fn := range fnOpts {
			fn(o.UseEC2IMDSRegion)
		}
		return nil
	}
}

// getCredentialsProvider returns the credentials value
func (o LoadOptions) getCredentialsProvider(ctx context.Context) (aws.CredentialsProvider, bool, error) {
	if o.Credentials == nil {
		return nil, false, nil
	}

	return o.Credentials, true, nil
}

// WithCredentialsProvider is a helper function to construct functional options
// that sets Credential provider value on config's LoadOptions. If credentials
// provider is set to nil, the credentials provider value will be ignored.
// If multiple WithCredentialsProvider calls are made, the last call overrides
// the previous call values.
func WithCredentialsProvider(v aws.CredentialsProvider) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.Credentials = v
		return nil
	}
}

// getCredentialsCacheOptionsProvider returns the wrapped function to set aws.CredentialsCacheOptions
func (o LoadOptions) getCredentialsCacheOptions(ctx context.Context) (func(*aws.CredentialsCacheOptions), bool, error) {
	if o.CredentialsCacheOptions == nil {
		return nil, false, nil
	}

	return o.CredentialsCacheOptions, true, nil
}

// WithCredentialsCacheOptions is a helper function to construct functional
// options that sets a function to modify the aws.CredentialsCacheOptions the
// aws.CredentialsCache will be configured with, if the CredentialsCache is used
// by the configuration loader.
//
// If multiple WithCredentialsCacheOptions calls are made, the last call
// overrides the previous call values.
func WithCredentialsCacheOptions(v func(*aws.CredentialsCacheOptions)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.CredentialsCacheOptions = v
		return nil
	}
}

// getBearerAuthTokenProvider returns the credentials value
func (o LoadOptions) getBearerAuthTokenProvider(ctx context.Context) (smithybearer.TokenProvider, bool, error) {
	if o.BearerAuthTokenProvider == nil {
		return nil, false, nil
	}

	return o.BearerAuthTokenProvider, true, nil
}

// WithBearerAuthTokenProvider is a helper function to construct functional options
// that sets Credential provider value on config's LoadOptions. If credentials
// provider is set to nil, the credentials provider value will be ignored.
// If multiple WithBearerAuthTokenProvider calls are made, the last call overrides
// the previous call values.
func WithBearerAuthTokenProvider(v smithybearer.TokenProvider) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.BearerAuthTokenProvider = v
		return nil
	}
}

// getBearerAuthTokenCacheOptionsProvider returns the wrapped function to set smithybearer.TokenCacheOptions
func (o LoadOptions) getBearerAuthTokenCacheOptions(ctx context.Context) (func(*smithybearer.TokenCacheOptions), bool, error) {
	if o.BearerAuthTokenCacheOptions == nil {
		return nil, false, nil
	}

	return o.BearerAuthTokenCacheOptions, true, nil
}

// WithBearerAuthTokenCacheOptions is a helper function to construct functional options
// that sets a function to modify the TokenCacheOptions the smithy-go
// auth/bearer#TokenCache will be configured with, if the TokenCache is used by
// the configuration loader.
//
// If multiple WithBearerAuthTokenCacheOptions calls are made, the last call overrides
// the previous call values.
func WithBearerAuthTokenCacheOptions(v func(*smithybearer.TokenCacheOptions)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.BearerAuthTokenCacheOptions = v
		return nil
	}
}

// getSSOTokenProviderOptionsProvider returns the wrapped function to set smithybearer.TokenCacheOptions
func (o LoadOptions) getSSOTokenProviderOptions(ctx context.Context) (func(*ssocreds.SSOTokenProviderOptions), bool, error) {
	if o.SSOTokenProviderOptions == nil {
		return nil, false, nil
	}

	return o.SSOTokenProviderOptions, true, nil
}

// WithSSOTokenProviderOptions is a helper function to construct functional
// options that sets a function to modify the SSOtokenProviderOptions the SDK's
// credentials/ssocreds#SSOProvider will be configured with, if the
// SSOTokenProvider is used by the configuration loader.
//
// If multiple WithSSOTokenProviderOptions calls are made, the last call overrides
// the previous call values.
func WithSSOTokenProviderOptions(v func(*ssocreds.SSOTokenProviderOptions)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.SSOTokenProviderOptions = v
		return nil
	}
}

// getProcessCredentialOptions returns the wrapped function to set processcreds.Options
func (o LoadOptions) getProcessCredentialOptions(ctx context.Context) (func(*processcreds.Options), bool, error) {
	if o.ProcessCredentialOptions == nil {
		return nil, false, nil
	}

	return o.ProcessCredentialOptions, true, nil
}

// WithProcessCredentialOptions is a helper function to construct functional options
// that sets a function to use processcreds.Options on config's LoadOptions.
// If process credential options is set to nil, the process credential value will
// be ignored. If multiple WithProcessCredentialOptions calls are made, the last call
// overrides the previous call values.
func WithProcessCredentialOptions(v func(*processcreds.Options)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.ProcessCredentialOptions = v
		return nil
	}
}

// getEC2RoleCredentialOptions returns the wrapped function to set the ec2rolecreds.Options
func (o LoadOptions) getEC2RoleCredentialOptions(ctx context.Context) (func(*ec2rolecreds.Options), bool, error) {
	if o.EC2RoleCredentialOptions == nil {
		return nil, false, nil
	}

	return o.EC2RoleCredentialOptions, true, nil
}

// WithEC2RoleCredentialOptions is a helper function to construct functional options
// that sets a function to use ec2rolecreds.Options on config's LoadOptions. If
// EC2 role credential options is set to nil, the EC2 role credential options value
// will be ignored. If multiple WithEC2RoleCredentialOptions calls are made,
// the last call overrides the previous call values.
func WithEC2RoleCredentialOptions(v func(*ec2rolecreds.Options)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.EC2RoleCredentialOptions = v
		return nil
	}
}

// getEndpointCredentialOptions returns the wrapped function to set endpointcreds.Options
func (o LoadOptions) getEndpointCredentialOptions(context.Context) (func(*endpointcreds.Options), bool, error) {
	if o.EndpointCredentialOptions == nil {
		return nil, false, nil
	}

	return o.EndpointCredentialOptions, true, nil
}

// WithEndpointCredentialOptions is a helper function to construct functional options
// that sets a function to use endpointcreds.Options on config's LoadOptions. If
// endpoint credential options is set to nil, the endpoint credential options
// value will be ignored. If multiple WithEndpointCredentialOptions calls are made,
// the last call overrides the previous call values.
func WithEndpointCredentialOptions(v func(*endpointcreds.Options)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.EndpointCredentialOptions = v
		return nil
	}
}

// getWebIdentityRoleCredentialOptions returns the wrapped function
func (o LoadOptions) getWebIdentityRoleCredentialOptions(context.Context) (func(*stscreds.WebIdentityRoleOptions), bool, error) {
	if o.WebIdentityRoleCredentialOptions == nil {
		return nil, false, nil
	}

	return o.WebIdentityRoleCredentialOptions, true, nil
}

// WithWebIdentityRoleCredentialOptions is a helper function to construct
// functional options that sets a function to use stscreds.WebIdentityRoleOptions
// on config's LoadOptions. If web identity role credentials options is set to nil,
// the web identity role credentials value will be ignored. If multiple
// WithWebIdentityRoleCredentialOptions calls are made, the last call
// overrides the previous call values.
func WithWebIdentityRoleCredentialOptions(v func(*stscreds.WebIdentityRoleOptions)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.WebIdentityRoleCredentialOptions = v
		return nil
	}
}

// getAssumeRoleCredentialOptions returns AssumeRoleCredentialOptions from LoadOptions
func (o LoadOptions) getAssumeRoleCredentialOptions(context.Context) (func(options *stscreds.AssumeRoleOptions), bool, error) {
	if o.AssumeRoleCredentialOptions == nil {
		return nil, false, nil
	}

	return o.AssumeRoleCredentialOptions, true, nil
}

// WithAssumeRoleCredentialOptions  is a helper function to construct
// functional options that sets a function to use stscreds.AssumeRoleOptions
// on config's LoadOptions. If assume role credentials options is set to nil,
// the assume role credentials value will be ignored. If multiple
// WithAssumeRoleCredentialOptions calls are made, the last call overrides
// the previous call values.
func WithAssumeRoleCredentialOptions(v func(*stscreds.AssumeRoleOptions)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.AssumeRoleCredentialOptions = v
		return nil
	}
}

func (o LoadOptions) getHTTPClient(ctx context.Context) (HTTPClient, bool, error) {
	if o.HTTPClient == nil {
		return nil, false, nil
	}

	return o.HTTPClient, true, nil
}

// WithHTTPClient is a helper function to construct functional options
// that sets HTTPClient on LoadOptions. If HTTPClient is set to nil,
// the HTTPClient value will be ignored.
// If multiple WithHTTPClient calls are made, the last call overrides
// the previous call values.
func WithHTTPClient(v HTTPClient) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.HTTPClient = v
		return nil
	}
}

func (o LoadOptions) getAPIOptions(ctx context.Context) ([]func(*middleware.Stack) error, bool, error) {
	if o.APIOptions == nil {
		return nil, false, nil
	}

	return o.APIOptions, true, nil
}

// WithAPIOptions is a helper function to construct functional options
// that sets APIOptions on LoadOptions. If APIOptions is set to nil, the
// APIOptions value is ignored. If multiple WithAPIOptions calls are
// made, the last call overrides the previous call values.
func WithAPIOptions(v []func(*middleware.Stack) error) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		if v == nil {
			return nil
		}

		o.APIOptions = append(o.APIOptions, v...)
		return nil
	}
}

func (o LoadOptions) getRetryMaxAttempts(ctx context.Context) (int, bool, error) {
	if o.RetryMaxAttempts == 0 {
		return 0, false, nil
	}

	return o.RetryMaxAttempts, true, nil
}

// WithRetryMaxAttempts is a helper function to construct functional options that sets
// RetryMaxAttempts on LoadOptions. If RetryMaxAttempts is unset, the RetryMaxAttempts value is
// ignored. If multiple WithRetryMaxAttempts calls are made, the last call overrides
// the previous call values.
//
// Will be ignored of LoadOptions.Retryer or WithRetryer are used.
func WithRetryMaxAttempts(v int) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.RetryMaxAttempts = v
		return nil
	}
}

func (o LoadOptions) getRetryMode(ctx context.Context) (aws.RetryMode, bool, error) {
	if o.RetryMode == "" {
		return "", false, nil
	}

	return o.RetryMode, true, nil
}

// WithRetryMode is a helper function to construct functional options that sets
// RetryMode on LoadOptions. If RetryMode is unset, the RetryMode value is
// ignored. If multiple WithRetryMode calls are made, the last call overrides
// the previous call values.
//
// Will be ignored of LoadOptions.Retryer or WithRetryer are used.
func WithRetryMode(v aws.RetryMode) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.RetryMode = v
		return nil
	}
}

func (o LoadOptions) getRetryer(ctx context.Context) (func() aws.Retryer, bool, error) {
	if o.Retryer == nil {
		return nil, false, nil
	}

	return o.Retryer, true, nil
}

// WithRetryer is a helper function to construct functional options
// that sets Retryer on LoadOptions. If Retryer is set to nil, the
// Retryer value is ignored. If multiple WithRetryer calls are
// made, the last call overrides the previous call values.
func WithRetryer(v func() aws.Retryer) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.Retryer = v
		return nil
	}
}

func (o LoadOptions) getEndpointResolver(ctx context.Context) (aws.EndpointResolver, bool, error) {
	if o.EndpointResolver == nil {
		return nil, false, nil
	}

	return o.EndpointResolver, true, nil
}

// WithEndpointResolver is a helper function to construct functional options
// that sets the EndpointResolver on LoadOptions. If the EndpointResolver is set to nil,
// the EndpointResolver value is ignored. If multiple WithEndpointResolver calls
// are made, the last call overrides the previous call values.
//
// Deprecated: See WithEndpointResolverWithOptions
func WithEndpointResolver(v aws.EndpointResolver) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.EndpointResolver = v
		return nil
	}
}

func (o LoadOptions) getEndpointResolverWithOptions(ctx context.Context) (aws.EndpointResolverWithOptions, bool, error) {
	if o.EndpointResolverWithOptions == nil {
		return nil, false, nil
	}

	return o.EndpointResolverWithOptions, true, nil
}

// WithEndpointResolverWithOptions is a helper function to construct functional options
// that sets the EndpointResolverWithOptions on LoadOptions. If the EndpointResolverWithOptions is set to nil,
// the EndpointResolver value is ignored. If multiple WithEndpointResolver calls
// are made, the last call overrides the previous call values.
func WithEndpointResolverWithOptions(v aws.EndpointResolverWithOptions) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.EndpointResolverWithOptions = v
		return nil
	}
}

func (o LoadOptions) getLogger(ctx context.Context) (logging.Logger, bool, error) {
	if o.Logger == nil {
		return nil, false, nil
	}

	return o.Logger, true, nil
}

// WithLogger is a helper function to construct functional options
// that sets Logger on LoadOptions. If Logger is set to nil, the
// Logger value will be ignored. If multiple WithLogger calls are made,
// the last call overrides the previous call values.
func WithLogger(v logging.Logger) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.Logger = v
		return nil
	}
}

func (o LoadOptions) getClientLogMode(ctx context.Context) (aws.ClientLogMode, bool, error) {
	if o.ClientLogMode == nil {
		return 0, false, nil
	}

	return *o.ClientLogMode, true, nil
}

// WithClientLogMode is a helper function to construct functional options
// that sets client log mode on LoadOptions. If client log mode is set to nil,
// the client log mode value will be ignored. If multiple WithClientLogMode calls are made,
// the last call overrides the previous call values.
func WithClientLogMode(v aws.ClientLogMode) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.ClientLogMode = &v
		return nil
	}
}

func (o LoadOptions) getLogConfigurationWarnings(ctx context.Context) (v bool, found bool, err error) {
	if o.LogConfigurationWarnings == nil {
		return false, false, nil
	}
	return *o.LogConfigurationWarnings, true, nil
}

// WithLogConfigurationWarnings is a helper function to construct
// functional options that can be used to set LogConfigurationWarnings
// on LoadOptions.
//
// If multiple WithLogConfigurationWarnings calls are made, the last call
// overrides the previous call values.
func WithLogConfigurationWarnings(v bool) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.LogConfigurationWarnings = &v
		return nil
	}
}

// GetS3UseARNRegion returns whether to allow ARNs to direct the region
// the S3 client's requests are sent to.
func (o LoadOptions) GetS3UseARNRegion(ctx context.Context) (v bool, found bool, err error) {
	if o.S3UseARNRegion == nil {
		return false, false, nil
	}
	return *o.S3UseARNRegion, true, nil
}

// WithS3UseARNRegion is a helper function to construct functional options
// that can be used to set S3UseARNRegion on LoadOptions.
// If multiple WithS3UseARNRegion calls are made, the last call overrides
// the previous call values.
func WithS3UseARNRegion(v bool) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.S3UseARNRegion = &v
		return nil
	}
}

// GetEnableEndpointDiscovery returns if the EnableEndpointDiscovery flag is set.
func (o LoadOptions) GetEnableEndpointDiscovery(ctx context.Context) (value aws.EndpointDiscoveryEnableState, ok bool, err error) {
	if o.EnableEndpointDiscovery == aws.EndpointDiscoveryUnset {
		return aws.EndpointDiscoveryUnset, false, nil
	}
	return o.EnableEndpointDiscovery, true, nil
}

// WithEndpointDiscovery is a helper function to construct functional options
// that can be used to enable endpoint discovery on LoadOptions for supported clients.
// If multiple WithEndpointDiscovery calls are made, the last call overrides
// the previous call values.
func WithEndpointDiscovery(v aws.EndpointDiscoveryEnableState) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.EnableEndpointDiscovery = v
		return nil
	}
}

// getSSOProviderOptions returns AssumeRoleCredentialOptions from LoadOptions
func (o LoadOptions) getSSOProviderOptions(context.Context) (func(options *ssocreds.Options), bool, error) {
	if o.SSOProviderOptions == nil {
		return nil, false, nil
	}

	return o.SSOProviderOptions, true, nil
}

// WithSSOProviderOptions is a helper function to construct
// functional options that sets a function to use ssocreds.Options
// on config's LoadOptions. If the SSO credential provider options is set to nil,
// the sso provider options value will be ignored. If multiple
// WithSSOProviderOptions calls are made, the last call overrides
// the previous call values.
func WithSSOProviderOptions(v func(*ssocreds.Options)) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.SSOProviderOptions = v
		return nil
	}
}

// GetEC2IMDSClientEnableState implements a EC2IMDSClientEnableState options resolver interface.
func (o LoadOptions) GetEC2IMDSClientEnableState() (imds.ClientEnableState, bool, error) {
	if o.EC2IMDSClientEnableState == imds.ClientDefaultEnableState {
		return imds.ClientDefaultEnableState, false, nil
	}

	return o.EC2IMDSClientEnableState, true, nil
}

// GetEC2IMDSEndpointMode implements a EC2IMDSEndpointMode option resolver interface.
func (o LoadOptions) GetEC2IMDSEndpointMode() (imds.EndpointModeState, bool, error) {
	if o.EC2IMDSEndpointMode == imds.EndpointModeStateUnset {
		return imds.EndpointModeStateUnset, false, nil
	}

	return o.EC2IMDSEndpointMode, true, nil
}

// GetEC2IMDSEndpoint implements a EC2IMDSEndpoint option resolver interface.
func (o LoadOptions) GetEC2IMDSEndpoint() (string, bool, error) {
	if len(o.EC2IMDSEndpoint) == 0 {
		return "", false, nil
	}

	return o.EC2IMDSEndpoint, true, nil
}

// WithEC2IMDSClientEnableState is a helper function to construct functional options that sets the EC2IMDSClientEnableState.
func WithEC2IMDSClientEnableState(v imds.ClientEnableState) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.EC2IMDSClientEnableState = v
		return nil
	}
}

// WithEC2IMDSEndpointMode is a helper function to construct functional options that sets the EC2IMDSEndpointMode.
func WithEC2IMDSEndpointMode(v imds.EndpointModeState) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.EC2IMDSEndpointMode = v
		return nil
	}
}

// WithEC2IMDSEndpoint is a helper function to construct functional options that sets the EC2IMDSEndpoint.
func WithEC2IMDSEndpoint(v string) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.EC2IMDSEndpoint = v
		return nil
	}
}

// WithUseDualStackEndpoint is a helper function to construct
// functional options that can be used to set UseDualStackEndpoint on LoadOptions.
func WithUseDualStackEndpoint(v aws.DualStackEndpointState) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.UseDualStackEndpoint = v
		return nil
	}
}

// GetUseDualStackEndpoint returns whether the service's dual-stack endpoint should be
// used for requests.
func (o LoadOptions) GetUseDualStackEndpoint(ctx context.Context) (value aws.DualStackEndpointState, found bool, err error) {
	if o.UseDualStackEndpoint == aws.DualStackEndpointStateUnset {
		return aws.DualStackEndpointStateUnset, false, nil
	}
	return o.UseDualStackEndpoint, true, nil
}

// WithUseFIPSEndpoint is a helper function to construct
// functional options that can be used to set UseFIPSEndpoint on LoadOptions.
func WithUseFIPSEndpoint(v aws.FIPSEndpointState) LoadOptionsFunc {
	return func(o *LoadOptions) error {
		o.UseFIPSEndpoint = v
		return nil
	}
}

// GetUseFIPSEndpoint returns whether the service's FIPS endpoint should be
// used for requests.
func (o LoadOptions) GetUseFIPSEndpoint(ctx context.Context) (value aws.FIPSEndpointState, found bool, err error) {
	if o.UseFIPSEndpoint == aws.FIPSEndpointStateUnset {
		return aws.FIPSEndpointStateUnset, false, nil
	}
	return o.UseFIPSEndpoint, true, nil
}

// WithDefaultsMode sets the SDK defaults configuration mode to the value provided.
//
// Zero or more functional options can be provided to provide configuration options for performing
// environment discovery when using aws.DefaultsModeAuto.
func WithDefaultsMode(mode aws.DefaultsMode, optFns ...func(options *DefaultsModeOptions)) LoadOptionsFunc {
	do := DefaultsModeOptions{
		Mode: mode,
	}
	for _, fn := range optFns {
		fn(&do)
	}
	return func(options *LoadOptions) error {
		options.DefaultsModeOptions = do
		return nil
	}
}
