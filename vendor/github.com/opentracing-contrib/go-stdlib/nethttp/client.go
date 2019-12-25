// +build go1.7

package nethttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptrace"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
)

type contextKey int

const (
	keyTracer contextKey = iota
)

const defaultComponentName = "net/http"

// Transport wraps a RoundTripper. If a request is being traced with
// Tracer, Transport will inject the current span into the headers,
// and set HTTP related tags on the span.
type Transport struct {
	// The actual RoundTripper to use for the request. A nil
	// RoundTripper defaults to http.DefaultTransport.
	http.RoundTripper
}

type clientOptions struct {
	operationName      string
	componentName      string
	disableClientTrace bool
}

// ClientOption contols the behavior of TraceRequest.
type ClientOption func(*clientOptions)

// OperationName returns a ClientOption that sets the operation
// name for the client-side span.
func OperationName(operationName string) ClientOption {
	return func(options *clientOptions) {
		options.operationName = operationName
	}
}

// ComponentName returns a ClientOption that sets the component
// name for the client-side span.
func ComponentName(componentName string) ClientOption {
	return func(options *clientOptions) {
		options.componentName = componentName
	}
}

// ClientTrace returns a ClientOption that turns on or off
// extra instrumentation via httptrace.WithClientTrace.
func ClientTrace(enabled bool) ClientOption {
	return func(options *clientOptions) {
		options.disableClientTrace = !enabled
	}
}

// TraceRequest adds a ClientTracer to req, tracing the request and
// all requests caused due to redirects. When tracing requests this
// way you must also use Transport.
//
// Example:
//
// 	func AskGoogle(ctx context.Context) error {
// 		client := &http.Client{Transport: &nethttp.Transport{}}
// 		req, err := http.NewRequest("GET", "http://google.com", nil)
// 		if err != nil {
// 			return err
// 		}
// 		req = req.WithContext(ctx) // extend existing trace, if any
//
// 		req, ht := nethttp.TraceRequest(tracer, req)
// 		defer ht.Finish()
//
// 		res, err := client.Do(req)
// 		if err != nil {
// 			return err
// 		}
// 		res.Body.Close()
// 		return nil
// 	}
func TraceRequest(tr opentracing.Tracer, req *http.Request, options ...ClientOption) (*http.Request, *Tracer) {
	opts := &clientOptions{}
	for _, opt := range options {
		opt(opts)
	}
	ht := &Tracer{tr: tr, opts: opts}
	ctx := req.Context()
	if !opts.disableClientTrace {
		ctx = httptrace.WithClientTrace(ctx, ht.clientTrace())
	}
	req = req.WithContext(context.WithValue(ctx, keyTracer, ht))
	return req, ht
}

type closeTracker struct {
	io.ReadCloser
	sp opentracing.Span
}

func (c closeTracker) Close() error {
	err := c.ReadCloser.Close()
	c.sp.LogFields(log.String("event", "ClosedBody"))
	c.sp.Finish()
	return err
}

// RoundTrip implements the RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := t.RoundTripper
	if rt == nil {
		rt = http.DefaultTransport
	}
	tracer, ok := req.Context().Value(keyTracer).(*Tracer)
	if !ok {
		return rt.RoundTrip(req)
	}

	tracer.start(req)

	ext.HTTPMethod.Set(tracer.sp, req.Method)
	ext.HTTPUrl.Set(tracer.sp, req.URL.String())

	carrier := opentracing.HTTPHeadersCarrier(req.Header)
	tracer.sp.Tracer().Inject(tracer.sp.Context(), opentracing.HTTPHeaders, carrier)
	resp, err := rt.RoundTrip(req)

	if err != nil {
		tracer.sp.Finish()
		return resp, err
	}
	ext.HTTPStatusCode.Set(tracer.sp, uint16(resp.StatusCode))
	if req.Method == "HEAD" {
		tracer.sp.Finish()
	} else {
		resp.Body = closeTracker{resp.Body, tracer.sp}
	}
	return resp, nil
}

// Tracer holds tracing details for one HTTP request.
type Tracer struct {
	tr   opentracing.Tracer
	root opentracing.Span
	sp   opentracing.Span
	opts *clientOptions
}

func (h *Tracer) start(req *http.Request) opentracing.Span {
	if h.root == nil {
		parent := opentracing.SpanFromContext(req.Context())
		var spanctx opentracing.SpanContext
		if parent != nil {
			spanctx = parent.Context()
		}
		operationName := h.opts.operationName
		if operationName == "" {
			operationName = "HTTP Client"
		}
		root := h.tr.StartSpan(operationName, opentracing.ChildOf(spanctx))
		h.root = root
	}

	ctx := h.root.Context()
	h.sp = h.tr.StartSpan("HTTP "+req.Method, opentracing.ChildOf(ctx))
	ext.SpanKindRPCClient.Set(h.sp)

	componentName := h.opts.componentName
	if componentName == "" {
		componentName = defaultComponentName
	}
	ext.Component.Set(h.sp, componentName)

	return h.sp
}

// Finish finishes the span of the traced request.
func (h *Tracer) Finish() {
	if h.root != nil {
		h.root.Finish()
	}
}

// Span returns the root span of the traced request. This function
// should only be called after the request has been executed.
func (h *Tracer) Span() opentracing.Span {
	return h.root
}

func (h *Tracer) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn:              h.getConn,
		GotConn:              h.gotConn,
		PutIdleConn:          h.putIdleConn,
		GotFirstResponseByte: h.gotFirstResponseByte,
		Got100Continue:       h.got100Continue,
		DNSStart:             h.dnsStart,
		DNSDone:              h.dnsDone,
		ConnectStart:         h.connectStart,
		ConnectDone:          h.connectDone,
		WroteHeaders:         h.wroteHeaders,
		Wait100Continue:      h.wait100Continue,
		WroteRequest:         h.wroteRequest,
	}
}

func (h *Tracer) getConn(hostPort string) {
	ext.HTTPUrl.Set(h.sp, hostPort)
	h.sp.LogFields(log.String("event", "GetConn"))
}

func (h *Tracer) gotConn(info httptrace.GotConnInfo) {
	h.sp.SetTag("net/http.reused", info.Reused)
	h.sp.SetTag("net/http.was_idle", info.WasIdle)
	h.sp.LogFields(log.String("event", "GotConn"))
}

func (h *Tracer) putIdleConn(error) {
	h.sp.LogFields(log.String("event", "PutIdleConn"))
}

func (h *Tracer) gotFirstResponseByte() {
	h.sp.LogFields(log.String("event", "GotFirstResponseByte"))
}

func (h *Tracer) got100Continue() {
	h.sp.LogFields(log.String("event", "Got100Continue"))
}

func (h *Tracer) dnsStart(info httptrace.DNSStartInfo) {
	h.sp.LogFields(
		log.String("event", "DNSStart"),
		log.String("host", info.Host),
	)
}

func (h *Tracer) dnsDone(info httptrace.DNSDoneInfo) {
	fields := []log.Field{log.String("event", "DNSDone")}
	for _, addr := range info.Addrs {
		fields = append(fields, log.String("addr", addr.String()))
	}
	if info.Err != nil {
		fields = append(fields, log.Error(info.Err))
	}
	h.sp.LogFields(fields...)
}

func (h *Tracer) connectStart(network, addr string) {
	h.sp.LogFields(
		log.String("event", "ConnectStart"),
		log.String("network", network),
		log.String("addr", addr),
	)
}

func (h *Tracer) connectDone(network, addr string, err error) {
	if err != nil {
		h.sp.LogFields(
			log.String("message", "ConnectDone"),
			log.String("network", network),
			log.String("addr", addr),
			log.String("event", "error"),
			log.Error(err),
		)
	} else {
		h.sp.LogFields(
			log.String("event", "ConnectDone"),
			log.String("network", network),
			log.String("addr", addr),
		)
	}
}

func (h *Tracer) wroteHeaders() {
	h.sp.LogFields(log.String("event", "WroteHeaders"))
}

func (h *Tracer) wait100Continue() {
	h.sp.LogFields(log.String("event", "Wait100Continue"))
}

func (h *Tracer) wroteRequest(info httptrace.WroteRequestInfo) {
	if info.Err != nil {
		h.sp.LogFields(
			log.String("message", "WroteRequest"),
			log.String("event", "error"),
			log.Error(info.Err),
		)
		ext.Error.Set(h.sp, true)
	} else {
		h.sp.LogFields(log.String("event", "WroteRequest"))
	}
}
