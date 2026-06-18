package eventstream

import (
	"fmt"
	"math/big"
	"time"

	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/traits"
)

// ShapeDeserializer wraps a [smithy.ShapeDeserializer] to handle event stream
// message binding traits.
type ShapeDeserializer struct {
	Message *Message

	inner smithy.ShapeDeserializer

	depth  int
	schema *smithy.Schema

	bindings   []*smithy.Schema
	bindIdx    int
	inBindings bool

	inBody     bool
	hasPayload bool
	hasBody    bool
}

var _ smithy.ShapeDeserializer = (*ShapeDeserializer)(nil)

// NewShapeDeserializer returns a deserializer for a Message.
func NewShapeDeserializer(msg *Message, inner smithy.ShapeDeserializer) *ShapeDeserializer {
	return &ShapeDeserializer{
		Message: msg,
		inner:   inner,
	}
}

func (d *ShapeDeserializer) ReadStruct(s *smithy.Schema) error {
	d.depth++
	if d.depth > 1 {
		return d.inner.ReadStruct(s)
	}
	d.schema = s
	for _, m := range s.Members() {
		if _, ok := smithy.SchemaTrait[*traits.EventPayload](m); ok {
			d.hasPayload = true
		}
		if isEventBound(m) {
			d.bindings = append(d.bindings, m)
		} else {
			d.hasBody = true
		}
	}
	return nil
}

func (d *ShapeDeserializer) ReadStructMember() (*smithy.Schema, error) {
	if d.depth > 1 {
		ms, err := d.inner.ReadStructMember()
		if ms == nil {
			d.depth--
		}
		return ms, err
	}

	// like httpbinding, throw back the bound stuff first before we drop into
	// the body
	for d.bindIdx < len(d.bindings) {
		m := d.bindings[d.bindIdx]
		d.bindIdx++
		if isEventHeader(m) && d.Message.Headers.Get(m.MemberName()) == nil {
			continue
		}
		d.inBindings = true
		return m, nil
	}
	d.inBindings = false

	if d.hasPayload {
		d.depth--
		return nil, nil
	}

	if !d.hasBody {
		d.depth--
		return nil, nil
	}

	if !d.inBody {
		d.inBody = true
		if err := d.inner.ReadStruct(d.schema); err != nil {
			return nil, err
		}
	}

	ms, err := d.inner.ReadStructMember()
	if ms == nil {
		d.depth--
	}

	return ms, err
}

func (d *ShapeDeserializer) ReadString(s *smithy.Schema, v *string) error {
	if d.inBindings {
		if isEventHeader(s) {
			hv := d.Message.Headers.Get(s.MemberName())
			if hv == nil {
				return nil
			}
			sv, ok := hv.(StringValue)
			if !ok {
				return fmt.Errorf("event header %q: expected string, got %T", s.MemberName(), hv)
			}
			*v = string(sv)
			return nil
		}
		if isEventPayload(s) {
			*v = string(d.Message.Payload)
			return nil
		}
	}
	return d.inner.ReadString(s, v)
}

func (d *ShapeDeserializer) ReadBool(s *smithy.Schema, v *bool) error {
	if d.inBindings && isEventHeader(s) {
		hv := d.Message.Headers.Get(s.MemberName())
		if hv == nil {
			return nil
		}
		bv, ok := hv.(BoolValue)
		if !ok {
			return fmt.Errorf("event header %q: expected bool, got %T", s.MemberName(), hv)
		}
		*v = bool(bv)
		return nil
	}
	return d.inner.ReadBool(s, v)
}

func (d *ShapeDeserializer) readHeaderInt64(name string) (int64, bool, error) {
	hv := d.Message.Headers.Get(name)
	if hv == nil {
		return 0, false, nil
	}
	switch v := hv.(type) {
	case Int8Value:
		return int64(v), true, nil
	case Int16Value:
		return int64(v), true, nil
	case Int32Value:
		return int64(v), true, nil
	case Int64Value:
		return int64(v), true, nil
	default:
		return 0, false, fmt.Errorf("event header %q: expected integer, got %T", name, hv)
	}
}

type intn interface {
	int8 | int16 | int32 | int64
}

func readEventHeaderInt[T intn](d *ShapeDeserializer, s *smithy.Schema, v *T) error {
	n, ok, err := d.readHeaderInt64(s.MemberName())
	if err != nil || !ok {
		return err
	}
	*v = T(n)
	return nil
}

func (d *ShapeDeserializer) ReadInt8(s *smithy.Schema, v *int8) error {
	if d.inBindings && isEventHeader(s) {
		return readEventHeaderInt(d, s, v)
	}
	return d.inner.ReadInt8(s, v)
}

func (d *ShapeDeserializer) ReadInt16(s *smithy.Schema, v *int16) error {
	if d.inBindings && isEventHeader(s) {
		return readEventHeaderInt(d, s, v)
	}
	return d.inner.ReadInt16(s, v)
}

func (d *ShapeDeserializer) ReadInt32(s *smithy.Schema, v *int32) error {
	if d.inBindings && isEventHeader(s) {
		return readEventHeaderInt(d, s, v)
	}
	return d.inner.ReadInt32(s, v)
}

func (d *ShapeDeserializer) ReadInt64(s *smithy.Schema, v *int64) error {
	if d.inBindings && isEventHeader(s) {
		return readEventHeaderInt(d, s, v)
	}
	return d.inner.ReadInt64(s, v)
}

func (d *ShapeDeserializer) ReadFloat32(s *smithy.Schema, v *float32) error {
	return d.inner.ReadFloat32(s, v)
}

func (d *ShapeDeserializer) ReadFloat64(s *smithy.Schema, v *float64) error {
	return d.inner.ReadFloat64(s, v)
}

func (d *ShapeDeserializer) ReadBlob(s *smithy.Schema, v *[]byte) error {
	if d.inBindings {
		if isEventHeader(s) {
			hv := d.Message.Headers.Get(s.MemberName())
			if hv == nil {
				return nil
			}
			bv, ok := hv.(BytesValue)
			if !ok {
				return fmt.Errorf("event header %q: expected bytes, got %T", s.MemberName(), hv)
			}
			*v = []byte(bv)
			return nil
		}
		if isEventPayload(s) {
			*v = d.Message.Payload
			return nil
		}
	}
	return d.inner.ReadBlob(s, v)
}

func (d *ShapeDeserializer) ReadTime(s *smithy.Schema, v *time.Time) error {
	if d.inBindings && isEventHeader(s) {
		hv := d.Message.Headers.Get(s.MemberName())
		if hv == nil {
			return nil
		}
		tv, ok := hv.(TimestampValue)
		if !ok {
			return fmt.Errorf("event header %q: expected timestamp, got %T", s.MemberName(), hv)
		}
		*v = time.Time(tv)
		return nil
	}
	return d.inner.ReadTime(s, v)
}

func (d *ShapeDeserializer) ReadList(s *smithy.Schema) error {
	return d.inner.ReadList(s)
}

func (d *ShapeDeserializer) ReadListItem(s *smithy.Schema) (bool, error) {
	return d.inner.ReadListItem(s)
}

func (d *ShapeDeserializer) ReadMap(s *smithy.Schema) error {
	return d.inner.ReadMap(s)
}

func (d *ShapeDeserializer) ReadMapKey(s *smithy.Schema) (string, bool, error) {
	return d.inner.ReadMapKey(s)
}

func (d *ShapeDeserializer) ReadUnion(s *smithy.Schema) (*smithy.Schema, error) {
	return d.inner.ReadUnion(s)
}

func (d *ShapeDeserializer) ReadNil(s *smithy.Schema) (bool, error) {
	return d.inner.ReadNil(s)
}

func (d *ShapeDeserializer) ReadDocument(s *smithy.Schema, v *document.Value) error {
	return d.inner.ReadDocument(s, v)
}

func isEventBound(schema *smithy.Schema) bool {
	_, h := smithy.SchemaTrait[*traits.EventHeader](schema)
	_, p := smithy.SchemaTrait[*traits.EventPayload](schema)
	return h || p
}

// ReadBigInt is unimplemented and will return an error.
func (d *ShapeDeserializer) ReadBigInt(_ *smithy.Schema, _ *big.Int) error {
	return fmt.Errorf("unimplemented")
}

// ReadBigFloat is unimplemented and will return an error.
func (d *ShapeDeserializer) ReadBigFloat(_ *smithy.Schema, _ *big.Float) error {
	return fmt.Errorf("unimplemented")
}
