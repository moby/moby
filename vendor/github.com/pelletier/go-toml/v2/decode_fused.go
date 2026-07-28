package toml

import (
	"errors"
	"reflect"
	"strings"

	"github.com/pelletier/go-toml/v2/internal/parserbridge"
	"github.com/pelletier/go-toml/v2/unstable"
)

// unmarshalFused decodes a whole document into a native map[string]interface{}
// tree with no reflection on the document structure, and without building an
// AST for table headers and scalar key-values. Only container values (arrays
// and inline tables) are parsed into the parser arena, so that the seen-tracker
// can validate them and decodeAny can presize the resulting slices and maps —
// the AST is what makes that cheap O(1) presizing possible.
//
// It is used when the target is a fully generic value (interface{} or
// map[string]interface{}) and the unmarshaler interface is disabled. The
// seen-tracker validates the document (duplicate keys, type consistency), so
// the builder creates and merges containers without revalidating. Strict mode
// never applies to a generic target (a map has no "unknown fields"), and
// captures never apply (a generic value implements no Unmarshaler).
func (d *decoder) unmarshalFused(root reflect.Value, data []byte) error {
	var m map[string]interface{}
	if !root.IsNil() {
		// Decode into (merge with) an existing generic map when present.
		if em, ok := root.Interface().(map[string]interface{}); ok {
			m = em
		}
	}
	if m == nil {
		m = map[string]interface{}{}
	}

	if err := d.fusedDocument(m, data); err != nil {
		return d.wrapFusedError(data, err)
	}

	if root.CanSet() {
		root.Set(reflect.ValueOf(m))
	}
	return nil
}

// fusedDocument runs the top-level expression loop, mirroring
// Parser.NextExpression but storing values directly into native maps.
func (d *decoder) fusedDocument(m map[string]interface{}, b []byte) error {
	cur := m
	for {
		b = fusedSkipWS(b)
		if len(b) == 0 {
			return nil
		}
		switch b[0] {
		case '\n':
			b = b[1:]
		case '\r':
			if len(b) > 1 && b[1] == '\n' {
				b = b[2:]
				continue
			}
			return unstable.NewParserError(b[:1], "expected newline but got %#U", b[0])
		case '#':
			_, rest, err := parserbridge.ScanComment(b)
			if err != nil {
				return err
			}
			rest, err = fusedConsumeEOL(rest)
			if err != nil {
				return err
			}
			b = rest
		case '[':
			rest, err := d.fusedTable(b, m, &cur)
			if err != nil {
				return err
			}
			b = rest
		default:
			rest, err := d.fusedKeyVal(b, cur)
			if err != nil {
				return err
			}
			b = rest
		}
	}
}

// fusedTable handles a [table] or [[array table]] header. b starts at '['. It
// updates *cur to the table the following key-values belong to.
func (d *decoder) fusedTable(b []byte, root map[string]interface{}, cur *map[string]interface{}) ([]byte, error) {
	arrayTable := len(b) > 1 && b[1] == '['

	var start []byte
	if arrayTable {
		start = fusedSkipWS(b[2:])
	} else {
		start = fusedSkipWS(b[1:])
	}

	var err error
	var rawKey []byte
	d.keyParts, rawKey, b, err = parserbridge.ScanKey(&d.p, start, d.keyParts[:0])
	if err != nil {
		return nil, err
	}

	if arrayTable {
		if len(b) < 2 || b[0] != ']' || b[1] != ']' {
			return nil, unstable.NewParserError(fusedHL1(b), "expected ']]' to close array table name")
		}
		b = b[2:]
	} else {
		if len(b) == 0 || b[0] != ']' {
			return nil, unstable.NewParserError(fusedHL1(b), "expected ']' to close table name")
		}
		b = b[1:]
	}

	// The whole expression (including its line termination) is parsed before
	// it is validated, to keep error precedence identical to the AST path.
	b, err = d.fusedFinishLine(b)
	if err != nil {
		return nil, err
	}

	if arrayTable {
		first, err := d.seen.CheckArrayTable(d.keyParts)
		if err != nil {
			return nil, d.fusedSeenError(rawKey, d.keyParts, err)
		}
		*cur = d.anyArrayTableParts(root, d.keyParts, first)
	} else {
		if _, err := d.seen.CheckTable(d.keyParts); err != nil {
			return nil, d.fusedSeenError(rawKey, d.keyParts, err)
		}
		*cur = d.anyTableParts(root, d.keyParts)
	}
	return b, nil
}

// fusedKeyVal handles a `key = value` expression relative to the current table
// cur. b starts at the first character of the key.
func (d *decoder) fusedKeyVal(b []byte, cur map[string]interface{}) ([]byte, error) {
	var err error
	var rawKey []byte
	d.keyParts, rawKey, b, err = parserbridge.ScanKey(&d.p, b, d.keyParts[:0])
	if err != nil {
		return nil, err
	}
	if len(b) == 0 || b[0] != '=' {
		return nil, unstable.NewParserError(fusedHL1(b), "expected '=' after key")
	}
	b = fusedSkipWS(b[1:])
	if len(b) == 0 {
		return nil, unstable.NewParserError(b, "expected value, not end of input")
	}

	if c := b[0]; c == '[' || c == '{' {
		// Container value: build its AST so the seen-tracker can validate it
		// and decodeAny can presize the resulting slices and maps.
		nodeAny, rest, err := parserbridge.ParseValue(&d.p, b)
		if err != nil {
			return nil, err
		}
		node := nodeAny.(*unstable.Node)
		rest, err = d.fusedFinishLine(rest)
		if err != nil {
			return nil, err
		}
		leafID, err := d.seen.CheckKeyValue(d.keyParts)
		if err != nil {
			return nil, d.fusedSeenError(rawKey, d.keyParts, err)
		}
		if err := d.seen.CheckValueUnder(leafID, node); err != nil {
			return nil, d.fusedSeenError(rawKey, d.keyParts, err)
		}
		av, err := d.decodeAny(node)
		if err != nil {
			return nil, err
		}
		d.setFusedLeaf(cur, d.keyParts, av)
		return rest, nil
	}

	// Scalar value: scan it without building a node, then validate and convert
	// it natively.
	k, _, value, rest, err := parserbridge.ScanScalar(&d.p, b)
	if err != nil {
		return nil, err
	}
	kind := unstable.Kind(k)
	rest, err = d.fusedFinishLine(rest)
	if err != nil {
		return nil, err
	}
	if _, err := d.seen.CheckKeyValue(d.keyParts); err != nil {
		return nil, d.fusedSeenError(rawKey, d.keyParts, err)
	}
	av, err := d.fusedScalar(kind, value)
	if err != nil {
		return nil, err
	}
	d.setFusedLeaf(cur, d.keyParts, av)
	return rest, nil
}

// fusedSeenError turns a bare error returned by a SeenTracker parts-method
// into a ParserError carrying the position (the raw key span) and key path of
// the offending expression, so that it is reported as a DecodeError with
// context. It mirrors decoder.wrapSeenError for the fused (AST-less) path.
func (d *decoder) fusedSeenError(rawKey []byte, parts [][]byte, err error) error {
	key := make(Key, len(parts))
	for i, p := range parts {
		key[i] = string(p)
	}
	return &unstable.ParserError{
		Highlight: rawKey,
		Message:   strings.TrimPrefix(err.Error(), "toml: "),
		Key:       key,
	}
}

// fusedScalar converts a scanned scalar value into the native Go value used
// for generic targets. It mirrors the scalar cases of decodeAny.
func (d *decoder) fusedScalar(kind unstable.Kind, value []byte) (interface{}, error) {
	switch kind {
	case unstable.String:
		return string(value), nil
	case unstable.Integer:
		i, err := parseInteger(value)
		return i, err
	case unstable.Float:
		f, err := parseFloat(value)
		return f, err
	case unstable.Bool:
		return value[0] == 't', nil
	case unstable.DateTime:
		t, err := parseDateTime(value)
		return t, err
	case unstable.LocalDateTime:
		dt, rest, err := parseLocalDateTime(value)
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			return nil, unstable.NewParserError(rest, "extra characters at the end of a local date time")
		}
		return dt, nil
	case unstable.LocalDate:
		date, err := parseLocalDate(value)
		return date, err
	case unstable.LocalTime:
		t, rest, err := parseLocalTime(value)
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			return nil, unstable.NewParserError(rest, "extra characters at the end of a local time")
		}
		return t, nil
	default:
		return nil, unstable.NewParserError(value, "unsupported value kind %s", kind)
	}
}

// anyTableParts navigates a [table] header (given its key parts) to the map it
// designates, creating intermediate tables as needed.
func (d *decoder) anyTableParts(m map[string]interface{}, parts [][]byte) map[string]interface{} {
	cur := m
	for _, p := range parts {
		cur = d.anyChildTable(cur, d.intern(p))
	}
	return cur
}

// anyArrayTableParts navigates a [[array table]] header (given its key parts),
// appends a fresh element to the designated array, and returns it. first is
// true the first time this header is seen, in which case any pre-existing array
// (from a reused target) is reset.
func (d *decoder) anyArrayTableParts(m map[string]interface{}, parts [][]byte, first bool) map[string]interface{} {
	cur := m
	name := d.intern(parts[0])
	for i := 1; i < len(parts); i++ {
		cur = d.anyChildTable(cur, name)
		name = d.intern(parts[i])
	}
	s, _ := cur[name].([]interface{})
	if first {
		s = s[:0]
	}
	elem := map[string]interface{}{}
	cur[name] = append(s, elem)
	return elem
}

// setFusedLeaf assigns av at the (possibly dotted) key parts within cur,
// creating intermediate maps as needed.
func (d *decoder) setFusedLeaf(cur map[string]interface{}, parts [][]byte, av interface{}) {
	for i := 0; i < len(parts)-1; i++ {
		cur = d.anyChildTable(cur, d.intern(parts[i]))
	}
	cur[d.intern(parts[len(parts)-1])] = av
}

// wrapFusedError gives document context to errors produced by the fused
// decoder.
func (d *decoder) wrapFusedError(data []byte, err error) error {
	var perr *unstable.ParserError
	if errors.As(err, &perr) && len(perr.Highlight) == 0 {
		// Mirror NextExpression: give end-of-input errors a usable position by
		// extending the empty highlight to the last byte of the document.
		if offset := cap(data) - cap(perr.Highlight); offset > 0 && offset == len(data) {
			perr.Highlight = data[offset-1 : offset]
		}
	}
	return d.wrapError(data, err)
}

func fusedSkipWS(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}

func fusedConsumeEOL(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return b, nil
	}
	switch b[0] {
	case '\n':
		return b[1:], nil
	case '\r':
		if len(b) > 1 && b[1] == '\n' {
			return b[2:], nil
		}
	}
	return nil, unstable.NewParserError(b[:1], "expected newline but got %#U", b[0])
}

// fusedFinishLine consumes `ws [comment] (newline|eof)` after an expression.
func (d *decoder) fusedFinishLine(b []byte) ([]byte, error) {
	b = fusedSkipWS(b)
	if len(b) > 0 && b[0] == '#' {
		_, rest, err := parserbridge.ScanComment(b)
		if err != nil {
			return nil, err
		}
		b = rest
	}
	return fusedConsumeEOL(b)
}

func fusedHL1(b []byte) []byte {
	if len(b) > 0 {
		return b[:1]
	}
	return b
}
