package xml

import (
	"encoding/base64"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/internal/serde"
	smithytime "github.com/aws/smithy-go/time"
	"github.com/aws/smithy-go/traits"
)

type serCtx struct {
	kind   ctxKind
	schema *smithy.Schema
	flat   bool

	wrapperName string // open/close ename for struct and NON-flat list/map
	itemName    string // per-item element name for list items and map _entries_
	inMapEntry  bool

	// we have to buffer inner xml for structs because they may have
	// @xmlAttribute members written in any order
	//
	// @xmlNamespace is just resolved at flush time
	w     *writer
	attrs []attr
}

// ShapeSerializer implements marshaling of Smithy shapes to XML.
type ShapeSerializer struct {
	w *writer

	stack serde.Stack[serCtx]

	opts ShapeSerializerOptions
}

// ShapeSerializerOptions configures ShapeSerializer.
type ShapeSerializerOptions struct {
	RootNamespaceURI    string
	RootNamespacePrefix string
}

var _ smithy.ShapeSerializer = (*ShapeSerializer)(nil)

// NewShapeSerializer creates a new ShapeSerializer.
func NewShapeSerializer(opts ...func(*ShapeSerializerOptions)) *ShapeSerializer {
	o := ShapeSerializerOptions{}
	for _, fn := range opts {
		fn(&o)
	}
	return &ShapeSerializer{
		w:     newWriter(),
		stack: serde.NewStack[serCtx](),
		opts:  o,
	}
}

// Bytes returns the serialized XML bytes.
func (s *ShapeSerializer) Bytes() []byte {
	return s.w.Bytes()
}

// WriteInt8 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteInt8(schema *smithy.Schema, v int8) {
	s.writeScalar(schema, strconv.FormatInt(int64(v), 10))
}

// WriteInt16 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteInt16(schema *smithy.Schema, v int16) {
	s.writeScalar(schema, strconv.FormatInt(int64(v), 10))
}

// WriteInt32 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteInt32(schema *smithy.Schema, v int32) {
	s.writeScalar(schema, strconv.FormatInt(int64(v), 10))
}

// WriteInt64 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteInt64(schema *smithy.Schema, v int64) {
	s.writeScalar(schema, strconv.FormatInt(v, 10))
}

// WriteFloat32 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteFloat32(schema *smithy.Schema, v float32) {
	s.writeScalar(schema, formatFloat(float64(v), 32))
}

// WriteFloat64 implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteFloat64(schema *smithy.Schema, v float64) {
	s.writeScalar(schema, formatFloat(v, 64))
}

// WriteBool implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteBool(schema *smithy.Schema, v bool) {
	s.writeScalar(schema, strconv.FormatBool(v))
}

// WriteString implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteString(schema *smithy.Schema, v string) {
	s.writeScalar(schema, v)
}

// WriteBigInt is unimplemented.
func (s *ShapeSerializer) WriteBigInt(_ *smithy.Schema, _ *big.Int) {
	panic("BigInteger not supported")
}

// WriteBigFloat is unimplemented.
func (s *ShapeSerializer) WriteBigFloat(_ *smithy.Schema, _ *big.Float) {
	panic("BigDecimal not supported")
}

// WriteBlob implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteBlob(schema *smithy.Schema, v []byte) {
	if v == nil {
		return
	}
	s.writeScalar(schema, base64.StdEncoding.EncodeToString(v))
}

// WriteTime implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteTime(schema *smithy.Schema, v time.Time) {
	format := "date-time"
	if t, ok := smithy.SchemaTrait[*traits.TimestampFormat](schema); ok {
		format = t.Format
	}

	switch format {
	case "http-date":
		s.writeScalar(schema, smithytime.FormatHTTPDate(v))
	case "epoch-seconds":
		s.writeScalar(schema, strconv.FormatFloat(smithytime.FormatEpochSeconds(v), 'f', -1, 64))
	default:
		s.writeScalar(schema, smithytime.FormatDateTime(v))
	}
}

// WriteStruct implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteStruct(schema *smithy.Schema) {
	name := s.structEname(schema)

	// @xmlNamespace on a target structure does NOT propagate through member
	// references (the spec says nothing about inheritance, and the protocol
	// tests confirm: XmlNamespaceNested has @xmlNamespace but the <nested>
	// element does not get it). The exception is @httpPayload, where the
	// member and its target represent the same XML element.

	ctx := serCtx{
		kind:        ctxKindStruct,
		wrapperName: name,
		schema:      schema,
		w:           s.w,
	}
	s.w = newWriter()
	s.stack.Push(ctx)
}

// CloseStruct implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) CloseStruct() {
	ctx := s.stack.Pop()

	var ns *traits.XMLNamespace
	if isPayload(ctx.schema) {
		ns, _ = smithy.SchemaTrait[*traits.XMLNamespace](ctx.schema)
	} else {
		ns, _ = smithy.SchemaDirectTrait[*traits.XMLNamespace](ctx.schema)
	}

	// special case for the root struct where the service set a namespace
	if ns == nil && s.stack.Len() == 0 && s.opts.RootNamespaceURI != "" {
		ns = &traits.XMLNamespace{
			URI:    s.opts.RootNamespaceURI,
			Prefix: s.opts.RootNamespacePrefix,
		}
	}

	inner := s.w
	s.w = ctx.w
	s.w.writeStart(ctx.wrapperName, ns, ctx.attrs)
	s.w.writeInner(inner)
	s.w.writeEnd(ctx.wrapperName)
}

// WriteUnion implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteUnion(schema, variant *smithy.Schema) {
	name := s.ctxEname(schema)
	s.w.writeStart(name, s.xmlns(schema), nil)

	// the variant must not resolve its element name from an enclosing list or
	// map, which would name it as another item
	s.stack.Push(serCtx{
		kind:        ctxKindUnion,
		wrapperName: name,
		schema:      schema,
	})
}

// CloseUnion implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) CloseUnion() {
	ctx := s.stack.Pop()
	s.w.writeEnd(ctx.wrapperName)
}

// WriteNil implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteNil(schema *smithy.Schema) {
}

// WriteList implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteList(schema *smithy.Schema) {
	_, flat := smithy.SchemaTrait[*traits.XMLFlattened](schema)

	ename := s.ename(schema)
	iname := ename
	if !flat {
		ns, _ := smithy.SchemaDirectTrait[*traits.XMLNamespace](schema)
		s.w.writeStart(ename, ns, nil)
		iname = s.ename(schema.ListMember())
	} else {
		ename = ""
	}

	s.stack.Push(serCtx{
		kind:        ctxKindList,
		wrapperName: ename,
		itemName:    iname,
		schema:      schema,
		flat:        flat,
	})
}

// CloseList implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) CloseList() {
	ctx := s.stack.Pop()
	if !ctx.flat {
		s.w.writeEnd(ctx.wrapperName)
	}
}

// WriteMap implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteMap(schema *smithy.Schema) {
	_, flattened := smithy.SchemaTrait[*traits.XMLFlattened](schema)

	wrapperName := s.ename(schema)
	itemName := "entry"
	if flattened {
		itemName = wrapperName
		wrapperName = ""
	} else {
		ns, _ := smithy.SchemaDirectTrait[*traits.XMLNamespace](schema)
		s.w.writeStart(wrapperName, ns, nil)
	}

	s.stack.Push(serCtx{
		kind:        ctxKindMap,
		wrapperName: wrapperName,
		itemName:    itemName,
		schema:      schema,
		flat:        flattened,
	})
}

// WriteKey implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) WriteKey(schema *smithy.Schema, k string) {
	top := s.stack.Top()
	if top == nil || top.kind != ctxKindMap {
		return
	}

	if top.inMapEntry {
		s.w.writeEnd(top.itemName)
		top.inMapEntry = false
	}

	s.w.writeStart(top.itemName, nil, nil)

	keyName := s.ename(schema)
	ns, _ := smithy.SchemaDirectTrait[*traits.XMLNamespace](top.schema.MapKey())
	s.w.writeStart(keyName, ns, nil)
	s.w.writeChardata(k)
	s.w.writeEnd(keyName)

	top.inMapEntry = true
}

// CloseMap implements [smithy.ShapeSerializer].
func (s *ShapeSerializer) CloseMap() {
	ctx := s.stack.Pop()

	if ctx.inMapEntry {
		s.w.writeEnd(ctx.itemName)
	}

	if ctx.wrapperName != "" {
		s.w.writeEnd(ctx.wrapperName)
	}
}

// WriteDocument is unimplemented for XML.
func (s *ShapeSerializer) WriteDocument(schema *smithy.Schema, v document.Value) {
	panic("WriteDocument not supported for XML")
}

func (s *ShapeSerializer) writeScalar(schema *smithy.Schema, v string) {
	if s.bufferAttribute(schema, v) {
		return
	}

	name := s.ctxEname(schema)
	s.w.writeStart(name, s.xmlns(schema), nil)
	s.w.writeChardata(v)
	s.w.writeEnd(name)
}

func (s *ShapeSerializer) bufferAttribute(schema *smithy.Schema, v string) bool {
	if _, ok := smithy.SchemaTrait[*traits.XMLAttribute](schema); !ok {
		return false
	}

	top := s.stack.Top()
	if top == nil || top.kind != ctxKindStruct {
		return false
	}

	top.attrs = append(top.attrs, attr{
		name:  s.ename(schema),
		value: v,
	})
	return true
}

func formatFloat(v float64, bits int) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	}
	return strconv.FormatFloat(v, 'g', -1, bits)
}

func isPayload(schema *smithy.Schema) bool {
	_, ok := smithy.SchemaTrait[*traits.HTTPPayload](schema)
	return ok
}
