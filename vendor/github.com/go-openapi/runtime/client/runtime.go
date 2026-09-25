// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/client/internal/request"
	"github.com/go-openapi/runtime/logger"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/runtime/server-middleware/mediatype"
	"github.com/go-openapi/runtime/yamlpc"
	"github.com/go-openapi/strfmt"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// DefaultTimeout the default request timeout.
var DefaultTimeout = 30 * time.Second

// Runtime represents an API client that uses the transport
// to make [http] requests based on a swagger specification.
type Runtime struct {
	DefaultMediaType      string
	DefaultAuthentication runtime.ClientAuthInfoWriter
	Consumers             map[string]runtime.Consumer
	Producers             map[string]runtime.Producer

	Transport http.RoundTripper
	Jar       http.CookieJar
	// Spec      *spec.Document
	Host     string
	BasePath string
	Formats  strfmt.Registry
	// Deprecated: prefer [runtime.ContextualTransport.SubmitContext] to pass the request context explicitly.
	Context context.Context //nolint:containedctx  // we precisely want this type to contain the request context

	Debug bool

	// Trace enables connection-level diagnostic output via
	// [net/http/httptrace]. When true, the runtime narrates the
	// connection lifecycle of every request through r.logger.Debugf:
	// DNS, dial, TLS handshake, idle-pool reuse, request body
	// transfer, time-to-first-byte, response body transfer, and a
	// trailing per-request summary line.
	//
	// Trace is orthogonal to Debug: Debug dumps wire bytes (request
	// and response headers and body), Trace narrates how the
	// connection got there. Both may be enabled independently.
	//
	// Trace is not coupled to the SWAGGER_DEBUG / DEBUG environment
	// variables: it defaults to false and is only enabled by
	// explicit assignment.
	//
	// Trace is primarily intended as a problem-investigation tool
	// (the local equivalent of curl -vvv), not an always-on tracer.
	// For distributed-trace correlation, use the OpenTelemetry
	// integration ([Runtime.WithOpenTelemetry]).
	Trace bool

	logger logger.Logger

	// MatchSuffix enables RFC 6839 structured-syntax suffix tolerance
	// for codec lookup. When true, a response with Content-Type
	// "application/problem+json" finds the JSON consumer registered
	// under "application/json"; with the default false, the lookup
	// is strict and falls through to the "*/*" wildcard if present.
	// See [mediatype.AllowSuffix] for the semantics.
	MatchSuffix bool

	clientOnce *sync.Once
	client     *http.Client
	schemes    []string
	response   ClientResponseFunc
}

var _ runtime.ContextualTransport = &Runtime{}

// New creates a new default runtime for a swagger api runtime.Client.
func New(host, basePath string, schemes []string) *Runtime {
	var rt Runtime
	rt.DefaultMediaType = runtime.JSONMime

	// Enhancement proposal: https://github.com/go-openapi/runtime/issues/385
	rt.Consumers = map[string]runtime.Consumer{
		runtime.YAMLMime:           yamlpc.YAMLConsumer(),
		runtime.JSONMime:           runtime.JSONConsumer(),
		runtime.XMLMime:            runtime.XMLConsumer(),
		runtime.TextMime:           runtime.TextConsumer(),
		runtime.HTMLMime:           runtime.TextConsumer(),
		runtime.CSVMime:            runtime.CSVConsumer(),
		runtime.MultipartFormMime:  runtime.ByteStreamConsumer(),
		runtime.URLencodedFormMime: runtime.ByteStreamConsumer(),
		runtime.DefaultMime:        runtime.ByteStreamConsumer(),
	}
	rt.Producers = map[string]runtime.Producer{
		runtime.YAMLMime:    yamlpc.YAMLProducer(),
		runtime.JSONMime:    runtime.JSONProducer(),
		runtime.XMLMime:     runtime.XMLProducer(),
		runtime.TextMime:    runtime.TextProducer(),
		runtime.HTMLMime:    runtime.TextProducer(),
		runtime.CSVMime:     runtime.CSVProducer(),
		runtime.DefaultMime: runtime.ByteStreamProducer(),
	}
	rt.Transport = http.DefaultTransport
	rt.Jar = nil
	rt.Host = host
	rt.BasePath = basePath
	rt.Context = context.Background()
	rt.clientOnce = new(sync.Once)
	if !strings.HasPrefix(rt.BasePath, "/") {
		rt.BasePath = "/" + rt.BasePath
	}

	rt.Debug = logger.DebugEnabled()
	rt.logger = logger.StandardLogger{}
	rt.response = newResponse

	if len(schemes) > 0 {
		rt.schemes = schemes
	}
	return &rt
}

// NewWithClient allows you to create a new transport with a configured [http.Client].
func NewWithClient(host, basePath string, schemes []string, client *http.Client) *Runtime {
	rt := New(host, basePath, schemes)
	if client != nil {
		rt.clientOnce.Do(func() {
			rt.client = client
		})
	}
	return rt
}

// EnableConnectionReuse drains the remaining body from a response
// so that go will reuse the TCP connections.
//
// This is not enabled by default because there are servers where
// the response never gets closed and that would make the code hang forever.
// So instead it's provided as a [http] client [middleware] that can be used to override
// any request.
func (r *Runtime) EnableConnectionReuse() {
	if r.client == nil {
		r.Transport = KeepAliveTransport(
			transportOrDefault(r.Transport, http.DefaultTransport),
		)
		return
	}

	r.client.Transport = KeepAliveTransport(
		transportOrDefault(r.client.Transport,
			transportOrDefault(r.Transport, http.DefaultTransport),
		),
	)
}

// CreateHTTPRequestContext creates the requests and bind the parameters, but does not send it over the wire
// like [Runtime.SubmitContext].
//
// The [http.Request] is complete with authentication, headers and body (including streamed body) and ready for callers
// to submit it to a [http.Client] of their choice, then consume the [http.Response].
//
// Most users would simply use [Runtime.SubmitContext], which wraps all these operations in one call.
func (r *Runtime) CreateHTTPRequestContext(ctx context.Context, operation *runtime.ClientOperation) (req *http.Request, cancel context.CancelFunc, err error) {
	req, cancel, err = r.createHTTPRequestContext(ctx, operation)
	return
}

// CreateHttpRequest builds the [http.Request] for the given operation, using
// [context.Background] as the request context.
//
// Any per-operation timeout declared by the operation's [runtime.ClientRequestWriter]
// is silently ignored here, which can leak a context-cancellation channel if the
// caller relies on it.
//
// Deprecated: use [Runtime.CreateHTTPRequestContext] instead, with explicit
// control over the request context and its cancellation.
func (r *Runtime) CreateHttpRequest(operation *runtime.ClientOperation) (req *http.Request, err error) { //nolint:revive
	req, _, err = r.createHTTPRequestContext(context.Background(), operation)
	return
}

// Submit a request and when there is a body on success it will turn that into the result
// all other things are turned into an api error for swagger which retains the status code.
//
// This call inherits the context possibly put in the operation, otherwise the one possibly put in the [Runtime].
// If none are set, use [context.Background].
//
// Any timeout set by parameters is honored.
func (r *Runtime) Submit(operation *runtime.ClientOperation) (any, error) {
	return r.SubmitContext(r.ensureContext(operation), operation)
}

// SubmitContext submits a request and returns the result.
//
// Errors are turned into an api error for swagger which retains the status code.
//
// Unlike [Submit], [SubmitContext] only injects the context provided by the caller:
// contexts possibly cached in operation or runtime are ignored.
//
// On the other hand, a timeout set by parameters is honored.
func (r *Runtime) SubmitContext(parentCtx context.Context, operation *runtime.ClientOperation) (any, error) {
	req, cancel, err := r.createHTTPRequestContext(parentCtx, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()

	r.ensureClient()

	if err := r.dumpRequest(req); err != nil {
		return nil, err
	}

	// Attach the trace session before Do so the httptrace hooks
	// fire during the round-trip. The session emits its trailing
	// summary on finish; the response body is consumed by
	// ReadResponse downstream, after which finish is called.
	var trace *traceSession
	if r.Trace {
		trace = newTraceSession(r.logger, req.Method, req.URL.String(),
			introspectTLSConfig(r.pickClient(operation)))
		//nolint:contextcheck // We intentionally derive from req.Context() to layer the trace hooks onto the existing request context.
		req = req.WithContext(trace.attach(req.Context()))
		if req.Body != nil {
			req.Body = trace.wrapRequestBody(req.Body)
		}
		defer trace.finish()
	}

	res, err := r.pickClient(operation).Do(req)
	if err != nil {
		if trace != nil {
			trace.onRoundTripError(err)
		}
		return nil, err
	}
	defer res.Body.Close()

	if trace != nil {
		trace.onResponse(res.StatusCode)
		res.Body = trace.wrapResponseBody(res.Body)
	}

	ct := res.Header.Get(runtime.HeaderContentType)
	if ct == "" { // this should really never occur
		ct = r.DefaultMediaType
	}

	if err := r.dumpResponse(res, ct); err != nil {
		return nil, err
	}

	cons, err := r.resolveConsumer(ct)
	if err != nil {
		return nil, err
	}

	return operation.Reader.ReadResponse(r.response(res), cons)
}

// SetDebug changes the debug flag.
// It ensures that client and middlewares have the set debug level.
func (r *Runtime) SetDebug(debug bool) {
	r.Debug = debug
	middleware.Debug = debug
}

// SetLogger changes the logger stream.
// It ensures that client and middlewares use the same logger.
func (r *Runtime) SetLogger(logger logger.Logger) {
	r.logger = logger
	middleware.Logger = logger
}

type ClientResponseFunc = func(*http.Response) runtime.ClientResponse //nolint:revive

// SetResponseReader changes the response reader implementation.
func (r *Runtime) SetResponseReader(f ClientResponseFunc) {
	if f == nil {
		return
	}
	r.response = f
}

func (r *Runtime) ensureContext(operation *runtime.ClientOperation) context.Context {
	switch {
	case operation.Context != nil: //nolint:staticcheck // kept for backward compatibility
		return operation.Context
	case r.Context != nil:
		return r.Context
	default:
		return context.Background()
	}
}

func (r *Runtime) pickScheme(schemes []string) string {
	if v := r.selectScheme(r.schemes); v != "" {
		return v
	}
	if v := r.selectScheme(schemes); v != "" {
		return v
	}
	return schemeHTTP
}

func (r *Runtime) selectScheme(schemes []string) string {
	schLen := len(schemes)
	if schLen == 0 {
		return ""
	}

	scheme := schemes[0]
	// prefer https, but skip when not possible
	if scheme != schemeHTTPS && schLen > 1 {
		for _, sch := range schemes {
			if sch == schemeHTTPS {
				scheme = sch
				break
			}
		}
	}
	return scheme
}

func transportOrDefault(left, right http.RoundTripper) http.RoundTripper {
	if left == nil {
		return right
	}
	return left
}

// ensureClient lazily initializes r.client from r.Transport and r.Jar
// on first use. Safe under concurrent calls via sync.Once.
func (r *Runtime) ensureClient() {
	r.clientOnce.Do(func() {
		r.client = &http.Client{
			Transport: r.Transport,
			Jar:       r.Jar,
		}
	})
}

// pickClient returns the http.Client to use for this operation: the
// per-operation override if set, else the runtime's shared client.
func (r *Runtime) pickClient(operation *runtime.ClientOperation) *http.Client {
	if operation.Client != nil {
		return operation.Client
	}
	return r.client
}

// dumpRequest writes the outgoing request to the debug logger when
// r.Debug is enabled. No-op otherwise. Returns the dump error so the
// caller can decide whether to abort the submit.
func (r *Runtime) dumpRequest(req *http.Request) error {
	if !r.Debug {
		return nil
	}
	b, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return err
	}
	r.logger.Debugf("%s\n", string(b))
	return nil
}

// dumpResponse writes the incoming response to the debug logger when
// r.Debug is enabled. The body is omitted for runtime.DefaultMime
// (binary blob). No-op otherwise.
func (r *Runtime) dumpResponse(res *http.Response, ct string) error {
	if !r.Debug {
		return nil
	}
	printBody := ct != runtime.DefaultMime // Spare the terminal from a binary blob.
	b, err := httputil.DumpResponse(res, printBody)
	if err != nil {
		return err
	}
	r.logger.Debugf("%s\n", string(b))
	return nil
}

// resolveConsumer parses ct and returns the registered Consumer for
// that media type. Lookup is alias-aware (RFC 9512 §2.1 — yaml
// aliases) and, when [Runtime.MatchSuffix] is true, also tolerates
// RFC 6839 structured-syntax suffix media types (+json, +xml, +yaml).
// Falls back to the "*/*" entry if no match found.
func (r *Runtime) resolveConsumer(ct string) (runtime.Consumer, error) {
	if _, _, err := mime.ParseMediaType(ct); err != nil {
		return nil, fmt.Errorf("parse content type: %w", err)
	}
	if cons, ok := mediatype.Lookup(r.Consumers, ct, r.matchOpts()...); ok {
		return cons, nil
	}
	if cons, ok := r.Consumers["*/*"]; ok {
		return cons, nil
	}
	// scream about not knowing what to do
	return nil, fmt.Errorf("no consumer: %q", ct)
}

// matchOpts builds the mediatype.MatchOption slice for codec
// lookups on the Runtime, currently just the AllowSuffix opt-in.
func (r *Runtime) matchOpts() []mediatype.MatchOption {
	if !r.MatchSuffix {
		return nil
	}

	return []mediatype.MatchOption{mediatype.AllowSuffix()}
}

// createHTTPRequestContext is the context-aware builder of a [http.Request].
//
// The returned [http.Request] carries a context derived from parentCtx that
// honors the per-request timeout set during WriteToRequest. Callers must
// invoke cancel once the response is fully read.
func (r *Runtime) createHTTPRequestContext(parentCtx context.Context, operation *runtime.ClientOperation) (*http.Request, context.CancelFunc, error) {
	req, cmt, auth, err := r.prepareRequest(operation)
	if err != nil {
		return nil, nil, err
	}

	httpReq, cancel, err := req.BuildHTTPContext(parentCtx, cmt, r.BasePath, r.Producers, r.Formats, auth)
	if err != nil {
		return nil, nil, err
	}

	r.applyHostScheme(httpReq, operation)

	return httpReq, cancel, nil
}

// prepareRequest performs the operation-to-request setup that is
// independent of how the http.Request is finally assembled: parameters,
// headers, default authentication, and consumes-media-type selection.
func (r *Runtime) prepareRequest(operation *runtime.ClientOperation) (*request.Request, string, runtime.ClientAuthInfoWriter, error) {
	params, _, auth := operation.Params, operation.Reader, operation.AuthInfo

	req := request.New(operation.Method, operation.PathPattern, params)
	_ = req.SetTimeout(DefaultTimeout) // the timeout may be overridden by ClientRequestWriter
	req.SetConsumes(operation.ConsumesMediaTypes)

	accept := make([]string, 0, len(operation.ProducesMediaTypes))
	accept = append(accept, operation.ProducesMediaTypes...)
	if err := req.SetHeaderParam(runtime.HeaderAccept, accept...); err != nil {
		return nil, "", nil, err
	}

	if auth == nil && r.DefaultAuthentication != nil {
		auth = runtime.ClientAuthInfoWriterFunc(func(req runtime.ClientRequest, reg strfmt.Registry) error {
			if req.GetHeaderParams().Get(runtime.HeaderAuthorization) != "" {
				return nil
			}
			return r.DefaultAuthentication.AuthenticateRequest(req, reg)
		})
	}

	cmt := pickConsumesMediaType(operation.ConsumesMediaTypes, r.Producers, r.DefaultMediaType, r.matchOpts()...)
	if _, ok := mediatype.Lookup(r.Producers, cmt, r.matchOpts()...); !ok && cmt != runtime.MultipartFormMime && cmt != runtime.URLencodedFormMime {
		return nil, "", nil, fmt.Errorf("none of producers: %v registered. try %s", r.Producers, cmt)
	}

	return req, cmt, auth, nil
}

// applyHostScheme stamps the runtime's host and the operation-selected
// scheme onto the freshly built http.Request.
func (r *Runtime) applyHostScheme(httpReq *http.Request, operation *runtime.ClientOperation) {
	httpReq.URL.Scheme = r.pickScheme(operation.Schemes)
	httpReq.URL.Host = r.Host
	httpReq.Host = r.Host
}

// pickConsumesMediaType selects which Content-Type the client will send.
//
// Selection rules, in priority order:
//
//  1. multipart/form-data if any consumes entry advertises it (it streams
//     and preserves per-file Content-Type, regardless of codegen ordering;
//     resolves issue #286);
//  2. the first non-empty entry whose mime is either structural
//     (multipart/form-data or application/x-www-form-urlencoded — these
//     do not need a producer in the map) or has a producer registered in
//     producers — this lets the client gracefully skip unregistered
//     spec entries instead of erroring at the gate that follows;
//  3. the first non-empty entry overall (preserves the historical error
//     path: the gate at the call site reports "none of producers" with
//     the unregistered mime, so the diagnostic is unchanged when nothing
//     in consumes is registered);
//  4. def, if consumes is empty or all empty strings.
//
// Step 2 closes part of issues #32 and #386: an operation declaring
// `consumes: [application/x-vendor, application/json]` with no vendor
// producer registered now silently uses JSON instead of erroring.
func pickConsumesMediaType(consumes []string, producers map[string]runtime.Producer, def string, opts ...mediatype.MatchOption) string {
	for _, mt := range consumes {
		if strings.EqualFold(mt, runtime.MultipartFormMime) {
			return mt
		}
	}
	var firstNonEmpty string
	for _, mt := range consumes {
		if mt == "" {
			continue
		}
		if firstNonEmpty == "" {
			firstNonEmpty = mt
		}
		if isStructuralMime(mt) {
			return mt
		}
		if _, ok := mediatype.Lookup(producers, mt, opts...); ok {
			return mt
		}
	}
	if firstNonEmpty != "" {
		return firstNonEmpty
	}
	return def
}

// isStructuralMime reports whether mt is a media type whose body shape
// is owned by the runtime (multipart envelope, urlencoded form). These
// do not require an entry in the producers map.
func isStructuralMime(mt string) bool {
	return strings.EqualFold(mt, runtime.MultipartFormMime) ||
		strings.EqualFold(mt, runtime.URLencodedFormMime)
}
