package eventstream

import (
	"math/big"
	"time"

	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/traits"
)

// ShapeSerializer wraps a [smithy.ShapeSerializer], much like the internal
// httpbinding serializer, to handle event stream message binding traits.
type ShapeSerializer struct {
	Message *Message

	inner       smithy.ShapeSerializer
	contentType string // may be inflenced by bindings
	depth       int
	hasBody     bool
}

var _ smithy.ShapeSerializer = (*ShapeSerializer)(nil)

// NewShapeSerializer returns a serializer for a single Message.
func NewShapeSerializer(msg *Message, inner smithy.ShapeSerializer) *ShapeSerializer {
	return &ShapeSerializer{
		Message: msg,
		inner:   inner,
	}
}

// ContentType returns the resolved content type for the event message payload
// after serialization, which may be affected by bindings.
func (s *ShapeSerializer) ContentType() string {
	return s.contentType
}

// Bytes returns the serialized body bytes.
func (s *ShapeSerializer) Bytes() []byte {
	return s.inner.Bytes()
}

// WriteBool implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteBool(schema *smithy.Schema, v bool) {
	if isEventHeader(schema) {
		s.Message.Headers.Set(schema.MemberName(), BoolValue(v))
		return
	}
	s.inner.WriteBool(schema, v)
}

// WriteInt8 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteInt8(schema *smithy.Schema, v int8) {
	if isEventHeader(schema) {
		s.Message.Headers.Set(schema.MemberName(), Int8Value(v))
		return
	}
	s.inner.WriteInt8(schema, v)
}

// WriteInt16 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteInt16(schema *smithy.Schema, v int16) {
	if isEventHeader(schema) {
		s.Message.Headers.Set(schema.MemberName(), Int16Value(v))
		return
	}
	s.inner.WriteInt16(schema, v)
}

// WriteInt32 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteInt32(schema *smithy.Schema, v int32) {
	if isEventHeader(schema) {
		s.Message.Headers.Set(schema.MemberName(), Int32Value(v))
		return
	}
	s.inner.WriteInt32(schema, v)
}

// WriteInt64 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteInt64(schema *smithy.Schema, v int64) {
	if isEventHeader(schema) {
		s.Message.Headers.Set(schema.MemberName(), Int64Value(v))
		return
	}
	s.inner.WriteInt64(schema, v)
}

// WriteFloat32 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteFloat32(schema *smithy.Schema, v float32) {
	s.inner.WriteFloat32(schema, v)
}

// WriteFloat64 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteFloat64(schema *smithy.Schema, v float64) {
	s.inner.WriteFloat64(schema, v)
}

// WriteString implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteString(schema *smithy.Schema, v string) {
	if isEventHeader(schema) {
		s.Message.Headers.Set(schema.MemberName(), StringValue(v))
		return
	}
	if isEventPayload(schema) {
		s.Message.Payload = []byte(v)
		s.contentType = "text/plain"
		return
	}
	s.inner.WriteString(schema, v)
}

// WriteBlob implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteBlob(schema *smithy.Schema, v []byte) {
	if isEventHeader(schema) {
		s.Message.Headers.Set(schema.MemberName(), BytesValue(v))
		return
	}
	if isEventPayload(schema) {
		s.Message.Payload = v
		s.contentType = "application/octet-stream"
		return
	}
	s.inner.WriteBlob(schema, v)
}

// WriteTime implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteTime(schema *smithy.Schema, v time.Time) {
	if isEventHeader(schema) {
		s.Message.Headers.Set(schema.MemberName(), TimestampValue(v))
		return
	}
	s.inner.WriteTime(schema, v)
}

// WriteBigInt implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteBigInt(schema *smithy.Schema, v *big.Int) {
	s.inner.WriteBigInt(schema, v)
}

// WriteBigFloat implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteBigFloat(schema *smithy.Schema, v *big.Float) {
	s.inner.WriteBigFloat(schema, v)
}

// WriteStruct implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteStruct(schema *smithy.Schema) {
	s.depth++
	if s.depth > 1 {
		s.inner.WriteStruct(schema)
		return
	}
	// At depth 1 (the event struct itself), start a JSON body if there are
	// implicit body members (members without @eventHeader or @eventPayload).
	for _, m := range schema.Members() {
		if !isEventBound(m) {
			s.inner.WriteStruct(schema)
			s.hasBody = true
			return
		}
	}
}

// CloseStruct implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) CloseStruct() {
	if s.depth > 1 || s.hasBody {
		s.inner.CloseStruct()
	}
	if s.depth == 1 {
		s.hasBody = false
	}
	s.depth--
}

// WriteUnion implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteUnion(schema, variant *smithy.Schema) {
	s.inner.WriteUnion(schema, variant)
}

// CloseUnion implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) CloseUnion() {
	s.inner.CloseUnion()
}

// WriteNil implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteNil(schema *smithy.Schema) {
	s.inner.WriteNil(schema)
}

// WriteList implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteList(schema *smithy.Schema) {
	s.inner.WriteList(schema)
}

// CloseList implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) CloseList() {
	s.inner.CloseList()
}

// WriteMap implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteMap(schema *smithy.Schema) {
	s.inner.WriteMap(schema)
}

// WriteKey implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteKey(schema *smithy.Schema, key string) {
	s.inner.WriteKey(schema, key)
}

// CloseMap implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) CloseMap() {
	s.inner.CloseMap()
}

// WriteDocument implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteDocument(schema *smithy.Schema, v document.Value) {
	s.inner.WriteDocument(schema, v)
}

func isEventHeader(schema *smithy.Schema) bool {
	_, ok := smithy.SchemaTrait[*traits.EventHeader](schema)
	return ok
}

func isEventPayload(schema *smithy.Schema) bool {
	_, ok := smithy.SchemaTrait[*traits.EventPayload](schema)
	return ok
}
