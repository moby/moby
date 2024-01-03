package v4

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	v4Internal "github.com/aws/aws-sdk-go-v2/aws/signer/internal/v4"
	"github.com/aws/aws-sdk-go-v2/internal/sdk"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const computePayloadHashMiddlewareID = "ComputePayloadHash"

// HashComputationError indicates an error occurred while computing the signing hash
type HashComputationError struct {
	Err error
}

// Error is the error message
func (e *HashComputationError) Error() string {
	return fmt.Sprintf("failed to compute payload hash: %v", e.Err)
}

// Unwrap returns the underlying error if one is set
func (e *HashComputationError) Unwrap() error {
	return e.Err
}

// SigningError indicates an error condition occurred while performing SigV4 signing
type SigningError struct {
	Err error
}

func (e *SigningError) Error() string {
	return fmt.Sprintf("failed to sign request: %v", e.Err)
}

// Unwrap returns the underlying error cause
func (e *SigningError) Unwrap() error {
	return e.Err
}

// UseDynamicPayloadSigningMiddleware swaps the compute payload sha256 middleware with a resolver middleware that
// switches between unsigned and signed payload based on TLS state for request.
// This middleware should not be used for AWS APIs that do not support unsigned payload signing auth.
// By default, SDK uses this middleware for known AWS APIs that support such TLS based auth selection .
//
// Usage example -
// S3 PutObject API allows unsigned payload signing auth usage when TLS is enabled, and uses this middleware to
// dynamically switch between unsigned and signed payload based on TLS state for request.
func UseDynamicPayloadSigningMiddleware(stack *middleware.Stack) error {
	_, err := stack.Build.Swap(computePayloadHashMiddlewareID, &dynamicPayloadSigningMiddleware{})
	return err
}

// dynamicPayloadSigningMiddleware dynamically resolves the middleware that computes and set payload sha256 middleware.
type dynamicPayloadSigningMiddleware struct {
}

// ID returns the resolver identifier
func (m *dynamicPayloadSigningMiddleware) ID() string {
	return computePayloadHashMiddlewareID
}

// HandleBuild sets a resolver that directs to the payload sha256 compute handler.
func (m *dynamicPayloadSigningMiddleware) HandleBuild(
	ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, fmt.Errorf("unknown transport type %T", in.Request)
	}

	// if TLS is enabled, use unsigned payload when supported
	if req.IsHTTPS() {
		return (&unsignedPayload{}).HandleBuild(ctx, in, next)
	}

	// else fall back to signed payload
	return (&computePayloadSHA256{}).HandleBuild(ctx, in, next)
}

// unsignedPayload sets the SigV4 request payload hash to unsigned.
//
// Will not set the Unsigned Payload magic SHA value, if a SHA has already been
// stored in the context. (e.g. application pre-computed SHA256 before making
// API call).
//
// This middleware does not check the X-Amz-Content-Sha256 header, if that
// header is serialized a middleware must translate it into the context.
type unsignedPayload struct{}

// AddUnsignedPayloadMiddleware adds unsignedPayload to the operation
// middleware stack
func AddUnsignedPayloadMiddleware(stack *middleware.Stack) error {
	return stack.Build.Add(&unsignedPayload{}, middleware.After)
}

// ID returns the unsignedPayload identifier
func (m *unsignedPayload) ID() string {
	return computePayloadHashMiddlewareID
}

// HandleBuild sets the payload hash to be an unsigned payload
func (m *unsignedPayload) HandleBuild(
	ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	// This should not compute the content SHA256 if the value is already
	// known. (e.g. application pre-computed SHA256 before making API call).
	// Does not have any tight coupling to the X-Amz-Content-Sha256 header, if
	// that header is provided a middleware must translate it into the context.
	contentSHA := GetPayloadHash(ctx)
	if len(contentSHA) == 0 {
		contentSHA = v4Internal.UnsignedPayload
	}

	ctx = SetPayloadHash(ctx, contentSHA)
	return next.HandleBuild(ctx, in)
}

// computePayloadSHA256 computes SHA256 payload hash to sign.
//
// Will not set the Unsigned Payload magic SHA value, if a SHA has already been
// stored in the context. (e.g. application pre-computed SHA256 before making
// API call).
//
// This middleware does not check the X-Amz-Content-Sha256 header, if that
// header is serialized a middleware must translate it into the context.
type computePayloadSHA256 struct{}

// AddComputePayloadSHA256Middleware adds computePayloadSHA256 to the
// operation middleware stack
func AddComputePayloadSHA256Middleware(stack *middleware.Stack) error {
	return stack.Build.Add(&computePayloadSHA256{}, middleware.After)
}

// RemoveComputePayloadSHA256Middleware removes computePayloadSHA256 from the
// operation middleware stack
func RemoveComputePayloadSHA256Middleware(stack *middleware.Stack) error {
	_, err := stack.Build.Remove(computePayloadHashMiddlewareID)
	return err
}

// ID is the middleware name
func (m *computePayloadSHA256) ID() string {
	return computePayloadHashMiddlewareID
}

// HandleBuild compute the payload hash for the request payload
func (m *computePayloadSHA256) HandleBuild(
	ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, &HashComputationError{
			Err: fmt.Errorf("unexpected request middleware type %T", in.Request),
		}
	}

	// This should not compute the content SHA256 if the value is already
	// known. (e.g. application pre-computed SHA256 before making API call)
	// Does not have any tight coupling to the X-Amz-Content-Sha256 header, if
	// that header is provided a middleware must translate it into the context.
	if contentSHA := GetPayloadHash(ctx); len(contentSHA) != 0 {
		return next.HandleBuild(ctx, in)
	}

	hash := sha256.New()
	if stream := req.GetStream(); stream != nil {
		_, err = io.Copy(hash, stream)
		if err != nil {
			return out, metadata, &HashComputationError{
				Err: fmt.Errorf("failed to compute payload hash, %w", err),
			}
		}

		if err := req.RewindStream(); err != nil {
			return out, metadata, &HashComputationError{
				Err: fmt.Errorf("failed to seek body to start, %w", err),
			}
		}
	}

	ctx = SetPayloadHash(ctx, hex.EncodeToString(hash.Sum(nil)))

	return next.HandleBuild(ctx, in)
}

// SwapComputePayloadSHA256ForUnsignedPayloadMiddleware replaces the
// ComputePayloadSHA256 middleware with the UnsignedPayload middleware.
//
// Use this to disable computing the Payload SHA256 checksum and instead use
// UNSIGNED-PAYLOAD for the SHA256 value.
func SwapComputePayloadSHA256ForUnsignedPayloadMiddleware(stack *middleware.Stack) error {
	_, err := stack.Build.Swap(computePayloadHashMiddlewareID, &unsignedPayload{})
	return err
}

// contentSHA256Header sets the X-Amz-Content-Sha256 header value to
// the Payload hash stored in the context.
type contentSHA256Header struct{}

// AddContentSHA256HeaderMiddleware adds ContentSHA256Header to the
// operation middleware stack
func AddContentSHA256HeaderMiddleware(stack *middleware.Stack) error {
	return stack.Build.Insert(&contentSHA256Header{}, computePayloadHashMiddlewareID, middleware.After)
}

// RemoveContentSHA256HeaderMiddleware removes contentSHA256Header middleware
// from the operation middleware stack
func RemoveContentSHA256HeaderMiddleware(stack *middleware.Stack) error {
	_, err := stack.Build.Remove((*contentSHA256Header)(nil).ID())
	return err
}

// ID returns the ContentSHA256HeaderMiddleware identifier
func (m *contentSHA256Header) ID() string {
	return "SigV4ContentSHA256Header"
}

// HandleBuild sets the X-Amz-Content-Sha256 header value to the Payload hash
// stored in the context.
func (m *contentSHA256Header) HandleBuild(
	ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, &HashComputationError{Err: fmt.Errorf("unexpected request middleware type %T", in.Request)}
	}

	req.Header.Set(v4Internal.ContentSHAKey, GetPayloadHash(ctx))

	return next.HandleBuild(ctx, in)
}

// SignHTTPRequestMiddlewareOptions is the configuration options for the SignHTTPRequestMiddleware middleware.
type SignHTTPRequestMiddlewareOptions struct {
	CredentialsProvider aws.CredentialsProvider
	Signer              HTTPSigner
	LogSigning          bool
}

// SignHTTPRequestMiddleware is a `FinalizeMiddleware` implementation for SigV4 HTTP Signing
type SignHTTPRequestMiddleware struct {
	credentialsProvider aws.CredentialsProvider
	signer              HTTPSigner
	logSigning          bool
}

// NewSignHTTPRequestMiddleware constructs a SignHTTPRequestMiddleware using the given Signer for signing requests
func NewSignHTTPRequestMiddleware(options SignHTTPRequestMiddlewareOptions) *SignHTTPRequestMiddleware {
	return &SignHTTPRequestMiddleware{
		credentialsProvider: options.CredentialsProvider,
		signer:              options.Signer,
		logSigning:          options.LogSigning,
	}
}

// ID is the SignHTTPRequestMiddleware identifier
func (s *SignHTTPRequestMiddleware) ID() string {
	return "Signing"
}

// HandleFinalize will take the provided input and sign the request using the SigV4 authentication scheme
func (s *SignHTTPRequestMiddleware) HandleFinalize(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (
	out middleware.FinalizeOutput, metadata middleware.Metadata, err error,
) {
	if !haveCredentialProvider(s.credentialsProvider) {
		return next.HandleFinalize(ctx, in)
	}

	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return out, metadata, &SigningError{Err: fmt.Errorf("unexpected request middleware type %T", in.Request)}
	}

	signingName, signingRegion := awsmiddleware.GetSigningName(ctx), awsmiddleware.GetSigningRegion(ctx)
	payloadHash := GetPayloadHash(ctx)
	if len(payloadHash) == 0 {
		return out, metadata, &SigningError{Err: fmt.Errorf("computed payload hash missing from context")}
	}

	credentials, err := s.credentialsProvider.Retrieve(ctx)
	if err != nil {
		return out, metadata, &SigningError{Err: fmt.Errorf("failed to retrieve credentials: %w", err)}
	}

	err = s.signer.SignHTTP(ctx, credentials, req.Request, payloadHash, signingName, signingRegion, sdk.NowTime(),
		func(o *SignerOptions) {
			o.Logger = middleware.GetLogger(ctx)
			o.LogSigning = s.logSigning
		})
	if err != nil {
		return out, metadata, &SigningError{Err: fmt.Errorf("failed to sign http request, %w", err)}
	}

	ctx = awsmiddleware.SetSigningCredentials(ctx, credentials)

	return next.HandleFinalize(ctx, in)
}

type streamingEventsPayload struct{}

// AddStreamingEventsPayload adds the streamingEventsPayload middleware to the stack.
func AddStreamingEventsPayload(stack *middleware.Stack) error {
	return stack.Build.Add(&streamingEventsPayload{}, middleware.After)
}

func (s *streamingEventsPayload) ID() string {
	return computePayloadHashMiddlewareID
}

func (s *streamingEventsPayload) HandleBuild(
	ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	contentSHA := GetPayloadHash(ctx)
	if len(contentSHA) == 0 {
		contentSHA = v4Internal.StreamingEventsPayload
	}

	ctx = SetPayloadHash(ctx, contentSHA)

	return next.HandleBuild(ctx, in)
}

// GetSignedRequestSignature attempts to extract the signature of the request.
// Returning an error if the request is unsigned, or unable to extract the
// signature.
func GetSignedRequestSignature(r *http.Request) ([]byte, error) {
	const authHeaderSignatureElem = "Signature="

	if auth := r.Header.Get(authorizationHeader); len(auth) != 0 {
		ps := strings.Split(auth, ", ")
		for _, p := range ps {
			if idx := strings.Index(p, authHeaderSignatureElem); idx >= 0 {
				sig := p[len(authHeaderSignatureElem):]
				if len(sig) == 0 {
					return nil, fmt.Errorf("invalid request signature authorization header")
				}
				return hex.DecodeString(sig)
			}
		}
	}

	if sig := r.URL.Query().Get("X-Amz-Signature"); len(sig) != 0 {
		return hex.DecodeString(sig)
	}

	return nil, fmt.Errorf("request not signed")
}

func haveCredentialProvider(p aws.CredentialsProvider) bool {
	if p == nil {
		return false
	}

	return !aws.IsCredentialsProvider(p, (*aws.AnonymousCredentials)(nil))
}

type payloadHashKey struct{}

// GetPayloadHash retrieves the payload hash to use for signing
//
// Scoped to stack values. Use github.com/aws/smithy-go/middleware#ClearStackValues
// to clear all stack values.
func GetPayloadHash(ctx context.Context) (v string) {
	v, _ = middleware.GetStackValue(ctx, payloadHashKey{}).(string)
	return v
}

// SetPayloadHash sets the payload hash to be used for signing the request
//
// Scoped to stack values. Use github.com/aws/smithy-go/middleware#ClearStackValues
// to clear all stack values.
func SetPayloadHash(ctx context.Context, hash string) context.Context {
	return middleware.WithStackValue(ctx, payloadHashKey{}, hash)
}
