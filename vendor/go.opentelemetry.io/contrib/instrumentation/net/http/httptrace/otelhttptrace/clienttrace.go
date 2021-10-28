// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otelhttptrace

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
	"net/textproto"
	"strings"
	"sync"

	"go.opentelemetry.io/contrib"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTP attributes.
var (
	HTTPStatus                 = attribute.Key("http.status")
	HTTPHeaderMIME             = attribute.Key("http.mime")
	HTTPRemoteAddr             = attribute.Key("http.remote")
	HTTPLocalAddr              = attribute.Key("http.local")
	HTTPConnectionReused       = attribute.Key("http.conn.reused")
	HTTPConnectionWasIdle      = attribute.Key("http.conn.wasidle")
	HTTPConnectionIdleTime     = attribute.Key("http.conn.idletime")
	HTTPConnectionStartNetwork = attribute.Key("http.conn.start.network")
	HTTPConnectionDoneNetwork  = attribute.Key("http.conn.done.network")
	HTTPConnectionDoneAddr     = attribute.Key("http.conn.done.addr")
	HTTPDNSAddrs               = attribute.Key("http.dns.addrs")
)

var (
	hookMap = map[string]string{
		"http.dns":     "http.getconn",
		"http.connect": "http.getconn",
		"http.tls":     "http.getconn",
	}
)

func parentHook(hook string) string {
	if strings.HasPrefix(hook, "http.connect") {
		return hookMap["http.connect"]
	}
	return hookMap[hook]
}

// ClientTraceOption allows customizations to how the httptrace.Client
// collects information.
type ClientTraceOption func(*clientTracer)

// WithoutSubSpans will modify the httptrace.Client to only collect data
// as Events and Attributes on a span found in the context.  By default
// sub-spans will be generated.
func WithoutSubSpans() ClientTraceOption {
	return func(ct *clientTracer) {
		ct.useSpans = false
	}
}

// WithRedactedHeaders will be replaced by fixed '****' values for the header
// names provided.  These are in addition to the sensitive headers already
// redacted by default: Authorization, WWW-Authenticate, Proxy-Authenticate
// Proxy-Authorization, Cookie, Set-Cookie
func WithRedactedHeaders(headers ...string) ClientTraceOption {
	return func(ct *clientTracer) {
		for _, header := range headers {
			ct.redactedHeaders[strings.ToLower(header)] = struct{}{}
		}
	}
}

// WithoutHeaders will disable adding span annotations for the http headers
// and values.
func WithoutHeaders() ClientTraceOption {
	return func(ct *clientTracer) {
		ct.addHeaders = false
	}
}

type clientTracer struct {
	context.Context

	tr trace.Tracer

	activeHooks     map[string]context.Context
	root            trace.Span
	mtx             sync.Mutex
	redactedHeaders map[string]struct{}
	addHeaders      bool
	useSpans        bool
}

func NewClientTrace(ctx context.Context, opts ...ClientTraceOption) *httptrace.ClientTrace {
	ct := &clientTracer{
		Context:     ctx,
		activeHooks: make(map[string]context.Context),
		redactedHeaders: map[string]struct{}{
			"authorization":       {},
			"www-authenticate":    {},
			"proxy-authenticate":  {},
			"proxy-authorization": {},
			"cookie":              {},
			"set-cookie":          {},
		},
		addHeaders: true,
		useSpans:   true,
	}
	for _, opt := range opts {
		opt(ct)
	}

	var tp trace.TracerProvider
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		tp = span.TracerProvider()
	} else {
		tp = otel.GetTracerProvider()
	}

	ct.tr = tp.Tracer(
		"go.opentelemetry.io/otel/instrumentation/httptrace",
		trace.WithInstrumentationVersion(contrib.SemVersion()),
	)

	return &httptrace.ClientTrace{
		GetConn:              ct.getConn,
		GotConn:              ct.gotConn,
		PutIdleConn:          ct.putIdleConn,
		GotFirstResponseByte: ct.gotFirstResponseByte,
		Got100Continue:       ct.got100Continue,
		Got1xxResponse:       ct.got1xxResponse,
		DNSStart:             ct.dnsStart,
		DNSDone:              ct.dnsDone,
		ConnectStart:         ct.connectStart,
		ConnectDone:          ct.connectDone,
		TLSHandshakeStart:    ct.tlsHandshakeStart,
		TLSHandshakeDone:     ct.tlsHandshakeDone,
		WroteHeaderField:     ct.wroteHeaderField,
		WroteHeaders:         ct.wroteHeaders,
		Wait100Continue:      ct.wait100Continue,
		WroteRequest:         ct.wroteRequest,
	}
}

func (ct *clientTracer) start(hook, spanName string, attrs ...attribute.KeyValue) {
	if !ct.useSpans {
		if ct.root == nil {
			ct.root = trace.SpanFromContext(ct.Context)
		}
		ct.root.AddEvent(hook+".start", trace.WithAttributes(attrs...))
		return
	}

	ct.mtx.Lock()
	defer ct.mtx.Unlock()

	if hookCtx, found := ct.activeHooks[hook]; !found {
		var sp trace.Span
		ct.activeHooks[hook], sp = ct.tr.Start(ct.getParentContext(hook), spanName, trace.WithAttributes(attrs...), trace.WithSpanKind(trace.SpanKindClient))
		if ct.root == nil {
			ct.root = sp
		}
	} else {
		// end was called before start finished, add the start attributes and end the span here
		span := trace.SpanFromContext(hookCtx)
		span.SetAttributes(attrs...)
		span.End()

		delete(ct.activeHooks, hook)
	}
}

func (ct *clientTracer) end(hook string, err error, attrs ...attribute.KeyValue) {
	if !ct.useSpans {
		if err != nil {
			attrs = append(attrs, attribute.String(hook+".error", err.Error()))
		}
		ct.root.AddEvent(hook+".done", trace.WithAttributes(attrs...))
		return
	}

	ct.mtx.Lock()
	defer ct.mtx.Unlock()
	if ctx, ok := ct.activeHooks[hook]; ok {
		span := trace.SpanFromContext(ctx)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
		}
		span.SetAttributes(attrs...)
		span.End()
		delete(ct.activeHooks, hook)
	} else {
		// start is not finished before end is called.
		// Start a span here with the ending attributes that will be finished when start finishes.
		// Yes, it's backwards. v0v
		ctx, span := ct.tr.Start(ct.getParentContext(hook), hook, trace.WithAttributes(attrs...), trace.WithSpanKind(trace.SpanKindClient))
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
		}
		ct.activeHooks[hook] = ctx
	}
}

func (ct *clientTracer) getParentContext(hook string) context.Context {
	ctx, ok := ct.activeHooks[parentHook(hook)]
	if !ok {
		return ct.Context
	}
	return ctx
}

func (ct *clientTracer) span(hook string) trace.Span {
	ct.mtx.Lock()
	defer ct.mtx.Unlock()
	if ctx, ok := ct.activeHooks[hook]; ok {
		return trace.SpanFromContext(ctx)
	}
	return nil
}

func (ct *clientTracer) getConn(host string) {
	ct.start("http.getconn", "http.getconn", semconv.HTTPHostKey.String(host))
}

func (ct *clientTracer) gotConn(info httptrace.GotConnInfo) {
	attrs := []attribute.KeyValue{
		HTTPRemoteAddr.String(info.Conn.RemoteAddr().String()),
		HTTPLocalAddr.String(info.Conn.LocalAddr().String()),
		HTTPConnectionReused.Bool(info.Reused),
		HTTPConnectionWasIdle.Bool(info.WasIdle),
	}
	if info.WasIdle {
		attrs = append(attrs, HTTPConnectionIdleTime.String(info.IdleTime.String()))
	}
	ct.end("http.getconn", nil, attrs...)
}

func (ct *clientTracer) putIdleConn(err error) {
	ct.end("http.receive", err)
}

func (ct *clientTracer) gotFirstResponseByte() {
	ct.start("http.receive", "http.receive")
}

func (ct *clientTracer) dnsStart(info httptrace.DNSStartInfo) {
	ct.start("http.dns", "http.dns", semconv.HTTPHostKey.String(info.Host))
}

func (ct *clientTracer) dnsDone(info httptrace.DNSDoneInfo) {
	var addrs []string
	for _, netAddr := range info.Addrs {
		addrs = append(addrs, netAddr.String())
	}
	ct.end("http.dns", info.Err, HTTPDNSAddrs.String(sliceToString(addrs)))
}

func (ct *clientTracer) connectStart(network, addr string) {
	ct.start("http.connect."+addr, "http.connect",
		HTTPRemoteAddr.String(addr),
		HTTPConnectionStartNetwork.String(network),
	)
}

func (ct *clientTracer) connectDone(network, addr string, err error) {
	ct.end("http.connect."+addr, err,
		HTTPConnectionDoneAddr.String(addr),
		HTTPConnectionDoneNetwork.String(network),
	)
}

func (ct *clientTracer) tlsHandshakeStart() {
	ct.start("http.tls", "http.tls")
}

func (ct *clientTracer) tlsHandshakeDone(_ tls.ConnectionState, err error) {
	ct.end("http.tls", err)
}

func (ct *clientTracer) wroteHeaderField(k string, v []string) {
	if ct.useSpans && ct.span("http.headers") == nil {
		ct.start("http.headers", "http.headers")
	}
	if !ct.addHeaders {
		return
	}
	k = strings.ToLower(k)
	value := sliceToString(v)
	if _, ok := ct.redactedHeaders[k]; ok {
		value = "****"
	}
	ct.root.SetAttributes(attribute.String("http."+k, value))
}

func (ct *clientTracer) wroteHeaders() {
	if ct.useSpans && ct.span("http.headers") != nil {
		ct.end("http.headers", nil)
	}
	ct.start("http.send", "http.send")
}

func (ct *clientTracer) wroteRequest(info httptrace.WroteRequestInfo) {
	if info.Err != nil {
		ct.root.SetStatus(codes.Error, info.Err.Error())
	}
	ct.end("http.send", info.Err)
}

func (ct *clientTracer) got100Continue() {
	span := ct.root
	if ct.useSpans {
		span = ct.span("http.receive")
	}
	span.AddEvent("GOT 100 - Continue")
}

func (ct *clientTracer) wait100Continue() {
	span := ct.root
	if ct.useSpans {
		span = ct.span("http.receive")
	}
	span.AddEvent("GOT 100 - Wait")
}

func (ct *clientTracer) got1xxResponse(code int, header textproto.MIMEHeader) error {
	span := ct.root
	if ct.useSpans {
		span = ct.span("http.receive")
	}
	span.AddEvent("GOT 1xx", trace.WithAttributes(
		HTTPStatus.Int(code),
		HTTPHeaderMIME.String(sm2s(header)),
	))
	return nil
}

func sliceToString(value []string) string {
	if len(value) == 0 {
		return "undefined"
	}
	return strings.Join(value, ",")
}

func sm2s(value map[string][]string) string {
	var buf strings.Builder
	for k, v := range value {
		if buf.Len() != 0 {
			buf.WriteString(",")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(sliceToString(v))
	}
	return buf.String()
}
