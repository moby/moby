// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package request

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

var _ runtime.ClientRequest = new(Request) // ensure compliance to the interface

// Request represents a swagger client request.
// It binds parameters to a HTTP request.
//
// The main purpose of this struct is to hide the machinery of adding OpenAPI v2 parameters to a transport request.
//
// A generated client only implements what is necessary to turn a parameter into a valid value for these methods.
//
// There is no parameter validation here, it is assumed to be used after a spec has been validated.
//
// # Request binding
//
// The binding of parameters is carried out by method [Request.BuildHTTPContext].
//
// It analyzes parameters, which may come in different flavors:
//
//   - a file or multipart form containing a file
//   - a body which is a [io.Reader]
//   - a buffered body (regular schema body, including urlencoded form)
//
// In all cases, we may also have query or path parameters encoded in the URL, or header parameters.
//
// The result is a [http.Request], with the following properties:
//
//   - file, multipart form or [io.Reader] body: a streaming request with an attached go routine that consumes the [io.Reader].
//   - buffered body: a simple request
//
// The caller passes the parent [context.Context] to [Request.BuildHTTPContext] and receives back a cancel
// function to release the resources held by the derived request context once the response is consumed.
//
// # Authentication
//
// Authentication is built in the request by using a [runtime.ClientAuthInfoWriter].
// This helper may need to inspect the body of the request before sending authentication info.
// To cover that case, streaming bodies use a copy of the body [io.Reader] for the [runtime.ClientAuthInfoWriter]
// to consume if it wants to.
//
// # Content negotiation
//
// The [Request] detects `multipart/form-data` to switch to streamed request.
//
// `application/x-www-form-urlencoded` is also honored, even for file parameters, which are not streamed in this case.
// File parameters default behavior is `multipart/form-data`.
//
// The natural way to define the `Content-Type` header is to use the `contentType` parameter to switch to the map of
// available body producers.
//
// For buffered requests, this setting override any `Content-Type` header possibly set by calling [Request.SetHeaderParam].
//
// For streamed requests, users may want more flexibility, as we enter custom territory, with use-cases not supported by OpenAPI v2.
//
// The `Content-Type` header of a streamed request is defined using the following sequence:
//
//  1. if the caller sets an explicit value already in header — the user set it via
//     [Request.SetHeaderParam] during WriteToRequest, and we treat that as an intentional escape hatch
//  2. use payload's [runtime.ContentTyper] declaration (in this case, the produced payload knows its content type)
//  3. use `application/octet-stream` if it is available in the registered producers
//  4. otherwise set the picker's mediaType
//
// For multi-part requests, the content type of each part is auto-detected using the following sequence:
//
//  1. use [runtime.ContentTyper] declaration (in this case, the file payload knows its content type)
//  2. use [http.DetectContentType] on the first 512 bytes of the file
//
// # Concurrency
//
// A [Request] is a disposable object that is NOT intended to be reused or called concurrently.
//
// # Future evolutions
//
// There might be other similar structs that convert to other transports.
type Request struct {
	pathPattern string
	method      string
	writer      runtime.ClientRequestWriter

	pathParams map[string]string
	header     http.Header
	query      url.Values
	formFields url.Values
	fileFields map[string][]runtime.NamedReadCloser
	payload    any
	// consumes carries the operation's full ConsumesMediaTypes list so
	// that buildHTTP — which runs after the writer populates the payload
	// — can apply payload-aware fallback rules (see streamFallbackMime).
	//
	// This is set by Runtime.createHttpRequest.
	consumes []string
	timeout  time.Duration
	buf      *bytes.Buffer

	getBody func(r *Request) []byte
}

// New creates a new http client [Request] to handle OpenAPI v2 parameters.
func New(method, pathPattern string, writer runtime.ClientRequestWriter) *Request {
	return &Request{
		pathPattern: pathPattern,
		method:      method,
		writer:      writer,
		header:      make(http.Header),
		query:       make(url.Values),
		timeout:     0,
		getBody:     getRequestBuffer,
	}
}

// GetMethod yields the method being used.
func (r *Request) GetMethod() string {
	return r.method
}

// GetPath yields the URL path being used.
func (r *Request) GetPath() string {
	pth := r.pathPattern
	for k, v := range r.pathParams {
		pth = strings.ReplaceAll(pth, "{"+k+"}", v)
	}

	return pth
}

// GetBody returns the request body, if any.
//
// For streaming requests, this is a copy of the original [io.Reader].
func (r *Request) GetBody() []byte {
	return r.getBody(r)
}

// SetHeaderParam adds a header parameter to the request.
//
// The header key is always canonicalized.
//
//   - when there is only 1 value provided, it will set it.
//   - when there are several values provided, it will add all of those (no overriding).
func (r *Request) SetHeaderParam(name string, values ...string) error {
	if r.header == nil {
		r.header = make(http.Header)
	}
	r.header[http.CanonicalHeaderKey(name)] = values

	return nil
}

// GetHeaderParams returns all headers currently set for the request.
func (r *Request) GetHeaderParams() http.Header {
	return r.header
}

// SetQueryParam adds a query parameter to the request.
//
//   - when there is only 1 value provided, it will set it.
//   - when there are several values provided, it will add all of those (no overriding).
func (r *Request) SetQueryParam(name string, values ...string) error {
	if r.query == nil {
		r.query = make(url.Values)
	}
	r.query[name] = values

	return nil
}

// GetQueryParams returns a copy of all query params currently set for the request.
func (r *Request) GetQueryParams() url.Values {
	result := make(url.Values, len(r.query))
	for key, values := range r.query {
		result[key] = append([]string{}, values...)
	}

	return result
}

// SetFormParam adds a form param to the request.
//
//   - when there is only 1 value provided, it will set it.
//   - when there are several values provided, it will add all of those (no overriding).
func (r *Request) SetFormParam(name string, values ...string) error {
	if r.formFields == nil {
		r.formFields = make(url.Values)
	}
	r.formFields[name] = values

	return nil
}

// SetPathParam adds a path param to the request.
func (r *Request) SetPathParam(name string, value string) error {
	if r.pathParams == nil {
		r.pathParams = make(map[string]string)
	}

	r.pathParams[name] = value

	return nil
}

// SetFileParam adds a file parameter to the request.
//
// Files must implement [runtime.NamedReadCloser].
//
// [runtime.File] is proposed as the default concrete implementation.
func (r *Request) SetFileParam(name string, files ...runtime.NamedReadCloser) error {
	for _, file := range files {
		if actualFile, ok := file.(*os.File); ok {
			fi, err := os.Stat(actualFile.Name())
			if err != nil {
				return err
			}

			if fi.IsDir() {
				return fmt.Errorf("%q is a directory, only files are supported", file.Name())
			}
		}
	}

	if r.fileFields == nil {
		r.fileFields = make(map[string][]runtime.NamedReadCloser)
	}

	if r.formFields == nil {
		r.formFields = make(url.Values)
	}

	r.fileFields[name] = files

	return nil
}

// GetFileParam yields all file parameters.
func (r *Request) GetFileParam() map[string][]runtime.NamedReadCloser {
	return r.fileFields
}

// SetBodyParam sets a body parameter on the request.
//
// This does not yet serialize the object: actual serialization happens as late as possible.
func (r *Request) SetBodyParam(payload any) error {
	r.payload = payload

	return nil
}

// GetBodyParam returns the body payload.
func (r *Request) GetBodyParam() any {
	return r.payload
}

// GetTimeout sets the timeout for a request.
func (r *Request) GetTimeout() time.Duration {
	return r.timeout
}

// SetTimeout sets the timeout for a request.
func (r *Request) SetTimeout(timeout time.Duration) error {
	r.timeout = timeout

	return nil
}

// SetConsumes sets the list of registered consumed content for a request.
func (r *Request) SetConsumes(consumers []string) {
	r.consumes = consumers
}

// BuildHTTPContext binds the request parameters and returns a ready-to-send [http.Request].
//
// Dispatch picks one of two end-to-end builders based on whether:
//
//   - the body source is a stream (multipart pipe or stream payload)
//   - or a buffer (urlencoded form, producer output, or no body)
//
// It starts by writing the request, then proceed with adding authentication,
// then finally assembling URL or header parameters.
//
// The split mirrors the auth question: streaming bodies require a lazy body-copy closure during [AuthenticateRequest],
// whereas buffered bodies do not.
//
// The returned [http.Request] carries a context derived from parentCtx that:
//
//   - inherits any deadline or cancellation already set on parentCtx;
//   - additionally honors the per-request timeout set via [Request.SetTimeout]
//     (the [runtime.ClientRequestWriter] may override the runtime default during
//     WriteToRequest, which is why the derivation happens here rather than
//     at the call site).
//
// The returned cancel must be invoked by the caller (typically deferred)
// once the response has been fully read; otherwise resources held by the
// derived context — including any timeout timer — are leaked.
//
// On error the cancel is invoked internally and a no-op cancel is returned,
// so callers can defer cancel unconditionally.
func (r *Request) BuildHTTPContext(parentCtx context.Context, mediaType, basePath string,
	producers map[string]runtime.Producer, registry strfmt.Registry, auth runtime.ClientAuthInfoWriter,
) (*http.Request, context.CancelFunc, error) {
	if err := r.writer.WriteToRequest(r, registry); err != nil {
		return nil, noop, err
	}

	ctx, cancel := deriveRequestContext(parentCtx, r.timeout)
	r.buf = bytes.NewBuffer(nil)

	var (
		httpReq *http.Request
		err     error
	)
	if r.usesStreamingBody(mediaType) {
		httpReq, err = r.buildStreamingRequest(ctx, mediaType, basePath, producers, registry, auth)
	} else {
		httpReq, err = r.buildBufferedRequest(ctx, mediaType, basePath, producers, registry, auth)
	}
	if err != nil {
		cancel()
		return nil, noop, err
	}
	return httpReq, cancel, nil
}

func noop() {}

// deriveRequestContext returns a child of parent bounded by timeout.
// If timeout == 0 the child is only canceled when the caller invokes
// cancel; any deadline already on parent is preserved. If timeout > 0
// the child uses the shortest of timeout and parent's existing deadline.
func deriveRequestContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout == 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

// usesStreamingBody reports whether the request body must be assembled
// as a stream (an io.Pipe for multipart, or the payload's own reader
// for stream payloads).
//
// The complementary case is a fully buffered body in r.buf — urlencoded form, producer output, or no body at all.
func (r *Request) usesStreamingBody(mediaType string) bool {
	if (len(r.formFields) > 0 || len(r.fileFields) > 0) && r.isMultipart(mediaType) {
		return true
	}

	if r.payload != nil {
		if _, ok := r.payload.(io.Reader); ok {
			return true
		}
	}

	return false
}

func (r *Request) isMultipart(mediaType string) bool {
	// Strip media-type parameters before comparing: callers may legally
	// pass `multipart/form-data; boundary=…` or
	// `application/x-www-form-urlencoded; charset=utf-8` per RFC 7231,
	// and a bare-string compare would route those to the wrong flow.
	//
	// mime.ParseMediaType lowercases the type/subtype and is
	// case-insensitive on input, so plain == against our (lowercase)
	// constants is sufficient on the happy path.
	base, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		// Malformed mediaType: only the file-presence shortcut can
		// fire — by definition we cannot recognize either canonical
		// form mime in unparseable input.
		return len(r.fileFields) > 0
	}

	// An explicit application/x-www-form-urlencoded choice is honored even when
	// file fields are present: the spec allows files to travel as URL-encoded
	// form values, although it does not stream and is discouraged. Without this
	// short-circuit, picking urlencoded with files would silently fall back to
	// multipart and emit an inconsistent Content-Type.
	if base == runtime.URLencodedFormMime {
		return false
	}

	if len(r.fileFields) > 0 {
		return true
	}

	return base == runtime.MultipartFormMime
}

// buildBufferedRequest assembles a request whose body is fully
// buffered in r.buf before AuthenticateRequest runs — urlencoded form,
// producer-serialized payload, or no body.
//
// Auth is trivial in this flow because the buffer is already populated when the auth helper
// asks for the body via r.GetBody().
func (r *Request) buildBufferedRequest(ctx context.Context, mediaType, basePath string,
	producers map[string]runtime.Producer, registry strfmt.Registry, auth runtime.ClientAuthInfoWriter,
) (*http.Request, error) {
	var body io.Reader
	var err error

	switch {
	case len(r.formFields) > 0 || len(r.fileFields) > 0:
		body, err = r.writeURLEncodedBody(mediaType)
	case r.payload != nil:
		body, err = r.writeNonStreamPayload(mediaType, producers)
	}
	if err != nil {
		return nil, err
	}

	if runtime.CanHaveBody(r.method) && body != nil && r.header.Get(runtime.HeaderContentType) == "" {
		r.header.Set(runtime.HeaderContentType, mediaType)
	}

	if auth != nil {
		if err := auth.AuthenticateRequest(r, registry); err != nil {
			return nil, err
		}
	}

	return r.assembleRequest(ctx, basePath, body)
}

// buildStreamingRequest assembles a request whose body is a stream —
// either an io.Pipe filled by the multipart goroutine, or the
// payload's own io.Reader.
//
// AuthenticateRequest consumes the body lazily through the getBody closure installed by
// applyAuthWithBodyCopy, which buffers the stream into r.buf so the http.Request can use the buffered copy.
//
// On any error path before the http.Request takes ownership of body, we close the body to release
// the underlying resource.
//
// For multipart this unblocks the spawned writer goroutine
// (it would otherwise park forever on pw.Write with no reader).
//
// For stream payloads it closes the user-provided io.ReadCloser.
func (r *Request) buildStreamingRequest(ctx context.Context, mediaType, basePath string,
	producers map[string]runtime.Producer, registry strfmt.Registry, auth runtime.ClientAuthInfoWriter,
) (req *http.Request, retErr error) {
	var body io.Reader
	if len(r.formFields) > 0 || len(r.fileFields) > 0 {
		body = r.writeMultipartBody(ctx, mediaType)
	} else {
		body = r.writeStreamPayload(mediaType, producers)
	}

	defer func() {
		if retErr == nil {
			return
		}
		if c, ok := body.(io.Closer); ok {
			_ = c.Close()
		}
	}()

	if runtime.CanHaveBody(r.method) && body != nil && r.header.Get(runtime.HeaderContentType) == "" {
		r.header.Set(runtime.HeaderContentType, mediaType)
	}

	body, err := r.applyAuthWithBodyCopy(auth, body, registry)
	if err != nil {
		return nil, err
	}

	return r.assembleRequest(ctx, basePath, body)
}

// assembleRequest is the shared tail of both flows: build the URL
// path, create the http.Request, merge static query parameters, and
// finalize headers/query.
func (r *Request) assembleRequest(ctx context.Context, basePath string, body io.Reader) (*http.Request, error) {
	urlPath, staticQueryParams, err := r.resolveURLPath(basePath)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, r.method, urlPath, body)
	if err != nil {
		return nil, err
	}

	if err := r.mergeStaticQuery(staticQueryParams); err != nil {
		return nil, err
	}

	req.URL.RawQuery = r.query.Encode()
	req.Header = r.header

	return req, nil
}

// resolveURLPath builds the final url path string and returns the static
// query parameters extracted from basePath and r.pathPattern.
//
// Static query parameters from the path pattern take precedence over those
// from the base path; merging with r.query is the caller's responsibility
// (see [request.mergeStaticQuery]).
//
// The path is assembled from basePath + pathPattern with path-param
// substitution and trailing-slash preservation when the original
// pathPattern carried one.
func (r *Request) resolveURLPath(basePath string) (string, url.Values, error) {
	basePathURL, err := url.Parse(basePath)
	if err != nil {
		return "", nil, err
	}
	staticQueryParams := basePathURL.Query()

	pathPatternURL, err := url.Parse(r.pathPattern)
	if err != nil {
		return "", nil, err
	}
	for name, values := range pathPatternURL.Query() {
		if _, present := staticQueryParams[name]; present {
			staticQueryParams.Del(name)
		}
		for _, value := range values {
			staticQueryParams.Add(name, value)
		}
	}

	// path.Join strips trailing slashes; reinstate one whenever the
	// pathPattern carried it, including the bare-root case ("/" under a
	// non-empty basePath, which path.Join would collapse to "/basepath").
	// The HasSuffix check on urlPath keeps the rewrite idempotent and
	// avoids producing "//" when basePath is "/" or empty.
	reinstateSlash := strings.HasSuffix(pathPatternURL.Path, "/")

	urlPath := path.Join(basePathURL.Path, pathPatternURL.Path)
	for k, v := range r.pathParams {
		urlPath = strings.ReplaceAll(urlPath, "{"+k+"}", url.PathEscape(v))
	}
	if reinstateSlash && !strings.HasSuffix(urlPath, "/") {
		urlPath += "/"
	}

	return urlPath, staticQueryParams, nil
}

// applyAuthWithBodyCopy runs auth.AuthenticateRequest for the
// streaming flow, where the http.Request body is a pipe or a payload
// reader rather than r.buf. If AuthenticateRequest asks for the body
// via r.GetBody(), the lazy closure copies the stream into r.buf on
// demand and reassigns body to r.buf so the post-auth source passed
// to http.NewRequestWithContext is the buffered copy.
//
// The closure is registered lazily because there is no way to know
// ahead of time whether AuthenticateRequest will read the body.
//
// On error precedence: a copy error is reported in preference to the
// AuthenticateRequest error, because a mis-read body may have
// interfered with auth.
//
// No-op when auth is nil; returns body unchanged.
func (r *Request) applyAuthWithBodyCopy(auth runtime.ClientAuthInfoWriter, body io.Reader, registry strfmt.Registry) (io.Reader, error) {
	if auth == nil {
		return body, nil
	}

	var copyErr error
	var copied bool
	r.getBody = func(r *Request) []byte {
		if copied {
			return getRequestBuffer(r)
		}

		defer func() {
			copied = true
		}()

		if _, copyErr = io.Copy(r.buf, body); copyErr != nil {
			return nil
		}

		if closer, ok := body.(io.ReadCloser); ok {
			if copyErr = closer.Close(); copyErr != nil {
				return nil
			}
		}

		body = r.buf
		return getRequestBuffer(r)
	}

	authErr := auth.AuthenticateRequest(r, registry)

	// On error we return body alongside the error so the caller's
	// cleanup defer (in buildStreamingRequest) can close the
	// underlying pipe/stream. Caller treats body as ignorable when
	// err != nil per Go convention; the defer reads it via closure.
	if copyErr != nil {
		return body, fmt.Errorf("error copying the request body: %w", copyErr)
	}

	if authErr != nil {
		return body, authErr
	}

	return body, nil
}

// mergeStaticQuery overlays staticQuery onto r.query. On conflict r.query
// wins — the parameters set by the client take precedence over the ones
// extracted from basePath / pathPattern.
func (r *Request) mergeStaticQuery(staticQuery url.Values) error {
	originalParams := r.GetQueryParams()
	for k, v := range staticQuery {
		if _, present := originalParams[k]; present {
			continue
		}
		if err := r.SetQueryParam(k, v...); err != nil {
			return err
		}
	}
	return nil
}

// writeURLEncodedBody serializes form fields (and any file fields, per
// Swagger 2.0 fallback semantics) into r.buf as
// application/x-www-form-urlencoded. Sets Content-Type to mediaType and
// returns r.buf as the body source.
//
// Per Swagger 2.0, file form parameters can be sent under
// application/x-www-form-urlencoded by including the file content as a
// regular form-field value. The whole form is then percent-encoded as
// usual. This buffers the entire payload and does not preserve a
// per-file Content-Type — multipart/form-data is preferred when both
// are advertised by the operation.
func (r *Request) writeURLEncodedBody(mediaType string) (io.Reader, error) {
	r.header.Set(runtime.HeaderContentType, mediaType)
	values := url.Values{}
	for k, vs := range r.formFields {
		values[k] = append(values[k], vs...)
	}
	for fn, ff := range r.fileFields {
		for _, fi := range ff {
			data, ferr := io.ReadAll(fi)
			if cerr := fi.Close(); cerr != nil && ferr == nil {
				ferr = cerr
			}
			if ferr != nil {
				return nil, ferr
			}
			values.Add(fn, string(data))
		}
	}
	r.buf.WriteString(values.Encode())
	return r.buf, nil
}

// writeMultipartBody assembles a multipart/form-data body via an
// io.Pipe. A goroutine streams form fields and files into the pipe
// writer; the pipe reader is returned as the body. Sets Content-Type to
// the multipart media type with the writer's boundary parameter.
//
// The goroutine owns the pipe writer's lifecycle: it closes the
// multipart writer (flushing the closing boundary) and the pipe writer
// when it finishes or hits an error.
func (r *Request) writeMultipartBody(ctx context.Context, mediaType string) io.Reader {
	pr, pw := io.Pipe()
	mp := multipart.NewWriter(pw)
	r.header.Set(runtime.HeaderContentType, mangleContentType(mediaType, mp.Boundary()))

	go r.streamMultipartParts(ctx, mp, pw)

	return pr
}

// streamMultipartParts writes form fields then file fields to mp,
// closing mp and pw when done.
//
// Errors are reported by closing pw with the error so the consumer of pr observes them on its next Read.
//
// Context cancellation is observed at iteration boundaries (between
// fields and between files) and during file copy via a context-aware
// reader. When ctx is canceled the pipe writer is closed with ctx.Err()
// so the body consumer surfaces the cancellation as the read error.
func (r *Request) streamMultipartParts(ctx context.Context, mp *multipart.Writer, pw *io.PipeWriter) {
	defer func() {
		mp.Close()
		pw.Close()
	}()

	for fn, v := range r.formFields {
		for _, vi := range v {
			if err := ctx.Err(); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if err := mp.WriteField(fn, vi); err != nil {
				logClose(err, pw)
				return
			}
		}
	}

	defer func() {
		for _, ff := range r.fileFields {
			for _, ffi := range ff {
				ffi.Close()
			}
		}
	}()

	for fn, f := range r.fileFields {
		for _, fi := range f {
			if err := ctx.Err(); err != nil {
				_ = pw.CloseWithError(err)
				return
			}

			var fileContentType string
			if p, ok := fi.(runtime.ContentTyper); ok {
				fileContentType = p.ContentType()
			} else {
				// Need to read the data so that we can detect the content type
				const contentTypeBufferSize = 512
				buf := make([]byte, contentTypeBufferSize)
				size, err := fi.Read(buf)
				if err != nil && !errors.Is(err, io.EOF) {
					logClose(err, pw)
					return
				}
				fileContentType = http.DetectContentType(buf)
				fi = runtime.NamedReader(fi.Name(), io.MultiReader(bytes.NewReader(buf[:size]), fi))
			}

			// Create the MIME headers for the new part
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
					escapeQuotes(fn), escapeQuotes(filepath.Base(fi.Name()))))
			h.Set("Content-Type", fileContentType)

			wrtr, err := mp.CreatePart(h)
			if err != nil {
				logClose(err, pw)
				return
			}
			if _, err := io.Copy(wrtr, &ctxReader{ctx: ctx, r: fi}); err != nil {
				logClose(err, pw)
				return
			}
		}
	}
}

// ctxReader wraps an [io.Reader] with a context check on each Read. Once
// ctx is done, subsequent Reads return ctx.Err() instead of delegating
// to the underlying reader. It does not preempt a Read already in flight
// — that is the source's responsibility (e.g. *os.File honors Close from
// another goroutine, network sources honor SetDeadline).
type ctxReader struct {
	ctx context.Context //nolint:containedctx  // io.Reader's Read method has no ctx parameter, so the wrapper must carry it on the struct
	r   io.Reader
}

func (cr *ctxReader) Read(p []byte) (int, error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}

// writeStreamPayload handles a stream payload (io.Reader /
// io.ReadCloser). The bytes flow through verbatim — no producer is
// invoked. The wire Content-Type is resolved via setStreamContentType
// (priority: existing header, payload's ContentTyper,
// streamFallbackMime, mediaType).
//
// Caller must ensure r.payload satisfies io.Reader (see
// [request.usesStreamingBody]).
func (r *Request) writeStreamPayload(mediaType string, producers map[string]runtime.Producer) io.Reader {
	setStreamContentType(r.header, r.payload, mediaType, r.consumes, producers)
	if rdr, ok := r.payload.(io.ReadCloser); ok {
		return rdr
	}

	rdr, ok := r.payload.(io.Reader)
	if !ok {
		panic("internal error: payload expected to be an io.Reader") // guaranteed by earlier checks
	}

	return rdr
}

// writeNonStreamPayload runs the producer registered for mediaType
// against r.payload, writing into r.buf. The Content-Type header
// reflects the picker.
//
// SetHeaderParam("Content-Type", …) is intentionally NOT honored on
// the producer path because the producer is dispatched off mediaType —
// the wire header would otherwise misrepresent the body.
//
// The same reasoning applies to the form/multipart branch.
func (r *Request) writeNonStreamPayload(mediaType string, producers map[string]runtime.Producer) (io.Reader, error) {
	r.header.Set(runtime.HeaderContentType, mediaType)
	producer, ok := producers[mediaType]
	if !ok {
		return nil, fmt.Errorf("no producer registered for content type %q (register one with Runtime.Producers)", mediaType)
	}

	if err := producer.Produce(r.buf, r.payload); err != nil {
		return nil, err
	}
	return r.buf, nil
}

var quoter = strings.NewReplacer(
	"\\", "\\\\",
	`"`, "\\\"",
	"\r", "_",
	"\n", "_",
)

// escapeQuotes escapes backslash and double-quote for embedding in a
// quoted-string Content-Disposition parameter value, and rewrites
// CR / LF to '_' to prevent header-injection through attacker-influenced
// field names or filenames.
//
// RFC 7578 §4.2 limits parameter values to printable characters; this
// is the conservative subset relevant to security (control characters
// that would split the header line into a forged header or part).
// Mirrors the known stdlib gap golang/go#19038.
func escapeQuotes(s string) string {
	return quoter.Replace(s)
}

// setStreamContentType resolves and writes the wire Content-Type for a
// stream payload (io.Reader / io.ReadCloser). Priority:
//
//  1. an explicit value already in header — the user set it via
//     SetHeaderParam during [ClientRequestWriter.WriteToRequest], and we treat that as an
//     intentional escape hatch;
//  2. payload's [runtime.ContentTyper] declaration;
//  3. [streamFallbackMime] (Stage-2 octet-stream upgrade);
//  4. the picker's mediaType (passed in as the chain's terminal
//     fallback).
//
// Does not apply to non-stream payloads or to form/multipart bodies —
// see the comment above the call site in [request.buildHTTP].
func setStreamContentType(
	header http.Header,
	payload any,
	mediaType string,
	candidates []string,
	producers map[string]runtime.Producer,
) {
	if header.Get(runtime.HeaderContentType) != "" {
		return
	}
	fallback := streamFallbackMime(mediaType, candidates, producers)
	header.Set(runtime.HeaderContentType, payloadContentType(payload, fallback))
}

// payloadContentType returns the payload's declared content type when
// it implements [runtime.ContentTyper] with a non-empty result, and
// fallback otherwise. Mirrors the per-file convention already used for
// multipart upload parts (see [request.buildHTTP] file-fields branch).
func payloadContentType(payload any, fallback string) string {
	if t, ok := payload.(runtime.ContentTyper); ok {
		if ct := t.ContentType(); ct != "" {
			return ct
		}
	}

	return fallback
}

// streamFallbackMime selects a wire content-type for a stream payload
// (io.Reader / io.ReadCloser) that has neither implemented
// `ContentType() string` nor declared an explicit value.
//
// The picker (Stage 1) ran without seeing the payload, so its choice
// may be wildly wrong for raw bytes — e.g. picking application/json
// for a payload that is just a stream of opaque data. When the
// candidate consumes list also offers application/octet-stream and
// the runtime has an octet-stream producer registered, that's a
// safer wire type than the picker's choice: it advertises "raw bytes"
// rather than making a structural claim about the body.
//
// If octet-stream is unavailable in either the candidate list or the
// producer set, the picker's choice is preserved. The wire header
// then continues to misrepresent the body — but no correct
// alternative exists and we cannot infer one without more
// information from the caller.
func streamFallbackMime(picked string, candidates []string, producers map[string]runtime.Producer) string {
	if strings.EqualFold(picked, runtime.DefaultMime) {
		return picked
	}

	for _, c := range candidates {
		if strings.EqualFold(c, runtime.DefaultMime) {
			if _, ok := producers[runtime.DefaultMime]; ok {
				return runtime.DefaultMime
			}
		}
	}

	return picked
}

func getRequestBuffer(r *Request) []byte {
	if r.buf == nil {
		return nil
	}
	return r.buf.Bytes()
}

func logClose(err error, pw *io.PipeWriter) {
	log.Println(err)
	closeErr := pw.CloseWithError(err)
	if closeErr != nil {
		log.Println(closeErr)
	}
}

func mangleContentType(mediaType, boundary string) string {
	_ = mediaType // reserved for future enhancement: honor caller-provided media type
	// Proposal for enhancement: honor caller's boundary if specified
	return "multipart/form-data; boundary=" + boundary
}
