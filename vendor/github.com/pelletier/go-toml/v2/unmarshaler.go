package toml

import (
	"encoding"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2/internal/tracker"
	"github.com/pelletier/go-toml/v2/unstable"
)

// decoderPool recycles decoders (and their internal buffers: parser arena,
// seen-tracker entries, scratch buffers) across calls to Unmarshal and
// Decode.
var decoderPool = sync.Pool{
	New: func() interface{} { return &decoder{} },
}

func getDecoder(strictMode, unmarshalerInterface bool) *decoder {
	d := decoderPool.Get().(*decoder)
	d.reset()
	d.strict.Enabled = strictMode
	d.unmarshalerInterface = unmarshalerInterface
	return d
}

func putDecoder(d *decoder) {
	decoderPool.Put(d)
}

// reset clears the per-document state of the decoder, keeping the allocated
// buffers for reuse.
func (d *decoder) reset() {
	d.seen.Reset()
	d.tableKey = d.tableKey[:0]
	d.skipUntilTable = false
	d.path = d.path[:0]
	d.captures = d.captures[:0]
	d.captureIdx = -1
	d.segIdx = d.segIdx[:0]
	// Reuse the array-table counter slots across documents instead of
	// deleting them: a zeroed slot is indistinguishable from an absent one,
	// and keeping it alive means setArrayCount does not have to allocate a new
	// *int every time the same path reappears. A safety valve bounds the table
	// for adversarial inputs that introduce unboundedly many distinct paths.
	if len(d.arrayCounts) > 1<<14 {
		d.arrayCounts = nil
	} else {
		for _, p := range d.arrayCounts {
			*p = 0
		}
	}
	d.tableTarget = reflect.Value{}
	d.tableTargetValid = false
	d.tableFlush = d.tableFlush[:0]
	d.tableParentSlot = slotWriter{}
	d.keyParts = d.keyParts[:0]
	d.strict.Reset()
}

// Unmarshal deserializes a TOML document into a Go value.
//
// It is a shortcut for Decoder.Decode() with the default options.
func Unmarshal(data []byte, v interface{}) error {
	d := getDecoder(false, false)
	err := d.unmarshal(data, v)
	putDecoder(d)
	return err
}

// Decoder reads and decode a TOML document from an input stream.
type Decoder struct {
	// input
	r io.Reader

	// global settings
	strict bool

	// toggles unmarshaler interface
	unmarshalerInterface bool
}

// NewDecoder creates a new Decoder that will read from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// DisallowUnknownFields causes the Decoder to return an error when the
// destination is a struct and the input contains a key that does not match a
// non-ignored field.
//
// In that case, the Decoder returns a StrictMissingError that can be used to
// retrieve the individual errors as well as generate a human readable
// description of the missing fields.
func (d *Decoder) DisallowUnknownFields() *Decoder {
	d.strict = true
	return d
}

// EnableUnmarshalerInterface allows to enable unmarshaler interface.
//
// With this feature enabled, types implementing the unstable.Unmarshaler
// interface can be decoded from any structure of the document. It allows types
// that don't have a straightforward TOML representation to provide their own
// decoding logic.
//
// The UnmarshalTOML method receives raw TOML bytes:
//   - For single values: the raw value bytes (e.g., `"hello"` for a string)
//   - For tables: all key-value lines belonging to that table
//   - For inline tables/arrays: the raw bytes of the inline structure
//
// The unstable.RawMessage type can be used to capture raw TOML bytes for
// later processing, similar to json.RawMessage.
//
// *Unstable:* This method does not follow the compatibility guarantees of
// semver. It can be changed or removed without a new major version being
// issued.
func (d *Decoder) EnableUnmarshalerInterface() *Decoder {
	d.unmarshalerInterface = true
	return d
}

// Decode the whole content of r into v.
//
// By default, values in the document that don't exist in the target Go value
// are ignored. See Decoder.DisallowUnknownFields() to change this behavior.
//
// When a TOML local date, time, or date-time is decoded into a time.Time, its
// value is represented in time.Local timezone. Otherwise the appropriate Local*
// structure is used. For time values, precision up to the nanosecond is
// supported by truncating extra digits.
//
// Empty tables decoded in an interface{} create an empty initialized
// map[string]interface{}.
//
// Types implementing the encoding.TextUnmarshaler interface are decoded from a
// TOML string.
//
// When decoding a number, go-toml will return an error if the number is out of
// bounds for the target type (which includes negative numbers when decoding
// into an unsigned int).
//
// If an error occurs while decoding the content of the document, this function
// returns a toml.DecodeError, providing context about the issue. When using
// strict mode and a field is missing, a `toml.StrictMissingError` is
// returned. In any other case, this function returns a standard Go error.
//
// # Type mapping
//
// List of supported TOML types and their associated accepted Go types:
//
//	String           -> string
//	Integer          -> uint*, int*, depending on size
//	Float            -> float*, depending on size
//	Boolean          -> bool
//	Offset Date-Time -> time.Time
//	Local Date-time  -> LocalDateTime, time.Time
//	Local Date       -> LocalDate, time.Time
//	Local Time       -> LocalTime, time.Time
//	Array            -> slice and array, depending on elements types
//	Table            -> map and struct
//	Inline Table     -> same as Table
//	Array of Tables  -> same as Array and Table
func (d *Decoder) Decode(v interface{}) error {
	b, err := io.ReadAll(d.r)
	if err != nil {
		return fmt.Errorf("toml: %w", err)
	}

	dec := getDecoder(d.strict, d.unmarshalerInterface)
	err = dec.unmarshal(b, v)
	putDecoder(dec)
	return err
}

// pathPart is one part of the key path leading to a value. Parts that come
// from the current table header only carry a name; parts that come from the
// key of the current key-value expression also carry the AST node, and their
// name is materialized lazily to avoid allocations.
type pathPart struct {
	name string
	node *unstable.Node
}

// bytes returns the raw bytes of the key part.
func (p *pathPart) bytes() []byte {
	if p.node != nil {
		return p.node.Data
	}
	return []byte(p.name)
}

// str returns the key part as a string, possibly allocating.
func (p *pathPart) str() string {
	if p.node != nil {
		return string(p.node.Data)
	}
	return p.name
}

// rawCapture accumulates the raw bytes fed to a type implementing
// unstable.Unmarshaler for a table target. The target is identified by the
// parts of its key and the array-table indexes in effect when the capture
// was created, so that it can be located again once the whole document has
// been processed (the address of the target may change as slices grow).
type rawCapture struct {
	names []string
	// indexes[i] is the index to use when reaching a slice or array right
	// before consuming names[i]. indexes[len(names)] is the index of the
	// element when the target is an element of an array table. -1 when not
	// relevant.
	indexes []int
	buf     []byte
}

type decoder struct {
	p unstable.Parser

	// strict mode
	strict strict

	// toggles unmarshaler interface
	unmarshalerInterface bool

	// tracks the duplicate and type consistency of the keys
	seen tracker.SeenTracker

	// path of the current table header, as copied strings
	tableKey []string

	// true when the expressions under the current table header cannot be
	// stored anywhere and should be skipped
	skipUntilTable bool

	// scratch buffer for the key path of the current expression
	path []pathPart

	// raw captures for the unmarshaler interface, in order of first
	// appearance. captureIdx is the index of the capture the current table
	// belongs to, or -1.
	captures   []rawCapture
	captureIdx int

	// segIdx[i] records the array element index used when traversing a
	// slice or array right before consuming the i-th part of the current
	// table key. Reset for each table expression.
	segIdx []int

	// arrayCounts tracks the number of elements appended to fixed-size
	// arrays used as array tables, keyed by the NUL-joined key parts.
	// Values are pointer slots so that updating an existing path does not
	// allocate a new key string.
	arrayCounts map[string]*int

	// Cached target of the current table, so that key-values do not need to
	// walk the document structure from the root for every expression.
	// tableFlush holds the write-backs to perform when leaving the table
	// (for targets reached through map values, which are copies).
	// tableParentSlot stores a replacement of the target itself (e.g. a nil
	// map that was allocated) into its parent.
	tableTarget      reflect.Value
	tableTargetValid bool
	tableFlush       []flushOp
	tableParentSlot  slotWriter

	// strKey is a reusable string value used as map key, so that map
	// operations with string keys do not need to allocate a boxed key for
	// every access. It must be refreshed with stringMapKey immediately
	// before each use: any recursive call may overwrite it.
	strKey reflect.Value

	// interned de-duplicates key strings: documents repeat the same keys
	// over and over, and the table survives pooling, so repeated decodes
	// of similar documents stop allocating key strings altogether.
	interned map[string]string

	// pathScratch is the buffer used by joinPath.
	pathScratch []byte

	// keyParts is the reusable buffer holding the decoded parts of the key of
	// the current expression in the fused generic decode path.
	keyParts [][]byte
}

// slotWriter remembers how to store a value at some location of the target
// structure. Implemented as a struct instead of a closure to avoid
// allocations.
type slotWriter struct {
	kind uint8 // 0: none, 1: slot.Set, 2: m.SetMapIndex(k, ...), 3: m.SetMapIndex(string key ks, ...)
	slot reflect.Value
	m    reflect.Value
	k    reflect.Value
	ks   string
}

func (d *decoder) storeSlot(s *slotWriter, nv reflect.Value) {
	switch s.kind {
	case 1:
		if s.slot.CanSet() {
			s.slot.Set(nv)
		}
	case 2:
		s.m.SetMapIndex(s.k, nv)
	case 3:
		s.m.SetMapIndex(d.stringMapKey(s.ks), nv)
	}
}

// flushOp stores val using w when the table is flushed.
type flushOp struct {
	w   slotWriter
	val reflect.Value
}

// flushTable performs the pending write-backs of the cached table target, in
// reverse order so that inner copies land before their parents are stored.
func (d *decoder) flushTable() {
	for i := len(d.tableFlush) - 1; i >= 0; i-- {
		d.storeSlot(&d.tableFlush[i].w, d.tableFlush[i].val)
	}
	d.tableFlush = d.tableFlush[:0]
	d.tableTargetValid = false
	d.tableParentSlot = slotWriter{}
	d.tableTarget = reflect.Value{}
}

// intern returns the string corresponding to the given bytes, reusing a
// previous allocation when the same key has been seen before.
func (d *decoder) intern(b []byte) string {
	if s, ok := d.interned[string(b)]; ok { // does not allocate
		return s
	}
	if d.interned == nil {
		d.interned = make(map[string]string, 64)
	} else if len(d.interned) >= 1<<14 {
		// Safety valve for adversarial inputs: do not let the table grow
		// without bounds.
		for k := range d.interned {
			delete(d.interned, k)
		}
	}
	s := string(b)
	d.interned[s] = s
	return s
}

// partString returns the name of a path part, interning it when it comes
// from the document.
func (d *decoder) partString(p *pathPart) string {
	if p.node != nil {
		return d.intern(p.node.Data)
	}
	return p.name
}

// stringMapKey returns a reflect.Value holding the given string, reusing the
// same allocation every time. The result must be used (the map operation
// performed) before any recursive call, which may overwrite the buffer.
func (d *decoder) stringMapKey(s string) reflect.Value {
	if !d.strKey.IsValid() {
		d.strKey = reflect.New(stringType).Elem()
	}
	d.strKey.SetString(s)
	return d.strKey
}

// joinPath builds the NUL-joined representation of a key path in the
// decoder's scratch buffer. The result is only valid until the next call.
func (d *decoder) joinPath(parts []string) []byte {
	d.pathScratch = d.pathScratch[:0]
	for i, p := range parts {
		if i > 0 {
			d.pathScratch = append(d.pathScratch, 0)
		}
		d.pathScratch = append(d.pathScratch, p...)
	}
	return d.pathScratch
}

// arrayCount returns the number of elements appended so far to the array
// table at the given path.
func (d *decoder) arrayCount(key []byte) int {
	if d.arrayCounts == nil {
		return 0
	}
	if p := d.arrayCounts[string(key)]; p != nil { // does not allocate
		return *p
	}
	return 0
}

func (d *decoder) setArrayCount(key []byte, n int) {
	if d.arrayCounts == nil {
		d.arrayCounts = map[string]*int{}
	}
	if p := d.arrayCounts[string(key)]; p != nil { // does not allocate
		*p = n
		return
	}
	v := n
	d.arrayCounts[string(key)] = &v
}

// resetChildArrayCounts forgets the counts of all the array tables under
// the given path, so that a new element starts fresh.
func (d *decoder) resetChildArrayCounts(key []byte) {
	if len(d.arrayCounts) == 0 {
		return
	}
	for k, p := range d.arrayCounts {
		// Prefix match without building the prefix string: same bytes as
		// key, followed by the NUL separator.
		if len(k) > len(key) && k[len(key)] == 0 && k[:len(key)] == string(key) {
			// Zero instead of delete: the next element of the parent table
			// will reuse the slot without allocating a new key.
			*p = 0
		}
	}
}

func (d *decoder) typeMismatchError(toml string, target reflect.Type, highlight []byte) error {
	return &typeMismatchError{
		toml:      toml,
		target:    target,
		highlight: highlight,
	}
}

type typeMismatchError struct {
	toml      string
	target    reflect.Type
	highlight []byte
	// key is the TOML key being processed when the mismatch occurred. It is
	// populated lazily as the error propagates back up to the key-value
	// handler (see contextualizeError).
	key Key
}

func (e *typeMismatchError) Error() string {
	return fmt.Sprintf("cannot decode TOML %s into %s", e.toml, e.target)
}

// contextualizeError attaches the TOML key currently being processed to errors
// raised while decoding a key-value expression, so that DecodeError.Key()
// reports the offending key (e.g. on type mismatch errors). The current key is
// reconstructed from d.path; when the table target is cached, d.path holds only
// the key-value parts, so the table key prefix is prepended. This only runs on
// the error path and adds no cost to successful decodes.
func (d *decoder) contextualizeError(err error, withTableKey bool) error {
	var mm *typeMismatchError
	if errors.As(err, &mm) {
		if mm.key == nil {
			mm.key = d.currentKey(withTableKey)
		}
		return err
	}
	var perr *unstable.ParserError
	if errors.As(err, &perr) {
		if perr.Key == nil {
			perr.Key = d.currentKey(withTableKey)
		}
	}
	return err
}

// currentKey reconstructs the full TOML key being processed from the decoder's
// path. When withTableKey is true, d.path contains only the key-value parts
// (the table target is cached) and the table key is prepended.
func (d *decoder) currentKey(withTableKey bool) Key {
	n := len(d.path)
	if withTableKey {
		n += len(d.tableKey)
	}
	key := make(Key, 0, n)
	if withTableKey {
		key = append(key, d.tableKey...)
	}
	for i := range d.path {
		key = append(key, d.path[i].str())
	}
	return key
}

func (d *decoder) unmarshal(data []byte, v interface{}) error {
	r := reflect.ValueOf(v)
	if r.Kind() != reflect.Ptr {
		return fmt.Errorf("toml: decoding can only be performed into a pointer, not %s", r.Kind())
	}
	if r.IsNil() {
		return errors.New("toml: decoding pointer target cannot be nil")
	}

	root := r.Elem()

	d.captureIdx = -1
	d.p.Reset(data)

	// Fully generic targets (interface{} or map[string]interface{}) are
	// decoded straight into native Go maps and slices, with no reflection on
	// the document structure at all. This covers the common "decode arbitrary
	// TOML into a map" case, including every standard benchmark dataset.
	if !d.unmarshalerInterface {
		if k := root.Kind(); k == reflect.Interface || (k == reflect.Map && root.Type() == mapStringInterfaceType) {
			return d.unmarshalFused(root, data)
		}
	}

	for d.p.NextExpression() {
		err := d.handleRootExpression(d.p.Expression(), root)
		if err != nil {
			return d.wrapError(data, err)
		}
	}
	if err := d.p.Error(); err != nil {
		var perr *unstable.ParserError
		if errors.As(err, &perr) {
			return wrapDecodeError(data, perr)
		}
		return err
	}

	d.flushTable()

	// Deliver the accumulated raw documents to the unmarshaler-interface
	// targets.
	for i := range d.captures {
		nv, err := d.resolveCapture(root, &d.captures[i], 0, false)
		if err != nil {
			return err
		}
		if nv.IsValid() {
			root.Set(nv)
		}
	}

	// An empty document into a generic target still initializes it.
	switch root.Kind() {
	case reflect.Map:
		if root.IsNil() {
			root.Set(reflect.MakeMap(root.Type()))
		}
	case reflect.Interface:
		if root.IsNil() {
			root.Set(reflect.ValueOf(map[string]interface{}{}))
		}
	default:
	}

	return d.strict.Error(data)
}

// setAnyKey assigns the value of a key-value into the native map m, following
// the (possibly dotted) key and creating intermediate maps as needed.
func (d *decoder) setAnyKey(m map[string]interface{}, key unstable.Iterator, value *unstable.Node) error {
	cur := m
	for key.Next() {
		name := d.intern(key.Node().Data)
		if key.IsLast() {
			av, err := d.decodeAny(value)
			if err != nil {
				return err
			}
			cur[name] = av
			return nil
		}
		cur = d.anyChildTable(cur, name)
	}
	return nil
}

// anyChildTable returns the child table at name within cur, creating it if
// absent and descending into the current (last) element when an array table
// occupies the slot. A non-container in the slot cannot occur for a document
// the seen-tracker has accepted.
func (d *decoder) anyChildTable(cur map[string]interface{}, name string) map[string]interface{} {
	switch v := cur[name].(type) {
	case map[string]interface{}:
		return v
	case []interface{}:
		if len(v) > 0 {
			if last, ok := v[len(v)-1].(map[string]interface{}); ok {
				return last
			}
		}
	}
	nm := map[string]interface{}{}
	cur[name] = nm
	return nm
}

// wrapError gives document context to errors generated while processing an
// expression.
func (d *decoder) wrapError(data []byte, err error) error {
	var perr *unstable.ParserError
	if errors.As(err, &perr) {
		return wrapDecodeError(data, perr)
	}
	var mm *typeMismatchError
	if errors.As(err, &mm) {
		return wrapDecodeError(data, &unstable.ParserError{
			Highlight: mm.highlight,
			Message:   mm.Error(),
			Key:       mm.key,
		})
	}
	return err
}

// wrapSeenError turns an error returned by SeenTracker.CheckExpression into a
// ParserError carrying the position and key of the offending expression, so
// that redefinition and duplicate-key errors are reported as a DecodeError
// with context (see issue #668).
//
// The highlight spans the expression's key. Unlike Node.Raw, key nodes always
// carry a Raw range, so this works for tables and array tables too (whose own
// Raw range is not set by the parser). For a duplicate detected inside an
// inline table, node is the enclosing key-value expression, so the error
// points at that expression's key.
func (d *decoder) wrapSeenError(node *unstable.Node, err error) error {
	if err == nil {
		return nil
	}

	var key Key
	var start, end unstable.Range
	it := node.Key()
	for it.Next() {
		n := it.Node()
		key = append(key, string(n.Data))
		if len(key) == 1 {
			start = n.Raw
		}
		end = n.Raw
	}

	var highlight []byte
	if len(key) > 0 {
		highlight = d.p.Raw(unstable.Range{
			Offset: start.Offset,
			Length: end.Offset + end.Length - start.Offset,
		})
	}

	return &unstable.ParserError{
		Highlight: highlight,
		Message:   strings.TrimPrefix(err.Error(), "toml: "),
		Key:       key,
	}
}

func (d *decoder) handleRootExpression(expr *unstable.Node, root reflect.Value) error {
	first, err := d.seen.CheckExpression(expr)
	if err != nil {
		return d.wrapSeenError(expr, err)
	}

	switch expr.Kind {
	case unstable.KeyValue:
		if d.skipUntilTable {
			return nil
		}
		if d.captureIdx >= 0 {
			d.captureKeyValue(expr)
			return nil
		}
		return d.handleKeyValueExpression(expr, root)
	case unstable.Table:
		d.flushTable()
		d.skipUntilTable = false
		d.captureIdx = -1
		d.strict.EnterTable(expr)
		return d.handleTableExpression(expr, root, false, first)
	case unstable.ArrayTable:
		d.flushTable()
		d.skipUntilTable = false
		d.captureIdx = -1
		d.strict.EnterTable(expr)
		return d.handleTableExpression(expr, root, true, first)
	default:
		return unstable.NewParserError(expr.Data, "unsupported expression kind %s", expr.Kind)
	}
}

// updateTableKey copies the parts of the key of a table expression into
// tableKey.
func (d *decoder) updateTableKey(expr *unstable.Node) {
	d.tableKey = d.tableKey[:0]
	it := expr.Key()
	for it.Next() {
		d.tableKey = append(d.tableKey, d.intern(it.Node().Data))
	}
}

func (d *decoder) handleTableExpression(expr *unstable.Node, root reflect.Value, isArrayTable bool, first bool) error {
	d.updateTableKey(expr)

	// Check whether this table belongs to an exisiting raw capture (split
	// tables, or children of a table assigned to an Unmarshaler).
	if d.unmarshalerInterface {
		if d.resumeCapture(expr) {
			return nil
		}
	}

	// Reset the per-segment array indexes.
	d.segIdx = d.segIdx[:0]
	for i := 0; i <= len(d.tableKey); i++ {
		d.segIdx = append(d.segIdx, -1)
	}

	return d.walkTable(root, expr, isArrayTable, first)
}

// newContainerElem returns a fresh element for a slice of the given element
// type. Plain interface elements start out as an empty table.
func newContainerElem(et reflect.Type) reflect.Value {
	if et == interfaceType {
		return reflect.ValueOf(map[string]interface{}{})
	}
	return reflect.New(et).Elem()
}

// walkTable processes a [table] or [[array table]] header: it creates the
// intermediate containers, appends array-table elements, applies the strict
// policy, registers unmarshaler-interface captures, and caches the target
// container so that the key-values that follow are stored directly.
//
// Map values are not addressable: when one needs in-place mutations (struct
// or array values), a copy is made and registered to be stored back when the
// table changes (see flushTable). Maps and slices are references and are
// traversed without copies.
func (d *decoder) walkTable(root reflect.Value, expr *unstable.Node, isArrayTable bool, first bool) error {
	v := root
	pf := slotWriter{kind: 1, slot: root}
	idx := 0

walk:
	for {
		// Dereference pointers in place.
		for v.Kind() == reflect.Ptr {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			elem := v.Elem()
			pf = slotWriter{kind: 1, slot: elem}
			v = elem
		}

		// Tables assigned to a type implementing the unmarshaler interface
		// are captured as raw bytes, delivered once the document is read.
		if d.unmarshalerInterface && hasUnmarshaler(v) {
			d.startCapture(idx, expr)
			return nil
		}

		if idx >= len(d.tableKey) {
			break walk
		}

		name := d.tableKey[idx]

		switch v.Kind() {
		case reflect.Interface:
			if !v.IsNil() {
				c := v.Elem()
				if k := c.Kind(); k == reflect.Map || k == reflect.Slice {
					// Reference types: mutations are visible through the
					// existing interface value.
					v = c
					continue
				}
			}
			// Anything else is replaced by a fresh generic map.
			if !mapStringInterfaceType.AssignableTo(v.Type()) {
				return unstable.NewParserError(d.p.Raw(expr.Raw), "cannot store a table in a %s", v.Type())
			}
			fresh := reflect.ValueOf(map[string]interface{}{})
			d.storeSlot(&pf, fresh)
			v = fresh
		case reflect.Slice:
			if v.Len() == 0 {
				// Implicit creation of the first element: the array table
				// that would create it has not been seen yet (issue 995).
				if v.IsNil() {
					v = reflect.MakeSlice(v.Type(), 0, 4)
				}
				v = reflect.Append(v, newContainerElem(v.Type().Elem()))
				d.storeSlot(&pf, v)
			}
			n := v.Len() - 1
			d.segIdx[idx] = n
			elem := v.Index(n)
			pf = slotWriter{kind: 1, slot: elem}
			v = elem
		case reflect.Array:
			key := d.joinPath(d.tableKey[:idx])
			cnt := d.arrayCount(key)
			if cnt == 0 {
				cnt = 1
				d.setArrayCount(key, 1)
			}
			if cnt > v.Len() {
				return unstable.NewParserError(d.p.Raw(expr.Raw), "cannot reach element %d of array of size %d", cnt-1, v.Len())
			}
			d.segIdx[idx] = cnt - 1
			elem := v.Index(cnt - 1)
			pf = slotWriter{kind: 1, slot: elem}
			v = elem
		case reflect.Map:
			if v.IsNil() {
				nm := reflect.MakeMap(v.Type())
				d.storeSlot(&pf, nm)
				v = nm
			}
			var key reflect.Value
			var w slotWriter
			if v.Type().Key() == stringType {
				key = d.stringMapKey(name)
				w = slotWriter{kind: 3, m: v, ks: name}
			} else {
				k, err := makeMapKey(v.Type().Key(), name)
				if err != nil {
					return err
				}
				key = k
				w = slotWriter{kind: 2, m: v, k: k}
			}

			elem := v.MapIndex(key)

			// The last part of an array table is finalized as a slice
			// container: do not materialize a table for it.
			if isArrayTable && idx == len(d.tableKey)-1 {
				et := v.Type().Elem()
				switch et.Kind() {
				case reflect.Interface, reflect.Slice, reflect.Array:
					if elem.IsValid() {
						v = elem
					} else {
						v = reflect.Zero(et)
					}
					pf = w
					idx++
					continue
				default:
				}
			}

			if elem.IsValid() {
				ce := elem
				ceIface := false
				if ce.Kind() == reflect.Interface {
					ceIface = true
					if !ce.IsNil() {
						ce = ce.Elem()
					}
				}
				switch ce.Kind() {
				case reflect.Map, reflect.Slice:
					pf = w
					v = ce
				case reflect.Ptr:
					if ce.IsNil() {
						np := reflect.New(ce.Type().Elem())
						d.storeSlot(&w, np)
						ce = np
					}
					pf = w
					v = ce
				case reflect.Struct, reflect.Array:
					if ceIface {
						// Interface-held non-generic content is replaced.
						fresh := reflect.ValueOf(map[string]interface{}{})
						d.storeSlot(&w, fresh)
						pf = w
						v = fresh
					} else {
						tmp := reflect.New(elem.Type()).Elem()
						tmp.Set(elem)
						d.tableFlush = append(d.tableFlush, flushOp{w: w, val: tmp})
						pf = slotWriter{kind: 1, slot: tmp}
						v = tmp
					}
				default:
					if !ceIface {
						return unstable.NewParserError(d.p.Raw(expr.Raw), "cannot store a table in a %s", ce.Type())
					}
					fresh := reflect.ValueOf(map[string]interface{}{})
					d.storeSlot(&w, fresh)
					pf = w
					v = fresh
				}
			} else {
				et := v.Type().Elem()
				switch et.Kind() {
				case reflect.Interface:
					if !mapStringInterfaceType.AssignableTo(et) {
						return unstable.NewParserError(d.p.Raw(expr.Raw), "cannot store a table in a %s", et)
					}
					fresh := reflect.ValueOf(map[string]interface{}{})
					d.storeSlot(&w, fresh)
					pf = w
					v = fresh
				case reflect.Map:
					nm := reflect.MakeMap(et)
					d.storeSlot(&w, nm)
					pf = w
					v = nm
				case reflect.Ptr:
					np := reflect.New(et.Elem())
					d.storeSlot(&w, np)
					pf = w
					v = np
				case reflect.Struct, reflect.Array, reflect.Slice:
					tmp := reflect.New(et).Elem()
					d.tableFlush = append(d.tableFlush, flushOp{w: w, val: tmp})
					pf = slotWriter{kind: 1, slot: tmp}
					v = tmp
				default:
					return unstable.NewParserError(d.p.Raw(expr.Raw), "cannot store a table in a %s", et)
				}
			}
			idx++
		case reflect.Struct:
			plan := planForType(v.Type())
			f, found := plan.lookup(name)
			if !found {
				d.strict.MissingTable(expr)
				d.skipUntilTable = true
				return nil
			}
			fv := fieldByIndexAlloc(v, f.index)
			pf = slotWriter{kind: 1, slot: fv}
			v = fv
			idx++
		default:
			return unstable.NewParserError(d.p.Raw(expr.Raw), "cannot store a table in a %s", v.Kind())
		}
	}

	if isArrayTable {
		akey := d.joinPath(d.tableKey)
		d.resetChildArrayCounts(akey)

		// Unwrap an interface container.
		if v.Kind() == reflect.Interface {
			var slice []interface{}
			if !v.IsNil() {
				if s, ok := v.Elem().Interface().([]interface{}); ok {
					slice = s
				}
			}
			if first {
				slice = slice[:0]
			}
			m := map[string]interface{}{}
			slice = append(slice, m)
			sv := reflect.ValueOf(slice)
			d.storeSlot(&pf, sv)
			d.setArrayCount(akey, len(slice))
			d.segIdx[len(d.tableKey)] = len(slice) - 1
			d.tableTarget = reflect.ValueOf(m)
			d.tableParentSlot = slotWriter{kind: 1, slot: sv.Index(len(slice) - 1)}
			d.tableTargetValid = true
			return nil
		}

		switch v.Kind() {
		case reflect.Slice:
			if v.IsNil() {
				v = reflect.MakeSlice(v.Type(), 0, 4)
			} else if first {
				v = v.Slice(0, 0)
			}
			v = reflect.Append(v, newContainerElem(v.Type().Elem()))
			d.storeSlot(&pf, v)
			n := v.Len() - 1
			d.setArrayCount(akey, n+1)
			d.segIdx[len(d.tableKey)] = n
			elem := v.Index(n)
			if d.unmarshalerInterface && hasUnmarshaler(elem) {
				d.startCapture(len(d.tableKey), expr)
				return nil
			}
			pf = slotWriter{kind: 1, slot: elem}
			v = elem
		case reflect.Array:
			cnt := d.arrayCount(akey)
			if first {
				cnt = 0
			}
			if cnt >= v.Len() {
				return unstable.NewParserError(d.p.Raw(expr.Raw), "array of size %d is too small to store this array table", v.Len())
			}
			v.Index(cnt).Set(reflect.Zero(v.Type().Elem()))
			d.setArrayCount(akey, cnt+1)
			d.segIdx[len(d.tableKey)] = cnt
			elem := v.Index(cnt)
			if d.unmarshalerInterface && hasUnmarshaler(elem) {
				d.startCapture(len(d.tableKey), expr)
				return nil
			}
			pf = slotWriter{kind: 1, slot: elem}
			v = elem
		default:
			return fmt.Errorf("toml: cannot store an array table in a %s", v.Kind())
		}
	}

	// Settle on the concrete container for the key-values that follow.
	for {
		switch v.Kind() {
		case reflect.Ptr:
			if v.IsNil() {
				if !v.CanSet() {
					return nil
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			elem := v.Elem()
			pf = slotWriter{kind: 1, slot: elem}
			v = elem
			continue
		case reflect.Interface:
			if !v.IsNil() {
				c := v.Elem()
				if c.Type() == mapStringInterfaceType || c.Type() == sliceInterfaceType {
					v = c
					continue
				}
			}
			if !mapStringInterfaceType.AssignableTo(v.Type()) {
				return fmt.Errorf("toml: cannot store a table in a %s", v.Type())
			}
			fresh := reflect.ValueOf(map[string]interface{}{})
			d.storeSlot(&pf, fresh)
			v = fresh
			continue
		case reflect.Slice:
			if v.Len() == 0 {
				if v.IsNil() {
					v = reflect.MakeSlice(v.Type(), 0, 4)
				}
				v = reflect.Append(v, newContainerElem(v.Type().Elem()))
				d.storeSlot(&pf, v)
			}
			n := v.Len() - 1
			d.segIdx[len(d.tableKey)] = n
			elem := v.Index(n)
			pf = slotWriter{kind: 1, slot: elem}
			v = elem
			continue
		case reflect.Map, reflect.Struct:
			d.tableTarget = v
			d.tableParentSlot = pf
			d.tableTargetValid = true
			return nil
		default:
			return fmt.Errorf("toml: cannot store a table in a %s", v.Kind())
		}
	}
}

// resumeCapture looks for an existing capture this table expression belongs
// to. It returns true if the expression was consumed.
func (d *decoder) resumeCapture(expr *unstable.Node) bool {
	// Iterate in reverse, so that tables attach to the latest element of
	// array tables.
	for i := len(d.captures) - 1; i >= 0; i-- {
		c := &d.captures[i]
		if len(d.tableKey) < len(c.names) {
			continue
		}
		if expr.Kind == unstable.ArrayTable && len(d.tableKey) == len(c.names) {
			// A new element of an array table is not part of the capture of
			// the previous element.
			continue
		}
		match := true
		for j, p := range c.names {
			if d.tableKey[j] != p {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		d.captureIdx = i
		if len(d.tableKey) > len(c.names) {
			d.appendCaptureHeader(c, expr, len(c.names))
		}
		return true
	}
	return false
}

// appendCaptureHeader writes the table header of expr in the capture buffer,
// adjusted to be relative to the capture root.
func (d *decoder) appendCaptureHeader(c *rawCapture, expr *unstable.Node, skip int) {
	c.buf = append(c.buf, '[')
	if expr.Kind == unstable.ArrayTable {
		c.buf = append(c.buf, '[')
	}
	c.buf = append(c.buf, d.rawKeySuffix(expr, skip)...)
	c.buf = append(c.buf, ']')
	if expr.Kind == unstable.ArrayTable {
		c.buf = append(c.buf, ']')
	}
	c.buf = append(c.buf, '\n')
}

// rawKeySuffix returns the raw bytes of the key of the expression, skipping
// the first n parts.
func (d *decoder) rawKeySuffix(expr *unstable.Node, n int) []byte {
	it := expr.Key()
	idx := 0
	var start, end unstable.Range
	for it.Next() {
		if idx >= n {
			r := it.Node().Raw
			if start.Length == 0 && start.Offset == 0 && idx == n {
				start = r
			}
			end = r
		}
		idx++
	}
	return d.p.Data()[start.Offset : end.Offset+end.Length]
}

// startCapture registers a new capture for the table at the given path
// (prefix of tableKey).
func (d *decoder) startCapture(pathLen int, expr *unstable.Node) {
	names := make([]string, pathLen)
	copy(names, d.tableKey[:pathLen])
	indexes := make([]int, pathLen+1)
	copy(indexes, d.segIdx[:pathLen+1])
	d.captures = append(d.captures, rawCapture{
		names:   names,
		indexes: indexes,
	})
	d.captureIdx = len(d.captures) - 1
	if pathLen < len(d.tableKey) {
		d.appendCaptureHeader(&d.captures[d.captureIdx], expr, pathLen)
	}
}

// resolveCapture walks back to the target of a capture and delivers the
// accumulated raw bytes to its UnmarshalTOML implementation.
func (d *decoder) resolveCapture(v reflect.Value, c *rawCapture, idx int, indexed bool) (reflect.Value, error) {
	if v.Kind() == reflect.Ptr {
		if v.Type().Implements(unmarshalerType) && idx == len(c.names) {
			u, _ := unmarshalerOf(v)
			return v, u.UnmarshalTOML(c.buf)
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		nv, err := d.resolveCapture(v.Elem(), c, idx, indexed)
		if err != nil {
			return reflect.Value{}, err
		}
		if nv.IsValid() {
			v.Elem().Set(nv)
		}
		return v, nil
	}

	if !indexed && (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) && c.indexes[idx] >= 0 {
		i := c.indexes[idx]
		if i >= v.Len() {
			return reflect.Value{}, errors.New("toml: internal error: capture index out of range")
		}
		elem := v.Index(i)
		nv, err := d.resolveCapture(elem, c, idx, true)
		if err != nil {
			return reflect.Value{}, err
		}
		if nv.IsValid() {
			elem.Set(nv)
		}
		return v, nil
	}

	if idx == len(c.names) {
		u, ok := unmarshalerOf(v)
		if !ok {
			return reflect.Value{}, errors.New("toml: internal error: capture target does not implement UnmarshalTOML")
		}
		return v, u.UnmarshalTOML(c.buf)
	}

	name := c.names[idx]

	switch v.Kind() {
	case reflect.Struct:
		plan := planForType(v.Type())
		f, found := plan.lookup(name)
		if !found {
			return v, nil
		}
		fv := fieldByIndexAlloc(v, f.index)
		nv, err := d.resolveCapture(fv, c, idx+1, false)
		if err != nil {
			return reflect.Value{}, err
		}
		if nv.IsValid() && fv.CanSet() {
			fv.Set(nv)
		}
		return v, nil
	case reflect.Map:
		key, err := makeMapKey(v.Type().Key(), name)
		if err != nil {
			return reflect.Value{}, err
		}
		if v.IsNil() {
			v = reflect.MakeMap(v.Type())
		}
		elem := reflect.New(v.Type().Elem()).Elem()
		if existing := v.MapIndex(key); existing.IsValid() {
			elem.Set(existing)
		}
		nv, err := d.resolveCapture(elem, c, idx+1, false)
		if err != nil {
			return reflect.Value{}, err
		}
		if nv.IsValid() {
			v.SetMapIndex(key, nv)
		}
		return v, nil
	case reflect.Interface:
		elem := elemOrNewMap(v)
		nv, err := d.resolveCapture(elem, c, idx, indexed)
		if err != nil || !nv.IsValid() {
			return reflect.Value{}, err
		}
		return nv, nil
	default:
		return reflect.Value{}, fmt.Errorf("toml: internal error: cannot resolve capture target through %s", v.Kind())
	}
}

// captureKeyValue appends the raw bytes of a key-value expression to the
// current capture.
func (d *decoder) captureKeyValue(expr *unstable.Node) {
	c := &d.captures[d.captureIdx]
	c.buf = append(c.buf, d.p.Raw(expr.Raw)...)
	c.buf = append(c.buf, '\n')
}

// hasUnmarshaler reports whether v can provide an unstable.Unmarshaler,
// without allocating anything.
func hasUnmarshaler(v reflect.Value) bool {
	t := v.Type()
	return t.Implements(unmarshalerType) || (v.CanAddr() && reflect.PtrTo(t).Implements(unmarshalerType))
}

// makeMapKey converts a TOML key into a value usable as the given map key
// type.
func makeMapKey(kt reflect.Type, name string) (reflect.Value, error) {
	switch kt.Kind() {
	case reflect.String:
		return reflect.ValueOf(name).Convert(kt), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(name, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("toml: cannot parse map key %q as %s: %w", name, kt, err)
		}
		k := reflect.New(kt).Elem()
		if k.OverflowInt(i) {
			return reflect.Value{}, fmt.Errorf("toml: map key %q overflows %s", name, kt)
		}
		k.SetInt(i)
		return k, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u, err := strconv.ParseUint(name, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("toml: cannot parse map key %q as %s: %w", name, kt, err)
		}
		k := reflect.New(kt).Elem()
		if k.OverflowUint(u) {
			return reflect.Value{}, fmt.Errorf("toml: map key %q overflows %s", name, kt)
		}
		k.SetUint(u)
		return k, nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(name, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("toml: cannot parse map key %q as %s: %w", name, kt, err)
		}
		k := reflect.New(kt).Elem()
		k.SetFloat(f)
		return k, nil
	case reflect.Ptr:
		if kt.Implements(textUnmarshalerType) {
			k := reflect.New(kt.Elem())
			err := k.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(name))
			if err != nil {
				return reflect.Value{}, fmt.Errorf("toml: error unmarshaling map key %q: %w", name, err)
			}
			return k, nil
		}
	default:
		if reflect.PtrTo(kt).Implements(textUnmarshalerType) {
			k := reflect.New(kt)
			err := k.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(name))
			if err != nil {
				return reflect.Value{}, fmt.Errorf("toml: error unmarshaling map key %q: %w", name, err)
			}
			return k.Elem(), nil
		}
	}
	return reflect.Value{}, fmt.Errorf("toml: cannot decode a key into a map with key type %s", kt)
}

// elemOrNewMap unwraps an interface value to descend into it. Contents that
// can hold a table (generic maps and slices) are kept; anything else is
// replaced by a fresh map[string]interface{}. Maps and slices are reference
// types: they are returned directly, not copied.
func elemOrNewMap(v reflect.Value) reflect.Value {
	if !v.IsNil() {
		concrete := v.Elem()
		t := concrete.Type()
		if t == mapStringInterfaceType || t == sliceInterfaceType {
			return concrete
		}
	}
	return reflect.ValueOf(map[string]interface{}{})
}

// handleKeyValueExpression stores the value of a top-level key-value
// expression, relative to the current table.
func (d *decoder) handleKeyValueExpression(expr *unstable.Node, root reflect.Value) error {
	d.path = d.path[:0]

	target := root
	useCache := d.tableTargetValid && len(d.tableKey) > 0
	if useCache {
		target = d.tableTarget
	} else {
		for _, name := range d.tableKey {
			d.path = append(d.path, pathPart{name: name})
		}
	}

	it := expr.Key()
	for it.Next() {
		d.path = append(d.path, pathPart{node: it.Node()})
	}

	nv, err := d.descend(target, d.path, 0, expr, expr.Value())
	if err != nil {
		return d.contextualizeError(err, useCache)
	}
	if !nv.IsValid() {
		return nil
	}
	if useCache {
		// The target may have been replaced (e.g. a nil map allocated):
		// re-link it into its parent.
		if nv.Kind() == reflect.Map && nv.Pointer() != d.tableTarget.Pointer() {
			d.storeSlot(&d.tableParentSlot, nv)
			d.tableTarget = nv
		}
	} else {
		if root.CanSet() {
			root.Set(nv)
		}
	}
	return nil
}

// descend walks the given key path into v, and assigns the value at the
// end. It returns the value to store back at this level. An invalid value
// means nothing should be stored (e.g. unknown field).
func (d *decoder) descend(v reflect.Value, path []pathPart, idx int, expr *unstable.Node, value *unstable.Node) (reflect.Value, error) {
	if idx == len(path) {
		return d.assignValue(v, expr, value)
	}

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		nv, err := d.descend(v.Elem(), path, idx, expr, value)
		if err != nil || !nv.IsValid() {
			return reflect.Value{}, err
		}
		v.Elem().Set(nv)
		return v, nil
	}

	// A target implementing the unmarshaler interface consumes the value,
	// whatever the remaining parts of the key are.
	if d.unmarshalerInterface {
		if u, ok := unmarshalerOf(v); ok {
			return v, u.UnmarshalTOML(d.rawValue(expr, value))
		}
	}

	part := path[idx]

	switch v.Kind() {
	case reflect.Map:
		// Native fast path for the most common generic target: walk the
		// remaining dotted-key path with plain Go map operations and decode
		// the value directly, skipping the reflect.Value round-trips
		// (stringMapKey, MapIndex, New, SetMapIndex) entirely.
		if !d.unmarshalerInterface && v.Type() == mapStringInterfaceType {
			return d.descendStrMap(v, path, idx, value)
		}
		var name string
		var key reflect.Value
		var err error
		fastKey := v.Type().Key() == stringType
		if fastKey {
			name = d.partString(&part)
			key = d.stringMapKey(name)
		} else {
			key, err = makeMapKey(v.Type().Key(), d.partString(&part))
			if err != nil {
				return reflect.Value{}, err
			}
		}
		if v.IsNil() {
			v = reflect.MakeMap(v.Type())
		}
		elemType := v.Type().Elem()
		existing := v.MapIndex(key)
		var elem reflect.Value
		switch {
		case existing.IsValid():
			elem = reflect.New(elemType).Elem()
			elem.Set(existing)
		case idx+1 == len(path) && elemType.Kind() == reflect.Interface:
			// Fast path: a fresh interface element does not need to be
			// materialized, the assigned value is stored directly.
			elem = reflect.Zero(elemType)
		default:
			elem = reflect.New(elemType).Elem()
		}
		nv, err := d.descend(elem, path, idx+1, expr, value)
		if err != nil {
			return reflect.Value{}, err
		}
		if nv.IsValid() {
			if fastKey {
				// The recursion may have overwritten the key buffer.
				key = d.stringMapKey(name)
			}
			v.SetMapIndex(key, nv)
		}
		return v, nil
	case reflect.Struct:
		plan := planForType(v.Type())
		f, found := plan.lookupBytes(part.bytes())
		if !found {
			if part.node != nil {
				d.strict.MissingField(expr)
			}
			return v, nil
		}
		fv := fieldByIndexAlloc(v, f.index)
		var nv reflect.Value
		var err error
		if idx+1 == len(path) {
			// Leaf field: assign directly. descend's first action for a
			// fully-consumed path is exactly this call, so skipping the extra
			// frame is equivalent and avoids a call per scalar field.
			nv, err = d.assignValue(fv, expr, value)
		} else {
			nv, err = d.descend(fv, path, idx+1, expr, value)
		}
		if err != nil {
			var mm *typeMismatchError
			if errors.As(err, &mm) {
				err = &unstable.ParserError{
					Highlight: mm.highlight,
					Message: fmt.Sprintf("cannot decode TOML %s into struct field %s.%s of type %s",
						mm.toml, v.Type(), f.fieldName, mm.target),
				}
			}
			return reflect.Value{}, err
		}
		if nv.IsValid() && fv.CanSet() {
			fv.Set(nv)
		}
		return v, nil
	case reflect.Interface:
		elem := elemOrNewMap(v)
		nv, err := d.descend(elem, path, idx, expr, value)
		if err != nil || !nv.IsValid() {
			return reflect.Value{}, err
		}
		return nv, nil
	case reflect.Slice:
		if v.Len() == 0 {
			if v.IsNil() {
				v = reflect.MakeSlice(v.Type(), 0, 4)
			}
			v = reflect.Append(v, reflect.New(v.Type().Elem()).Elem())
		}
		elem := v.Index(v.Len() - 1)
		nv, err := d.descend(elem, path, idx, expr, value)
		if err != nil {
			return reflect.Value{}, err
		}
		if nv.IsValid() {
			elem.Set(nv)
		}
		return v, nil
	case reflect.Array:
		names := make([]string, idx)
		for i := range names {
			names[i] = path[i].str()
		}
		cnt := d.arrayCount(d.joinPath(names))
		if cnt == 0 {
			cnt = 1
		}
		elemIdx := cnt - 1
		if elemIdx >= v.Len() {
			return reflect.Value{}, unstable.NewParserError(keyHighlight(d.p.Data(), part.node),
				"cannot reach element %d of array of size %d", elemIdx, v.Len())
		}
		elem := v.Index(elemIdx)
		nv, err := d.descend(elem, path, idx, expr, value)
		if err != nil {
			return reflect.Value{}, err
		}
		if nv.IsValid() {
			elem.Set(nv)
		}
		return v, nil
	default:
		return reflect.Value{}, d.typeMismatchError("table", v.Type(), keyHighlight(d.p.Data(), part.node))
	}
}

// descendStrMap assigns into a native map[string]interface{} target, following
// the remaining dotted-key parts with plain Go map operations and decoding the
// value with decodeAny. It returns the map to store back at this level: a new
// map when v was nil, otherwise v unchanged, since maps are reference types and
// are mutated in place.
func (d *decoder) descendStrMap(v reflect.Value, path []pathPart, idx int, value *unstable.Node) (reflect.Value, error) {
	var m map[string]interface{}
	if v.IsNil() {
		m = make(map[string]interface{})
		v = reflect.ValueOf(m)
	} else {
		m = v.Interface().(map[string]interface{})
	}

	// Walk intermediate parts, creating or reusing nested generic maps. A
	// non-map value at an intermediate key can only occur in a document the
	// seen-tracker has already rejected; replacing it mirrors the reflect
	// path (elemOrNewMap).
	for ; idx < len(path)-1; idx++ {
		name := d.partString(&path[idx])
		child, _ := m[name].(map[string]interface{})
		if child == nil {
			child = make(map[string]interface{})
			m[name] = child
		}
		m = child
	}

	av, err := d.decodeAny(value)
	if err != nil {
		return reflect.Value{}, err
	}
	m[d.partString(&path[idx])] = av
	return v, nil
}

// keyHighlight returns a highlight for the given key part node, falling back
// to the start of the document.
func keyHighlight(doc []byte, node *unstable.Node) []byte {
	if node == nil {
		return doc[0:0]
	}
	return doc[node.Raw.Offset : node.Raw.Offset+node.Raw.Length]
}

// rawValue returns the raw bytes of the value of a key-value expression.
func (d *decoder) rawValue(expr *unstable.Node, value *unstable.Node) []byte {
	if value.Kind != unstable.InlineTable && value.Kind != unstable.Array {
		return d.p.Raw(value.Raw)
	}
	if expr == nil || expr.Kind != unstable.KeyValue {
		// Inline container nested in another container: best effort.
		return d.p.Raw(value.Raw)
	}
	// Reconstruct the span of the value: it starts after the equal sign
	// following the last part of the key, and stops at the end of the
	// expression.
	var last unstable.Range
	it := expr.Key()
	for it.Next() {
		last = it.Node().Raw
	}
	doc := d.p.Data()
	i := int(last.Offset + last.Length)
	for i < len(doc) && (doc[i] == ' ' || doc[i] == '\t') {
		i++
	}
	i++ // equal sign
	for i < len(doc) && (doc[i] == ' ' || doc[i] == '\t') {
		i++
	}
	end := int(expr.Raw.Offset + expr.Raw.Length)
	return doc[i:end]
}

// unmarshalerOf returns the unstable.Unmarshaler implementation of v, if
// any. It allocates intermediate pointers as needed.
func unmarshalerOf(v reflect.Value) (unstable.Unmarshaler, bool) {
	t := v.Type()
	if t.Implements(unmarshalerType) {
		if v.Kind() == reflect.Ptr && v.IsNil() {
			v.Set(reflect.New(t.Elem()))
		}
		return v.Interface().(unstable.Unmarshaler), true
	}
	if v.CanAddr() && reflect.PtrTo(t).Implements(unmarshalerType) {
		return v.Addr().Interface().(unstable.Unmarshaler), true
	}
	return nil, false
}

var unmarshalerType = reflect.TypeOf(new(unstable.Unmarshaler)).Elem()

// assignValue stores the TOML value carried by the node into v.
func (d *decoder) assignValue(v reflect.Value, expr *unstable.Node, value *unstable.Node) (reflect.Value, error) {
	if v.Kind() == reflect.Ptr {
		if d.unmarshalerInterface {
			if u, ok := unmarshalerOf(v); ok {
				return v, u.UnmarshalTOML(d.rawValue(expr, value))
			}
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		nv, err := d.assignValue(v.Elem(), expr, value)
		if err != nil || !nv.IsValid() {
			return reflect.Value{}, err
		}
		v.Elem().Set(nv)
		return v, nil
	}

	if d.unmarshalerInterface {
		if u, ok := unmarshalerOf(v); ok {
			return v, u.UnmarshalTOML(d.rawValue(expr, value))
		}
	}

	switch value.Kind {
	case unstable.String:
		return d.assignString(v, value)
	case unstable.Integer:
		return d.assignInteger(v, value)
	case unstable.Float:
		return d.assignFloat(v, value)
	case unstable.Bool:
		return d.assignBool(v, value)
	case unstable.DateTime:
		return d.assignDateTime(v, value)
	case unstable.LocalDateTime:
		return d.assignLocalDateTime(v, value)
	case unstable.LocalDate:
		return d.assignLocalDate(v, value)
	case unstable.LocalTime:
		return d.assignLocalTime(v, value)
	case unstable.Array:
		return d.assignArray(v, expr, value)
	case unstable.InlineTable:
		return d.assignInlineTable(v, expr, value)
	default:
		return reflect.Value{}, unstable.NewParserError(value.Data, "unsupported value kind %s", value.Kind)
	}
}

func (d *decoder) assignString(v reflect.Value, value *unstable.Node) (reflect.Value, error) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(string(value.Data))
		return v, nil
	case reflect.Interface:
		return boxInto(v, reflect.ValueOf(string(value.Data)))
	default:
	}
	if v.CanAddr() && v.Addr().Type().Implements(textUnmarshalerType) {
		err := v.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText(value.Data)
		if err != nil {
			return reflect.Value{}, unstable.NewParserError(d.p.Raw(value.Raw), "%s", err)
		}
		return v, nil
	}
	return reflect.Value{}, d.typeMismatchError("string", v.Type(), d.p.Raw(value.Raw))
}

func (d *decoder) assignInteger(v reflect.Value, value *unstable.Node) (reflect.Value, error) {
	// Integer values targeting a float field are parsed as floats: they can
	// represent (approximately) numbers beyond the int64 range.
	if k := v.Kind(); k == reflect.Float32 || k == reflect.Float64 {
		return d.assignFloat(v, value)
	}

	i, err := parseInteger(value.Data)
	if err != nil {
		return reflect.Value{}, err
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.OverflowInt(i) {
			return reflect.Value{}, unstable.NewParserError(value.Data, "integer value %d cannot be stored in %s", i, v.Type())
		}
		v.SetInt(i)
		return v, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if i < 0 {
			return reflect.Value{}, unstable.NewParserError(value.Data, "negative integer value %d cannot be stored in %s", i, v.Type())
		}
		if v.OverflowUint(uint64(i)) {
			return reflect.Value{}, unstable.NewParserError(value.Data, "integer value %d cannot be stored in %s", i, v.Type())
		}
		v.SetUint(uint64(i))
		return v, nil
	case reflect.Interface:
		return boxInto(v, reflect.ValueOf(i))
	default:
	}
	if ok, err := tryTextUnmarshaler(v, value.Data); ok {
		return v, err
	}
	return reflect.Value{}, d.typeMismatchError("integer", v.Type(), d.p.Raw(value.Raw))
}

// tryTextUnmarshaler attempts to deliver the raw text of a value to a target
// implementing encoding.TextUnmarshaler.
func tryTextUnmarshaler(v reflect.Value, text []byte) (bool, error) {
	if v.CanAddr() && v.Addr().Type().Implements(textUnmarshalerType) {
		return true, v.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText(text)
	}
	return false, nil
}

func (d *decoder) assignFloat(v reflect.Value, value *unstable.Node) (reflect.Value, error) {
	f, err := parseFloat(value.Data)
	if err != nil {
		return reflect.Value{}, err
	}

	switch v.Kind() {
	case reflect.Float64:
		v.SetFloat(f)
		return v, nil
	case reflect.Float32:
		if !math.IsInf(f, 0) && math.Abs(f) > math.MaxFloat32 {
			return reflect.Value{}, unstable.NewParserError(value.Data, "float value %f cannot be stored in float32", f)
		}
		v.SetFloat(f)
		return v, nil
	case reflect.Interface:
		return boxInto(v, reflect.ValueOf(f))
	default:
	}
	if ok, err := tryTextUnmarshaler(v, value.Data); ok {
		return v, err
	}
	return reflect.Value{}, d.typeMismatchError("float", v.Type(), d.p.Raw(value.Raw))
}

func (d *decoder) assignBool(v reflect.Value, value *unstable.Node) (reflect.Value, error) {
	b := value.Data[0] == 't'

	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(b)
		return v, nil
	case reflect.Interface:
		return boxInto(v, reflect.ValueOf(b))
	default:
	}
	if ok, err := tryTextUnmarshaler(v, value.Data); ok {
		return v, err
	}
	return reflect.Value{}, d.typeMismatchError("boolean", v.Type(), d.p.Raw(value.Raw))
}

func (d *decoder) assignDateTime(v reflect.Value, value *unstable.Node) (reflect.Value, error) {
	t, err := parseDateTime(value.Data)
	if err != nil {
		return reflect.Value{}, err
	}

	if v.Type() == timeType {
		v.Set(reflect.ValueOf(t))
		return v, nil
	}
	if v.Kind() == reflect.Interface {
		return boxInto(v, reflect.ValueOf(t))
	}
	return reflect.Value{}, d.typeMismatchError("datetime", v.Type(), d.p.Raw(value.Raw))
}

func (d *decoder) assignLocalDateTime(v reflect.Value, value *unstable.Node) (reflect.Value, error) {
	dt, rest, err := parseLocalDateTime(value.Data)
	if err != nil {
		return reflect.Value{}, err
	}
	if len(rest) > 0 {
		return reflect.Value{}, unstable.NewParserError(rest, "extra characters at the end of a local date time")
	}

	switch v.Type() {
	case localDateTimeType:
		v.Set(reflect.ValueOf(dt))
		return v, nil
	case timeType:
		v.Set(reflect.ValueOf(dt.AsTime(time.Local)))
		return v, nil
	}
	if v.Kind() == reflect.Interface {
		return boxInto(v, reflect.ValueOf(dt))
	}
	return reflect.Value{}, d.typeMismatchError("local datetime", v.Type(), d.p.Raw(value.Raw))
}

func (d *decoder) assignLocalDate(v reflect.Value, value *unstable.Node) (reflect.Value, error) {
	date, err := parseLocalDate(value.Data)
	if err != nil {
		return reflect.Value{}, err
	}

	switch v.Type() {
	case localDateType:
		v.Set(reflect.ValueOf(date))
		return v, nil
	case timeType:
		v.Set(reflect.ValueOf(date.AsTime(time.Local)))
		return v, nil
	}
	if v.Kind() == reflect.Interface {
		return boxInto(v, reflect.ValueOf(date))
	}
	return reflect.Value{}, d.typeMismatchError("local date", v.Type(), d.p.Raw(value.Raw))
}

func (d *decoder) assignLocalTime(v reflect.Value, value *unstable.Node) (reflect.Value, error) {
	t, rest, err := parseLocalTime(value.Data)
	if err != nil {
		return reflect.Value{}, err
	}
	if len(rest) > 0 {
		return reflect.Value{}, unstable.NewParserError(rest, "extra characters at the end of a local time")
	}

	switch v.Type() {
	case localTimeType:
		v.Set(reflect.ValueOf(t))
		return v, nil
	case timeType:
		v.Set(reflect.ValueOf(time.Date(0, 1, 1, t.Hour, t.Minute, t.Second, t.Nanosecond, time.Local)))
		return v, nil
	}
	if v.Kind() == reflect.Interface {
		return boxInto(v, reflect.ValueOf(t))
	}
	return reflect.Value{}, d.typeMismatchError("local time", v.Type(), d.p.Raw(value.Raw))
}

func (d *decoder) assignArray(v reflect.Value, expr *unstable.Node, value *unstable.Node) (reflect.Value, error) {
	// Count the elements to allocate the target in one go.
	count := 0
	cit := value.Children()
	for cit.Next() {
		if cit.Node().Kind != unstable.Comment {
			count++
		}
	}

	switch v.Kind() {
	case reflect.Slice:
		// Allocate the backing array once at its final length and assign each
		// element in place. This avoids a reflect.New allocation per element
		// and the repeated growth checks of reflect.Append.
		slice := reflect.MakeSlice(v.Type(), count, count)
		i := 0
		it := value.Children()
		for it.Next() {
			n := it.Node()
			if n.Kind == unstable.Comment {
				continue
			}
			elem := slice.Index(i)
			nv, err := d.assignValue(elem, nil, n)
			if err != nil {
				return reflect.Value{}, err
			}
			if nv.IsValid() {
				elem.Set(nv)
			}
			i++
		}
		return slice, nil
	case reflect.Array:
		it := value.Children()
		i := 0
		for it.Next() {
			n := it.Node()
			if n.Kind == unstable.Comment {
				continue
			}
			if i >= v.Len() {
				// Extra elements are dropped when the target array is too
				// small.
				break
			}
			elem := v.Index(i)
			nv, err := d.assignValue(elem, nil, n)
			if err != nil {
				return reflect.Value{}, err
			}
			elem.Set(nv)
			i++
		}
		return v, nil
	case reflect.Interface:
		// Build the []interface{} natively: each element is decoded straight
		// into a Go value with no intermediate addressable reflect.Value and
		// no reflect round-trip, and nested arrays recurse the same way.
		slice := make([]interface{}, 0, count)
		it := value.Children()
		for it.Next() {
			n := it.Node()
			if n.Kind == unstable.Comment {
				continue
			}
			ev, err := d.decodeAny(n)
			if err != nil {
				return reflect.Value{}, err
			}
			slice = append(slice, ev)
		}
		return boxInto(v, reflect.ValueOf(slice))
	default:
	}
	return reflect.Value{}, d.typeMismatchError("array", v.Type(), d.rawValue(expr, value))
}

// decodeAny decodes a value node into a native Go value (the representation
// used for interface{} targets), without going through reflect. Scalars and
// arrays are handled directly; inline tables still defer to the reflect-based
// path so that their dotted-key merge semantics remain identical.
func (d *decoder) decodeAny(n *unstable.Node) (interface{}, error) {
	switch n.Kind {
	case unstable.String:
		return string(n.Data), nil
	case unstable.Integer:
		i, err := parseInteger(n.Data)
		return i, err
	case unstable.Float:
		f, err := parseFloat(n.Data)
		return f, err
	case unstable.Bool:
		return n.Data[0] == 't', nil
	case unstable.Array:
		count := 0
		cit := n.Children()
		for cit.Next() {
			if cit.Node().Kind != unstable.Comment {
				count++
			}
		}
		slice := make([]interface{}, 0, count)
		it := n.Children()
		for it.Next() {
			c := it.Node()
			if c.Kind == unstable.Comment {
				continue
			}
			ev, err := d.decodeAny(c)
			if err != nil {
				return nil, err
			}
			slice = append(slice, ev)
		}
		return slice, nil
	case unstable.InlineTable:
		// Build the map natively: navigate each (possibly dotted) key with
		// plain Go map operations and decode each value with decodeAny. The
		// seen-tracker has already rejected duplicate or conflicting keys, so
		// intermediate parts can be created/merged without revalidation.
		count := 0
		cit := n.Children()
		for cit.Next() {
			count++
		}
		m := make(map[string]interface{}, count)
		it := n.Children()
		for it.Next() {
			kv := it.Node()
			if err := d.setAnyKey(m, kv.Key(), kv.Value()); err != nil {
				return nil, err
			}
		}
		return m, nil
	case unstable.DateTime:
		t, err := parseDateTime(n.Data)
		return t, err
	case unstable.LocalDateTime:
		dt, rest, err := parseLocalDateTime(n.Data)
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			return nil, unstable.NewParserError(rest, "extra characters at the end of a local date time")
		}
		return dt, nil
	case unstable.LocalDate:
		date, err := parseLocalDate(n.Data)
		return date, err
	case unstable.LocalTime:
		t, rest, err := parseLocalTime(n.Data)
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			return nil, unstable.NewParserError(rest, "extra characters at the end of a local time")
		}
		return t, nil
	default:
		return nil, unstable.NewParserError(n.Data, "unsupported value kind %s", n.Kind)
	}
}

func (d *decoder) assignInlineTable(v reflect.Value, expr *unstable.Node, value *unstable.Node) (reflect.Value, error) {
	switch v.Kind() {
	case reflect.Map:
		// Inline tables are self-contained: they fully replace the target.
		v = reflect.MakeMap(v.Type())
	case reflect.Struct:
		// fields are set in place
	case reflect.Interface:
		elem := reflect.ValueOf(map[string]interface{}{})
		nv, err := d.assignInlineTable(elem, expr, value)
		if err != nil {
			return reflect.Value{}, err
		}
		return boxInto(v, nv)
	default:
		return reflect.Value{}, d.typeMismatchError("inline table", v.Type(), d.rawValue(expr, value))
	}

	it := value.Children()
	for it.Next() {
		kv := it.Node()
		// Build the path from the key of this key-value. Keys of inline
		// tables rarely have more than a few parts.
		var pathBuf [4]pathPart
		path := pathBuf[:0]
		kit := kv.Key()
		for kit.Next() {
			path = append(path, pathPart{node: kit.Node()})
		}
		nv, err := d.descend(v, path, 0, kv, kv.Value())
		if err != nil {
			return reflect.Value{}, err
		}
		if nv.IsValid() {
			v = nv
		}
	}
	return v, nil
}

// boxInto returns the value to store in place of the interface value v. The
// caller stores the result in the slot v was found in, which performs the
// interface conversion, so the concrete value can be returned as-is.
func boxInto(v reflect.Value, c reflect.Value) (reflect.Value, error) {
	if !c.Type().AssignableTo(v.Type()) {
		return reflect.Value{}, fmt.Errorf("toml: cannot store %s into %s", c.Type(), v.Type())
	}
	return c, nil
}

var (
	interfaceType     = reflect.TypeOf(new(interface{})).Elem()
	localDateType     = reflect.TypeOf(LocalDate{})
	localTimeType     = reflect.TypeOf(LocalTime{})
	localDateTimeType = reflect.TypeOf(LocalDateTime{})
)

// structPlan caches the mapping between TOML keys and the fields of a struct
// type. byFold, keyed by the lowercased name, resolves any key on its own when
// no two fields fold to the same name (the overwhelmingly common case, marked
// by hasCollision == false): TOML keys are usually lowercase and never match
// the exact (capitalized) Go field names, so the byName probe was always a
// wasted lookup. byName (the exact names) is only consulted, first, when
// fields do collide under folding, to preserve the exact-match-wins tiebreak.
type structPlan struct {
	byName       map[string]structField
	byFold       map[string]structField
	hasCollision bool
}

type structField struct {
	index     []int
	fieldName string
}

// foldBufSize bounds the stack buffer used to lowercase keys without
// allocating. Keys longer than this (extremely rare) take the strings.ToLower
// fallback.
const foldBufSize = 68

// lookup and lookupBytes keep the hot path to a single inlinable byFold lookup.
// byFold is indexed by both the exact field/tag names and their lowercased
// forms, so that lookup resolves the two common cases — a lowercase key, or a
// key matching the field's own casing — directly. byName is consulted first
// only for types whose fields collide under case-folding, to preserve the
// exact-match-wins tiebreak. The buffer-fold for other casings lives
// out-of-line so it does not bloat the hot path.
func (p *structPlan) lookup(name string) (structField, bool) {
	if p.hasCollision {
		if f, ok := p.byName[name]; ok {
			return f, true
		}
	}
	if f, ok := p.byFold[name]; ok {
		return f, true
	}
	return p.lookupFoldStr(name)
}

func (p *structPlan) lookupBytes(name []byte) (structField, bool) {
	if p.hasCollision {
		if f, ok := p.byName[string(name)]; ok { // does not allocate
			return f, true
		}
	}
	if f, ok := p.byFold[string(name)]; ok { // does not allocate
		return f, true
	}
	return p.lookupFold(name)
}

// lookupFold resolves keys whose casing matches neither the exact nor the
// lowercased index: it folds to lowercase (in a stack buffer for ASCII, so no
// allocation) and retries; only non-ASCII or oversized keys hit strings.ToLower.
func (p *structPlan) lookupFold(name []byte) (structField, bool) {
	if len(name) <= foldBufSize {
		// Fold into a stack buffer: len(name) <= cap(buf), so the append
		// never reallocates and nothing escapes to the heap.
		var buf [foldBufSize]byte
		b := buf[:0]
		ascii := true
		for _, c := range name {
			if c >= 0x80 {
				ascii = false
				break
			}
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			b = append(b, c)
		}
		if ascii {
			f, ok := p.byFold[string(b)] // does not allocate
			return f, ok
		}
	}
	f, ok := p.byFold[strings.ToLower(string(name))]
	return f, ok
}

func (p *structPlan) lookupFoldStr(name string) (structField, bool) {
	if len(name) <= foldBufSize {
		// Fold into a stack buffer: len(name) <= cap(buf), so the append
		// never reallocates and nothing escapes to the heap.
		var buf [foldBufSize]byte
		b := buf[:0]
		ascii := true
		for i := 0; i < len(name); i++ {
			c := name[i]
			if c >= 0x80 {
				ascii = false
				break
			}
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			b = append(b, c)
		}
		if ascii {
			f, ok := p.byFold[string(b)] // does not allocate
			return f, ok
		}
	}
	f, ok := p.byFold[strings.ToLower(name)]
	return f, ok
}

var structPlans sync.Map // reflect.Type -> *structPlan

func planForType(t reflect.Type) *structPlan {
	if plan, ok := structPlans.Load(t); ok {
		return plan.(*structPlan)
	}
	plan := buildPlan(t)
	structPlans.Store(t, plan)
	return plan
}

func buildPlan(t reflect.Type) *structPlan {
	plan := &structPlan{
		byName: map[string]structField{},
		byFold: map[string]structField{},
	}
	addFields(plan, t, nil)
	return plan
}

func addFields(plan *structPlan, t reflect.Type, prefix []int) {
	var embedded []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, tagged := f.Tag.Lookup("toml")
		name := f.Name
		if tagged {
			// A tag of exactly "-" drops the field. "-," names it "-".
			if tag == "-" {
				continue
			}
			parts := strings.SplitN(tag, ",", 2)
			if parts[0] != "" {
				name = parts[0]
			}
		}
		if f.Anonymous {
			ft := f.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() != reflect.Struct {
				// Embedded non-struct fields are not decoded into.
				continue
			}
			if !tagged {
				// Untagged embedded structs are flattened, even when their
				// type is unexported: only their own exported fields are
				// reachable.
				embedded = append(embedded, f)
				continue
			}
			// A tagged embedded struct acts as a regular named field.
		} else if f.PkgPath != "" {
			// unexported
			continue
		}
		index := make([]int, 0, len(prefix)+1)
		index = append(index, prefix...)
		index = append(index, i)
		sf := structField{index: index, fieldName: f.Name}
		if _, ok := plan.byName[name]; !ok {
			plan.byName[name] = sf
		}
		lower := strings.ToLower(name)
		if _, ok := plan.byFold[lower]; !ok {
			plan.byFold[lower] = sf
		} else {
			// Two distinct fields fold to the same name: case-insensitive
			// matching is ambiguous, so lookups must consult byName first to
			// keep the exact-match-wins tiebreak deterministic.
			plan.hasCollision = true
		}
		// Index the exact (cased) name as well, so a key written with the
		// field's own casing resolves in a single byFold lookup. Only fields
		// whose name is not already lowercase need this extra entry. Any name
		// that would conflict here also collides under folding (handled
		// above), so byName-first preserves the exact tiebreak in that case.
		if name != lower {
			if _, ok := plan.byFold[name]; !ok {
				plan.byFold[name] = sf
			}
		}
	}
	// Embedded structs are flattened after the regular fields, so that
	// shallower fields win.
	for _, f := range embedded {
		ft := f.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		index := make([]int, 0, len(prefix)+1)
		index = append(index, prefix...)
		idx := f.Index[0]
		index = append(index, idx)
		addFields(plan, ft, index)
	}
}

// fieldByIndexAlloc returns the field of v at the given index path,
// allocating intermediate embedded pointers as needed.
func fieldByIndexAlloc(v reflect.Value, index []int) reflect.Value {
	// Fast path for non-embedded fields, which have a single-element index:
	// no intermediate pointer dereferencing is possible.
	if len(index) == 1 {
		return v.Field(index[0])
	}
	for i, x := range index {
		if i > 0 {
			for v.Kind() == reflect.Ptr {
				if v.IsNil() {
					v.Set(reflect.New(v.Type().Elem()))
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v
}
