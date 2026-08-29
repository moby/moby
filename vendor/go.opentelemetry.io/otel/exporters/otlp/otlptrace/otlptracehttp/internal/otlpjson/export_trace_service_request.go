// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package otlpjson implements OTLP JSON Protobuf encoding for trace data.
//
// The encoding conforms to the OTLP specs
// (https://opentelemetry.io/docs/specs/otlp/#json-protobuf-encoding):
//   - trace ID and span ID byte arrays are encoded as case-insensitive hex-encoded strings
//   - enum values encoded as integers
//   - field names in lowerCamelCase
//   - 64-bit integers encoded as quoted decimal strings (ProtoJSON specs)
package otlpjson

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// ExportTraceServiceRequest corresponds to coltracepb.ExportTraceServiceRequest.
type ExportTraceServiceRequest struct {
	ResourceSpans []*ResourceSpans `json:"resourceSpans,omitempty"`
}

// ResourceSpans corresponds to tracepb.ResourceSpans.
type ResourceSpans struct {
	Resource   *Resource     `json:"resource,omitempty"`
	ScopeSpans []*ScopeSpans `json:"scopeSpans,omitempty"`
	SchemaURL  string        `json:"schemaUrl,omitempty"`
}

// Resource corresponds to resourcepb.Resource.
type Resource struct {
	Attributes             []*KeyValue  `json:"attributes,omitempty"`
	DroppedAttributesCount uint32       `json:"droppedAttributesCount,omitempty"`
	EntityRefs             []*EntityRef `json:"entityRefs,omitempty"`
}

// ScopeSpans corresponds to tracepb.ScopeSpans.
type ScopeSpans struct {
	Scope     *InstrumentationScope `json:"scope,omitempty"`
	Spans     []*Span               `json:"spans,omitempty"`
	SchemaURL string                `json:"schemaUrl,omitempty"`
}

// InstrumentationScope corresponds to tracepb.InstrumentationScope.
type InstrumentationScope struct {
	Name                   string      `json:"name,omitempty"`
	Version                string      `json:"version,omitempty"`
	Attributes             []*KeyValue `json:"attributes,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
}

// EntityRef corresponds to resourcepb.EntityRef.
type EntityRef struct {
	SchemaURL       string   `json:"schemaUrl,omitempty"`
	Type            string   `json:"type,omitempty"`
	IdKeys          []string `json:"idKeys,omitempty"`
	DescriptionKeys []string `json:"descriptionKeys,omitempty"`
}

// KeyValue corresponds to commonpb.KeyValue.
type KeyValue struct {
	Key   string    `json:"key"`
	Value *AnyValue `json:"value,omitempty"`
}

// AnyValue corresponds to commonpb.AnyValue.
type AnyValue struct {
	StringValue *string      `json:"stringValue,omitempty"`
	BoolValue   *bool        `json:"boolValue,omitempty"`
	IntValue    *Int64       `json:"intValue,omitempty"`
	DoubleValue *Float64     `json:"doubleValue,omitempty"`
	ArrayValue  *ArrayValue  `json:"arrayValue,omitempty"`
	KvlistValue *KvlistValue `json:"kvlistValue,omitempty"`
	BytesValue  []byte       `json:"bytesValue,omitempty"`
}

// ArrayValue corresponds to commonpb.ArrayValue.
type ArrayValue struct {
	Values []*AnyValue `json:"values,omitempty"`
}

// KvlistValue corresponds to commonpb.KeyValueList.
type KvlistValue struct {
	Values []*KeyValue `json:"values,omitempty"`
}

// Span corresponds to tracepb.Span.
type Span struct {
	TraceID                TraceID     `json:"traceId"`
	SpanID                 SpanID      `json:"spanId"`
	TraceState             string      `json:"traceState,omitempty"`
	ParentSpanID           *SpanID     `json:"parentSpanId,omitempty"`
	Flags                  uint32      `json:"flags,omitempty"`
	Name                   string      `json:"name,omitempty"`
	Kind                   int32       `json:"kind,omitempty"`
	StartTimeUnixNano      Uint64      `json:"startTimeUnixNano,omitempty"`
	EndTimeUnixNano        Uint64      `json:"endTimeUnixNano,omitempty"`
	Attributes             []*KeyValue `json:"attributes,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
	Events                 []*Event    `json:"events,omitempty"`
	DroppedEventsCount     uint32      `json:"droppedEventsCount,omitempty"`
	Links                  []*Link     `json:"links,omitempty"`
	DroppedLinksCount      uint32      `json:"droppedLinksCount,omitempty"`
	Status                 *Status     `json:"status,omitempty"`
}

// Event corresponds to tracepb.Span_Event.
type Event struct {
	TimeUnixNano           Uint64      `json:"timeUnixNano,omitempty"`
	Name                   string      `json:"name,omitempty"`
	Attributes             []*KeyValue `json:"attributes,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
}

// Link corresponds to tracepb.Span_Link.
type Link struct {
	TraceID                TraceID     `json:"traceId"`
	SpanID                 SpanID      `json:"spanId"`
	TraceState             string      `json:"traceState,omitempty"`
	Attributes             []*KeyValue `json:"attributes,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
	Flags                  uint32      `json:"flags,omitempty"`
}

// Status corresponds to tracepb.Status.
type Status struct {
	Message string `json:"message,omitempty"`
	Code    int32  `json:"code,omitempty"`
}

// MarshalExportTraceServiceRequest encodes an ExportTraceServiceRequest as JSON Protobuf encoded bytes.
func MarshalExportTraceServiceRequest(req *coltracepb.ExportTraceServiceRequest) ([]byte, error) {
	if req == nil {
		return []byte("{}"), nil
	}
	r := &ExportTraceServiceRequest{}
	for _, rs := range req.ResourceSpans {
		r.ResourceSpans = append(r.ResourceSpans, encodeResourceSpans(rs))
	}
	return json.Marshal(r)
}

func encodeResourceSpans(rs *tracepb.ResourceSpans) *ResourceSpans {
	if rs == nil {
		return nil
	}
	out := &ResourceSpans{SchemaURL: rs.SchemaUrl}
	if rs.Resource != nil {
		out.Resource = &Resource{
			Attributes:             encodeKeyValues(rs.Resource.Attributes),
			DroppedAttributesCount: rs.Resource.DroppedAttributesCount,
			EntityRefs:             encodeEntityRefs(rs.Resource.EntityRefs),
		}
	}
	for _, ss := range rs.ScopeSpans {
		out.ScopeSpans = append(out.ScopeSpans, encodeScopeSpans(ss))
	}
	return out
}

func encodeEntityRefs(ers []*commonpb.EntityRef) []*EntityRef {
	if len(ers) == 0 {
		return nil
	}
	out := make([]*EntityRef, len(ers))
	for i, er := range ers {
		if er == nil {
			continue
		}
		out[i] = &EntityRef{
			SchemaURL:       er.SchemaUrl,
			Type:            er.Type,
			IdKeys:          er.IdKeys,
			DescriptionKeys: er.DescriptionKeys,
		}
	}
	return out
}

func encodeScopeSpans(ss *tracepb.ScopeSpans) *ScopeSpans {
	if ss == nil {
		return nil
	}
	out := &ScopeSpans{SchemaURL: ss.SchemaUrl}
	if ss.Scope != nil {
		out.Scope = &InstrumentationScope{
			Name:                   ss.Scope.Name,
			Version:                ss.Scope.Version,
			Attributes:             encodeKeyValues(ss.Scope.Attributes),
			DroppedAttributesCount: ss.Scope.DroppedAttributesCount,
		}
	}
	for _, s := range ss.Spans {
		out.Spans = append(out.Spans, encodeSpan(s))
	}
	return out
}

func encodeSpan(s *tracepb.Span) *Span {
	if s == nil {
		return nil
	}
	out := &Span{
		TraceState:             s.TraceState,
		Flags:                  s.Flags,
		Name:                   s.Name,
		Kind:                   int32(s.Kind),
		StartTimeUnixNano:      Uint64(s.StartTimeUnixNano),
		EndTimeUnixNano:        Uint64(s.EndTimeUnixNano),
		Attributes:             encodeKeyValues(s.Attributes),
		DroppedAttributesCount: s.DroppedAttributesCount,
		DroppedEventsCount:     s.DroppedEventsCount,
		DroppedLinksCount:      s.DroppedLinksCount,
	}

	copy(out.TraceID[:], s.TraceId)
	copy(out.SpanID[:], s.SpanId)

	if len(s.ParentSpanId) > 0 {
		var psid SpanID
		copy(psid[:], s.ParentSpanId)
		if psid != (SpanID{}) {
			out.ParentSpanID = &psid
		}
	}

	for _, e := range s.Events {
		if e == nil {
			continue
		}
		out.Events = append(out.Events, &Event{
			TimeUnixNano:           Uint64(e.TimeUnixNano),
			Name:                   e.Name,
			Attributes:             encodeKeyValues(e.Attributes),
			DroppedAttributesCount: e.DroppedAttributesCount,
		})
	}
	for _, l := range s.Links {
		out.Links = append(out.Links, encodeLink(l))
	}
	if s.Status != nil {
		out.Status = &Status{
			Message: s.Status.Message,
			Code:    int32(s.Status.Code),
		}
	}
	return out
}

func encodeLink(l *tracepb.Span_Link) *Link {
	if l == nil {
		return nil
	}
	out := &Link{
		TraceState:             l.TraceState,
		Attributes:             encodeKeyValues(l.Attributes),
		DroppedAttributesCount: l.DroppedAttributesCount,
		Flags:                  l.Flags,
	}
	copy(out.TraceID[:], l.TraceId)
	copy(out.SpanID[:], l.SpanId)
	return out
}

func encodeKeyValues(kvs []*commonpb.KeyValue) []*KeyValue {
	if len(kvs) == 0 {
		return nil
	}
	out := make([]*KeyValue, len(kvs))
	for i, kv := range kvs {
		if kv == nil {
			continue
		}
		out[i] = &KeyValue{
			Key:   kv.Key,
			Value: encodeAnyValue(kv.Value),
		}
	}
	return out
}

func encodeAnyValue(av *commonpb.AnyValue) *AnyValue {
	if av == nil {
		return nil
	}
	out := &AnyValue{}
	switch v := av.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		out.StringValue = &v.StringValue
	case *commonpb.AnyValue_BoolValue:
		out.BoolValue = &v.BoolValue
	case *commonpb.AnyValue_IntValue:
		iv := Int64(v.IntValue)
		out.IntValue = &iv
	case *commonpb.AnyValue_DoubleValue:
		dv := Float64(v.DoubleValue)
		out.DoubleValue = &dv
	case *commonpb.AnyValue_ArrayValue:
		if v.ArrayValue != nil {
			arr := &ArrayValue{}
			for _, val := range v.ArrayValue.Values {
				arr.Values = append(arr.Values, encodeAnyValue(val))
			}
			out.ArrayValue = arr
		}
	case *commonpb.AnyValue_KvlistValue:
		if v.KvlistValue != nil {
			out.KvlistValue = &KvlistValue{
				Values: encodeKeyValues(v.KvlistValue.Values),
			}
		}
	case *commonpb.AnyValue_BytesValue:
		out.BytesValue = v.BytesValue
	}
	return out
}

// UnmarshalExportTraceServiceRequest decodes JSON Protobuf encoded payload into an ExportTraceServiceRequest.
func UnmarshalExportTraceServiceRequest(data []byte, req *coltracepb.ExportTraceServiceRequest) error {
	var jr ExportTraceServiceRequest
	if err := json.Unmarshal(data, &jr); err != nil {
		return err
	}

	for _, rs := range jr.ResourceSpans {
		req.ResourceSpans = append(req.ResourceSpans, decodeResourceSpans(rs))
	}
	return nil
}

func decodeResourceSpans(jrs *ResourceSpans) *tracepb.ResourceSpans {
	rs := &tracepb.ResourceSpans{SchemaUrl: jrs.SchemaURL}
	if jrs.Resource != nil {
		rs.Resource = &resourcepb.Resource{
			Attributes:             decodeKeyValues(jrs.Resource.Attributes),
			DroppedAttributesCount: jrs.Resource.DroppedAttributesCount,
			EntityRefs:             decodeEntityRefs(jrs.Resource.EntityRefs),
		}
	}
	for _, ss := range jrs.ScopeSpans {
		rs.ScopeSpans = append(rs.ScopeSpans, decodeScopeSpans(ss))
	}
	return rs
}

func decodeEntityRefs(jers []*EntityRef) []*commonpb.EntityRef {
	if len(jers) == 0 {
		return nil
	}
	ers := make([]*commonpb.EntityRef, len(jers))
	for i, jer := range jers {
		if jer == nil {
			continue
		}
		ers[i] = &commonpb.EntityRef{
			SchemaUrl:       jer.SchemaURL,
			Type:            jer.Type,
			IdKeys:          jer.IdKeys,
			DescriptionKeys: jer.DescriptionKeys,
		}
	}
	return ers
}

func decodeScopeSpans(jss *ScopeSpans) *tracepb.ScopeSpans {
	ss := &tracepb.ScopeSpans{SchemaUrl: jss.SchemaURL}
	if jss.Scope != nil {
		ss.Scope = &commonpb.InstrumentationScope{
			Name:                   jss.Scope.Name,
			Version:                jss.Scope.Version,
			Attributes:             decodeKeyValues(jss.Scope.Attributes),
			DroppedAttributesCount: jss.Scope.DroppedAttributesCount,
		}
	}
	for _, s := range jss.Spans {
		ss.Spans = append(ss.Spans, decodeSpan(s))
	}
	return ss
}

func decodeSpan(js *Span) *tracepb.Span {
	s := &tracepb.Span{
		TraceId:                js.TraceID[:],
		SpanId:                 js.SpanID[:],
		TraceState:             js.TraceState,
		Flags:                  js.Flags,
		Name:                   js.Name,
		Kind:                   tracepb.Span_SpanKind(js.Kind),
		StartTimeUnixNano:      uint64(js.StartTimeUnixNano),
		EndTimeUnixNano:        uint64(js.EndTimeUnixNano),
		Attributes:             decodeKeyValues(js.Attributes),
		DroppedAttributesCount: js.DroppedAttributesCount,
		DroppedEventsCount:     js.DroppedEventsCount,
		DroppedLinksCount:      js.DroppedLinksCount,
	}
	if js.ParentSpanID != nil {
		s.ParentSpanId = js.ParentSpanID[:]
	}
	for _, e := range js.Events {
		s.Events = append(s.Events, &tracepb.Span_Event{
			TimeUnixNano:           uint64(e.TimeUnixNano),
			Name:                   e.Name,
			Attributes:             decodeKeyValues(e.Attributes),
			DroppedAttributesCount: e.DroppedAttributesCount,
		})
	}
	for _, l := range js.Links {
		s.Links = append(s.Links, decodeLink(l))
	}
	if js.Status != nil {
		s.Status = &tracepb.Status{
			Message: js.Status.Message,
			Code:    tracepb.Status_StatusCode(js.Status.Code),
		}
	}
	return s
}

func decodeLink(jl *Link) *tracepb.Span_Link {
	return &tracepb.Span_Link{
		TraceId:                jl.TraceID[:],
		SpanId:                 jl.SpanID[:],
		TraceState:             jl.TraceState,
		Attributes:             decodeKeyValues(jl.Attributes),
		DroppedAttributesCount: jl.DroppedAttributesCount,
		Flags:                  jl.Flags,
	}
}

func decodeKeyValues(jkvs []*KeyValue) []*commonpb.KeyValue {
	if len(jkvs) == 0 {
		return nil
	}
	kvs := make([]*commonpb.KeyValue, len(jkvs))
	for i, jkv := range jkvs {
		kvs[i] = &commonpb.KeyValue{
			Key:   jkv.Key,
			Value: decodeAnyValue(jkv.Value),
		}
	}
	return kvs
}

func decodeAnyValue(jav *AnyValue) *commonpb.AnyValue {
	if jav == nil {
		return nil
	}
	av := &commonpb.AnyValue{}
	switch {
	case jav.StringValue != nil:
		av.Value = &commonpb.AnyValue_StringValue{StringValue: *jav.StringValue}
	case jav.BoolValue != nil:
		av.Value = &commonpb.AnyValue_BoolValue{BoolValue: *jav.BoolValue}
	case jav.IntValue != nil:
		av.Value = &commonpb.AnyValue_IntValue{IntValue: int64(*jav.IntValue)}
	case jav.DoubleValue != nil:
		av.Value = &commonpb.AnyValue_DoubleValue{DoubleValue: float64(*jav.DoubleValue)}
	case jav.ArrayValue != nil:
		arr := &commonpb.ArrayValue{}
		for _, v := range jav.ArrayValue.Values {
			arr.Values = append(arr.Values, decodeAnyValue(v))
		}
		av.Value = &commonpb.AnyValue_ArrayValue{ArrayValue: arr}
	case jav.KvlistValue != nil:
		av.Value = &commonpb.AnyValue_KvlistValue{
			KvlistValue: &commonpb.KeyValueList{
				Values: decodeKeyValues(jav.KvlistValue.Values),
			},
		}
	case jav.BytesValue != nil:
		av.Value = &commonpb.AnyValue_BytesValue{BytesValue: jav.BytesValue}
	}
	return av
}

// Float64 encodes non-finite values as strings per ProtoJSON specs.
type Float64 float64

func (f Float64) MarshalJSON() ([]byte, error) {
	switch value := float64(f); {
	case math.IsNaN(value):
		return []byte(`"NaN"`), nil
	case math.IsInf(value, 1):
		return []byte(`"Infinity"`), nil
	case math.IsInf(value, -1):
		return []byte(`"-Infinity"`), nil
	default:
		return strconv.AppendFloat(nil, value, 'g', -1, 64), nil
	}
}

func (f *Float64) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		switch str {
		case "NaN":
			*f = Float64(math.NaN())
		case "Infinity":
			*f = Float64(math.Inf(1))
		case "-Infinity":
			*f = Float64(math.Inf(-1))
		default:
			var value *float64
			if str != strings.TrimSpace(str) || json.Unmarshal([]byte(str), &value) != nil || value == nil {
				return fmt.Errorf("invalid float value %q", str)
			}
			*f = Float64(*value)
		}
		return nil
	}

	var value float64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*f = Float64(value)
	return nil
}

// Int64 encodes int64 as a quoted decimal string per ProtoJSON specs.
type Int64 int64

func (i Int64) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatInt(int64(i), 10) + `"`), nil
}

func (i *Int64) UnmarshalJSON(data []byte) error {
	// expects either a string representation or a number
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*i = Int64(v)
		return nil
	}
	var v int64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*i = Int64(v)
	return nil
}

// Uint64 encodes uint64 as a quoted decimal string per ProtoJSON specs.
type Uint64 uint64

func (i Uint64) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatUint(uint64(i), 10) + `"`), nil
}

func (i *Uint64) UnmarshalJSON(data []byte) error {
	// expects either a string representation or a number
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		*i = Uint64(v)
		return nil
	}
	var v uint64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*i = Uint64(v)
	return nil
}

const base16Alphabets = "0123456789ABCDEF"

// TraceID encodes a 16-byte trace ID as a case-insensitive hex-encoded string.
type TraceID [16]byte

func (t TraceID) MarshalJSON() ([]byte, error) {
	var b [34]byte
	b[0] = '"'
	for i, v := range t {
		b[1+i*2] = base16Alphabets[v>>4]
		b[2+i*2] = base16Alphabets[v&0x0f]
	}
	b[33] = '"'
	return b[:], nil
}

func (t *TraceID) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	b, err := hex.DecodeString(str)
	if err != nil {
		return err
	}
	if len(b) != len(t) {
		return fmt.Errorf("invalid trace ID length: got %d, want %d", len(b), len(t))
	}
	copy(t[:], b)
	return nil
}

// SpanID encodes an 8-byte span ID as a case-insensitive hex-encoded string.
type SpanID [8]byte

func (s SpanID) MarshalJSON() ([]byte, error) {
	var b [18]byte
	b[0] = '"'
	for i, v := range s {
		b[1+i*2] = base16Alphabets[v>>4]
		b[2+i*2] = base16Alphabets[v&0x0f]
	}
	b[17] = '"'
	return b[:], nil
}

func (s *SpanID) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	b, err := hex.DecodeString(str)
	if err != nil {
		return err
	}
	if len(b) != len(s) {
		return fmt.Errorf("invalid span ID length: got %d, want %d", len(b), len(s))
	}
	copy(s[:], b)
	return nil
}
