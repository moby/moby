package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/internal/httprule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// UnescapingMode defines the behavior of ServeMux when unescaping path parameters.
type UnescapingMode int

const (
	// UnescapingModeLegacy is the default V2 behavior, which escapes the entire
	// path string before doing any routing.
	UnescapingModeLegacy UnescapingMode = iota

	// EscapingTypeExceptReserved unescapes all path parameters except RFC 6570
	// reserved characters.
	UnescapingModeAllExceptReserved

	// EscapingTypeExceptSlash unescapes URL path parameters except path
	// seperators, which will be left as "%2F".
	UnescapingModeAllExceptSlash

	// URL path parameters will be fully decoded.
	UnescapingModeAllCharacters

	// UnescapingModeDefault is the default escaping type.
	// TODO(v3): default this to UnescapingModeAllExceptReserved per grpc-httpjson-transcoding's
	// reference implementation
	UnescapingModeDefault = UnescapingModeLegacy
)

// A HandlerFunc handles a specific pair of path pattern and HTTP method.
type HandlerFunc func(w http.ResponseWriter, r *http.Request, pathParams map[string]string)

// ServeMux is a request multiplexer for grpc-gateway.
// It matches http requests to patterns and invokes the corresponding handler.
type ServeMux struct {
	// handlers maps HTTP method to a list of handlers.
	handlers                  map[string][]handler
	forwardResponseOptions    []func(context.Context, http.ResponseWriter, proto.Message) error
	marshalers                marshalerRegistry
	incomingHeaderMatcher     HeaderMatcherFunc
	outgoingHeaderMatcher     HeaderMatcherFunc
	metadataAnnotators        []func(context.Context, *http.Request) metadata.MD
	errorHandler              ErrorHandlerFunc
	streamErrorHandler        StreamErrorHandlerFunc
	routingErrorHandler       RoutingErrorHandlerFunc
	disablePathLengthFallback bool
	unescapingMode            UnescapingMode
}

// ServeMuxOption is an option that can be given to a ServeMux on construction.
type ServeMuxOption func(*ServeMux)

// WithForwardResponseOption returns a ServeMuxOption representing the forwardResponseOption.
//
// forwardResponseOption is an option that will be called on the relevant context.Context,
// http.ResponseWriter, and proto.Message before every forwarded response.
//
// The message may be nil in the case where just a header is being sent.
func WithForwardResponseOption(forwardResponseOption func(context.Context, http.ResponseWriter, proto.Message) error) ServeMuxOption {
	return func(serveMux *ServeMux) {
		serveMux.forwardResponseOptions = append(serveMux.forwardResponseOptions, forwardResponseOption)
	}
}

// WithEscapingType sets the escaping type. See the definitions of UnescapingMode
// for more information.
func WithUnescapingMode(mode UnescapingMode) ServeMuxOption {
	return func(serveMux *ServeMux) {
		serveMux.unescapingMode = mode
	}
}

// SetQueryParameterParser sets the query parameter parser, used to populate message from query parameters.
// Configuring this will mean the generated OpenAPI output is no longer correct, and it should be
// done with careful consideration.
func SetQueryParameterParser(queryParameterParser QueryParameterParser) ServeMuxOption {
	return func(serveMux *ServeMux) {
		currentQueryParser = queryParameterParser
	}
}

// HeaderMatcherFunc checks whether a header key should be forwarded to/from gRPC context.
type HeaderMatcherFunc func(string) (string, bool)

// DefaultHeaderMatcher is used to pass http request headers to/from gRPC context. This adds permanent HTTP header
// keys (as specified by the IANA) to gRPC context with grpcgateway- prefix. HTTP headers that start with
// 'Grpc-Metadata-' are mapped to gRPC metadata after removing prefix 'Grpc-Metadata-'.
func DefaultHeaderMatcher(key string) (string, bool) {
	key = textproto.CanonicalMIMEHeaderKey(key)
	if isPermanentHTTPHeader(key) {
		return MetadataPrefix + key, true
	} else if strings.HasPrefix(key, MetadataHeaderPrefix) {
		return key[len(MetadataHeaderPrefix):], true
	}
	return "", false
}

// WithIncomingHeaderMatcher returns a ServeMuxOption representing a headerMatcher for incoming request to gateway.
//
// This matcher will be called with each header in http.Request. If matcher returns true, that header will be
// passed to gRPC context. To transform the header before passing to gRPC context, matcher should return modified header.
func WithIncomingHeaderMatcher(fn HeaderMatcherFunc) ServeMuxOption {
	return func(mux *ServeMux) {
		mux.incomingHeaderMatcher = fn
	}
}

// WithOutgoingHeaderMatcher returns a ServeMuxOption representing a headerMatcher for outgoing response from gateway.
//
// This matcher will be called with each header in response header metadata. If matcher returns true, that header will be
// passed to http response returned from gateway. To transform the header before passing to response,
// matcher should return modified header.
func WithOutgoingHeaderMatcher(fn HeaderMatcherFunc) ServeMuxOption {
	return func(mux *ServeMux) {
		mux.outgoingHeaderMatcher = fn
	}
}

// WithMetadata returns a ServeMuxOption for passing metadata to a gRPC context.
//
// This can be used by services that need to read from http.Request and modify gRPC context. A common use case
// is reading token from cookie and adding it in gRPC context.
func WithMetadata(annotator func(context.Context, *http.Request) metadata.MD) ServeMuxOption {
	return func(serveMux *ServeMux) {
		serveMux.metadataAnnotators = append(serveMux.metadataAnnotators, annotator)
	}
}

// WithErrorHandler returns a ServeMuxOption for configuring a custom error handler.
//
// This can be used to configure a custom error response.
func WithErrorHandler(fn ErrorHandlerFunc) ServeMuxOption {
	return func(serveMux *ServeMux) {
		serveMux.errorHandler = fn
	}
}

// WithStreamErrorHandler returns a ServeMuxOption that will use the given custom stream
// error handler, which allows for customizing the error trailer for server-streaming
// calls.
//
// For stream errors that occur before any response has been written, the mux's
// ErrorHandler will be invoked. However, once data has been written, the errors must
// be handled differently: they must be included in the response body. The response body's
// final message will include the error details returned by the stream error handler.
func WithStreamErrorHandler(fn StreamErrorHandlerFunc) ServeMuxOption {
	return func(serveMux *ServeMux) {
		serveMux.streamErrorHandler = fn
	}
}

// WithRoutingErrorHandler returns a ServeMuxOption for configuring a custom error handler to  handle http routing errors.
//
// Method called for errors which can happen before gRPC route selected or executed.
// The following error codes: StatusMethodNotAllowed StatusNotFound StatusBadRequest
func WithRoutingErrorHandler(fn RoutingErrorHandlerFunc) ServeMuxOption {
	return func(serveMux *ServeMux) {
		serveMux.routingErrorHandler = fn
	}
}

// WithDisablePathLengthFallback returns a ServeMuxOption for disable path length fallback.
func WithDisablePathLengthFallback() ServeMuxOption {
	return func(serveMux *ServeMux) {
		serveMux.disablePathLengthFallback = true
	}
}

// NewServeMux returns a new ServeMux whose internal mapping is empty.
func NewServeMux(opts ...ServeMuxOption) *ServeMux {
	serveMux := &ServeMux{
		handlers:               make(map[string][]handler),
		forwardResponseOptions: make([]func(context.Context, http.ResponseWriter, proto.Message) error, 0),
		marshalers:             makeMarshalerMIMERegistry(),
		errorHandler:           DefaultHTTPErrorHandler,
		streamErrorHandler:     DefaultStreamErrorHandler,
		routingErrorHandler:    DefaultRoutingErrorHandler,
		unescapingMode:         UnescapingModeDefault,
	}

	for _, opt := range opts {
		opt(serveMux)
	}

	if serveMux.incomingHeaderMatcher == nil {
		serveMux.incomingHeaderMatcher = DefaultHeaderMatcher
	}

	if serveMux.outgoingHeaderMatcher == nil {
		serveMux.outgoingHeaderMatcher = func(key string) (string, bool) {
			return fmt.Sprintf("%s%s", MetadataHeaderPrefix, key), true
		}
	}

	return serveMux
}

// Handle associates "h" to the pair of HTTP method and path pattern.
func (s *ServeMux) Handle(meth string, pat Pattern, h HandlerFunc) {
	s.handlers[meth] = append([]handler{{pat: pat, h: h}}, s.handlers[meth]...)
}

// HandlePath allows users to configure custom path handlers.
// refer: https://grpc-ecosystem.github.io/grpc-gateway/docs/operations/inject_router/
func (s *ServeMux) HandlePath(meth string, pathPattern string, h HandlerFunc) error {
	compiler, err := httprule.Parse(pathPattern)
	if err != nil {
		return fmt.Errorf("parsing path pattern: %w", err)
	}
	tp := compiler.Compile()
	pattern, err := NewPattern(tp.Version, tp.OpCodes, tp.Pool, tp.Verb)
	if err != nil {
		return fmt.Errorf("creating new pattern: %w", err)
	}
	s.Handle(meth, pattern, h)
	return nil
}

// ServeHTTP dispatches the request to the first handler whose pattern matches to r.Method and r.Path.
func (s *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	path := r.URL.Path
	if !strings.HasPrefix(path, "/") {
		_, outboundMarshaler := MarshalerForRequest(s, r)
		s.routingErrorHandler(ctx, s, outboundMarshaler, w, r, http.StatusBadRequest)
		return
	}

	// TODO(v3): remove UnescapingModeLegacy
	if s.unescapingMode != UnescapingModeLegacy && r.URL.RawPath != "" {
		path = r.URL.RawPath
	}

	components := strings.Split(path[1:], "/")

	if override := r.Header.Get("X-HTTP-Method-Override"); override != "" && s.isPathLengthFallback(r) {
		r.Method = strings.ToUpper(override)
		if err := r.ParseForm(); err != nil {
			_, outboundMarshaler := MarshalerForRequest(s, r)
			sterr := status.Error(codes.InvalidArgument, err.Error())
			s.errorHandler(ctx, s, outboundMarshaler, w, r, sterr)
			return
		}
	}

	// Verb out here is to memoize for the fallback case below
	var verb string

	for _, h := range s.handlers[r.Method] {
		// If the pattern has a verb, explicitly look for a suffix in the last
		// component that matches a colon plus the verb. This allows us to
		// handle some cases that otherwise can't be correctly handled by the
		// former LastIndex case, such as when the verb literal itself contains
		// a colon. This should work for all cases that have run through the
		// parser because we know what verb we're looking for, however, there
		// are still some cases that the parser itself cannot disambiguate. See
		// the comment there if interested.
		patVerb := h.pat.Verb()
		l := len(components)
		lastComponent := components[l-1]
		var idx int = -1
		if patVerb != "" && strings.HasSuffix(lastComponent, ":"+patVerb) {
			idx = len(lastComponent) - len(patVerb) - 1
		}
		if idx == 0 {
			_, outboundMarshaler := MarshalerForRequest(s, r)
			s.routingErrorHandler(ctx, s, outboundMarshaler, w, r, http.StatusNotFound)
			return
		}
		if idx > 0 {
			components[l-1], verb = lastComponent[:idx], lastComponent[idx+1:]
		}

		pathParams, err := h.pat.MatchAndEscape(components, verb, s.unescapingMode)
		if err != nil {
			var mse MalformedSequenceError
			if ok := errors.As(err, &mse); ok {
				_, outboundMarshaler := MarshalerForRequest(s, r)
				s.errorHandler(ctx, s, outboundMarshaler, w, r, &HTTPStatusError{
					HTTPStatus: http.StatusBadRequest,
					Err:        mse,
				})
			}
			continue
		}
		h.h(w, r, pathParams)
		return
	}

	// lookup other methods to handle fallback from GET to POST and
	// to determine if it is NotImplemented or NotFound.
	for m, handlers := range s.handlers {
		if m == r.Method {
			continue
		}
		for _, h := range handlers {
			pathParams, err := h.pat.MatchAndEscape(components, verb, s.unescapingMode)
			if err != nil {
				var mse MalformedSequenceError
				if ok := errors.As(err, &mse); ok {
					_, outboundMarshaler := MarshalerForRequest(s, r)
					s.errorHandler(ctx, s, outboundMarshaler, w, r, &HTTPStatusError{
						HTTPStatus: http.StatusBadRequest,
						Err:        mse,
					})
				}
				continue
			}
			// X-HTTP-Method-Override is optional. Always allow fallback to POST.
			if s.isPathLengthFallback(r) {
				if err := r.ParseForm(); err != nil {
					_, outboundMarshaler := MarshalerForRequest(s, r)
					sterr := status.Error(codes.InvalidArgument, err.Error())
					s.errorHandler(ctx, s, outboundMarshaler, w, r, sterr)
					return
				}
				h.h(w, r, pathParams)
				return
			}
			_, outboundMarshaler := MarshalerForRequest(s, r)
			s.routingErrorHandler(ctx, s, outboundMarshaler, w, r, http.StatusMethodNotAllowed)
			return
		}
	}

	_, outboundMarshaler := MarshalerForRequest(s, r)
	s.routingErrorHandler(ctx, s, outboundMarshaler, w, r, http.StatusNotFound)
}

// GetForwardResponseOptions returns the ForwardResponseOptions associated with this ServeMux.
func (s *ServeMux) GetForwardResponseOptions() []func(context.Context, http.ResponseWriter, proto.Message) error {
	return s.forwardResponseOptions
}

func (s *ServeMux) isPathLengthFallback(r *http.Request) bool {
	return !s.disablePathLengthFallback && r.Method == "POST" && r.Header.Get("Content-Type") == "application/x-www-form-urlencoded"
}

type handler struct {
	pat Pattern
	h   HandlerFunc
}
