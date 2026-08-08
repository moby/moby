// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/internal/global"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/log/internal/attrnorm"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

const (
	// attributesInlineCount is the number of attributes that are efficiently
	// stored in an array within a Record. This value is borrowed from slog which
	// performed a quantitative survey of log library use and found this value to
	// cover 95% of all use-cases (https://go.dev/blog/slog#performance).
	attributesInlineCount = 5
	// maxUniqueSize helps reduce peak allocation.
	maxUniqueSize = 1028
)

var logAttrDropped = sync.OnceFunc(func() {
	global.Warn("limit reached: dropping log Record attributes")
})

var logKeyValuePairDropped = sync.OnceFunc(func() {
	global.Warn("key duplication: dropping key-value pair")
})

// uniquePool is a pool of unique attributes used for attributes de-duplication.
var uniquePool = sync.Pool{
	New: func() any { return new([]attribute.KeyValue) },
}

func getUnique() *[]attribute.KeyValue {
	return uniquePool.Get().(*[]attribute.KeyValue)
}

func putUnique(v *[]attribute.KeyValue) {
	if cap(*v) <= maxUniqueSize {
		clear(*v)
		*v = (*v)[:0]
		uniquePool.Put(v)
	}
}

// indexPool is a pool of index maps used for attributes de-duplication.
var indexPool = sync.Pool{
	New: func() any { return make(map[attribute.Key]int) },
}

func getIndex() map[attribute.Key]int {
	return indexPool.Get().(map[attribute.Key]int)
}

func putIndex(index map[attribute.Key]int) {
	clear(index)
	indexPool.Put(index)
}

// Record is a log record emitted by the Logger.
// A log record with non-empty event name is interpreted as an event record.
//
// Do not create instances of Record on your own in production code.
// You can use [go.opentelemetry.io/otel/sdk/log/logtest.RecordFactory]
// for testing purposes.
type Record struct {
	// Do not embed the log.Record. Attributes need to be overwrite-able and
	// deep-copying needs to be possible.

	eventName         string
	timestamp         time.Time
	observedTimestamp time.Time
	severity          log.Severity
	severityText      string
	body              attribute.Value

	// The fields below are for optimizing the implementation of Attributes and
	// AddAttributes. This design is borrowed from the slog Record type:
	// https://cs.opensource.google/go/go/+/refs/tags/go1.22.0:src/log/slog/record.go;l=20

	// Allocation optimization: an inline array sized to hold
	// the majority of log calls (based on examination of open-source
	// code). It holds the start of the list of attributes.
	front [attributesInlineCount]attribute.KeyValue

	// The number of attributes in front.
	nFront int

	// The list of attributes except for those in front.
	// Invariants:
	//   - len(back) > 0 if nFront == len(front)
	//   - Unused array elements are zero-ed. Used to detect mistakes.
	back []attribute.KeyValue

	// dropped is the count of attributes that have been dropped when limits
	// were reached.
	dropped int

	traceID    trace.TraceID
	spanID     trace.SpanID
	traceFlags trace.TraceFlags

	// resource represents the entity that collected the log.
	resource *resource.Resource

	// scope is the Scope that the Logger was created with.
	scope *instrumentation.Scope

	attributeValueLengthLimit int
	attributeCountLimit       int

	// specifies whether we should deduplicate any key value collections or not
	allowDupKeys bool

	noCmp [0]func() //nolint: unused  // This is indeed used.
}

func (r *Record) addDropped(n int) {
	r.dropped += n
	if n > 0 {
		logAttrDropped()
	}
}

// EventName returns the event name.
// A log record with non-empty event name is interpreted as an event record.
func (r *Record) EventName() string {
	return r.eventName
}

// SetEventName sets the event name.
// A log record with non-empty event name is interpreted as an event record.
func (r *Record) SetEventName(s string) {
	r.eventName = s
}

// Timestamp returns the time when the log record occurred.
func (r *Record) Timestamp() time.Time {
	return r.timestamp
}

// SetTimestamp sets the time when the log record occurred.
func (r *Record) SetTimestamp(t time.Time) {
	r.timestamp = t
}

// ObservedTimestamp returns the time when the log record was observed.
func (r *Record) ObservedTimestamp() time.Time {
	return r.observedTimestamp
}

// SetObservedTimestamp sets the time when the log record was observed.
func (r *Record) SetObservedTimestamp(t time.Time) {
	r.observedTimestamp = t
}

// Severity returns the severity of the log record.
func (r *Record) Severity() log.Severity {
	return r.severity
}

// SetSeverity sets the severity level of the log record.
func (r *Record) SetSeverity(level log.Severity) {
	r.severity = level
}

// SeverityText returns severity (also known as log level) text. This is the
// original string representation of the severity as it is known at the source.
func (r *Record) SeverityText() string {
	return r.severityText
}

// SetSeverityText sets severity (also known as log level) text. This is the
// original string representation of the severity as it is known at the source.
func (r *Record) SetSeverityText(text string) {
	r.severityText = text
}

// Body returns the body of the log record.
func (r *Record) Body() attribute.Value {
	return r.body
}

// SetBody sets the body of the log record.
func (r *Record) SetBody(v attribute.Value) {
	if !r.allowDupKeys {
		r.body, _ = attrnorm.Value(v)
	} else {
		r.body = v
	}
}

// WalkAttributes walks all attributes the log record holds by calling f for
// each on each [attribute.KeyValue] in the [Record]. Iteration stops if f returns false.
func (r *Record) WalkAttributes(f func(attribute.KeyValue) bool) {
	for i := 0; i < r.nFront; i++ {
		if !f(r.front[i]) {
			return
		}
	}
	for _, a := range r.back {
		if !f(a) {
			return
		}
	}
}

// AddAttributes adds attributes to the log record.
// Attributes in attrs will overwrite any attribute already added to r with the same key.
func (r *Record) AddAttributes(attrs ...attribute.KeyValue) {
	n := r.AttributesLen()
	if n == 0 {
		// Avoid the more complex duplicate map lookups below.
		var drop int
		if !r.allowDupKeys {
			attrs, drop = dedup(attrs)
			if drop > 0 {
				logKeyValuePairDropped()
			}
		}

		attrs, drop := r.head(attrs)
		r.addDropped(drop)

		r.addAttrs(attrs)
		return
	}

	if !r.allowDupKeys {
		// Use a slice from the pool to avoid modifying the original.
		// Note, do not iterate attrs twice by just calling dedup(attrs) here.
		unique := getUnique()
		defer putUnique(unique)

		// Used to find duplicates between attrs and existing attributes in r.
		rIndex := r.attrIndex()
		defer putIndex(rIndex)

		// Used to find duplicates within attrs itself.
		// The index value is the index of the element in unique.
		uIndex := getIndex()
		defer putIndex(uIndex)

		dropped := 0

		// Deduplicate attrs within the scope of all existing attributes.
		for _, a := range attrs {
			// Last-value-wins for any duplicates in attrs.
			idx, found := uIndex[a.Key]
			if found {
				dropped++
				logKeyValuePairDropped()
				(*unique)[idx] = a
				continue
			}

			idx, found = rIndex[a.Key]
			if found {
				// New attrs overwrite any existing with the same key.
				dropped++
				logKeyValuePairDropped()
				if idx < 0 {
					r.front[-(idx + 1)] = a
				} else {
					r.back[idx] = a
				}
			} else {
				// Unique attribute.
				(*unique) = append(*unique, a)
				uIndex[a.Key] = len(*unique) - 1
			}
		}

		if dropped > 0 {
			attrs = make([]attribute.KeyValue, len(*unique))
			copy(attrs, *unique)
		}
	}

	if r.hasAttributeCountLimit() && n+len(attrs) > r.attributeCountLimit {
		// Truncate the now unique attributes to comply with limit.
		//
		// Do not use head(attrs, r.attributeCountLimit - n) here. If
		// (r.attributeCountLimit - n) <= 0 attrs needs to be emptied.
		last := max(0, r.attributeCountLimit-n)
		r.addDropped(len(attrs) - last)
		attrs = attrs[:last]
	}

	r.addAttrs(attrs)
}

// attrIndex returns an index map for all attributes in the Record r. The index
// maps the attribute key to location the attribute is stored. If the value is
// < 0 then -(value + 1) (e.g. -1 -> 0, -2 -> 1, -3 -> 2) represents the index
// in r.nFront. Otherwise, the index is the exact index of r.back.
//
// The returned index is taken from the indexPool. It is the callers
// responsibility to return the index to that pool (putIndex) when done.
func (r *Record) attrIndex() map[attribute.Key]int {
	index := getIndex()
	for i := 0; i < r.nFront; i++ {
		key := r.front[i].Key
		index[key] = -i - 1 // stored in front: negative index.
	}
	for i := 0; i < len(r.back); i++ {
		key := r.back[i].Key
		index[key] = i // stored in back: positive index.
	}
	return index
}

// addAttrs adds attrs to the Record r. This does not validate any limits or
// duplication of attributes, these tasks are left to the caller to handle
// prior to calling.
func (r *Record) addAttrs(attrs []attribute.KeyValue) {
	var i int
	for i = 0; i < len(attrs) && r.nFront < len(r.front); i++ {
		a := attrs[i]
		r.front[r.nFront] = r.applyAttrLimitsAndDedup(a)
		r.nFront++
	}

	// Make a copy to avoid modifying the original.
	j := len(r.back)
	r.back = slices.Grow(r.back, len(attrs[i:]))
	r.back = append(r.back, attrs[i:]...)
	for i, a := range r.back[j:] {
		r.back[i+j] = r.applyAttrLimitsAndDedup(a)
	}
}

// SetAttributes sets (and overrides) attributes to the log record.
func (r *Record) SetAttributes(attrs ...attribute.KeyValue) {
	var drop int
	r.dropped = 0
	if !r.allowDupKeys {
		attrs, drop = dedup(attrs)
		if drop > 0 {
			logKeyValuePairDropped()
		}
	}

	attrs, drop = r.head(attrs)
	r.addDropped(drop)

	r.nFront = 0
	var i int
	for i = 0; i < len(attrs) && r.nFront < len(r.front); i++ {
		a := attrs[i]
		r.front[r.nFront] = r.applyAttrLimitsAndDedup(a)
		r.nFront++
	}

	r.back = slices.Clone(attrs[i:])
	for i, a := range r.back {
		r.back[i] = r.applyAttrLimitsAndDedup(a)
	}
}

// head returns the attributes r can retain along with the number dropped.
func (r *Record) head(kvs []attribute.KeyValue) (out []attribute.KeyValue, dropped int) {
	if r.attributeCountLimit < 0 || len(kvs) <= r.attributeCountLimit {
		return kvs, 0
	}
	if r.attributeCountLimit == 0 {
		return nil, len(kvs)
	}
	return kvs[:r.attributeCountLimit], len(kvs) - r.attributeCountLimit
}

func (r *Record) hasAttributeCountLimit() bool {
	return r.attributeCountLimit >= 0
}

// dedup deduplicates kvs front-to-back with the last value saved.
func dedup(kvs []attribute.KeyValue) (unique []attribute.KeyValue, dropped int) {
	if len(kvs) <= 1 {
		return kvs, 0 // No deduplication needed.
	}

	index := getIndex()
	defer putIndex(index)
	u := getUnique()
	defer putUnique(u)
	for _, a := range kvs {
		idx, found := index[a.Key]
		if found {
			dropped++
			(*u)[idx] = a
		} else {
			*u = append(*u, a)
			index[a.Key] = len(*u) - 1
		}
	}

	if dropped == 0 {
		return kvs, 0
	}

	unique = make([]attribute.KeyValue, len(*u))
	copy(unique, *u)
	return unique, dropped
}

// AttributesLen returns the number of attributes in the log record.
func (r *Record) AttributesLen() int {
	return r.nFront + len(r.back)
}

// DroppedAttributes returns the number of attributes dropped due to limits
// being reached.
func (r *Record) DroppedAttributes() int {
	return r.dropped
}

// TraceID returns the trace ID or empty array.
func (r *Record) TraceID() trace.TraceID {
	return r.traceID
}

// SetTraceID sets the trace ID.
func (r *Record) SetTraceID(id trace.TraceID) {
	r.traceID = id
}

// SpanID returns the span ID or empty array.
func (r *Record) SpanID() trace.SpanID {
	return r.spanID
}

// SetSpanID sets the span ID.
func (r *Record) SetSpanID(id trace.SpanID) {
	r.spanID = id
}

// TraceFlags returns the trace flags.
func (r *Record) TraceFlags() trace.TraceFlags {
	return r.traceFlags
}

// SetTraceFlags sets the trace flags.
func (r *Record) SetTraceFlags(flags trace.TraceFlags) {
	r.traceFlags = flags
}

// Resource returns the entity that collected the log.
func (r *Record) Resource() *resource.Resource {
	return r.resource
}

// InstrumentationScope returns the scope that the Logger was created with.
func (r *Record) InstrumentationScope() instrumentation.Scope {
	if r.scope == nil {
		return instrumentation.Scope{}
	}
	return *r.scope
}

// Clone returns a copy of the record with no shared state. The original record
// and the clone can both be modified without interfering with each other.
func (r *Record) Clone() Record {
	res := *r
	res.back = slices.Clone(r.back)
	return res
}

func (r *Record) applyAttrLimitsAndDedup(attr attribute.KeyValue) attribute.KeyValue {
	if !r.allowDupKeys {
		var changed bool
		attr, changed = attrnorm.KeyValue(attr)
		if changed {
			logKeyValuePairDropped()
		}
	}
	attr.Value = attrnorm.TruncateValue(r.attributeValueLengthLimit, attr.Value)
	return attr
}
