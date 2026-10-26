// Copyright 2026, Google Inc.
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package gax

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/googleapis/gax-go/v2/callctx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TransportTelemetryData contains mutable telemetry information that the transport
// layer (e.g. gRPC or HTTP) populates during an RPC. This allows gax.Invoke to
// correctly emit metric data without directly importing those transport layers.
// TransportTelemetryData is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
// It should not be used by external consumers.
type TransportTelemetryData struct {
	serverAddress  string
	serverPort     int
	httpStatusCode int
}

// SetServerAddress sets the server address.
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func (d *TransportTelemetryData) SetServerAddress(addr string) { d.serverAddress = addr }

// ServerAddress returns the server address.
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func (d *TransportTelemetryData) ServerAddress() string { return d.serverAddress }

// SetServerPort sets the server port.
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func (d *TransportTelemetryData) SetServerPort(port int) { d.serverPort = port }

// ServerPort returns the server port.
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func (d *TransportTelemetryData) ServerPort() int { return d.serverPort }

// SetHTTPStatusCode sets the HTTP status code.
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func (d *TransportTelemetryData) SetHTTPStatusCode(code int) { d.httpStatusCode = code }

// HTTPStatusCode returns the HTTP status code.
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func (d *TransportTelemetryData) HTTPStatusCode() int { return d.httpStatusCode }

// transportTelemetryKey is the private context key used to inject TransportTelemetryData
type transportTelemetryKey struct{}

// InjectTransportTelemetry injects a mutable TransportTelemetryData pointer into the context.
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func InjectTransportTelemetry(ctx context.Context, data *TransportTelemetryData) context.Context {
	return context.WithValue(ctx, transportTelemetryKey{}, data)
}

// ExtractTransportTelemetry retrieves a mutable TransportTelemetryData pointer from the context.
// It returns nil if the data is not present.
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func ExtractTransportTelemetry(ctx context.Context) *TransportTelemetryData {
	data, _ := ctx.Value(transportTelemetryKey{}).(*TransportTelemetryData)
	return data
}

const (
	metricName        = "gcp.client.request.duration"
	metricDescription = "Duration of the request to the Google Cloud API"

	// Constants for ClientMetrics configuration map keys.
	// These are used by generated clients to pass attributes to the ClientMetrics option.
	// Because they are used in generated code, these values must not be changed.

	// ClientService is the Google Cloud API service name. E.g. "storage".
	ClientService = "client_service"
	// ClientVersion is the version of the client. E.g. "1.43.0".
	ClientVersion = "client_version"
	// ClientArtifact is the library name. E.g. "cloud.google.com/go/storage".
	ClientArtifact = "client_artifact"
	// ClientRepo is the source repository for the client library. E.g. "googleapis/google-cloud-go".
	ClientRepo = "client_repo"
	// RPCSystem is the RPC system type. E.g. "grpc" or "http".
	RPCSystem = "rpc_system"
	// URLDomain is the nominal service domain. E.g. "storage.googleapis.com".
	URLDomain = "url_domain"

	// Constants for telemetry attribute keys.
	keyGCPClientArtifact = "gcp.client.artifact"
	keyGCPClientRepo     = "gcp.client.repo"
	keyGCPClientService  = "gcp.client.service"
	keyGCPClientVersion  = "gcp.client.version"
	keyRPCSystemName     = "rpc.system.name"
	keyURLDomain         = "url.domain"

	// SchemaURL specifies the OpenTelemetry schema version.
	schemaURL = "https://opentelemetry.io/schemas/1.39.0"
)

// Default bucket boundaries for the duration metric in seconds.
// An exponential-ish distribution.
var defaultHistogramBoundaries = []float64{
	0.0, 0.0001, 0.0005, 0.0010, 0.005, 0.010, 0.050, 0.100, 0.5, 1.0, 5.0, 10.0, 60.0, 300.0, 900.0, 3600.0,
}

// ClientMetrics contains the pre-allocated OpenTelemetry instruments and attributes
// for a specific generated Google Cloud client library.
// There should be exactly one ClientMetrics instance instantiated per generated client.
type ClientMetrics struct {
	get func() clientMetricsData
}

type clientMetricsData struct {
	duration metric.Float64Histogram
	attr     []attribute.KeyValue
}

type telemetryOptions struct {
	provider                 metric.MeterProvider
	attributes               map[string]string
	explicitBucketBoundaries []float64
	logger                   *slog.Logger
}

// TelemetryOption is an option to configure a ClientMetrics instance.
// TelemetryOption works by modifying relevant fields of telemetryOptions.
type TelemetryOption interface {
	// Resolve applies the option by modifying opts.
	Resolve(opts *telemetryOptions)
}

type providerOpt struct {
	p metric.MeterProvider
}

func (p providerOpt) Resolve(opts *telemetryOptions) {
	opts.provider = p.p
}

// WithMeterProvider specifies the metric.MeterProvider to use for instruments.
func WithMeterProvider(p metric.MeterProvider) TelemetryOption {
	return &providerOpt{p: p}
}

type attrOpt struct {
	attrs map[string]string
}

func (a attrOpt) Resolve(opts *telemetryOptions) {
	opts.attributes = a.attrs
}

func (a attrOpt) ResolveTracing(opts *tracingOptions) {
	if opts != nil {
		opts.attributes = a.attrs
	}
}

func (a attrOpt) ResolveLogging(opts *loggingOptions) {
	if opts == nil || len(a.attrs) == 0 {
		return
	}
	if opts.attributes == nil {
		opts.attributes = make(map[string]string, len(a.attrs))
	}
	for k, v := range a.attrs {
		opts.attributes[k] = v
	}
}

// WithTelemetryAttributes specifies the static attributes attachments.
func WithTelemetryAttributes(attr map[string]string) TelemetryOption {
	return &attrOpt{attrs: attr}
}

// WithTracingAttributes specifies the static attributes attachments for tracing.
func WithTracingAttributes(attr map[string]string) TracingOption {
	return &attrOpt{attrs: attr}
}

// WithLoggingAttributes specifies the static attributes attachments for logging.
func WithLoggingAttributes(attr map[string]string) LoggingOption {
	return &attrOpt{attrs: attr}
}

type boundariesOpt struct {
	boundaries []float64
}

func (b boundariesOpt) Resolve(opts *telemetryOptions) {
	opts.explicitBucketBoundaries = b.boundaries
}

// WithExplicitBucketBoundaries overrides the default histogram bucket boundaries.
func WithExplicitBucketBoundaries(boundaries []float64) TelemetryOption {
	return &boundariesOpt{boundaries: boundaries}
}

type loggerOpt struct {
	l *slog.Logger
}

func (l loggerOpt) Resolve(opts *telemetryOptions) {
	opts.logger = l.l
}

// WithTelemetryLogger specifies a logger to record internal telemetry errors.
func WithTelemetryLogger(l *slog.Logger) TelemetryOption {
	return &loggerOpt{l: l}
}

func (config *telemetryOptions) meterProvider() metric.MeterProvider {
	if config.provider != nil {
		return config.provider
	}
	return otel.GetMeterProvider()
}

func (config *telemetryOptions) bucketBoundaries() []float64 {
	if len(config.explicitBucketBoundaries) > 0 {
		return config.explicitBucketBoundaries
	}
	return defaultHistogramBoundaries
}

// NewClientMetrics initializes and returns a new ClientMetrics instance.
// It is intended to be called once per generated client during initialization.
func NewClientMetrics(opts ...TelemetryOption) *ClientMetrics {
	var config telemetryOptions
	for _, opt := range opts {
		opt.Resolve(&config)
	}

	return &ClientMetrics{
		get: sync.OnceValue(func() clientMetricsData {
			provider := config.meterProvider()

			var meterAttrs []attribute.KeyValue
			if val, ok := config.attributes[ClientService]; ok {
				meterAttrs = append(meterAttrs, attribute.KeyValue{Key: attribute.Key(keyGCPClientService), Value: attribute.StringValue(val)})
			}

			meterOpts := []metric.MeterOption{
				metric.WithInstrumentationVersion(config.attributes[ClientVersion]),
				metric.WithSchemaURL(schemaURL),
			}
			if len(meterAttrs) > 0 {
				meterOpts = append(meterOpts, metric.WithInstrumentationAttributes(meterAttrs...))
			}

			meter := provider.Meter(config.attributes[ClientArtifact], meterOpts...)

			boundaries := config.bucketBoundaries()

			duration, err := meter.Float64Histogram(
				metricName,
				metric.WithDescription(metricDescription),
				metric.WithUnit("s"),
				metric.WithExplicitBucketBoundaries(boundaries...),
			)
			if err != nil && config.logger != nil {
				config.logger.Warn("failed to initialize OTel duration histogram", "error", err)
			}

			var attr []attribute.KeyValue
			if val, ok := config.attributes[URLDomain]; ok {
				attr = append(attr, attribute.KeyValue{Key: attribute.Key(keyURLDomain), Value: attribute.StringValue(val)})
			}
			if val, ok := config.attributes[RPCSystem]; ok {
				attr = append(attr, attribute.KeyValue{Key: attribute.Key(keyRPCSystemName), Value: attribute.StringValue(val)})
			}
			return clientMetricsData{
				duration: duration,
				attr:     attr,
			}
		}),
	}
}

func (cm *ClientMetrics) durationHistogram() metric.Float64Histogram {
	if cm == nil || cm.get == nil {
		return nil
	}
	return cm.get().duration
}

func (cm *ClientMetrics) attributes() []attribute.KeyValue {
	if cm == nil || cm.get == nil {
		return nil
	}
	return cm.get().attr
}

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

// grpcCodeToStatusString converts a codes.Code to its string representation.
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func grpcCodeToStatusString(c codes.Code) string {
	if int(c) >= 0 && int(c) < len(codeToStr) {
		return codeToStr[c]
	}
	return "UNKNOWN"
}

// TelemetryErrorInfo contains the mapped error type and status code, as well as
// additional details like status message, domain, and metadata, extracted from an error
// for telemetry purposes.
type TelemetryErrorInfo struct {
	// ErrorType is a mapped string for the error type.
	// For stability, this maps client-side cancellations, timeouts, and known gRPC
	// status codes to standard string literals (e.g., "CLIENT_TIMEOUT",
	// "PERMISSION_DENIED"), and falls back to %T for unhandled types. If an
	// apierror.APIError is found, it uses its fine-grained Reason() (e.g.,
	// "SERVICE_DISABLED").
	// This is used by metrics, tracing, and logging.
	ErrorType string
	// StatusCode is the string representation of the RPC status code.
	// This is used by metrics, tracing, and logging.
	StatusCode string
	// StatusMessage is the raw message from the error.
	// This is used for structured logging.
	StatusMessage string
	// Domain is the domain of the error, extracted from an ErrorInfo, if available.
	// This is used for structured logging.
	Domain string
	// Metadata is the metadata of the error, extracted from an ErrorInfo, if available.
	// This is used for structured logging.
	Metadata map[string]string

	// _ struct{} prevents unkeyed struct literals, ensuring backwards
	// compatibility when new fields are added in the future.
	_ struct{}
}

// ExtractTelemetryErrorInfo parses an error into a TelemetryErrorInfo struct.
// It relies on standard gRPC status codes, apierror.APIError parsing, and
// context inspection to determine the most accurate error classification and
// provide detailed metadata for telemetry systems.
//
// Experimental: This function is experimental and may be modified or removed in future versions,
// regardless of any other documented package stability guarantees.
func ExtractTelemetryErrorInfo(ctx context.Context, err error) TelemetryErrorInfo {
	if err == nil {
		return TelemetryErrorInfo{ErrorType: "", StatusCode: "OK"}
	}

	st, ok := status.FromError(err)
	if !ok {
		st = status.FromContextError(err)
	}
	rpcStatusCode := grpcCodeToStatusString(st.Code())

	var errType string
	// 1. Check if the local context expired or was cancelled. This is the only
	// reliable way to distinguish a local client timeout from a server timeout
	// because gRPC does not wrap context errors in its status.Error types.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		errType = "CLIENT_TIMEOUT"
	} else if errors.Is(ctx.Err(), context.Canceled) {
		errType = "CLIENT_CANCELLED"
	} else if !ok || st.Code() == codes.Unknown || st.Code() == codes.Internal {
		// 2. If the error isn't a context breakdown and the gRPC framework
		// doesn't "understand" it (returning ok=false or a generic catch-all
		// bucket like Unknown/Internal), we "pack" the actual Go error type
		// name into error.type (e.g., "*net.OpError"). This is per the error.type
		// [spec](https://opentelemetry.io/docs/specs/semconv/registry/attributes/error/#error-type).
		// "When error.type is set to a type (e.g., an exception type), its canonical
		// class name identifying the type within the artifact SHOULD be used."
		errType = fmt.Sprintf("%T", err)
	} else {
		// 3. Otherwise, it is a well-understood gRPC protocol error (e.g.,
		// PERMISSION_DENIED) likely returned by the server.
		errType = rpcStatusCode
	}

	var msg, domain string
	var metadata map[string]string
	if ok {
		msg = st.Message()
	} else {
		msg = err.Error()
	}

	if parsedErr, parsedOk := apierror.ParseError(err, false); parsedOk {
		// If there's an actionable error, the reason takes precedence over our calculated error type.
		if reason := parsedErr.Reason(); reason != "" {
			errType = reason
		} else if httpCode := parsedErr.HTTPCode(); httpCode > 0 {
			errType = strconv.Itoa(httpCode)
		}
		if message := parsedErr.Message(); message != "" {
			msg = message
		} else if parsedErr.HTTPCode() > 0 {
			// For HTTP errors, avoid returning the raw, unformatted err.Error() (e.g. "googleapi: got HTTP response...")
			// if the actual parsed message from the response is empty.
			msg = ""
		}
		domain = parsedErr.Domain()
		metadata = parsedErr.Metadata()
	}

	return TelemetryErrorInfo{
		ErrorType:     errType,
		StatusCode:    rpcStatusCode,
		StatusMessage: msg,
		Domain:        domain,
		Metadata:      metadata,
	}
}

// recordMetric records a duration measurement for the configured metric.
func recordMetric(ctx context.Context, settings CallSettings, d time.Duration, err error) {
	errInfo := ExtractTelemetryErrorInfo(ctx, err)
	recordMetricWithInfo(ctx, settings, d, &errInfo)
}

// recordMetricWithInfo records a duration measurement using pre-extracted error info.
func recordMetricWithInfo(ctx context.Context, settings CallSettings, d time.Duration, errInfo *TelemetryErrorInfo) {
	if settings.clientMetrics == nil || settings.clientMetrics.durationHistogram() == nil {
		return
	}

	// Use context.WithoutCancel to ensure metric records even if context is canceled
	// preserving any trace context that might be required for exemplars.
	recordCtx := context.WithoutCancel(ctx)

	// Pre-allocate to avoid repeated appends (5 is the max number of dynamic attributes added here)
	attrs := make([]attribute.KeyValue, 0, len(settings.clientMetrics.attributes())+5)
	attrs = append(attrs, settings.clientMetrics.attributes()...)

	if td := ExtractTransportTelemetry(ctx); td != nil {
		if td.ServerAddress() != "" {
			attrs = append(attrs, attribute.String("server.address", td.ServerAddress()))
		}
		if td.ServerPort() != 0 {
			attrs = append(attrs, attribute.Int("server.port", td.ServerPort()))
		}
		if td.HTTPStatusCode() != 0 {
			attrs = append(attrs, attribute.Int("http.response.status_code", td.HTTPStatusCode()))
		}
	}

	if errInfo.ErrorType != "" {
		attrs = append(attrs, attribute.String("error.type", errInfo.ErrorType))
	}

	attrs = append(attrs, attribute.String("rpc.response.status_code", errInfo.StatusCode))

	if rpcMethod, ok := callctx.TelemetryFromContext(ctx, "rpc_method"); ok && rpcMethod != "" {
		attrs = append(attrs, attribute.String("rpc.method", rpcMethod))
	}
	if urlTemplate, ok := callctx.TelemetryFromContext(ctx, "url_template"); ok && urlTemplate != "" {
		attrs = append(attrs, attribute.String("url.template", urlTemplate))
	}

	settings.clientMetrics.durationHistogram().Record(recordCtx, d.Seconds(), metric.WithAttributes(attrs...))
}

// startSpan starts a client request span if ClientTracing and its tracer are configured.
func startSpan(ctx context.Context, ct *ClientTracing) (context.Context, trace.Span) {
	if ct == nil {
		return ctx, nil
	}
	tracer := ct.tracer()
	if tracer == nil {
		return ctx, nil
	}
	spanName := resolveSpanName(ctx)
	staticAttrs := ct.attributes()
	attrs := make([]attribute.KeyValue, 0, len(staticAttrs)+1)
	attrs = append(attrs, staticAttrs...)
	if urlTemplate, ok := callctx.TelemetryFromContext(ctx, "url_template"); ok && urlTemplate != "" {
		if sanitized := sanitizeURLTemplate(urlTemplate); sanitized != "" {
			attrs = append(attrs, attribute.String("url.template", sanitized))
		}
	}
	return tracer.Start(
		ctx,
		spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
}

// endSpan records terminal status, diagnostic error details, and ends the client span.
func endSpan(ctx context.Context, span trace.Span, errInfo *TelemetryErrorInfo, err error) {
	if span == nil {
		return
	}
	defer span.End()

	if errInfo == nil {
		info := ExtractTelemetryErrorInfo(ctx, err)
		errInfo = &info
	}

	attrs := make([]attribute.KeyValue, 0, 3+len(errInfo.Metadata))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, errInfo.StatusCode)
		attrs = append(attrs,
			attribute.String("error.type", errInfo.ErrorType),
			attribute.String("rpc.response.status_code", errInfo.StatusCode),
		)
		if errInfo.Domain != "" {
			attrs = append(attrs, attribute.String("gcp.errors.domain", errInfo.Domain))
		}
		for k, v := range errInfo.Metadata {
			attrs = append(attrs, attribute.String("gcp.errors.metadata."+k, v))
		}
	} else {
		span.SetStatus(otelcodes.Ok, "")
		attrs = append(attrs, attribute.String("rpc.response.status_code", "OK"))
	}

	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
}

// ClientTracing contains the pre-allocated OpenTelemetry tracer and attributes
// for a specific generated Google Cloud client library.
// There should be exactly one ClientTracing instance instantiated per generated client.
type ClientTracing struct {
	get func() clientTracingData
}

type clientTracingData struct {
	tracer trace.Tracer
	attr   []attribute.KeyValue
}

type tracingOptions struct {
	provider   trace.TracerProvider
	attributes map[string]string
}

// TracingOption is an option to configure a ClientTracing instance.
// TracingOption works by modifying relevant fields of tracingOptions.
type TracingOption interface {
	// ResolveTracing applies the option by modifying opts.
	ResolveTracing(opts *tracingOptions)
}

type tracerProviderOpt struct {
	p trace.TracerProvider
}

func (p tracerProviderOpt) ResolveTracing(opts *tracingOptions) {
	if opts != nil {
		opts.provider = p.p
	}
}

// WithTracerProvider specifies the trace.TracerProvider to use for client request tracing.
func WithTracerProvider(p trace.TracerProvider) TracingOption {
	return &tracerProviderOpt{p: p}
}

func (config *tracingOptions) tracerProvider() trace.TracerProvider {
	if config != nil && config.provider != nil {
		return config.provider
	}
	// Fall back to global tracer provider to prevent silent no-op bug!
	return otel.GetTracerProvider()
}

// NewClientTracing initializes and returns a new ClientTracing instance.
// It is intended to be called once per generated client during initialization.
func NewClientTracing(opts ...TracingOption) *ClientTracing {
	var config tracingOptions
	for _, opt := range opts {
		if opt != nil {
			opt.ResolveTracing(&config)
		}
	}

	return &ClientTracing{
		get: sync.OnceValue(func() clientTracingData {
			provider := config.tracerProvider()

			var tracerOpts []trace.TracerOption
			if val, ok := config.attributes[ClientVersion]; ok {
				tracerOpts = append(tracerOpts, trace.WithInstrumentationVersion(val))
			}
			tracerOpts = append(tracerOpts, trace.WithSchemaURL(schemaURL))

			tracer := provider.Tracer(config.attributes[ClientArtifact], tracerOpts...)

			var attr []attribute.KeyValue
			for _, m := range [...]struct{ attrKey, otelKey string }{
				{URLDomain, keyURLDomain},
				{RPCSystem, keyRPCSystemName},
				{ClientService, keyGCPClientService},
			} {
				if val, ok := config.attributes[m.attrKey]; ok {
					attr = append(attr, attribute.String(m.otelKey, val))
				}
			}
			attr = attr[:len(attr):len(attr)]

			return clientTracingData{
				tracer: tracer,
				attr:   attr,
			}
		}),
	}
}

func (ct *ClientTracing) tracer() trace.Tracer {
	if ct == nil || ct.get == nil {
		return nil
	}
	return ct.get().tracer
}

func (ct *ClientTracing) attributes() []attribute.KeyValue {
	if ct == nil || ct.get == nil {
		return nil
	}
	return ct.get().attr
}

// formatServiceMethod formats a protobuf RPC name (such as
// "google.cloud.secretmanager.v1.SecretManagerService/GetSecret")
// into a span name formatted as "Service.Method" (such as "SecretManager.GetSecret").
// If the input does not contain a slash separator, it returns the input unchanged.
func formatServiceMethod(rpcMethod string) string {
	slashIdx := strings.LastIndex(rpcMethod, "/")
	if slashIdx == -1 {
		return rpcMethod
	}

	service := rpcMethod[:slashIdx]
	if dotIdx := strings.LastIndex(service, "."); dotIdx != -1 {
		service = service[dotIdx+1:]
	}
	if trimmed := strings.TrimSuffix(service, "Service"); trimmed != "" {
		service = trimmed
	}

	method := rpcMethod[slashIdx+1:]
	if service == "" || method == "" {
		return rpcMethod
	}

	return service + "." + method
}

func resolveSpanName(ctx context.Context) string {
	if clientMethod, ok := callctx.TelemetryFromContext(ctx, "client_method"); ok && clientMethod != "" {
		return clientMethod
	}
	if rpcMethod, ok := callctx.TelemetryFromContext(ctx, "rpc_method"); ok && rpcMethod != "" {
		return formatServiceMethod(rpcMethod)
	}
	httpMethod, okHTTP := callctx.TelemetryFromContext(ctx, "http_method")
	urlTemplate, okURL := callctx.TelemetryFromContext(ctx, "url_template")
	if okHTTP && httpMethod != "" && okURL && urlTemplate != "" {
		if sanitized := sanitizeURLTemplate(urlTemplate); sanitized != "" {
			return httpMethod + " " + sanitized
		}
	}
	return "gcp.client.request"
}

// sanitizeURLTemplate removes query parameters and URL fragments from a raw URL template,
// ensuring only the path template is retained.
func sanitizeURLTemplate(rawURL string) string {
	if idx := strings.IndexAny(rawURL, "?#"); idx != -1 {
		return rawURL[:idx]
	}
	return rawURL
}

// ClientLogging contains the pre-allocated slog.Logger and static attributes
// for a specific generated Google Cloud client library.
// There should be exactly one ClientLogging instance instantiated per generated client.
type ClientLogging struct {
	get func() clientLoggingData
}

type clientLoggingData struct {
	logger *slog.Logger
	attr   []slog.Attr
}

type loggingOptions struct {
	logger     *slog.Logger
	attributes map[string]string
}

// LoggingOption is an option to configure a ClientLogging instance.
// LoggingOption works by modifying relevant fields of loggingOptions.
type LoggingOption interface {
	// ResolveLogging applies the option by modifying opts.
	ResolveLogging(opts *loggingOptions)
}

type loggerProviderOpt struct {
	logger *slog.Logger
}

func (p loggerProviderOpt) ResolveLogging(opts *loggingOptions) {
	if opts != nil {
		opts.logger = p.logger
	}
}

// WithLoggerProvider specifies the slog.Logger to use for client request logging.
func WithLoggerProvider(logger *slog.Logger) LoggingOption {
	return &loggerProviderOpt{logger: logger}
}

func (config *loggingOptions) loggerProvider() *slog.Logger {
	if config != nil && config.logger != nil {
		return config.logger
	}
	// Fall back to default slog logger to prevent silent no-op bug!
	return slog.Default()
}

// NewClientLogging initializes and returns a new ClientLogging instance.
// It is intended to be called once per generated client during initialization.
func NewClientLogging(opts ...LoggingOption) *ClientLogging {
	var config loggingOptions
	for _, opt := range opts {
		if opt != nil {
			opt.ResolveLogging(&config)
		}
	}

	var attr []slog.Attr
	for _, m := range [...]struct{ attrKey, slogKey string }{
		{ClientArtifact, keyGCPClientArtifact},
		{ClientRepo, keyGCPClientRepo},
		{ClientService, keyGCPClientService},
		{ClientVersion, keyGCPClientVersion},
		{RPCSystem, keyRPCSystemName},
		{URLDomain, keyURLDomain},
	} {
		if val, ok := config.attributes[m.attrKey]; ok {
			attr = append(attr, slog.String(m.slogKey, val))
		}
	}
	attr = attr[:len(attr):len(attr)]
	customLogger := config.logger

	return &ClientLogging{
		get: sync.OnceValue(func() clientLoggingData {
			logger := customLogger
			if logger == nil {
				logger = slog.Default()
			}

			return clientLoggingData{
				logger: logger,
				attr:   attr,
			}
		}),
	}
}

func (cl *ClientLogging) logger() *slog.Logger {
	if cl == nil || cl.get == nil {
		return nil
	}
	return cl.get().logger
}

func (cl *ClientLogging) attributes() []slog.Attr {
	if cl == nil || cl.get == nil {
		return nil
	}
	return cl.get().attr
}

// recordActionableLog emits a structured warning log strictly upon terminal failure.
func recordActionableLog(ctx context.Context, cl *ClientLogging, errInfo *TelemetryErrorInfo, retries int, err error) {
	if cl == nil || err == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	logger := cl.logger()
	if logger == nil || !logger.Enabled(ctx, slog.LevelWarn) {
		return
	}

	if errInfo == nil {
		info := ExtractTelemetryErrorInfo(ctx, err)
		errInfo = &info
	}

	attrs := append([]slog.Attr(nil), cl.attributes()...)

	if rpcMethod, ok := callctx.TelemetryFromContext(ctx, "rpc_method"); ok && rpcMethod != "" {
		attrs = append(attrs, slog.String("rpc.method", rpcMethod))
	}
	if urlTemplate, ok := callctx.TelemetryFromContext(ctx, "url_template"); ok && urlTemplate != "" {
		if sanitized := sanitizeURLTemplate(urlTemplate); sanitized != "" {
			attrs = append(attrs, slog.String("url.template", sanitized))
		}
	}
	if td := ExtractTransportTelemetry(ctx); td != nil {
		if td.ServerAddress() != "" {
			attrs = append(attrs, slog.String("server.address", td.ServerAddress()))
		}
		if td.ServerPort() != 0 {
			attrs = append(attrs, slog.Int("server.port", td.ServerPort()))
		}
	}
	if errInfo.ErrorType != "" {
		attrs = append(attrs, slog.String("error.type", errInfo.ErrorType))
	}
	if errInfo.StatusCode != "" {
		attrs = append(attrs, slog.String("rpc.response.status_code", errInfo.StatusCode))
	}
	attrs = append(attrs,
		slog.String("error.message", err.Error()),
		slog.Int("resend_count", retries),
	)
	if errInfo.Domain != "" {
		attrs = append(attrs, slog.String("gcp.errors.domain", errInfo.Domain))
	}
	for k, v := range errInfo.Metadata {
		attrs = append(attrs, slog.String("gcp.errors.metadata."+k, v))
	}

	logger.LogAttrs(ctx, slog.LevelWarn, "gcp.client.request", attrs...)
}
