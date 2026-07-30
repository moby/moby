package toml

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2/internal/parserbridge"
	"github.com/pelletier/go-toml/v2/unstable"
)

// Marshal serializes a Go value as a TOML document.
//
// It is a shortcut for Encoder.Encode() with the default options.
func Marshal(v interface{}) ([]byte, error) {
	enc := Encoder{indentSymbol: "  "}

	e := encoderStatePool.Get().(*encoderState)
	e.Encoder = &enc
	e.marshalerOn = enc.marshalerInterface
	e.buf = e.buf[:0]
	e.keyStack = e.keyStack[:0]
	e.lastWasHeader = false

	err := e.encodeRoot(v)
	if err != nil {
		encoderStatePool.Put(e)
		return nil, err
	}

	out := make([]byte, len(e.buf))
	copy(out, e.buf)
	encoderStatePool.Put(e)
	return out, nil
}

// Encoder writes a TOML document to an output stream.
type Encoder struct {
	// output
	w io.Writer

	// global settings
	tablesInline       bool
	arraysMultiline    bool
	indentSymbol       string
	indentTables       bool
	marshalJSONNumbers bool

	// toggles the unstable.Marshaler interface
	marshalerInterface bool
}

// NewEncoder returns a new Encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:            w,
		indentSymbol: "  ",
	}
}

// SetTablesInline forces the encoder to emit all tables inline.
//
// This behavior can be controlled on an individual struct field basis with
// the inline tag:
//
//	MyField `toml:",inline"`
func (enc *Encoder) SetTablesInline(inline bool) *Encoder {
	enc.tablesInline = inline
	return enc
}

// SetArraysMultiline forces the encoder to emit all arrays with one element
// per line.
//
// This behavior can be controlled on an individual struct field basis with
// the multiline tag:
//
//	MyField `multiline:"true"`
func (enc *Encoder) SetArraysMultiline(multiline bool) *Encoder {
	enc.arraysMultiline = multiline
	return enc
}

// SetIndentSymbol defines the string that should be used for indentation. The
// provided string is repeated for each indentation level. Defaults to two
// spaces.
func (enc *Encoder) SetIndentSymbol(s string) *Encoder {
	enc.indentSymbol = s
	return enc
}

// SetIndentTables forces the encoder to intent tables and array tables.
func (enc *Encoder) SetIndentTables(indent bool) *Encoder {
	enc.indentTables = indent
	return enc
}

// SetMarshalJSONNumbers forces the encoder to serialize `json.Number` as a
// float or integer instead of relying on TextMarshaler to emit a string.
//
// *Unstable:* This method does not follow the compatibility guarantees of
// semver. It can be changed or removed without a new major version being
// issued.
func (enc *Encoder) SetMarshalJSONNumbers(indent bool) *Encoder {
	enc.marshalJSONNumbers = indent
	return enc
}

// EnableMarshalerInterface enables the unstable.Marshaler interface.
//
// With this feature enabled, types implementing the unstable.Marshaler
// interface emit their own raw TOML instead of being encoded structurally. It
// is the encoding counterpart of Decoder.EnableUnmarshalerInterface, and allows
// types such as unstable.RawMessage to round-trip raw TOML bytes.
//
// The bytes returned by MarshalTOML are spliced into the document verbatim. The
// encoder parses them to decide between the valid positions:
//   - bytes forming a single value are emitted inline, as in `key = <raw>`;
//   - bytes forming key-value lines are emitted as the body of a `[key]` table;
//   - at the document root, the bytes are emitted as the whole document: the
//     encode counterpart of the decoder delivering the whole document to a
//     root unstable.Unmarshaler.
//
// An empty result is omitted from the output. Bytes that are not valid TOML
// for their position result in an error, as do bytes forming table content in
// a position where only a value is valid (an array element, an inline table,
// or a table forced inline). MarshalTOML can be called more than once for the
// same value during a single encode, so it must be deterministic.
//
// *Unstable:* This method does not follow the compatibility guarantees of
// semver. It can be changed or removed without a new major version being
// issued.
func (enc *Encoder) EnableMarshalerInterface() *Encoder {
	enc.marshalerInterface = true
	return enc
}

// Encode writes a TOML representation of v to the stream.
//
// If v cannot be represented to TOML it returns an error.
//
// # Encoding rules
//
// A top level slice containing only maps or structs is encoded as [[table
// array]].
//
// All slices not matching rule 1 are encoded as [array]. As a result, any map
// or struct they contain is encoded as an {inline table}.
//
// Nil interfaces and nil pointers are not supported.
//
// Keys in key-values always have one part.
//
// Intermediate tables are always printed.
//
// By default, strings are encoded as literal string, unless they contain
// either a newline character or a single quote. In that case they are emitted
// as quoted strings.
//
// Unsigned integers larger than math.MaxInt64 cannot be encoded. Doing so
// results in an error. This rule exists because the TOML specification only
// requires parsers to support at least the 64 bits integer range. Allowing
// larger numbers would create non-standard TOML documents, which may not be
// readable (at best) by other implementations. To encode such numbers, a
// solution is a custom type that implements encoding.TextMarshaler.
//
// When encoding structs, fields are encoded in order of definition, with
// their exact name.
//
// Tables and array tables are separated by empty lines. However, consecutive
// subtables definitions are not. For example:
//
//	[top1]
//
//	[top2]
//	[top2.child1]
//
//	[[array]]
//
//	[[array]]
//	[array.child2]
//
// # Struct tags
//
// The encoding of each public struct field can be customized by the format
// string in the "toml" key of the struct field's tag. This follows
// encoding/json's convention. The format string starts with the name of the
// field, optionally followed by a comma-separated list of options. The name
// may be empty in order to provide options without overriding the default
// name.
//
// The "multiline" option emits strings as quoted multi-line TOML strings, and
// arrays with one element per line. For strings, it only takes effect when the
// value contains a newline; single-line values are emitted as regular strings.
// It has no effect on fields that would not be encoded as strings or arrays.
//
// The "inline" option turns fields that would be emitted as tables into
// inline tables instead. It has no effect on other fields.
//
// The "omitempty" option prevents empty values or groups from being emitted.
//
// The "omitzero" option prevents zero values or groups from being emitted.
//
// The "commented" option prefixes the value and all its children with a
// comment symbol.
//
// In addition to the "toml" tag struct tag, a "comment" tag can be used to
// emit a TOML comment before the value being annotated. Comments are ignored
// inside inline tables. For array tables, the comment is only present before
// the first element of the array.
func (enc *Encoder) Encode(v interface{}) error {
	e := encoderStatePool.Get().(*encoderState)
	e.Encoder = enc
	e.marshalerOn = enc.marshalerInterface
	e.buf = e.buf[:0]
	e.keyStack = e.keyStack[:0]
	e.lastWasHeader = false

	err := e.encodeRoot(v)
	if err != nil {
		encoderStatePool.Put(e)
		return err
	}

	_, err = enc.w.Write(e.buf)
	encoderStatePool.Put(e)
	if err != nil {
		return fmt.Errorf("toml: cannot write: %w", err)
	}
	return nil
}

var encoderStatePool = sync.Pool{
	New: func() interface{} { return &encoderState{} },
}

type encoderState struct {
	*Encoder

	buf []byte

	// keyStack is the dotted key of the table being encoded, shared by the
	// whole encode as a stack.
	keyStack []string

	// entriesPool recycles entry slices across tables of the same encode.
	entriesPool [][]entry

	// lastWasHeader is true when the last line written was a table header,
	// used to avoid empty lines between consecutive table definitions.
	lastWasHeader bool

	// stringKeyBuf is a reusable buffer to read string map keys without
	// allocating one per map.
	stringKeyBuf reflect.Value

	// parser classifies the bytes returned by unstable.Marshaler values as a
	// single value or a table body. Only used when marshalerOn is set.
	parser unstable.Parser

	// marshalerOn mirrors Encoder.marshalerInterface, copied onto the state so
	// the per-value hot path checks a direct field instead of dereferencing the
	// embedded *Encoder on every value.
	marshalerOn bool
}

// valueOptions are the encoding options attached to one entry of a table.
type valueOptions struct {
	multiline bool
	inline    bool
	omitempty bool
	omitzero  bool
	commented bool
	// rawShape is the unstable.Marshaler classification of the entry's value,
	// resolved at encode time (not from tags). It lives here, in the byte of
	// padding the booleans already leave, so that entry does not grow and the
	// encoder's default (interface-disabled) path keeps its exact layout and
	// performance. It stays shapeUnknown for everything that is not a Marshaler.
	rawShape rawShape
	comment  string
}

// entry is a deferred key-value of a table being encoded.
type entry struct {
	key     string
	value   reflect.Value
	options valueOptions
}

// rawShape classifies the bytes produced by an unstable.Marshaler.
type rawShape uint8

const (
	// shapeUnknown is the default: not an unstable.Marshaler, or the interface
	// is disabled.
	shapeUnknown rawShape = iota
	// shapeEmpty is whitespace-only content: it has no TOML representation and
	// is omitted from the output.
	shapeEmpty
	// shapeValue is a single TOML value, emitted inline as `key = <raw>`.
	shapeValue
	// shapeTable is one or more key-value lines, emitted as a `[key]` body.
	shapeTable
)

func (e *encoderState) encodeRoot(v interface{}) error {
	if v == nil {
		return errors.New("toml: cannot encode a nil interface")
	}

	rv := reflect.ValueOf(v)
	rv, ok := resolve(rv)
	if !ok {
		return errors.New("toml: cannot encode a nil pointer")
	}

	if e.marshalerOn && encPropsForType(rv.Type()).marshaler != 0 {
		return e.encodeMarshalerRoot(rv)
	}

	switch rv.Kind() {
	case reflect.Map, reflect.Struct:
		if isValueKind(rv) {
			return fmt.Errorf("toml: cannot encode a %s as a document root", rv.Type())
		}
		return e.encodeTable(rv, false, 0)
	default:
		return fmt.Errorf("toml: cannot encode a %s as a document root", rv.Type())
	}
}

// encodeMarshalerRoot emits the bytes of a root-level unstable.Marshaler as
// the whole document: the encode counterpart of the decoder delivering the
// whole document to a root Unmarshaler. The bytes must form a TOML document
// (key-value lines and table headers); empty output produces an empty
// document.
func (e *encoderState) encodeMarshalerRoot(rv reflect.Value) error {
	raw, err := e.marshalerBytes(rv)
	if err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	if err := e.validateRawTableBody(rv.Type(), trimmed); err != nil {
		// A single TOML value has no meaning at the document root; report it
		// as such rather than as a syntax error.
		if shape, _ := e.classifyRaw(trimmed); shape == shapeValue {
			return fmt.Errorf("toml: cannot encode %s as a document root: MarshalTOML returned a single TOML value, not a document", rv.Type())
		}
		return err
	}
	e.buf = append(e.buf, trimmed...)
	e.buf = append(e.buf, '\n')
	e.lastWasHeader = false
	return nil
}

// resolve unwraps pointers and interfaces until a concrete value is found.
// Returns false if it resolves to nil.
func resolve(v reflect.Value) (reflect.Value, bool) {
	for {
		switch v.Kind() {
		case reflect.Ptr:
			if v.IsNil() {
				return v, false
			}
			v = v.Elem()
		case reflect.Interface:
			if v.IsNil() {
				return v, false
			}
			v = v.Elem()
		default:
			return v, true
		}
	}
}

// typeEncProps caches the per-type facts used on every value encode.
type typeEncProps struct {
	// 0: not a TextMarshaler, 1: the type implements it, 2: its pointer does
	text uint8
	// 0: not an unstable.Marshaler, 1: the type implements it, 2: its pointer
	// does. Only consulted when Encoder.marshalerInterface is set.
	marshaler uint8
	// encoded as a TOML value (as opposed to a table)
	isValue bool
}

var marshalerType = reflect.TypeOf(new(unstable.Marshaler)).Elem()

var typeEncPropsCache sync.Map // reflect.Type -> typeEncProps

func encPropsForType(t reflect.Type) typeEncProps {
	if p, ok := typeEncPropsCache.Load(t); ok {
		return p.(typeEncProps)
	}
	var p typeEncProps
	switch {
	case t.Implements(textMarshalerType):
		p.text = 1
	case reflect.PtrTo(t).Implements(textMarshalerType):
		p.text = 2
	}
	switch {
	case t.Implements(marshalerType):
		p.marshaler = 1
	case reflect.PtrTo(t).Implements(marshalerType):
		p.marshaler = 2
	}
	switch t {
	case timeType, localDateType, localTimeType, localDateTimeType:
		p.isValue = true
	default:
		if p.text != 0 {
			p.isValue = true
		} else {
			switch t.Kind() {
			case reflect.Map, reflect.Struct:
				p.isValue = false
			default:
				p.isValue = true
			}
		}
	}
	typeEncPropsCache.Store(t, p)
	return p
}

// isValueKind returns true when the resolved value is encoded as a TOML
// value (as opposed to a table).
func isValueKind(v reflect.Value) bool {
	return encPropsForType(v.Type()).isValue
}

// isTableLike returns true when the value should be encoded as a table (or
// an array of tables for slices).
func (e *encoderState) isTableLike(v reflect.Value) bool {
	v, ok := resolve(v)
	if !ok {
		// Unresolvable values (interface-held nil pointers) are encoded as
		// the zero value of their element type by the value path.
		return false
	}
	return !isValueKind(v)
}

// isArrayOfTables returns true when the value is a non-empty slice or array
// containing only table-like values.
func (e *encoderState) isArrayOfTables(v reflect.Value) bool {
	v, ok := resolve(v)
	if !ok {
		return false
	}
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return false
	}
	if v.Len() == 0 {
		return false
	}
	// The Marshaler-aware classification lives in a separate method, hoisted out
	// of this loop, so the default path keeps its exact original shape.
	if e.marshalerOn {
		return e.isMarshalerArrayOfTables(v)
	}
	for i := 0; i < v.Len(); i++ {
		elem, ok := resolve(v.Index(i))
		if !ok || isValueKind(elem) {
			return false
		}
	}
	return true
}

// isMarshalerArrayOfTables is the EnableMarshalerInterface variant of
// isArrayOfTables: a Marshaler element counts as a table only when its raw
// content is table shaped (key-value lines); one holding a single value makes
// the whole slice a plain array instead.
func (e *encoderState) isMarshalerArrayOfTables(v reflect.Value) bool {
	for i := 0; i < v.Len(); i++ {
		elem, ok := resolve(v.Index(i))
		if !ok {
			return false
		}
		if encPropsForType(elem.Type()).marshaler != 0 {
			raw, err := e.marshalerBytes(elem)
			if err != nil {
				return false
			}
			if shape, _ := e.classifyRaw(raw); shape != shapeTable {
				return false
			}
			continue
		}
		if isValueKind(elem) {
			return false
		}
	}
	return true
}

// marshalerBytes returns the raw TOML produced by v's unstable.Marshaler
// implementation. The caller guarantees v implements the interface
// (encPropsForType(v.Type()).marshaler != 0).
func (e *encoderState) marshalerBytes(v reflect.Value) ([]byte, error) {
	t := v.Type()
	var m unstable.Marshaler
	switch {
	case encPropsForType(t).marshaler == 1:
		// The type itself implements Marshaler (e.g. a value receiver).
		m = v.Interface().(unstable.Marshaler)
	case v.CanAddr():
		// Only the pointer implements it, and v is addressable.
		m = v.Addr().Interface().(unstable.Marshaler)
	default:
		// Only the pointer implements it, but v is not addressable: take the
		// address of a copy.
		tmp := reflect.New(t)
		tmp.Elem().Set(v)
		m = tmp.Interface().(unstable.Marshaler)
	}
	b, err := m.MarshalTOML()
	if err != nil {
		return nil, fmt.Errorf("toml: error calling MarshalTOML for type %s: %w", t, err)
	}
	return b, nil
}

// validateRawTableBody checks that trimmed — table-shaped Marshaler output
// about to be spliced verbatim — is syntactically valid TOML, so a Marshaler
// cannot silently corrupt the document. It reuses e.parser, like classifyRaw.
func (e *encoderState) validateRawTableBody(t reflect.Type, trimmed []byte) error {
	e.parser.Reset(trimmed)
	for e.parser.NextExpression() {
	}
	if err := e.parser.Error(); err != nil {
		return fmt.Errorf("toml: error calling MarshalTOML for type %s: invalid TOML: %w", t, err)
	}
	return nil
}

// classifyRaw decides whether b (the output of an unstable.Marshaler) is empty,
// a single TOML value, or a table body. It returns the trimmed bytes that
// should be spliced into the document. It reuses e.parser, so it is not safe
// for concurrent use (encoderState is not shared).
func (e *encoderState) classifyRaw(b []byte) (rawShape, []byte) {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 {
		return shapeEmpty, trimmed
	}
	e.parser.Reset(trimmed)
	_, rest, err := parserbridge.ParseValue(&e.parser, trimmed)
	if err == nil && len(bytes.TrimSpace(rest)) == 0 {
		return shapeValue, trimmed
	}
	return shapeTable, trimmed
}

// resolveMarshalerEntries classifies every entry whose value implements
// unstable.Marshaler, recording the shape on the entry so the two table passes
// can route it without re-classifying. Any error from MarshalTOML is surfaced
// eagerly. The marshaled bytes themselves are produced again at emit time by
// marshalerValue; that keeps entry small on the encoder's hot path, and only
// runs when the (opt-in) interface is enabled.
func (e *encoderState) resolveMarshalerEntries(entries []entry) error {
	for i := range entries {
		ent := &entries[i]
		v, ok := resolve(ent.value)
		if !ok || encPropsForType(v.Type()).marshaler == 0 {
			continue
		}
		raw, err := e.marshalerBytes(v)
		if err != nil {
			return err
		}
		ent.options.rawShape, _ = e.classifyRaw(raw)
	}
	return nil
}

// marshalerValue returns the trimmed bytes to splice for an entry already known
// to be an unstable.Marshaler (ent.options.rawShape != shapeUnknown).
func (e *encoderState) marshalerValue(ent *entry) ([]byte, error) {
	v, _ := resolve(ent.value)
	raw, err := e.marshalerBytes(v)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSpace(raw), nil
}

// appendMarshalerInline emits an unstable.Marshaler value in a value position
// (array element or inline-table member), where only a single TOML value is
// valid. done is false when t is not a Marshaler, so appendValue falls through
// to its normal handling. It is split out of appendValue (type check included)
// to keep that hot path lean.
func (e *encoderState) appendMarshalerInline(b []byte, v reflect.Value, t reflect.Type) (out []byte, done bool, err error) {
	if encPropsForType(t).marshaler == 0 {
		return b, false, nil
	}
	raw, err := e.marshalerBytes(v)
	if err != nil {
		return nil, true, err
	}
	shape, trimmed := e.classifyRaw(raw)
	switch shape {
	case shapeValue:
		return append(b, trimmed...), true, nil
	case shapeEmpty:
		return nil, true, fmt.Errorf("toml: cannot encode an empty %s as an inline value", t)
	default:
		return nil, true, fmt.Errorf("toml: cannot encode %s as an inline value: %q is not a single TOML value", t, trimmed)
	}
}

// encodeTable writes the content of a table at the given key path.
func (e *encoderState) encodeTable(v reflect.Value, commented bool, indent int) error {
	entries, err := e.collectEntries(v)
	if err != nil {
		return err
	}

	// Marshaler routing is hoisted behind a single local flag. When the (opt-in)
	// interface is off, mOn is false and both passes run the exact baseline
	// code, so the default Marshal path keeps its performance.
	mOn := e.marshalerOn
	if mOn {
		// Classify Marshaler entries once, up front, so the passes can route
		// them by shape and surface MarshalTOML errors eagerly.
		if err := e.resolveMarshalerEntries(entries); err != nil {
			return err
		}
	}

	// First pass: emit all key-values; tables are handled by the second
	// pass.
	for i := range entries {
		ent := &entries[i]
		if mOn {
			switch ent.options.rawShape {
			case shapeUnknown:
				// Not a Marshaler: handled by the baseline logic below.
			case shapeEmpty:
				// No TOML representation: omit the key.
				continue
			case shapeValue:
				if err := e.encodeKeyValue(*ent, commented, indent); err != nil {
					return err
				}
				continue
			case shapeTable:
				// A table body is emitted in the second pass, unless it is
				// forced inline (SetTablesInline / inline tag), which has no
				// valid inline form and is reported as an error by
				// encodeKeyValue.
				if e.tablesInline || ent.options.inline {
					if err := e.encodeKeyValue(*ent, commented, indent); err != nil {
						return err
					}
				}
				continue
			}
		}
		if e.entryIsTable(ent) {
			continue
		}
		if err := e.encodeKeyValue(*ent, commented, indent); err != nil {
			return err
		}
	}

	// Second pass: emit the sub-tables, extending the shared key stack.
	for i := range entries {
		ent := entries[i]
		if mOn {
			switch ent.options.rawShape {
			case shapeUnknown:
				// Not a Marshaler: handled by the baseline logic below.
			case shapeValue, shapeEmpty:
				// Not a table: already handled (or omitted) in the first pass.
				continue
			case shapeTable:
				// Emit the raw body verbatim under the freshly pushed header.
				// (The forced-inline case already errored in the first pass.)
				entCommented := commented || ent.options.commented
				e.keyStack = append(e.keyStack, ent.key)
				if err := e.encodeMarshalerTable(&ent, entCommented, indent); err != nil {
					return err
				}
				e.keyStack = e.keyStack[:len(e.keyStack)-1]
				continue
			}
		}
		if !e.entryIsTable(&ent) {
			continue
		}
		entCommented := commented || ent.options.commented
		e.keyStack = append(e.keyStack, ent.key)

		if e.isArrayOfTables(ent.value) {
			err := e.encodeArrayTable(ent, entCommented, indent)
			if err != nil {
				return err
			}
			e.keyStack = e.keyStack[:len(e.keyStack)-1]
			continue
		}

		// The value is resolvable: entryIsTable already resolved it.
		tv, _ := resolve(ent.value)

		e.writeTableHeader(ent.options.comment, entCommented, false, indent)

		err := e.encodeTable(tv, entCommented, indent+1)
		if err != nil {
			return err
		}
		e.keyStack = e.keyStack[:len(e.keyStack)-1]
	}

	e.putEntries(entries)
	return nil
}

// encodeMarshalerTable emits a table-shaped unstable.Marshaler entry: the
// header for the key currently on the stack, then the raw body verbatim.
func (e *encoderState) encodeMarshalerTable(ent *entry, commented bool, indent int) error {
	raw, err := e.marshalerValue(ent)
	if err != nil {
		return err
	}
	// Validate the bytes actually being spliced: MarshalTOML is called again
	// for the emit, so this both rejects invalid TOML and guards against an
	// implementation that returned different content than during
	// classification.
	if err := e.validateRawTableBody(ent.value.Type(), raw); err != nil {
		return err
	}
	e.writeTableHeader(ent.options.comment, commented, false, indent)
	e.spliceRawTableBody(raw, commented)
	return nil
}

// spliceRawTableBody appends a raw table body verbatim after its header. When
// the table is commented, every physical line is prefixed with the comment
// marker so the body does not leak into the document as live keys.
func (e *encoderState) spliceRawTableBody(raw []byte, commented bool) {
	if len(raw) == 0 {
		return
	}
	if commented {
		e.buf = append(e.buf, "# "...)
		e.buf = append(e.buf, bytes.ReplaceAll(raw, []byte("\n"), []byte("\n# "))...)
	} else {
		e.buf = append(e.buf, raw...)
	}
	e.buf = append(e.buf, '\n')
	e.lastWasHeader = false
}

// entryIsTable reports whether the entry is emitted as a (sub-)table rather
// than a key-value. Marshaler entries are routed by encodeTable before this is
// reached, so it carries no marshaler-specific cost.
func (e *encoderState) entryIsTable(ent *entry) bool {
	return !e.tablesInline && !ent.options.inline && (e.isTableLike(ent.value) || e.isArrayOfTables(ent.value))
}

// getEntries returns a reusable entry slice.
func (e *encoderState) getEntries() []entry {
	if n := len(e.entriesPool); n > 0 {
		s := e.entriesPool[n-1]
		e.entriesPool = e.entriesPool[:n-1]
		return s[:0]
	}
	return nil
}

// putEntries returns an entry slice to the pool.
func (e *encoderState) putEntries(s []entry) {
	if cap(s) > 0 {
		e.entriesPool = append(e.entriesPool, s)
	}
}

// encodeArrayTable writes all the elements of an array of tables.
func (e *encoderState) encodeArrayTable(ent entry, commented bool, indent int) error {
	v, _ := resolve(ent.value)
	comment := ent.options.comment
	for i := 0; i < v.Len(); i++ {
		// Elements are resolvable: isArrayOfTables already resolved them.
		elem, _ := resolve(v.Index(i))

		e.writeTableHeader(comment, commented, true, indent)
		// The comment is only present before the first element.
		comment = ""

		// A Marshaler element splices its raw table body verbatim. The shape
		// was checked by isArrayOfTables, but MarshalTOML is called again for
		// the emit, so the spliced bytes are validated here.
		if e.marshalerOn && encPropsForType(elem.Type()).marshaler != 0 {
			raw, err := e.marshalerBytes(elem)
			if err != nil {
				return err
			}
			trimmed := bytes.TrimSpace(raw)
			if err := e.validateRawTableBody(elem.Type(), trimmed); err != nil {
				return err
			}
			e.spliceRawTableBody(trimmed, commented)
			continue
		}

		err := e.encodeTable(elem, commented, indent+1)
		if err != nil {
			return err
		}
	}
	return nil
}

// writeTableHeader emits a [table] or [[array table]] header line, preceded
// by an empty line and comments as needed.
func (e *encoderState) writeTableHeader(comment string, commented bool, array bool, indent int) {
	key := e.keyStack
	if len(e.buf) > 0 && !e.lastWasHeader {
		e.buf = append(e.buf, '\n')
	}

	headerIndent := indent

	e.writeComment(comment, headerIndent)

	// The "commented" marker is emitted at column zero, ahead of any table
	// indentation, so that the indentation appears inside the comment
	// (e.g. `#   [a.b]`). This matches the historical v2.3 layout.
	if commented {
		e.buf = append(e.buf, "# "...)
	}
	e.writeIndent(headerIndent)
	e.buf = append(e.buf, '[')
	if array {
		e.buf = append(e.buf, '[')
	}
	for i, part := range key {
		if i > 0 {
			e.buf = append(e.buf, '.')
		}
		e.buf = e.appendKey(e.buf, part)
	}
	e.buf = append(e.buf, ']')
	if array {
		e.buf = append(e.buf, ']')
	}
	e.buf = append(e.buf, '\n')
	e.lastWasHeader = true
}

func (e *encoderState) writeIndent(indent int) {
	if !e.indentTables {
		return
	}
	for i := 0; i < indent; i++ {
		e.buf = append(e.buf, e.indentSymbol...)
	}
}

// writeComment emits the comment lines attached to an entry.
func (e *encoderState) writeComment(comment string, indent int) {
	if comment == "" {
		return
	}
	for _, line := range strings.Split(comment, "\n") {
		e.writeIndent(indent)
		e.buf = append(e.buf, "# "...)
		e.buf = append(e.buf, line...)
		e.buf = append(e.buf, '\n')
	}
}

// encodeKeyValue writes one `key = value` line of a table.
func (e *encoderState) encodeKeyValue(ent entry, commented bool, indent int) error {
	commented = commented || ent.options.commented

	e.writeComment(ent.options.comment, indent)

	// The "commented" marker is emitted at column zero, ahead of any table
	// indentation, so that the indentation appears inside the comment
	// (e.g. `#   key = value`). This matches the historical v2.3 layout.
	lineStart := len(e.buf)
	if commented {
		e.buf = append(e.buf, "# "...)
	}
	e.writeIndent(indent)
	e.buf = e.appendKey(e.buf, ent.key)
	e.buf = append(e.buf, " = "...)

	// When tables are not indented, the key is emitted at column zero
	// regardless of its nesting depth. Value continuation lines (most
	// notably the elements of a multiline array) must line up with that key,
	// so the value indentation starts from zero as well rather than from the
	// table nesting depth.
	valueIndent := indent
	if !e.indentTables {
		valueIndent = 0
	}

	// A Marshaler value is delegated, keeping this hot function lean for the
	// default path (rawShape stays shapeUnknown when the interface is off). It
	// shares the commented/newline handling below.
	var err error
	if ent.options.rawShape != shapeUnknown {
		e.buf, err = e.appendMarshalerInlineValue(e.buf, &ent)
	} else {
		e.buf, err = e.appendValue(e.buf, ent.value, ent.options, valueIndent)
	}
	if err != nil {
		return err
	}

	// A commented value that renders across multiple lines (a multiline string
	// or a multiline array) must have every physical line prefixed with the
	// comment marker, not just the first; otherwise the continuation lines are
	// emitted as live, syntactically invalid TOML.
	if commented {
		if bytes.IndexByte(e.buf[lineStart:], '\n') >= 0 {
			region := bytes.ReplaceAll(
				append([]byte(nil), e.buf[lineStart:]...),
				[]byte("\n"), []byte("\n# "),
			)
			e.buf = append(e.buf[:lineStart], region...)
		}
	}
	e.buf = append(e.buf, '\n')
	e.lastWasHeader = false
	return nil
}

// appendMarshalerInlineValue appends the value part of a `key = ` line for an
// unstable.Marshaler entry, leaving the trailing newline and commented handling
// to encodeKeyValue. Empty entries are filtered out before this point, so the
// value is either a single value (spliced verbatim) or, when forced inline by
// SetTablesInline or an inline tag, table content (an error).
func (e *encoderState) appendMarshalerInlineValue(b []byte, ent *entry) ([]byte, error) {
	errNotValue := func() error {
		return fmt.Errorf("toml: cannot encode %s as an inline value: not a single TOML value", ent.value.Type())
	}
	if ent.options.rawShape != shapeValue {
		return nil, errNotValue()
	}
	raw, err := e.marshalerValue(ent)
	if err != nil {
		return nil, err
	}
	// Classify the just-returned bytes rather than trusting the earlier pass:
	// MarshalTOML is called again for the emit, and an implementation that
	// returns different content must not splice non-value bytes into a
	// `key = ` position.
	if shape, trimmed := e.classifyRaw(raw); shape == shapeValue {
		return append(b, trimmed...), nil
	}
	return nil, errNotValue()
}

// collectEntries builds the ordered list of the entries of a table,
// applying tags and omission rules.
func (e *encoderState) collectEntries(v reflect.Value) ([]entry, error) {
	switch v.Kind() {
	case reflect.Map:
		return e.collectMapEntries(v)
	case reflect.Struct:
		entries := e.getEntries()
		e.collectStructEntries(&entries, v)
		return entries, nil
	default:
		return nil, fmt.Errorf("toml: cannot encode a %s as a table", v.Type())
	}
}

func (e *encoderState) collectMapEntries(v reflect.Value) ([]entry, error) {
	entries := e.getEntries()

	// Keys are converted to strings right away: read them into a reusable
	// buffer to avoid one allocation per key.
	var kbuf reflect.Value
	if v.Type().Key() == stringType {
		if !e.stringKeyBuf.IsValid() {
			e.stringKeyBuf = reflect.New(stringType).Elem()
		}
		kbuf = e.stringKeyBuf
	} else {
		kbuf = reflect.New(v.Type().Key()).Elem()
	}

	iter := v.MapRange()
	for iter.Next() {
		kbuf.SetIterKey(iter)
		key, err := mapKeyString(kbuf)
		if err != nil {
			return nil, err
		}
		value := iter.Value()
		if value.Kind() == reflect.Interface && value.IsNil() {
			// nil interface values are skipped
			continue
		}
		if value.Kind() == reflect.Ptr && value.IsNil() {
			// nil pointers in maps are encoded as their zero value
			value = reflect.New(value.Type().Elem()).Elem()
		}
		entries = append(entries, entry{key: key, value: value})
	}

	if len(entries) > 1 {
		// slices.SortFunc avoids boxing the slice into a sort.Interface (an
		// allocation that sort.Sort incurs for every table).
		slices.SortFunc(entries, func(a, b entry) int {
			return strings.Compare(a.key, b.key)
		})
	}

	return entries, nil
}

// mapKeyString converts a map key to its string representation.
func mapKeyString(k reflect.Value) (string, error) {
	kr, ok := resolve(k)
	if !ok {
		return "", errors.New("toml: cannot encode a nil map key")
	}
	if kr.Type().Implements(textMarshalerType) {
		b, err := kr.Interface().(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return "", fmt.Errorf("toml: cannot marshal map key: %w", err)
		}
		return string(b), nil
	}
	if kr.CanAddr() && reflect.PtrTo(kr.Type()).Implements(textMarshalerType) {
		b, err := kr.Addr().Interface().(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return "", fmt.Errorf("toml: cannot marshal map key: %w", err)
		}
		return string(b), nil
	}
	switch kr.Kind() {
	case reflect.String:
		return kr.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(kr.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(kr.Uint(), 10), nil
	case reflect.Float32:
		return strconv.FormatFloat(kr.Float(), 'f', -1, 32), nil
	case reflect.Float64:
		return strconv.FormatFloat(kr.Float(), 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("toml: cannot encode a map with key type %s", k.Type())
	}
}

// encPlanField is the static encoding information of one field of a struct.
type encPlanField struct {
	name    string
	index   []int
	depth   int
	options valueOptions
}

// encPlan caches the per-type information needed to encode a struct:
// flattened fields with parsed tags, in order of definition, with shadowed
// duplicates already removed.
type encPlan struct {
	fields []encPlanField
}

var encPlans sync.Map // reflect.Type -> *encPlan

func encPlanForType(t reflect.Type) *encPlan {
	if plan, ok := encPlans.Load(t); ok {
		return plan.(*encPlan)
	}
	plan := &encPlan{}
	visited := map[reflect.Type]bool{}
	buildEncPlan(plan, t, nil, 0, visited)
	dedupEncPlan(plan)
	encPlans.Store(t, plan)
	return plan
}

func buildEncPlan(plan *encPlan, t reflect.Type, prefix []int, depth int, visited map[reflect.Type]bool) {
	if visited[t] {
		return
	}
	visited[t] = true
	defer delete(visited, t)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		tag, tagged := f.Tag.Lookup("toml")
		if tag == "-" {
			continue
		}

		name := f.Name
		var opts valueOptions
		if tagged {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				name = parts[0]
			}
			for _, opt := range parts[1:] {
				switch opt {
				case "multiline":
					opts.multiline = true
				case "inline":
					opts.inline = true
				case "omitempty":
					opts.omitempty = true
				case "omitzero":
					opts.omitzero = true
				case "commented":
					opts.commented = true
				}
			}
		}
		// Standalone boolean tags, e.g. multiline:"true".
		const tagTrue = "true"
		if f.Tag.Get("multiline") == tagTrue {
			opts.multiline = true
		}
		if f.Tag.Get("inline") == tagTrue {
			opts.inline = true
		}
		if f.Tag.Get("commented") == tagTrue {
			opts.commented = true
		}
		opts.comment = f.Tag.Get("comment")

		index := make([]int, 0, len(prefix)+1)
		index = append(index, prefix...)
		index = append(index, i)

		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct && (!tagged || tagName(tag) == "") {
				buildEncPlan(plan, ft, index, depth+1, visited)
				continue
			}
			if f.PkgPath != "" && ft.Kind() != reflect.Interface {
				continue
			}
		} else if f.PkgPath != "" {
			// unexported
			continue
		}

		plan.fields = append(plan.fields, encPlanField{
			name:    name,
			index:   index,
			depth:   depth,
			options: opts,
		})
	}
}

// dedupEncPlan removes the fields shadowed by another one with the same
// name (the shallowest wins), keeping the order of first appearance.
func dedupEncPlan(plan *encPlan) {
	byName := make(map[string]int, len(plan.fields))
	drop := false
	for i := range plan.fields {
		f := &plan.fields[i]
		j, seen := byName[f.name]
		if !seen {
			byName[f.name] = i
			continue
		}
		drop = true
		// Shallowest wins; on equal depth, the first in order wins.
		if f.depth < plan.fields[j].depth {
			plan.fields[j].name = ""
			byName[f.name] = i
		} else {
			f.name = ""
		}
	}
	if !drop {
		return
	}
	out := plan.fields[:0]
	for _, f := range plan.fields {
		if f.name != "" {
			out = append(out, f)
		}
	}
	plan.fields = out
}

// collectStructEntries appends the entries of a struct, flattening embedded
// structs in place.
func (e *encoderState) collectStructEntries(entries *[]entry, v reflect.Value) {
	plan := encPlanForType(v.Type())

	for i := range plan.fields {
		f := &plan.fields[i]
		fv, ok := fieldByIndexSkipNil(v, f.index)
		if !ok {
			// nil embedded pointer on the way: skipped
			continue
		}

		// Anonymous interface fields that are nil are skipped.
		if fv.Kind() == reflect.Interface && fv.IsNil() {
			continue
		}
		// nil values in struct fields are skipped
		if (fv.Kind() == reflect.Ptr || fv.Kind() == reflect.Map) && fv.IsNil() {
			continue
		}

		if f.options.omitempty && isEmptyValue(fv) {
			continue
		}
		if f.options.omitzero && isZeroValue(fv) {
			continue
		}

		*entries = append(*entries, entry{key: f.name, value: fv, options: f.options})
	}
}

// fieldByIndexSkipNil returns the field at the given index path, reporting
// false if a nil embedded pointer is found on the way.
func fieldByIndexSkipNil(v reflect.Value, index []int) (reflect.Value, bool) {
	for i, x := range index {
		if i > 0 {
			for v.Kind() == reflect.Ptr {
				if v.IsNil() {
					return v, false
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v, true
}

func tagName(tag string) string {
	if idx := strings.IndexByte(tag, ','); idx >= 0 {
		return tag[:idx]
	}
	return tag
}

// isEmptyValue implements the omitempty rules.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Map, reflect.Slice, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		// Structs that encode as a scalar value (time.Time, the local
		// date/time types, or any TextMarshaler) are empty only when they
		// equal their zero value; their fields are typically unexported, so
		// recursing into them would be meaningless.
		if encPropsForType(v.Type()).isValue {
			return v.IsZero()
		}
		// Plain structs encode as tables and are empty when every field that
		// would be encoded is itself empty. This matches the recursive rule
		// used before the encoder rewrite and, in particular, treats a
		// non-nil but empty map or slice as empty (reflect.Value.IsZero does
		// not, which would otherwise emit an empty table header).
		return isEmptyStruct(v)
	default:
		return false
	}
}

// isEmptyStruct reports whether all of a table-valued struct's encodable
// fields are empty per isEmptyValue. It mirrors the field selection done by
// collectStructEntries (embedded flattening, shadowing, and "-" skips) so the
// emptiness decision matches what would actually be encoded.
func isEmptyStruct(v reflect.Value) bool {
	plan := encPlanForType(v.Type())
	for i := range plan.fields {
		fv, ok := fieldByIndexSkipNil(v, plan.fields[i].index)
		if !ok {
			// A nil embedded pointer along the path contributes nothing.
			continue
		}
		if !isEmptyValue(fv) {
			return false
		}
	}
	return true
}

// isZeroValue implements the omitzero rules: the type's own IsZero() when
// implemented, the reflect zero value otherwise.
func isZeroValue(v reflect.Value) bool {
	if v.Type().Implements(isZeroerType) {
		return v.Interface().(isZeroer).IsZero()
	}
	if v.CanAddr() && reflect.PtrTo(v.Type()).Implements(isZeroerType) {
		return v.Addr().Interface().(isZeroer).IsZero()
	}
	if !v.CanAddr() && reflect.PtrTo(v.Type()).Implements(isZeroerType) {
		tmp := reflect.New(v.Type())
		tmp.Elem().Set(v)
		return tmp.Interface().(isZeroer).IsZero()
	}
	return v.IsZero()
}

// appendKey emits a key, quoted only if necessary.
func (e *encoderState) appendKey(b []byte, key string) []byte {
	if isBareKey(key) {
		return append(b, key...)
	}
	return e.appendString(b, key)
}

func isBareKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	for _, c := range []byte(key) {
		if !isUnquotedKeyByte(c) {
			return false
		}
	}
	return true
}

func isUnquotedKeyByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
}

// appendValue emits a TOML value.
func (e *encoderState) appendValue(b []byte, v reflect.Value, opts valueOptions, indent int) ([]byte, error) {
	t := v.Type()

	// Special types take precedence over their kind.
	switch t {
	case timeType:
		return v.Interface().(time.Time).AppendFormat(b, "2006-01-02T15:04:05.999999999Z07:00"), nil
	case localDateType:
		return append(b, v.Interface().(LocalDate).String()...), nil
	case localTimeType:
		return append(b, v.Interface().(LocalTime).String()...), nil
	case localDateTimeType:
		return append(b, v.Interface().(LocalDateTime).String()...), nil
	case jsonNumberType:
		if e.marshalJSONNumbers {
			return appendJSONNumber(b, v.Interface().(json.Number))
		}
	}

	// A Marshaler reached through a value position (an array element or inline
	// table member) splices its bytes verbatim. Everything (including the type
	// check) lives in a separate method so this hot function keeps its default
	// layout: when the opt-in interface is off, only the bool test runs here.
	if e.marshalerOn {
		if b2, done, err := e.appendMarshalerInline(b, v, t); done {
			return b2, err
		}
	}

	switch encPropsForType(t).text {
	case 1:
		if t.Kind() != reflect.String {
			return e.appendTextMarshaler(b, v.Interface().(encoding.TextMarshaler))
		}
	case 2:
		if v.CanAddr() {
			return e.appendTextMarshaler(b, v.Addr().Interface().(encoding.TextMarshaler))
		}
		tmp := reflect.New(t)
		tmp.Elem().Set(v)
		return e.appendTextMarshaler(b, tmp.Interface().(encoding.TextMarshaler))
	}

	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			// nil pointers are encoded as the zero value of their element
			// type.
			return e.appendValue(b, reflect.Zero(t.Elem()), opts, indent)
		}
		return e.appendValue(b, v.Elem(), opts, indent)
	case reflect.Interface:
		if v.IsNil() {
			return nil, errors.New("toml: cannot encode a nil interface")
		}
		return e.appendValue(b, v.Elem(), opts, indent)
	case reflect.String:
		s := v.String()
		// The "multiline" option only takes effect when the string actually
		// contains a newline. Wrapping a single-line value such as "2" in a
		// """...""" block adds noise without improving readability, so it is
		// emitted as a regular single-line string instead.
		if opts.multiline && strings.IndexByte(s, '\n') >= 0 {
			return e.appendMultilineString(b, s), nil
		}
		return e.appendString(b, s), nil
	case reflect.Bool:
		if v.Bool() {
			return append(b, "true"...), nil
		}
		return append(b, "false"...), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.AppendInt(b, v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := v.Uint()
		if u > math.MaxInt64 {
			return nil, fmt.Errorf("toml: cannot encode an unsigned integer above math.MaxInt64: %d", u)
		}
		return strconv.AppendUint(b, u, 10), nil
	case reflect.Float32:
		return appendFloat(b, v.Float(), 32), nil
	case reflect.Float64:
		return appendFloat(b, v.Float(), 64), nil
	case reflect.Slice, reflect.Array:
		return e.appendArray(b, v, opts, indent)
	case reflect.Map:
		return e.appendInlineTable(b, v, indent)
	case reflect.Struct:
		return e.appendInlineTable(b, v, indent)
	default:
		return nil, fmt.Errorf("toml: cannot encode value of type %s", v.Type())
	}
}

var jsonNumberType = reflect.TypeOf(json.Number(""))

func appendJSONNumber(b []byte, n json.Number) ([]byte, error) {
	if n == "" {
		return append(b, '0'), nil
	}
	if i, err := n.Int64(); err == nil {
		return strconv.AppendInt(b, i, 10), nil
	}
	f, err := n.Float64()
	if err != nil {
		return nil, fmt.Errorf("toml: cannot encode json.Number %q: %w", string(n), err)
	}
	return appendFloat(b, f, 64), nil
}

func appendFloat(b []byte, f float64, bitSize int) []byte {
	switch {
	case math.IsNaN(f):
		return append(b, "nan"...)
	case math.IsInf(f, 1):
		return append(b, "inf"...)
	case math.IsInf(f, -1):
		return append(b, "-inf"...)
	}
	start := len(b)
	b = strconv.AppendFloat(b, f, 'f', -1, bitSize)
	// TOML floats must have a fractional part or an exponent.
	if !bytes.ContainsAny(b[start:], ".eE") {
		b = append(b, ".0"...)
	}
	return b
}

func (e *encoderState) appendTextMarshaler(b []byte, m encoding.TextMarshaler) ([]byte, error) {
	text, err := m.MarshalText()
	if err != nil {
		return nil, fmt.Errorf("toml: error calling MarshalText: %w", err)
	}
	return e.appendString(b, string(text)), nil
}

// appendArray encodes a slice or array value.
func (e *encoderState) appendArray(b []byte, v reflect.Value, opts valueOptions, indent int) ([]byte, error) {
	multiline := opts.multiline || e.arraysMultiline

	b = append(b, '[')
	if multiline && v.Len() > 0 {
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				b = append(b, ',')
			}
			b = append(b, '\n')
			for j := 0; j <= indent; j++ {
				b = append(b, e.indentSymbol...)
			}
			var err error
			b, err = e.appendValue(b, v.Index(i), valueOptions{}, indent+1)
			if err != nil {
				return nil, err
			}
		}
		b = append(b, '\n')
		for j := 0; j < indent; j++ {
			b = append(b, e.indentSymbol...)
		}
	} else {
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				b = append(b, ", "...)
			}
			var err error
			b, err = e.appendValue(b, v.Index(i), valueOptions{}, indent)
			if err != nil {
				return nil, err
			}
		}
	}
	return append(b, ']'), nil
}

// appendInlineTable encodes a map or a struct as an inline table.
func (e *encoderState) appendInlineTable(b []byte, v reflect.Value, indent int) ([]byte, error) {
	entries, err := e.collectEntries(v)
	if err != nil {
		return nil, err
	}

	b = append(b, '{')
	for i, ent := range entries {
		if i > 0 {
			b = append(b, ", "...)
		}
		b = e.appendKey(b, ent.key)
		b = append(b, " = "...)
		// multiline strings are not allowed inside inline tables: they
		// would break the single-line requirement.
		opts := ent.options
		opts.multiline = false
		b, err = e.appendValue(b, ent.value, opts, indent)
		if err != nil {
			return nil, err
		}
	}
	e.putEntries(entries)
	return append(b, '}'), nil
}

// appendString encodes a string, using a literal string when possible and a
// basic string otherwise.
func (e *encoderState) appendString(b []byte, s string) []byte {
	if canBeLiteral(s) {
		b = append(b, '\'')
		b = append(b, s...)
		return append(b, '\'')
	}
	return appendBasicString(b, s)
}

// canBeLiteral returns true when the string can be represented as a TOML
// literal string: no control characters, no single quote, no newline.
func canBeLiteral(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' || c == 0x7f || c < 0x20 {
			return false
		}
	}
	return utf8.ValidString(s)
}

// appendBasicString encodes a string as a TOML basic (double-quoted) string.
func appendBasicString(b []byte, s string) []byte {
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '"':
			b = append(b, '\\', '"')
			i++
		case c == '\\':
			b = append(b, '\\', '\\')
			i++
		case c == '\b':
			b = append(b, '\\', 'b')
			i++
		case c == '\f':
			b = append(b, '\\', 'f')
			i++
		case c == '\n':
			b = append(b, '\\', 'n')
			i++
		case c == '\r':
			b = append(b, '\\', 'r')
			i++
		case c == '\t':
			b = append(b, '\\', 't')
			i++
		case c < 0x20 || c == 0x7f:
			b = append(b, fmt.Sprintf("\\u%04X", c)...)
			i++
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				// Replace invalid bytes by the replacement character.
				b = append(b, fmt.Sprintf("\\u%04X", c)...)
				i++
				continue
			}
			b = append(b, s[i:i+size]...)
			i += size
		}
	}
	return append(b, '"')
}

// appendMultilineString encodes a string as a TOML multi-line basic string.
func appendMultilineString(b []byte, s string) []byte {
	b = append(b, `"""`...)
	b = append(b, '\n')
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '"':
			// Runs of three or more quotes must be escaped.
			j := i
			for j < len(s) && s[j] == '"' {
				j++
			}
			if j-i >= 3 {
				for ; i < j; i++ {
					b = append(b, '\\', '"')
				}
			} else {
				b = append(b, s[i:j]...)
				i = j
			}
		case c == '\\':
			b = append(b, '\\', '\\')
			i++
		case c == '\n':
			b = append(b, '\n')
			i++
		case c == '\b':
			b = append(b, '\\', 'b')
			i++
		case c == '\f':
			b = append(b, '\\', 'f')
			i++
		case c == '\r':
			b = append(b, '\\', 'r')
			i++
		case c == '\t':
			b = append(b, '\t')
			i++
		case c < 0x20 || c == 0x7f:
			b = append(b, fmt.Sprintf("\\u%04X", c)...)
			i++
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				b = append(b, fmt.Sprintf("\\u%04X", c)...)
				i++
				continue
			}
			b = append(b, s[i:i+size]...)
			i += size
		}
	}
	return append(b, `"""`...)
}

func (e *encoderState) appendMultilineString(b []byte, s string) []byte {
	return appendMultilineString(b, s)
}
