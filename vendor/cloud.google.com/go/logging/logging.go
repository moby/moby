// Copyright 2016 Google LLC
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

// API/gRPC features intentionally missing from this client:
// - You cannot have the server pick the time of the entry. This client
//   always sends a time.
// - There is no way to provide a protocol buffer payload.
// - No support for the "partial success" feature when writing log entries.

// TODO(jba): test whether forward-slash characters in the log ID must be URL-encoded.
// These features are missing now, but will likely be added:
// - There is no way to specify CallOptions.

package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	vkit "cloud.google.com/go/logging/apiv2"
	"cloud.google.com/go/logging/internal"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/api/option"
	"google.golang.org/api/support/bundler"
	mrpb "google.golang.org/genproto/googleapis/api/monitoredres"
	logtypepb "google.golang.org/genproto/googleapis/logging/type"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// ReadScope is the scope for reading from the logging service.
	ReadScope = "https://www.googleapis.com/auth/logging.read"

	// WriteScope is the scope for writing to the logging service.
	WriteScope = "https://www.googleapis.com/auth/logging.write"

	// AdminScope is the scope for administrative actions on the logging service.
	AdminScope = "https://www.googleapis.com/auth/logging.admin"
)

const (
	// defaultErrorCapacity is the capacity of the channel used to deliver
	// errors to the OnError function.
	defaultErrorCapacity = 10

	// DefaultDelayThreshold is the default value for the DelayThreshold LoggerOption.
	DefaultDelayThreshold = time.Second

	// DefaultEntryCountThreshold is the default value for the EntryCountThreshold LoggerOption.
	DefaultEntryCountThreshold = 1000

	// DefaultEntryByteThreshold is the default value for the EntryByteThreshold LoggerOption.
	DefaultEntryByteThreshold = 1 << 23 // 8MiB

	// DefaultBufferedByteLimit is the default value for the BufferedByteLimit LoggerOption.
	DefaultBufferedByteLimit = 1 << 30 // 1GiB

	// defaultWriteTimeout is the timeout for the underlying write API calls. As
	// write API calls are not idempotent, they are not retried on timeout. This
	// timeout is to allow clients to degrade gracefully if underlying logging
	// service is temporarily impaired for some reason.
	defaultWriteTimeout = 10 * time.Minute
)

var (
	// ErrRedirectProtoPayloadNotSupported is returned when Logger is configured to redirect output and
	// tries to redirect logs with protobuf payload.
	ErrRedirectProtoPayloadNotSupported = errors.New("printEntryToStdout: cannot find valid payload")

	// For testing:
	now = time.Now

	// ErrOverflow signals that the number of buffered entries for a Logger
	// exceeds its BufferLimit.
	ErrOverflow = bundler.ErrOverflow

	// ErrOversizedEntry signals that an entry's size exceeds the maximum number of
	// bytes that will be sent in a single call to the logging service.
	ErrOversizedEntry = bundler.ErrOversizedItem
)

// Client is a Logging client. A Client is associated with a single Cloud project.
type Client struct {
	client  *vkit.Client   // client for the logging service
	parent  string         // e.g. "projects/proj-id"
	errc    chan error     // should be buffered to minimize dropped errors
	donec   chan struct{}  // closed on Client.Close to close Logger bundlers
	loggers sync.WaitGroup // so we can wait for loggers to close
	closed  bool

	mu      sync.Mutex
	nErrs   int   // number of errors we saw
	lastErr error // last error we saw

	// OnError is called when an error occurs in a call to Log or Flush. The
	// error may be due to an invalid Entry, an overflow because BufferLimit
	// was reached (in which case the error will be ErrOverflow) or an error
	// communicating with the logging service. OnError is called with errors
	// from all Loggers. It is never called concurrently. OnError is expected
	// to return quickly; if errors occur while OnError is running, some may
	// not be reported. The default behavior is to call log.Printf.
	//
	// This field should be set only once, before any method of Client is called.
	OnError func(err error)
}

// NewClient returns a new logging client associated with the provided parent.
// A parent can take any of the following forms:
//
//	projects/PROJECT_ID
//	folders/FOLDER_ID
//	billingAccounts/ACCOUNT_ID
//	organizations/ORG_ID
//
// for backwards compatibility, a string with no '/' is also allowed and is interpreted
// as a project ID.
//
// By default NewClient uses WriteScope. To use a different scope, call
// NewClient using a WithScopes option (see https://godoc.org/google.golang.org/api/option#WithScopes).
func NewClient(ctx context.Context, parent string, opts ...option.ClientOption) (*Client, error) {
	parent, err := makeParent(parent)
	if err != nil {
		return nil, err
	}
	opts = append([]option.ClientOption{
		option.WithScopes(WriteScope),
	}, opts...)
	c, err := vkit.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	c.SetGoogleClientInfo("gccl", internal.Version)
	client := &Client{
		client:  c,
		parent:  parent,
		errc:    make(chan error, defaultErrorCapacity), // create a small buffer for errors
		donec:   make(chan struct{}),
		OnError: func(e error) { log.Printf("logging client: %v", e) },
	}
	// Call the user's function synchronously, to make life easier for them.
	go func() {
		for err := range client.errc {
			// This reference to OnError is memory-safe if the user sets OnError before
			// calling any client methods. The reference happens before the first read from
			// client.errc, which happens before the first write to client.errc, which
			// happens before any call, which happens before the user sets OnError.
			if fn := client.OnError; fn != nil {
				fn(err)
			} else {
				log.Printf("logging (parent %q): %v", parent, err)
			}
		}
	}()
	return client, nil
}

func makeParent(parent string) (string, error) {
	if !strings.ContainsRune(parent, '/') {
		return "projects/" + parent, nil
	}
	prefix := strings.Split(parent, "/")[0]
	if prefix != "projects" && prefix != "folders" && prefix != "billingAccounts" && prefix != "organizations" {
		return parent, fmt.Errorf("parent parameter must start with 'projects/' 'folders/' 'billingAccounts/' or 'organizations/'")
	}
	return parent, nil
}

// Ping reports whether the client's connection to the logging service and the
// authentication configuration are valid. To accomplish this, Ping writes a
// log entry "ping" to a log named "ping".
func (c *Client) Ping(ctx context.Context) error {
	unixZeroTimestamp, err := ptypes.TimestampProto(time.Unix(0, 0))
	if err != nil {
		return err
	}
	ent := &logpb.LogEntry{
		Payload:   &logpb.LogEntry_TextPayload{TextPayload: "ping"},
		Timestamp: unixZeroTimestamp, // Identical timestamps and insert IDs are both
		InsertId:  "ping",            // necessary for the service to dedup these entries.
	}
	_, err = c.client.WriteLogEntries(ctx, &logpb.WriteLogEntriesRequest{
		LogName:  internal.LogPath(c.parent, "ping"),
		Resource: monitoredResource(c.parent),
		Entries:  []*logpb.LogEntry{ent},
	})
	return err
}

// error puts the error on the client's error channel
// without blocking, and records summary error info.
func (c *Client) error(err error) {
	select {
	case c.errc <- err:
	default:
	}
	c.mu.Lock()
	c.lastErr = err
	c.nErrs++
	c.mu.Unlock()
}

func (c *Client) extractErrorInfo() error {
	var err error
	c.mu.Lock()
	if c.lastErr != nil {
		err = fmt.Errorf("saw %d errors; last: %w", c.nErrs, c.lastErr)
		c.nErrs = 0
		c.lastErr = nil
	}
	c.mu.Unlock()
	return err
}

// A Logger is used to write log messages to a single log. It can be configured
// with a log ID, common monitored resource, and a set of common labels.
type Logger struct {
	client     *Client
	logName    string // "projects/{projectID}/logs/{logID}"
	stdLoggers map[Severity]*log.Logger
	bundler    *bundler.Bundler

	// Options
	commonResource         *mrpb.MonitoredResource
	commonLabels           map[string]string
	ctxFunc                func() (context.Context, func())
	populateSourceLocation int
	partialSuccess         bool
	redirectOutputWriter   io.Writer
}

// Logger returns a Logger that will write entries with the given log ID, such as
// "syslog". A log ID must be less than 512 characters long and can only
// include the following characters: upper and lower case alphanumeric
// characters: [A-Za-z0-9]; and punctuation characters: forward-slash,
// underscore, hyphen, and period.
func (c *Client) Logger(logID string, opts ...LoggerOption) *Logger {
	r := detectResource()
	if r == nil {
		r = monitoredResource(c.parent)
	}
	l := &Logger{
		client:                 c,
		logName:                internal.LogPath(c.parent, logID),
		commonResource:         r,
		ctxFunc:                func() (context.Context, func()) { return context.Background(), nil },
		populateSourceLocation: DoNotPopulateSourceLocation,
		partialSuccess:         false,
		redirectOutputWriter:   nil,
	}
	l.bundler = bundler.NewBundler(&logpb.LogEntry{}, func(entries interface{}) {
		l.writeLogEntries(entries.([]*logpb.LogEntry))
	})
	l.bundler.DelayThreshold = DefaultDelayThreshold
	l.bundler.BundleCountThreshold = DefaultEntryCountThreshold
	l.bundler.BundleByteThreshold = DefaultEntryByteThreshold
	l.bundler.BufferedByteLimit = DefaultBufferedByteLimit
	for _, opt := range opts {
		opt.set(l)
	}
	l.stdLoggers = map[Severity]*log.Logger{}
	for s := range severityName {
		l.stdLoggers[s] = log.New(severityWriter{l, s}, "", 0)
	}

	c.loggers.Add(1)
	// Start a goroutine that cleans up the bundler, its channel
	// and the writer goroutines when the client is closed.
	go func() {
		defer c.loggers.Done()
		<-c.donec
		l.bundler.Flush()
	}()
	return l
}

type severityWriter struct {
	l *Logger
	s Severity
}

func (w severityWriter) Write(p []byte) (n int, err error) {
	w.l.Log(Entry{
		Severity: w.s,
		Payload:  string(p),
	})
	return len(p), nil
}

// Close waits for all opened loggers to be flushed and closes the client.
func (c *Client) Close() error {
	if c.closed {
		return nil
	}
	close(c.donec)   // close Logger bundlers
	c.loggers.Wait() // wait for all bundlers to flush and close
	// Now there can be no more errors.
	close(c.errc) // terminate error goroutine
	// Prefer errors arising from logging to the error returned from Close.
	err := c.extractErrorInfo()
	err2 := c.client.Close()
	if err == nil {
		err = err2
	}
	c.closed = true
	return err
}

// Severity is the severity of the event described in a log entry. These
// guideline severity levels are ordered, with numerically smaller levels
// treated as less severe than numerically larger levels.
type Severity int

const (
	// Default means the log entry has no assigned severity level.
	Default = Severity(logtypepb.LogSeverity_DEFAULT)
	// Debug means debug or trace information.
	Debug = Severity(logtypepb.LogSeverity_DEBUG)
	// Info means routine information, such as ongoing status or performance.
	Info = Severity(logtypepb.LogSeverity_INFO)
	// Notice means normal but significant events, such as start up, shut down, or configuration.
	Notice = Severity(logtypepb.LogSeverity_NOTICE)
	// Warning means events that might cause problems.
	Warning = Severity(logtypepb.LogSeverity_WARNING)
	// Error means events that are likely to cause problems.
	Error = Severity(logtypepb.LogSeverity_ERROR)
	// Critical means events that cause more severe problems or brief outages.
	Critical = Severity(logtypepb.LogSeverity_CRITICAL)
	// Alert means a person must take an action immediately.
	Alert = Severity(logtypepb.LogSeverity_ALERT)
	// Emergency means one or more systems are unusable.
	Emergency = Severity(logtypepb.LogSeverity_EMERGENCY)
)

var severityName = map[Severity]string{
	Default:   "Default",
	Debug:     "Debug",
	Info:      "Info",
	Notice:    "Notice",
	Warning:   "Warning",
	Error:     "Error",
	Critical:  "Critical",
	Alert:     "Alert",
	Emergency: "Emergency",
}

// String converts a severity level to a string.
func (v Severity) String() string {
	// same as proto.EnumName
	s, ok := severityName[v]
	if ok {
		return s
	}
	return strconv.Itoa(int(v))
}

// UnmarshalJSON turns a string representation of severity into the type
// Severity.
func (v *Severity) UnmarshalJSON(data []byte) error {
	var s string
	var i int
	if strErr := json.Unmarshal(data, &s); strErr == nil {
		*v = ParseSeverity(s)
	} else if intErr := json.Unmarshal(data, &i); intErr == nil {
		*v = Severity(i)
	} else {
		return fmt.Errorf("%v; %v", strErr, intErr)
	}
	return nil
}

// ParseSeverity returns the Severity whose name equals s, ignoring case. It
// returns Default if no Severity matches.
func ParseSeverity(s string) Severity {
	sl := strings.ToLower(s)
	for sev, name := range severityName {
		if strings.ToLower(name) == sl {
			return sev
		}
	}
	return Default
}

// Entry is a log entry.
// See https://cloud.google.com/logging/docs/view/logs_index for more about entries.
type Entry struct {
	// Timestamp is the time of the entry. If zero, the current time is used.
	Timestamp time.Time

	// Severity is the entry's severity level.
	// The zero value is Default.
	Severity Severity

	// Payload must be either a string, or something that marshals via the
	// encoding/json package to a JSON object (and not any other type of JSON value).
	Payload interface{}

	// Labels optionally specifies key/value labels for the log entry.
	// The Logger.Log method takes ownership of this map. See Logger.CommonLabels
	// for more about labels.
	Labels map[string]string

	// InsertID is a unique ID for the log entry. If you provide this field,
	// the logging service considers other log entries in the same log with the
	// same ID as duplicates which can be removed. If omitted, the logging
	// service will generate a unique ID for this log entry. Note that because
	// this client retries RPCs automatically, it is possible (though unlikely)
	// that an Entry without an InsertID will be written more than once.
	InsertID string

	// HTTPRequest optionally specifies metadata about the HTTP request
	// associated with this log entry, if applicable. It is optional.
	HTTPRequest *HTTPRequest

	// Operation optionally provides information about an operation associated
	// with the log entry, if applicable.
	Operation *logpb.LogEntryOperation

	// LogName is the full log name, in the form
	// "projects/{ProjectID}/logs/{LogID}". It is set by the client when
	// reading entries. It is an error to set it when writing entries.
	LogName string

	// Resource is the monitored resource associated with the entry.
	Resource *mrpb.MonitoredResource

	// Trace is the resource name of the trace associated with the log entry,
	// if any. If it contains a relative resource name, the name is assumed to
	// be relative to //tracing.googleapis.com.
	Trace string

	// ID of the span within the trace associated with the log entry.
	// The ID is a 16-character hexadecimal encoding of an 8-byte array.
	SpanID string

	// If set, symbolizes that this request was sampled.
	TraceSampled bool

	// Optional. Source code location information associated with the log entry,
	// if any.
	SourceLocation *logpb.LogEntrySourceLocation
}

// HTTPRequest contains an http.Request as well as additional
// information about the request and its response.
type HTTPRequest struct {
	// Request is the http.Request passed to the handler.
	Request *http.Request

	// RequestSize is the size of the HTTP request message in bytes, including
	// the request headers and the request body.
	RequestSize int64

	// Status is the response code indicating the status of the response.
	// Examples: 200, 404.
	Status int

	// ResponseSize is the size of the HTTP response message sent back to the client, in bytes,
	// including the response headers and the response body.
	ResponseSize int64

	// Latency is the request processing latency on the server, from the time the request was
	// received until the response was sent.
	Latency time.Duration

	// LocalIP is the IP address (IPv4 or IPv6) of the origin server that the request
	// was sent to.
	LocalIP string

	// RemoteIP is the IP address (IPv4 or IPv6) of the client that issued the
	// HTTP request. Examples: "192.168.1.1", "FE80::0202:B3FF:FE1E:8329".
	RemoteIP string

	// CacheHit reports whether an entity was served from cache (with or without
	// validation).
	CacheHit bool

	// CacheValidatedWithOriginServer reports whether the response was
	// validated with the origin server before being served from cache. This
	// field is only meaningful if CacheHit is true.
	CacheValidatedWithOriginServer bool

	// CacheFillBytes is the number of HTTP response bytes inserted into cache. Set only when a cache fill was attempted.
	CacheFillBytes int64

	// CacheLookup tells whether or not a cache lookup was attempted.
	CacheLookup bool
}

func fromHTTPRequest(r *HTTPRequest) (*logtypepb.HttpRequest, error) {
	if r == nil {
		return nil, nil
	}
	if r.Request == nil {
		return nil, errors.New("logging: HTTPRequest must have a non-nil Request")
	}
	u := *r.Request.URL
	u.Fragment = ""
	pb := &logtypepb.HttpRequest{
		RequestMethod:                  r.Request.Method,
		RequestUrl:                     fixUTF8(u.String()),
		RequestSize:                    r.RequestSize,
		Status:                         int32(r.Status),
		ResponseSize:                   r.ResponseSize,
		UserAgent:                      r.Request.UserAgent(),
		ServerIp:                       r.LocalIP,
		RemoteIp:                       r.RemoteIP, // TODO(jba): attempt to parse http.Request.RemoteAddr?
		Referer:                        r.Request.Referer(),
		CacheHit:                       r.CacheHit,
		CacheValidatedWithOriginServer: r.CacheValidatedWithOriginServer,
		Protocol:                       r.Request.Proto,
		CacheFillBytes:                 r.CacheFillBytes,
		CacheLookup:                    r.CacheLookup,
	}
	if r.Latency != 0 {
		pb.Latency = ptypes.DurationProto(r.Latency)
	}
	return pb, nil
}

// fixUTF8 is a helper that fixes an invalid UTF-8 string by replacing
// invalid UTF-8 runes with the Unicode replacement character (U+FFFD).
// See Issue https://github.com/googleapis/google-cloud-go/issues/1383.
func fixUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	// Otherwise time to build the sequence.
	buf := new(bytes.Buffer)
	buf.Grow(len(s))
	for _, r := range s {
		if utf8.ValidRune(r) {
			buf.WriteRune(r)
		} else {
			buf.WriteRune('\uFFFD')
		}
	}
	return buf.String()
}

// toProtoStruct converts v, which must marshal into a JSON object,
// into a Google Struct proto.
func toProtoStruct(v interface{}) (*structpb.Struct, error) {
	// Fast path: if v is already a *structpb.Struct, nothing to do.
	if s, ok := v.(*structpb.Struct); ok {
		return s, nil
	}
	// v is a Go value that supports JSON marshalling. We want a Struct
	// protobuf. Some day we may have a more direct way to get there, but right
	// now the only way is to marshal the Go value to JSON, unmarshal into a
	// map, and then build the Struct proto from the map.
	var jb []byte
	var err error
	if raw, ok := v.(json.RawMessage); ok { // needed for Go 1.7 and below
		jb = []byte(raw)
	} else {
		jb, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("logging: json.Marshal: %w", err)
		}
	}
	var m map[string]interface{}
	err = json.Unmarshal(jb, &m)
	if err != nil {
		return nil, fmt.Errorf("logging: json.Unmarshal: %w", err)
	}
	return jsonMapToProtoStruct(m), nil
}

func jsonMapToProtoStruct(m map[string]interface{}) *structpb.Struct {
	fields := map[string]*structpb.Value{}
	for k, v := range m {
		fields[k] = jsonValueToStructValue(v)
	}
	return &structpb.Struct{Fields: fields}
}

func jsonValueToStructValue(v interface{}) *structpb.Value {
	switch x := v.(type) {
	case bool:
		return &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: x}}
	case float64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: x}}
	case string:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: x}}
	case nil:
		return &structpb.Value{Kind: &structpb.Value_NullValue{}}
	case map[string]interface{}:
		return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: jsonMapToProtoStruct(x)}}
	case []interface{}:
		var vals []*structpb.Value
		for _, e := range x {
			vals = append(vals, jsonValueToStructValue(e))
		}
		return &structpb.Value{Kind: &structpb.Value_ListValue{ListValue: &structpb.ListValue{Values: vals}}}
	default:
		return &structpb.Value{Kind: &structpb.Value_NullValue{}}
	}
}

// LogSync logs the Entry synchronously without any buffering. Because LogSync is slow
// and will block, it is intended primarily for debugging or critical errors.
// Prefer Log for most uses.
func (l *Logger) LogSync(ctx context.Context, e Entry) error {
	ent, err := toLogEntryInternal(e, l, l.client.parent, 1)
	if err != nil {
		return err
	}
	entries, hasInstrumentation := l.instrumentLogs([]*logpb.LogEntry{ent})
	if l.redirectOutputWriter != nil {
		for _, ent = range entries {
			err = serializeEntryToWriter(ent, l.redirectOutputWriter)
			if err != nil {
				break
			}
		}
		return err
	}
	_, err = l.client.client.WriteLogEntries(ctx, &logpb.WriteLogEntriesRequest{
		LogName:        l.logName,
		Resource:       l.commonResource,
		Labels:         l.commonLabels,
		Entries:        entries,
		PartialSuccess: l.partialSuccess || hasInstrumentation,
	})
	return err
}

// Log buffers the Entry for output to the logging service. It never blocks.
func (l *Logger) Log(e Entry) {
	ent, err := toLogEntryInternal(e, l, l.client.parent, 1)
	if err != nil {
		l.client.error(err)
		return
	}

	entries, _ := l.instrumentLogs([]*logpb.LogEntry{ent})
	if l.redirectOutputWriter != nil {
		for _, ent = range entries {
			err = serializeEntryToWriter(ent, l.redirectOutputWriter)
			if err != nil {
				l.client.error(err)
			}
		}
		return
	}
	for _, ent = range entries {
		if err := l.bundler.Add(ent, proto.Size(ent)); err != nil {
			l.client.error(err)
		}
	}
}

// Flush blocks until all currently buffered log entries are sent.
//
// If any errors occurred since the last call to Flush from any Logger, or the
// creation of the client if this is the first call, then Flush returns a non-nil
// error with summary information about the errors. This information is unlikely to
// be actionable. For more accurate error reporting, set Client.OnError.
func (l *Logger) Flush() error {
	l.bundler.Flush()
	return l.client.extractErrorInfo()
}

func (l *Logger) writeLogEntries(entries []*logpb.LogEntry) {
	partialSuccess := l.partialSuccess
	if len(entries) > 1 {
		partialSuccess = partialSuccess || hasInstrumentation(entries)
	}
	req := &logpb.WriteLogEntriesRequest{
		LogName:        l.logName,
		Resource:       l.commonResource,
		Labels:         l.commonLabels,
		Entries:        entries,
		PartialSuccess: partialSuccess,
	}
	ctx, afterCall := l.ctxFunc()
	ctx, cancel := context.WithTimeout(ctx, defaultWriteTimeout)
	defer cancel()
	_, err := l.client.client.WriteLogEntries(ctx, req)
	if err != nil {
		l.client.error(err)
	}
	if afterCall != nil {
		afterCall()
	}
}

// StandardLogger returns a *log.Logger for the provided severity.
//
// This method is cheap. A single log.Logger is pre-allocated for each
// severity level in each Logger. Callers may mutate the returned log.Logger
// (for example by calling SetFlags or SetPrefix).
func (l *Logger) StandardLogger(s Severity) *log.Logger { return l.stdLoggers[s] }

func populateTraceInfo(e *Entry, req *http.Request) bool {
	if req == nil {
		if e.HTTPRequest != nil && e.HTTPRequest.Request != nil {
			req = e.HTTPRequest.Request
		} else {
			return false
		}
	}
	header := req.Header.Get("Traceparent")
	if header != "" {
		// do not use traceSampled flag defined by traceparent because
		// flag's definition differs from expected by Cloud Tracing
		traceID, spanID, _ := deconstructTraceParent(header)
		if traceID != "" {
			e.Trace = traceID
			e.SpanID = spanID
			return true
		}
	}
	header = req.Header.Get("X-Cloud-Trace-Context")
	if header != "" {
		traceID, spanID, traceSampled := deconstructXCloudTraceContext(header)
		if traceID != "" {
			e.Trace = traceID
			e.SpanID = spanID
			// enforce sampling if required
			e.TraceSampled = e.TraceSampled || traceSampled
			return true
		}
	}
	return false
}

// As per format described at https://www.w3.org/TR/trace-context/#traceparent-header-field-values
var validTraceParentExpression = regexp.MustCompile(`^(00)-([a-fA-F\d]{32})-([a-f\d]{16})-([a-fA-F\d]{2})$`)

func deconstructTraceParent(s string) (traceID, spanID string, traceSampled bool) {
	matches := validTraceParentExpression.FindStringSubmatch(s)
	if matches != nil {
		// regexp package does not support negative lookahead preventing all 0 validations
		if matches[2] == "00000000000000000000000000000000" || matches[3] == "0000000000000000" {
			return
		}
		flags, err := strconv.ParseInt(matches[4], 16, 16)
		if err == nil {
			traceSampled = (flags & 0x01) == 1
		}
		traceID, spanID = matches[2], matches[3]
	}
	return
}

var validXCloudTraceContext = regexp.MustCompile(
	// Matches on "TRACE_ID"
	`([a-f\d]+)?` +
		// Matches on "/SPAN_ID"
		`(?:/([a-f\d]+))?` +
		// Matches on ";0=TRACE_TRUE"
		`(?:;o=(\d))?`)

func deconstructXCloudTraceContext(s string) (traceID, spanID string, traceSampled bool) {
	// As per the format described at https://cloud.google.com/trace/docs/setup#force-trace
	//    "X-Cloud-Trace-Context: TRACE_ID/SPAN_ID;o=TRACE_TRUE"
	// for example:
	//    "X-Cloud-Trace-Context: 105445aa7843bc8bf206b120001000/1;o=1"
	//
	// We expect:
	//   * traceID (optional): 			"105445aa7843bc8bf206b120001000"
	//   * spanID (optional):       	"1"
	//   * traceSampled (optional): 	true
	matches := validXCloudTraceContext.FindStringSubmatch(s)

	if matches != nil {
		traceID, spanID, traceSampled = matches[1], matches[2], matches[3] == "1"
	}

	if spanID == "0" {
		spanID = ""
	}

	return
}

// ToLogEntry takes an Entry structure and converts it to the LogEntry proto.
// A parent can take any of the following forms:
//
//	projects/PROJECT_ID
//	folders/FOLDER_ID
//	billingAccounts/ACCOUNT_ID
//	organizations/ORG_ID
//
// for backwards compatibility, a string with no '/' is also allowed and is interpreted
// as a project ID.
//
// ToLogEntry is implied when users invoke Logger.Log or Logger.LogSync,
// but its exported as a pub function here to give users additional flexibility
// when using the library. Don't call this method manually if Logger.Log or
// Logger.LogSync are used, it is intended to be used together with direct call
// to WriteLogEntries method.
func ToLogEntry(e Entry, parent string) (*logpb.LogEntry, error) {
	var l Logger
	return l.ToLogEntry(e, parent)
}

// ToLogEntry for Logger instance
func (l *Logger) ToLogEntry(e Entry, parent string) (*logpb.LogEntry, error) {
	parent, err := makeParent(parent)
	if err != nil {
		return nil, err
	}
	return toLogEntryInternal(e, l, parent, 1)
}

func toLogEntryInternal(e Entry, l *Logger, parent string, skipLevels int) (*logpb.LogEntry, error) {
	if e.LogName != "" {
		return nil, errors.New("logging: Entry.LogName should be not be set when writing")
	}
	t := e.Timestamp
	if t.IsZero() {
		t = now()
	}
	ts := timestamppb.New(t)
	if l != nil && l.populateSourceLocation != DoNotPopulateSourceLocation && e.SourceLocation == nil {
		if l.populateSourceLocation == AlwaysPopulateSourceLocation ||
			l.populateSourceLocation == PopulateSourceLocationForDebugEntries && e.Severity == Severity(Debug) {
			// filename and line are captured for source code that calls
			// skipLevels up the goroutine calling stack + 1 for this func.
			pc, file, line, ok := runtime.Caller(skipLevels + 1)
			if ok {
				details := runtime.FuncForPC(pc)
				e.SourceLocation = &logpb.LogEntrySourceLocation{
					File:     file,
					Function: details.Name(),
					Line:     int64(line),
				}
			}
		}
	}
	if e.Trace == "" {
		populateTraceInfo(&e, nil)
		// format trace
		if e.Trace != "" && !strings.Contains(e.Trace, "/traces/") {
			e.Trace = fmt.Sprintf("%s/traces/%s", parent, e.Trace)
		}
	}
	req, err := fromHTTPRequest(e.HTTPRequest)
	if err != nil {
		if l != nil && l.client != nil {
			l.client.error(err)
		} else {
			return nil, err
		}
	}
	ent := &logpb.LogEntry{
		Timestamp:      ts,
		Severity:       logtypepb.LogSeverity(e.Severity),
		InsertId:       e.InsertID,
		HttpRequest:    req,
		Operation:      e.Operation,
		Labels:         e.Labels,
		Trace:          e.Trace,
		SpanId:         e.SpanID,
		Resource:       e.Resource,
		SourceLocation: e.SourceLocation,
		TraceSampled:   e.TraceSampled,
	}
	switch p := e.Payload.(type) {
	case string:
		ent.Payload = &logpb.LogEntry_TextPayload{TextPayload: p}
	case *anypb.Any:
		ent.Payload = &logpb.LogEntry_ProtoPayload{ProtoPayload: p}
	default:
		s, err := toProtoStruct(p)
		if err != nil {
			return nil, err
		}
		ent.Payload = &logpb.LogEntry_JsonPayload{JsonPayload: s}
	}
	return ent, nil
}

// entry represents the fields of a logging.Entry that can be parsed by Logging agent.
// See the mappings at https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
type structuredLogEntry struct {
	// JsonMessage    map[string]interface{}        `json:"message,omitempty"`
	// TextMessage    string                        `json:"message,omitempty"`
	Message        json.RawMessage               `json:"message"`
	Severity       string                        `json:"severity,omitempty"`
	HTTPRequest    *logtypepb.HttpRequest        `json:"httpRequest,omitempty"`
	Timestamp      string                        `json:"timestamp,omitempty"`
	Labels         map[string]string             `json:"logging.googleapis.com/labels,omitempty"`
	InsertID       string                        `json:"logging.googleapis.com/insertId,omitempty"`
	Operation      *logpb.LogEntryOperation      `json:"logging.googleapis.com/operation,omitempty"`
	SourceLocation *logpb.LogEntrySourceLocation `json:"logging.googleapis.com/sourceLocation,omitempty"`
	SpanID         string                        `json:"logging.googleapis.com/spanId,omitempty"`
	Trace          string                        `json:"logging.googleapis.com/trace,omitempty"`
	TraceSampled   bool                          `json:"logging.googleapis.com/trace_sampled,omitempty"`
}

func convertSnakeToMixedCase(snakeStr string) string {
	words := strings.Split(snakeStr, "_")
	mixedStr := words[0]
	for _, word := range words[1:] {
		mixedStr += strings.Title(word)
	}
	return mixedStr
}

func (s structuredLogEntry) MarshalJSON() ([]byte, error) {
	// extract structuredLogEntry into json map
	type Alias structuredLogEntry
	var mapData map[string]interface{}
	data, err := json.Marshal(Alias(s))
	if err == nil {
		err = json.Unmarshal(data, &mapData)
	}
	if err == nil {
		// ensure all inner dicts use mixed case instead of snake case
		innerDicts := [3]string{"httpRequest", "logging.googleapis.com/operation", "logging.googleapis.com/sourceLocation"}
		for _, field := range innerDicts {
			if fieldData, ok := mapData[field]; ok {
				formattedFieldData := make(map[string]interface{})
				for k, v := range fieldData.(map[string]interface{}) {
					formattedFieldData[convertSnakeToMixedCase(k)] = v
				}
				mapData[field] = formattedFieldData
			}
		}
		// serialize json map into raw bytes
		return json.Marshal(mapData)
	}
	return data, err
}

func serializeEntryToWriter(entry *logpb.LogEntry, w io.Writer) error {
	jsonifiedEntry := structuredLogEntry{
		Severity:       entry.Severity.String(),
		HTTPRequest:    entry.HttpRequest,
		Timestamp:      entry.Timestamp.String(),
		Labels:         entry.Labels,
		InsertID:       entry.InsertId,
		Operation:      entry.Operation,
		SourceLocation: entry.SourceLocation,
		SpanID:         entry.SpanId,
		Trace:          entry.Trace,
		TraceSampled:   entry.TraceSampled,
	}
	var err error
	if entry.GetTextPayload() != "" {
		jsonifiedEntry.Message, err = json.Marshal(entry.GetTextPayload())
	} else if entry.GetJsonPayload() != nil {
		jsonifiedEntry.Message, err = json.Marshal(entry.GetJsonPayload().AsMap())
	} else {
		return ErrRedirectProtoPayloadNotSupported
	}
	if err == nil {
		err = json.NewEncoder(w).Encode(jsonifiedEntry)
	}
	return err
}
