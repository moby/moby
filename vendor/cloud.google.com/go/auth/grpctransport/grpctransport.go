// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package grpctransport provides functionality for managing gRPC client
// connections to Google Cloud services.
package grpctransport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/internal"
	"cloud.google.com/go/auth/internal/transport"
	"cloud.google.com/go/auth/internal/transport/headers"
	"github.com/googleapis/gax-go/v2"
	"github.com/googleapis/gax-go/v2/callctx"
	"github.com/googleapis/gax-go/v2/internallog"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpccreds "google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

const (
	// Check env to disable DirectPath traffic.
	disableDirectPathEnvVar = "GOOGLE_CLOUD_DISABLE_DIRECT_PATH"

	// Check env to decide if using google-c2p resolver for DirectPath traffic.
	enableDirectPathXdsEnvVar = "GOOGLE_CLOUD_ENABLE_DIRECT_PATH_XDS"

	quotaProjectHeaderKey = "X-goog-user-project"
)

// codeToStr is a reversal of the `strToCode` map in
// https://github.com/grpc/grpc-go/blob/master/codes/codes.go
// The gRPC specification has exactly 17 status codes, defined
// as a contiguous block of integers from 0 to 16.
var codeToStr = [...]string{
	"OK",                  // codes.OK = 0
	"CANCELED",            // codes.Canceled = 1
	"UNKNOWN",             // codes.Unknown = 2
	"INVALID_ARGUMENT",    // codes.InvalidArgument = 3
	"DEADLINE_EXCEEDED",   // codes.DeadlineExceeded = 4
	"NOT_FOUND",           // codes.NotFound = 5
	"ALREADY_EXISTS",      // codes.AlreadyExists = 6
	"PERMISSION_DENIED",   // codes.PermissionDenied = 7
	"RESOURCE_EXHAUSTED",  // codes.ResourceExhausted = 8
	"FAILED_PRECONDITION", // codes.FailedPrecondition = 9
	"ABORTED",             // codes.Aborted = 10
	"OUT_OF_RANGE",        // codes.OutOfRange = 11
	"UNIMPLEMENTED",       // codes.Unimplemented = 12
	"INTERNAL",            // codes.Internal = 13
	"UNAVAILABLE",         // codes.Unavailable = 14
	"DATA_LOSS",           // codes.DataLoss = 15
	"UNAUTHENTICATED",     // codes.Unauthenticated = 16
}

var (
	// Set at init time by dial_socketopt.go. If nil, socketopt is not supported.
	timeoutDialerOption grpc.DialOption
)

// ClientCertProvider is a function that returns a TLS client certificate to be
// used when opening TLS connections. It follows the same semantics as
// [crypto/tls.Config.GetClientCertificate].
type ClientCertProvider = func(*tls.CertificateRequestInfo) (*tls.Certificate, error)

// Options used to configure a [GRPCClientConnPool] from [Dial].
type Options struct {
	// DisableTelemetry disables default telemetry (OpenTelemetry). An example
	// reason to do so would be to bind custom telemetry that overrides the
	// defaults.
	DisableTelemetry bool
	// DisableAuthentication specifies that no authentication should be used. It
	// is suitable only for testing and for accessing public resources, like
	// public Google Cloud Storage buckets.
	DisableAuthentication bool
	// Endpoint overrides the default endpoint to be used for a service.
	Endpoint string
	// Metadata is extra gRPC metadata that will be appended to every outgoing
	// request.
	Metadata map[string]string
	// GRPCDialOpts are dial options that will be passed to `grpc.Dial` when
	// establishing a`grpc.Conn``
	GRPCDialOpts []grpc.DialOption
	// PoolSize is specifies how many connections to balance between when making
	// requests. If unset or less than 1, the value defaults to 1.
	PoolSize int
	// Credentials used to add Authorization metadata to all requests. If set
	// DetectOpts are ignored.
	Credentials *auth.Credentials
	// ClientCertProvider is a function that returns a TLS client certificate to
	// be used when opening TLS connections. It follows the same semantics as
	// crypto/tls.Config.GetClientCertificate.
	ClientCertProvider ClientCertProvider
	// DetectOpts configures settings for detect Application Default
	// Credentials.
	DetectOpts *credentials.DetectOptions
	// UniverseDomain is the default service domain for a given Cloud universe.
	// The default value is "googleapis.com". This is the universe domain
	// configured for the client, which will be compared to the universe domain
	// that is separately configured for the credentials.
	UniverseDomain string
	// APIKey specifies an API key to be used as the basis for authentication.
	// If set DetectOpts are ignored.
	APIKey string
	// Logger is used for debug logging. If provided, logging will be enabled
	// at the loggers configured level. By default logging is disabled unless
	// enabled by setting GOOGLE_SDK_GO_LOGGING_LEVEL in which case a default
	// logger will be used. Optional.
	Logger *slog.Logger

	// InternalOptions are NOT meant to be set directly by consumers of this
	// package, they should only be set by generated client code.
	InternalOptions *InternalOptions
}

// client returns the client a user set for the detect options or nil if one was
// not set.
func (o *Options) client() *http.Client {
	if o.DetectOpts != nil && o.DetectOpts.Client != nil {
		return o.DetectOpts.Client
	}
	return nil
}

func (o *Options) logger() *slog.Logger {
	return internallog.New(o.Logger)
}

func (o *Options) validate() error {
	if o == nil {
		return errors.New("grpctransport: opts required to be non-nil")
	}
	if o.InternalOptions != nil && o.InternalOptions.SkipValidation {
		return nil
	}
	hasCreds := o.APIKey != "" ||
		o.Credentials != nil ||
		(o.DetectOpts != nil && len(o.DetectOpts.CredentialsJSON) > 0) ||
		(o.DetectOpts != nil && o.DetectOpts.CredentialsFile != "")
	if o.DisableAuthentication && hasCreds {
		return errors.New("grpctransport: DisableAuthentication is incompatible with options that set or detect credentials")
	}
	return nil
}

func (o *Options) resolveDetectOptions() *credentials.DetectOptions {
	io := o.InternalOptions
	// soft-clone these so we are not updating a ref the user holds and may reuse
	do := transport.CloneDetectOptions(o.DetectOpts)

	// If scoped JWTs are enabled user provided an aud, allow self-signed JWT.
	if (io != nil && io.EnableJWTWithScope) || do.Audience != "" {
		do.UseSelfSignedJWT = true
	}
	// Only default scopes if user did not also set an audience.
	if len(do.Scopes) == 0 && do.Audience == "" && io != nil && len(io.DefaultScopes) > 0 {
		do.Scopes = make([]string, len(io.DefaultScopes))
		copy(do.Scopes, io.DefaultScopes)
	}
	if len(do.Scopes) == 0 && do.Audience == "" && io != nil {
		do.Audience = o.InternalOptions.DefaultAudience
	}
	if o.ClientCertProvider != nil {
		tlsConfig := &tls.Config{
			GetClientCertificate: o.ClientCertProvider,
		}
		do.Client = transport.DefaultHTTPClientWithTLS(tlsConfig)
		do.TokenURL = credentials.GoogleMTLSTokenURL
	}
	if do.Logger == nil {
		do.Logger = o.logger()
	}
	return do
}

// InternalOptions are only meant to be set by generated client code. These are
// not meant to be set directly by consumers of this package. Configuration in
// this type is considered EXPERIMENTAL and may be removed at any time in the
// future without warning.
type InternalOptions struct {
	// EnableNonDefaultSAForDirectPath overrides the default requirement for
	// using the default service account for DirectPath.
	EnableNonDefaultSAForDirectPath bool
	// EnableDirectPath overrides the default attempt to use DirectPath.
	EnableDirectPath bool
	// EnableDirectPathXds overrides the default DirectPath type. It is only
	// valid when DirectPath is enabled.
	EnableDirectPathXds bool
	// EnableJWTWithScope specifies if scope can be used with self-signed JWT.
	EnableJWTWithScope bool
	// AllowHardBoundTokens allows libraries to request a hard-bound token.
	// Obtaining hard-bound tokens requires the connection to be established
	// using either ALTS or mTLS with S2A.
	AllowHardBoundTokens []string
	// DefaultAudience specifies a default audience to be used as the audience
	// field ("aud") for the JWT token authentication.
	DefaultAudience string
	// DefaultEndpointTemplate combined with UniverseDomain specifies
	// the default endpoint.
	DefaultEndpointTemplate string
	// DefaultMTLSEndpoint specifies the default mTLS endpoint.
	DefaultMTLSEndpoint string
	// DefaultScopes specifies the default OAuth2 scopes to be used for a
	// service.
	DefaultScopes []string
	// SkipValidation bypasses validation on Options. It should only be used
	// internally for clients that need more control over their transport.
	SkipValidation bool
	// TelemetryAttributes specifies a map of telemetry attributes to be added
	// to all OpenTelemetry signals, such as tracing and metrics, for purposes
	// including representing the static identity of the client (e.g., service
	// name, version). These attributes are expected to be consistent across all
	// signals to enable cross-signal correlation.
	//
	// It should only be used internally by generated clients. Callers should not
	// modify the map after it is passed in.
	TelemetryAttributes map[string]string
}

// Dial returns a GRPCClientConnPool that can be used to communicate with a
// Google cloud service, configured with the provided [Options]. It
// automatically appends Authorization metadata to all outgoing requests.
func Dial(ctx context.Context, secure bool, opts *Options) (GRPCClientConnPool, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	if opts.PoolSize <= 1 {
		conn, err := dial(ctx, secure, opts)
		if err != nil {
			return nil, err
		}
		return &singleConnPool{conn}, nil
	}
	pool := &roundRobinConnPool{}
	for i := 0; i < opts.PoolSize; i++ {
		conn, err := dial(ctx, secure, opts)
		if err != nil {
			// ignore close error, if any
			defer pool.Close()
			return nil, err
		}
		pool.conns = append(pool.conns, conn)
	}
	return pool, nil
}

// return a GRPCClientConnPool if pool == 1 or else a pool of of them if >1
func dial(ctx context.Context, secure bool, opts *Options) (*grpc.ClientConn, error) {
	tOpts := &transport.Options{
		Endpoint:           opts.Endpoint,
		ClientCertProvider: opts.ClientCertProvider,
		Client:             opts.client(),
		UniverseDomain:     opts.UniverseDomain,
		Logger:             opts.logger(),
	}
	if io := opts.InternalOptions; io != nil {
		tOpts.DefaultEndpointTemplate = io.DefaultEndpointTemplate
		tOpts.DefaultMTLSEndpoint = io.DefaultMTLSEndpoint
		tOpts.EnableDirectPath = io.EnableDirectPath
		tOpts.EnableDirectPathXds = io.EnableDirectPathXds
	}
	transportCreds, err := transport.GetGRPCTransportCredsAndEndpoint(tOpts)
	if err != nil {
		return nil, err
	}

	if !secure {
		transportCreds.TransportCredentials = grpcinsecure.NewCredentials()
	}

	// Initialize gRPC dial options with transport-level security options.
	grpcOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCreds),
	}

	// Ensure the token exchange HTTP transport uses the same ClientCertProvider as the GRPC API transport.
	opts.ClientCertProvider, err = transport.GetClientCertificateProvider(tOpts)
	if err != nil {
		return nil, err
	}

	if opts.APIKey != "" {
		grpcOpts = append(grpcOpts,
			grpc.WithPerRPCCredentials(&grpcKeyProvider{
				apiKey:   opts.APIKey,
				metadata: opts.Metadata,
				secure:   secure,
			}),
		)
	} else if !opts.DisableAuthentication {
		metadata := opts.Metadata

		var creds *auth.Credentials
		if opts.Credentials != nil {
			creds = opts.Credentials
		} else {
			// This condition is only met for non-DirectPath clients because
			// TransportTypeMTLSS2A is used only when InternalOptions.EnableDirectPath
			// is false.
			optsClone := opts.resolveDetectOptions()
			if transportCreds.TransportType == transport.TransportTypeMTLSS2A {
				// Check that the client allows requesting hard-bound token for the transport type mTLS using S2A.
				for _, ev := range opts.InternalOptions.AllowHardBoundTokens {
					if ev == "MTLS_S2A" {
						optsClone.TokenBindingType = credentials.MTLSHardBinding
						break
					}
				}
			}
			var err error
			creds, err = credentials.DetectDefault(optsClone)
			if err != nil {
				return nil, err
			}
		}

		qp, err := creds.QuotaProjectID(ctx)
		if err != nil {
			return nil, err
		}
		if qp != "" {
			if metadata == nil {
				metadata = make(map[string]string, 1)
			}
			// Don't overwrite user specified quota
			if _, ok := metadata[quotaProjectHeaderKey]; !ok {
				metadata[quotaProjectHeaderKey] = qp
			}
		}
		grpcOpts = append(grpcOpts,
			grpc.WithPerRPCCredentials(&grpcCredentialsProvider{
				creds:                creds,
				metadata:             metadata,
				clientUniverseDomain: opts.UniverseDomain,
			}),
		)
		// Attempt Direct Path
		grpcOpts, transportCreds.Endpoint, err = configureDirectPath(grpcOpts, opts, transportCreds.Endpoint, creds)
		if err != nil {
			return nil, err
		}
	}

	// Add tracing, but before the other options, so that clients can override the
	// gRPC stats handler.
	// This assumes that gRPC options are processed in order, left to right.
	grpcOpts = addOpenTelemetryStatsHandler(grpcOpts, opts)
	grpcOpts = append(grpcOpts, opts.GRPCDialOpts...)

	return grpc.DialContext(ctx, transportCreds.Endpoint, grpcOpts...)
}

// grpcKeyProvider satisfies https://pkg.go.dev/google.golang.org/grpc/credentials#PerRPCCredentials.
type grpcKeyProvider struct {
	apiKey   string
	metadata map[string]string
	secure   bool
}

func (g *grpcKeyProvider) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	metadata := make(map[string]string, len(g.metadata)+1)
	metadata["X-goog-api-key"] = g.apiKey
	for k, v := range g.metadata {
		metadata[k] = v
	}
	return metadata, nil
}

func (g *grpcKeyProvider) RequireTransportSecurity() bool {
	return g.secure
}

// grpcCredentialsProvider satisfies https://pkg.go.dev/google.golang.org/grpc/credentials#PerRPCCredentials.
type grpcCredentialsProvider struct {
	creds *auth.Credentials

	secure bool

	// Additional metadata attached as headers.
	metadata             map[string]string
	clientUniverseDomain string
}

// getClientUniverseDomain returns the default service domain for a given Cloud
// universe, with the following precedence:
//
// 1. A non-empty option.WithUniverseDomain or similar client option.
// 2. A non-empty environment variable GOOGLE_CLOUD_UNIVERSE_DOMAIN.
// 3. The default value "googleapis.com".
//
// This is the universe domain configured for the client, which will be compared
// to the universe domain that is separately configured for the credentials.
func (c *grpcCredentialsProvider) getClientUniverseDomain() string {
	if c.clientUniverseDomain != "" {
		return c.clientUniverseDomain
	}
	if envUD := os.Getenv(internal.UniverseDomainEnvVar); envUD != "" {
		return envUD
	}
	return internal.DefaultUniverseDomain
}

func (c *grpcCredentialsProvider) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	token, err := c.creds.Token(ctx)
	if err != nil {
		return nil, err
	}
	if token.MetadataString("auth.google.tokenSource") != "compute-metadata" {
		credentialsUniverseDomain, err := c.creds.UniverseDomain(ctx)
		if err != nil {
			return nil, err
		}
		if err := transport.ValidateUniverseDomain(c.getClientUniverseDomain(), credentialsUniverseDomain); err != nil {
			return nil, err
		}
	}
	if c.secure {
		ri, _ := grpccreds.RequestInfoFromContext(ctx)
		if err = grpccreds.CheckSecurityLevel(ri.AuthInfo, grpccreds.PrivacyAndIntegrity); err != nil {
			return nil, fmt.Errorf("unable to transfer credentials PerRPCCredentials: %v", err)
		}
	}
	metadata := make(map[string]string, len(c.metadata)+1)
	headers.SetAuthMetadata(token, metadata)
	for k, v := range c.metadata {
		metadata[k] = v
	}
	return metadata, nil
}

func (c *grpcCredentialsProvider) RequireTransportSecurity() bool {
	return c.secure
}

func addOpenTelemetryStatsHandler(dialOpts []grpc.DialOption, opts *Options) []grpc.DialOption {
	if opts.DisableTelemetry {
		return dialOpts
	}
	if !gax.IsFeatureEnabled("TRACING") {
		return append(dialOpts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	}
	var staticAttrs []attribute.KeyValue
	if opts.InternalOptions != nil {
		staticAttrs = transport.StaticTelemetryAttributes(opts.InternalOptions.TelemetryAttributes)
	}
	otelOpts := []otelgrpc.Option{
		otelgrpc.WithSpanAttributes(staticAttrs...),
	}
	return append(dialOpts, grpc.WithStatsHandler(&otelHandler{
		Handler: otelgrpc.NewClientHandler(otelOpts...),
	}))
}

// otelHandler is a wrapper around the OpenTelemetry gRPC client handler that
// adds custom Google Cloud-specific attributes to spans and metrics.
type otelHandler struct {
	stats.Handler
}

// TagRPC intercepts the RPC start to extract dynamic attributes like resource
// name and retry count from the outgoing context metadata and attach them to
// the current span.
func (h *otelHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	ctx = h.Handler.TagRPC(ctx, info)
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return ctx
	}
	var attrs []attribute.KeyValue
	if resName, ok := callctx.TelemetryFromContext(ctx, "resource_name"); ok {
		attrs = append(attrs, attribute.String("gcp.resource.destination.id", resName))
	}
	if resendCountStr, ok := callctx.TelemetryFromContext(ctx, "resend_count"); ok {
		if count, err := strconv.Atoi(resendCountStr); err == nil {
			attrs = append(attrs, attribute.Int("gcp.grpc.resend_count", count))
		}
	}
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx
}

// HandleRPC intercepts the RPC completion to capture and format error-related
// attributes ensuring they conform to Google Cloud observability standards.
func (h *otelHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	end, ok := s.(*stats.End)
	if !ok {
		h.Handler.HandleRPC(ctx, s)
		return
	}
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		h.Handler.HandleRPC(ctx, s)
		return
	}

	var attrs []attribute.KeyValue
	if end.Error != nil {
		st, ok := status.FromError(end.Error)
		rpcStatusCode := codeToCanonicalStr(st.Code())

		var errorType string
		// 1. Check if the local context expired or was cancelled. This is the only
		// reliable way to distinguish a local client timeout from a server timeout
		// because gRPC does not wrap context errors in its status.Error types.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			errorType = "CLIENT_TIMEOUT"
		} else if errors.Is(ctx.Err(), context.Canceled) {
			errorType = "CLIENT_CANCELLED"
		} else if !ok || st.Code() == codes.Unknown || st.Code() == codes.Internal {
			// 2. If the error isn't a context breakdown and the gRPC framework
			// doesn't "understand" it (returning ok=false or a generic catch-all
			// bucket like Unknown/Internal), we "pack" the actual Go error type
			// name into error.type (e.g., "*net.OpError"). This is per the error.type
			// [spec](https://opentelemetry.io/docs/specs/semconv/registry/attributes/error/#error-type).
			// "When error.type is set to a type (e.g., an exception type), its canonical
			// class name identifying the type within the artifact SHOULD be used."
			errorType = fmt.Sprintf("%T", end.Error)
		} else {
			// 3. Otherwise, it is a well-understood gRPC protocol error (e.g.,
			// PERMISSION_DENIED) likely returned by the server.
			errorType = rpcStatusCode
		}

		attrs = []attribute.KeyValue{
			attribute.String("error.type", errorType),
			attribute.String("status.message", st.Message()),
			attribute.String("rpc.response.status_code", rpcStatusCode),
			attribute.String("exception.type", fmt.Sprintf("%T", end.Error)),
		}
	} else {
		attrs = []attribute.KeyValue{
			attribute.String("rpc.response.status_code", "OK"),
		}
	}
	span.SetAttributes(attrs...)
	h.Handler.HandleRPC(ctx, s)
}

// codeToCanonicalStr returns the canonical name for each of the 17 gRPC
// status codes defined in https://github.com/grpc/grpc-go/blob/master/codes/codes.go.
// For any codes.Code that converts to an out-of-bounds int,
// it returns "UNKNOWN".
func codeToCanonicalStr(code codes.Code) string {
	if int(code) >= 0 && int(code) < len(codeToStr) {
		return codeToStr[code]
	}
	return "UNKNOWN"
}
