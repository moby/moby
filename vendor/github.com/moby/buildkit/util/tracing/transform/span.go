package transform

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	v11 "go.opentelemetry.io/proto/otlp/common/v1"
	v1 "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

const (
	maxMessageEventsPerSpan = 128
)

// Spans transforms slice of OTLP ResourceSpan into a slice of SpanSnapshots.
func Spans(sdl []*tracepb.ResourceSpans) []tracesdk.ReadOnlySpan {
	if len(sdl) == 0 {
		return nil
	}

	var out []tracesdk.ReadOnlySpan

	for _, sd := range sdl {
		if sd == nil {
			continue
		}

		for _, sdi := range sd.InstrumentationLibrarySpans {
			sda := make([]tracesdk.ReadOnlySpan, len(sdi.Spans))
			for i, s := range sdi.Spans {
				sda[i] = &readOnlySpan{
					pb:        s,
					il:        sdi.InstrumentationLibrary,
					resource:  sd.Resource,
					schemaURL: sd.SchemaUrl,
				}
			}
			out = append(out, sda...)
		}
	}

	return out
}

type readOnlySpan struct {
	// Embed the interface to implement the private method.
	tracesdk.ReadOnlySpan

	pb        *tracepb.Span
	il        *v11.InstrumentationLibrary
	resource  *v1.Resource
	schemaURL string
}

func (s *readOnlySpan) Name() string {
	return s.pb.Name
}

func (s *readOnlySpan) SpanContext() trace.SpanContext {
	var tid trace.TraceID
	copy(tid[:], s.pb.TraceId)
	var sid trace.SpanID
	copy(sid[:], s.pb.SpanId)

	st, _ := trace.ParseTraceState(s.pb.TraceState)

	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceState: st,
	})
}

func (s *readOnlySpan) Parent() trace.SpanContext {
	if len(s.pb.ParentSpanId) == 0 {
		return trace.SpanContext{}
	}
	var tid trace.TraceID
	copy(tid[:], s.pb.TraceId)
	var psid trace.SpanID
	copy(psid[:], s.pb.ParentSpanId)
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid,
		SpanID:  psid,
	})
}

func (s *readOnlySpan) SpanKind() trace.SpanKind {
	return spanKind(s.pb.Kind)
}

func (s *readOnlySpan) StartTime() time.Time {
	return time.Unix(0, int64(s.pb.StartTimeUnixNano))
}

func (s *readOnlySpan) EndTime() time.Time {
	return time.Unix(0, int64(s.pb.EndTimeUnixNano))
}

func (s *readOnlySpan) Attributes() []attribute.KeyValue {
	return Attributes(s.pb.Attributes)
}

func (s *readOnlySpan) Links() []tracesdk.Link {
	return links(s.pb.Links)
}

func (s *readOnlySpan) Events() []tracesdk.Event {
	return spanEvents(s.pb.Events)
}

func (s *readOnlySpan) Status() tracesdk.Status {
	return tracesdk.Status{
		Code:        statusCode(s.pb.Status),
		Description: s.pb.Status.GetMessage(),
	}
}

func (s *readOnlySpan) InstrumentationLibrary() instrumentation.Library {
	return instrumentationLibrary(s.il)
}

// Resource returns information about the entity that produced the span.
func (s *readOnlySpan) Resource() *resource.Resource {
	if s.resource == nil {
		return nil
	}
	if s.schemaURL != "" {
		return resource.NewWithAttributes(s.schemaURL, Attributes(s.resource.Attributes)...)
	}
	return resource.NewSchemaless(Attributes(s.resource.Attributes)...)
}

// DroppedAttributes returns the number of attributes dropped by the span
// due to limits being reached.
func (s *readOnlySpan) DroppedAttributes() int {
	return int(s.pb.DroppedAttributesCount)
}

// DroppedLinks returns the number of links dropped by the span due to
// limits being reached.
func (s *readOnlySpan) DroppedLinks() int {
	return int(s.pb.DroppedLinksCount)
}

// DroppedEvents returns the number of events dropped by the span due to
// limits being reached.
func (s *readOnlySpan) DroppedEvents() int {
	return int(s.pb.DroppedEventsCount)
}

// ChildSpanCount returns the count of spans that consider the span a
// direct parent.
func (s *readOnlySpan) ChildSpanCount() int {
	return 0
}

var _ tracesdk.ReadOnlySpan = &readOnlySpan{}

// status transform a OTLP span status into span code.
func statusCode(st *tracepb.Status) codes.Code {
	switch st.Code {
	case tracepb.Status_STATUS_CODE_ERROR:
		return codes.Error
	default:
		return codes.Ok
	}
}

// links transforms OTLP span links to span Links.
func links(links []*tracepb.Span_Link) []tracesdk.Link {
	if len(links) == 0 {
		return nil
	}

	sl := make([]tracesdk.Link, 0, len(links))
	for _, otLink := range links {
		// This redefinition is necessary to prevent otLink.*ID[:] copies
		// being reused -- in short we need a new otLink per iteration.
		otLink := otLink

		var tid trace.TraceID
		copy(tid[:], otLink.TraceId)
		var sid trace.SpanID
		copy(sid[:], otLink.SpanId)

		sctx := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: tid,
			SpanID:  sid,
		})

		sl = append(sl, tracesdk.Link{
			SpanContext: sctx,
			Attributes:  Attributes(otLink.Attributes),
		})
	}
	return sl
}

// spanEvents transforms OTLP span events to span Events.
func spanEvents(es []*tracepb.Span_Event) []tracesdk.Event {
	if len(es) == 0 {
		return nil
	}

	evCount := len(es)
	if evCount > maxMessageEventsPerSpan {
		evCount = maxMessageEventsPerSpan
	}
	events := make([]tracesdk.Event, 0, evCount)
	messageEvents := 0

	// Transform message events
	for _, e := range es {
		if messageEvents >= maxMessageEventsPerSpan {
			break
		}
		messageEvents++
		events = append(events,
			tracesdk.Event{
				Name:                  e.Name,
				Time:                  time.Unix(0, int64(e.TimeUnixNano)),
				Attributes:            Attributes(e.Attributes),
				DroppedAttributeCount: int(e.DroppedAttributesCount),
			},
		)
	}

	return events
}

// spanKind transforms a an OTLP span kind to SpanKind.
func spanKind(kind tracepb.Span_SpanKind) trace.SpanKind {
	switch kind {
	case tracepb.Span_SPAN_KIND_INTERNAL:
		return trace.SpanKindInternal
	case tracepb.Span_SPAN_KIND_CLIENT:
		return trace.SpanKindClient
	case tracepb.Span_SPAN_KIND_SERVER:
		return trace.SpanKindServer
	case tracepb.Span_SPAN_KIND_PRODUCER:
		return trace.SpanKindProducer
	case tracepb.Span_SPAN_KIND_CONSUMER:
		return trace.SpanKindConsumer
	default:
		return trace.SpanKindUnspecified
	}
}
