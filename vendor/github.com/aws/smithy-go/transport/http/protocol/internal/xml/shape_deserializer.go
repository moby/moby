package xml

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/internal/serde"
	smithytime "github.com/aws/smithy-go/time"
	"github.com/aws/smithy-go/traits"
)

type ctxKind byte

const (
	ctxKindStruct ctxKind = iota
	ctxKindList
	ctxKindMap
	ctxKindUnion
)

type deserCtx struct {
	kind      ctxKind
	schema    *smithy.Schema
	flattened bool

	startElem xml.StartElement // cached to read @xmlAttributes
	attrIndex int

	// for flattened list/map, ReadStructMember consumes the start element
	// when it discovers the member, subsequent calls to ReadMapKey or
	// ReadListItem will do that instead
	first bool

	// for map, ReadX after ReadMapKey will leave the stream at </value>
	// which the next call to ReadMapKey must consume
	inMapEntry bool
}

// ShapeDeserializer implements unmarshaling of XML into Smithy shapes.
//
// ShapeDeserializer consumes whole XML — the payload handed to
// NewShapeDeserializer must include the outermost tag that opens the shape
// being read. This is because the deserializer retains the outer
// StartElement so @xmlAttribute members can draw from its attributes.
//
// For example, an awsquery response body looks like this:
//
//	<[OperationName]Response>
//	    <[OperationName]Result>
//	        <Member1>...</Member1>
//	        <Member2>...</Member2>
//	        ...
//	        <MemberN>...</MemberN>
//	    </[OperationName]Result>
//	    <ResponseMetadata>
//	        ...
//	    </ResponseMetadata>
//	</[OperationName]Response>
//
// The deserializer must receive the Result element including its opening
// and closing tags — that is:
//
//	<[OperationName]Result>
//	    <Member1>...</Member1>
//	    ...
//	    <MemberN>...</MemberN>
//	</[OperationName]Result>
//
// The outer Response wrapper must not be included.
type ShapeDeserializer struct {
	dec    *xml.Decoder
	peeked xml.Token
	stack  serde.Stack[deserCtx]

	// most recent start element we saw in the token stream, when we go into a
	// struct context we grab it so we can read any @xmlAttributes
	currStart *xml.StartElement
	currAttr  *string
}

var _ smithy.ShapeDeserializer = (*ShapeDeserializer)(nil)

// NewShapeDeserializer returns a new ShapeDeserializer.
func NewShapeDeserializer(p []byte) *ShapeDeserializer {
	return &ShapeDeserializer{
		dec:   xml.NewDecoder(bytes.NewReader(p)),
		stack: serde.NewStack[deserCtx](),
	}
}

// NewShapeDeserializerAtElement scans payload for the first element named
// `name` and returns a ShapeDeserializer positioned to read that element as
// its opening tag. Used for protocols that have an arbitrary nesting of their
// modeled output shape in the wire response body.
//
// It forwards the underlying decoder's error, notably io.EOF, when the named
// element is not found.
func NewShapeDeserializerAtElement(payload []byte, name string) (*ShapeDeserializer, error) {
	dec := xml.NewDecoder(bytes.NewReader(payload))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}

		start, ok := tok.(xml.StartElement)
		if !ok || !strings.EqualFold(start.Name.Local, name) {
			continue
		}

		return &ShapeDeserializer{
			dec:       dec,
			currStart: &start,
			stack:     serde.NewStack[deserCtx](),
		}, nil
	}
}

func xmlMemberName(schema *smithy.Schema) string {
	if t, ok := smithy.SchemaDirectTrait[*traits.XMLName](schema); ok {
		return t.Name
	}
	return schema.MemberName()
}

func findMember(schema *smithy.Schema, elemName string) *smithy.Schema {
	if schema == nil {
		return nil
	}

	for _, m := range schema.Members() {
		mName := xmlMemberName(m)
		if strings.EqualFold(mName, elemName) {
			return m
		}
	}
	return nil
}

// ReadStruct implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadStruct(s *smithy.Schema) error {
	if err := d.ensureStart(); err != nil {
		return err
	}

	start := *d.currStart
	d.stack.Push(deserCtx{kind: ctxKindStruct, schema: s, startElem: start})
	return nil
}

// ReadStructMember implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadStructMember() (*smithy.Schema, error) {
	ctx := d.stack.Top()
	if ctx == nil || ctx.kind != ctxKindStruct {
		return nil, fmt.Errorf("ReadStructMember called without ReadStruct")
	}

	if m := d.nextAttrMember(ctx); m != nil {
		return m, nil
	}

	for {
		start, ok, err := d.nextStart()
		if err != nil {
			return nil, err
		}
		if !ok {
			d.stack.Pop()
			return nil, nil
		}

		member := findMember(ctx.schema, start.Name.Local)
		if member == nil {
			if err := d.skip(); err != nil {
				return nil, err
			}
			continue
		}

		if _, isAttr := smithy.SchemaTrait[*traits.XMLAttribute](member); isAttr {
			if err := d.skip(); err != nil {
				return nil, err
			}
			continue
		}

		return member, nil
	}
}

func (d *ShapeDeserializer) nextAttrMember(ctx *deserCtx) *smithy.Schema {
	attrs := ctx.startElem.Attr
	for ctx.attrIndex < len(attrs) {
		a := attrs[ctx.attrIndex]
		ctx.attrIndex++

		member := findAttrMember(ctx.schema, a.Name.Local)
		if member == nil {
			continue
		}

		val := a.Value
		d.currAttr = &val
		return member
	}
	return nil
}

func findAttrMember(schema *smithy.Schema, elemName string) *smithy.Schema {
	if schema == nil {
		return nil
	}

	for _, m := range schema.Members() {
		if _, ok := smithy.SchemaTrait[*traits.XMLAttribute](m); !ok {
			continue
		}
		if strings.EqualFold(stripPrefix(xmlMemberName(m)), elemName) {
			return m
		}
	}
	return nil
}

// ReadList implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadList(s *smithy.Schema) error {
	_, flattened := smithy.SchemaTrait[*traits.XMLFlattened](s)
	d.stack.Push(deserCtx{
		kind:      ctxKindList,
		schema:    s,
		flattened: flattened,
		first:     flattened,
	})
	return nil
}

// ReadListItem implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadListItem(_ *smithy.Schema) (bool, error) {
	ctx := d.stack.Top()
	if ctx.flattened {
		return d.readFlatListItem()
	}
	return d.readWrappedListItem()
}

func (d *ShapeDeserializer) readWrappedListItem() (bool, error) {
	// the old version used WrapNodeDecoder on each item and didn't check its
	// name (or for xmlName) so we're ignoring it too
	_, ok, err := d.nextStart()
	if err != nil {
		return false, err
	}
	if !ok {
		d.stack.Pop()
		return false, nil
	}

	return true, nil
}

func (d *ShapeDeserializer) readFlatListItem() (bool, error) {
	ctx := d.stack.Top()
	if ctx.first {
		ctx.first = false
		return true, nil
	}

	expectName := xmlMemberName(ctx.schema)

	for {
		tok, err := d.token()
		if err != nil {
			return false, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if strings.EqualFold(t.Name.Local, expectName) {
				return true, nil
			}

			d.peeked = t
			d.stack.Pop()
			return false, nil
		case xml.EndElement:
			d.peeked = t
			d.stack.Pop()
			return false, nil
		}
	}
}

// ReadMap implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadMap(s *smithy.Schema) error {
	_, flattened := smithy.SchemaTrait[*traits.XMLFlattened](s)
	d.stack.Push(deserCtx{
		kind:      ctxKindMap,
		schema:    s,
		flattened: flattened,
		first:     flattened,
	})
	return nil
}

// ReadMapKey implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadMapKey(ks *smithy.Schema) (string, bool, error) {
	ctx := d.stack.Top()
	if ctx.inMapEntry {
		ctx.inMapEntry = false
		if _, _, err := d.nextStart(); err != nil {
			return "", false, err
		}
	}

	vs := ctx.schema.MapValue()

	if ctx.flattened {
		return d.readFlatMapKey(ks, vs)
	}

	return d.readWrappedMapKey(ks, vs)
}

func (d *ShapeDeserializer) readWrappedMapKey(ks, vs *smithy.Schema) (string, bool, error) {
	for {
		start, a, err := d.nextStart()
		if err != nil {
			return "", false, err
		}
		if !a {
			d.stack.Pop()
			return "", false, nil
		}

		// unlike lists the old codegen actually DID check that the element was
		// named "entry", skipping if it wasn't
		if strings.EqualFold(start.Name.Local, "entry") {
			return d.readEntry(ks, vs)
		}

		if err := d.skip(); err != nil {
			return "", false, err
		}
	}
}

func (d *ShapeDeserializer) readFlatMapKey(ks, vs *smithy.Schema) (string, bool, error) {
	ctx := d.stack.Top()
	if ctx.first {
		ctx.first = false
		return d.readEntry(ks, vs)
	}

	expectName := xmlMemberName(ctx.schema)
	for {
		tok, err := d.token()
		if err != nil {
			return "", false, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if strings.EqualFold(t.Name.Local, expectName) {
				return d.readEntry(ks, vs)
			}

			d.peeked = t
			d.stack.Pop()
			return "", false, nil
		case xml.EndElement:
			d.peeked = t
			d.stack.Pop()
			return "", false, nil
		}
	}
}

func (d *ShapeDeserializer) readEntry(ks, vs *smithy.Schema) (string, bool, error) {
	ctx := d.stack.Top()

	kname := "key"
	if xn, ok := smithy.SchemaDirectTrait[*traits.XMLName](ks); ok {
		kname = xn.Name
	}

	vname := "value"
	if xn, ok := smithy.SchemaDirectTrait[*traits.XMLName](vs); ok {
		vname = xn.Name
	}

	var key string
	for {
		child, found, err := d.nextStart()
		if err != nil {
			return "", false, err
		}
		if !found {
			break
		}

		switch {
		case strings.EqualFold(child.Name.Local, kname):
			key, err = d.chardata()
			if err != nil {
				return "", false, err
			}
		case strings.EqualFold(child.Name.Local, vname):
			ctx.inMapEntry = true
			return key, true, nil
		default:
			if err := d.skip(); err != nil {
				return "", false, err
			}
		}
	}

	return key, true, nil
}

// ReadNil implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadNil(_ *smithy.Schema) (bool, error) {
	return false, nil
}

// ReadBool implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadBool(_ *smithy.Schema, v *bool) error {
	text, err := d.chardata()
	if err != nil {
		return err
	}

	b, err := strconv.ParseBool(text)
	if err != nil {
		return fmt.Errorf("parse bool %q: %w", text, err)
	}

	*v = b
	return nil
}

// ReadInt8 implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadInt8(s *smithy.Schema, v *int8) error {
	n, err := d.readInt(8)
	if err != nil {
		return err
	}

	*v = int8(n)
	return nil
}

// ReadInt16 implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadInt16(s *smithy.Schema, v *int16) error {
	n, err := d.readInt(16)
	if err != nil {
		return err
	}

	*v = int16(n)
	return nil
}

// ReadInt32 implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadInt32(s *smithy.Schema, v *int32) error {
	n, err := d.readInt(32)
	if err != nil {
		return err
	}

	*v = int32(n)
	return nil
}

// ReadInt64 implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadInt64(s *smithy.Schema, v *int64) error {
	n, err := d.readInt(64)
	if err != nil {
		return err
	}

	*v = n
	return nil
}

// ReadFloat32 implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadFloat32(s *smithy.Schema, v *float32) error {
	n, err := d.readFloat()
	if err != nil {
		return err
	}

	*v = float32(n)
	return nil
}

// ReadFloat64 implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadFloat64(s *smithy.Schema, v *float64) error {
	n, err := d.readFloat()
	if err != nil {
		return err
	}

	*v = n
	return nil
}

// ReadString implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadString(_ *smithy.Schema, v *string) error {
	text, err := d.chardata()
	if err != nil {
		return err
	}

	*v = text
	return nil
}

// ReadTime implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadTime(schema *smithy.Schema, v *time.Time) error {
	format := "date-time"
	if t, ok := smithy.SchemaTrait[*traits.TimestampFormat](schema); ok {
		format = t.Format
	}

	text, err := d.chardata()
	if err != nil {
		return err
	}

	switch format {
	case "date-time":
		t, err := smithytime.ParseDateTime(text)
		if err != nil {
			return err
		}
		*v = t
	case "http-date":
		t, err := smithytime.ParseHTTPDate(text)
		if err != nil {
			return err
		}
		*v = t
	case "epoch-seconds":
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return err
		}
		*v = smithytime.ParseEpochSeconds(n)
	default:
		return fmt.Errorf("unknown timestamp format: %s", format)
	}

	return nil
}

// ReadBlob implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadBlob(s *smithy.Schema, v *[]byte) error {
	text, err := d.chardata()
	if err != nil {
		return err
	}

	if text == "" {
		return nil
	}

	b, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return fmt.Errorf("decode base64 blob: %w", err)
	}
	*v = b
	return nil
}

// ReadUnion implements [smithy.ShapeDeserializer].
func (d *ShapeDeserializer) ReadUnion(s *smithy.Schema) (*smithy.Schema, error) {
	if err := d.ensureStart(); err != nil {
		return nil, err
	}

	for {
		start, ok, err := d.nextStart()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}

		member := findMember(s, start.Name.Local)
		if member == nil {
			if err := d.skip(); err != nil {
				return nil, err
			}
			continue
		}

		return member, nil
	}
}

// ReadDocument is unimplemented for XML.
func (d *ShapeDeserializer) ReadDocument(_ *smithy.Schema, _ *document.Value) error {
	return fmt.Errorf("Document not supported")
}

func (d *ShapeDeserializer) token() (xml.Token, error) {
	if d.peeked != nil {
		tok := d.peeked
		d.peeked = nil
		return tok, nil
	}

	tok, err := d.dec.Token()
	if err != nil {
		return nil, err
	}

	if start, ok := tok.(xml.StartElement); ok {
		d.currStart = &start
	}

	return tok, nil
}

func (d *ShapeDeserializer) chardata() (string, error) {
	if d.currAttr != nil {
		v := *d.currAttr
		d.currAttr = nil
		return v, nil
	}

	// a single "inner XML" node can be multiple xml.CharData so we need to
	// accumulate them
	var buf strings.Builder

	for {
		tok, err := d.token()
		if err != nil {
			return "", err
		}

		switch t := tok.(type) {
		case xml.CharData:
			buf.Write(t)

		// IMPORTANT: also consumes the closing tag AFTER the chardata, so
		// future ReadWhatevers don't have to think about that
		case xml.EndElement:
			return buf.String(), nil

		default:
			return "", fmt.Errorf("unexpected token %T", tok)
		}
	}
}

// there is xml.Decoder.Skip but this is a special case to assume we already
// consumed the start element and to handle any peeked tokens
func (d *ShapeDeserializer) skip() error {
	depth := 1
	for depth > 0 {
		tok, err := d.token()
		if err != nil {
			return err
		}

		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}

	return nil
}

func (d *ShapeDeserializer) nextStart() (xml.StartElement, bool, error) {
	for {
		tok, err := d.token()
		if err != nil {
			return xml.StartElement{}, false, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			return t, true, nil
		case xml.EndElement: // ie. the end of the struct/list/map
			return xml.StartElement{}, false, nil
		default:
			continue
		}
	}
}

// make sure we have the start element when we go into a struct/union context
func (d *ShapeDeserializer) ensureStart() error {
	if d.currStart == nil {
		if _, ok, err := d.nextStart(); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("expected start element")
		}
	}
	return nil
}

func (d *ShapeDeserializer) readInt(bits int) (int64, error) {
	text, err := d.chardata()
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(text, 10, bits)
}

func (d *ShapeDeserializer) readFloat() (float64, error) {
	text, err := d.chardata()
	if err != nil {
		return 0, err
	}

	switch {
	case strings.EqualFold(text, "NaN"):
		return math.NaN(), nil
	case strings.EqualFold(text, "Infinity"):
		return math.Inf(1), nil
	case strings.EqualFold(text, "-Infinity"):
		return math.Inf(-1), nil
	default:
		return strconv.ParseFloat(text, 64)
	}
}

func stripPrefix(name string) string {
	if _, after, ok := strings.Cut(name, ":"); ok {
		return after
	}
	return name
}

// ReadBigInt is unimplemented and will return an error.
func (d *ShapeDeserializer) ReadBigInt(_ *smithy.Schema, _ *big.Int) error {
	return fmt.Errorf("unimplemented")
}

// ReadBigFloat is unimplemented and will return an error.
func (d *ShapeDeserializer) ReadBigFloat(_ *smithy.Schema, _ *big.Float) error {
	return fmt.Errorf("unimplemented")
}
