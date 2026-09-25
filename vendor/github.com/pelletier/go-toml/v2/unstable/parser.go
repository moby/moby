package unstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

// ParserError describes an error relative to the content of the document.
//
// It cannot outlive the instance of Parser it refers to, and may cause panics
// if the parser is reset.
type ParserError struct {
	Highlight []byte
	Message   string
	Key       []string // optional
}

// Error is the implementation of the error interface.
func (e *ParserError) Error() string {
	return e.Message
}

// NewParserError is a convenience function to create a ParserError
//
// Warning: Highlight needs to be a subslice of Parser.data, so only slices
// returned by Parser.Raw are valid candidates.
func NewParserError(highlight []byte, format string, args ...interface{}) error {
	return &ParserError{
		Highlight: highlight,
		Message:   fmt.Errorf(format, args...).Error(),
	}
}

// Parser scans over a TOML-encoded document and generates an iterative AST.
//
// To prime the Parser, first reset it with the contents of a TOML document.
// Then, process all top-level expressions sequentially. See Example.
//
// Don't forget to check Error() after you're done parsing.
//
// Each top-level expression needs to be fully processed before calling
// NextExpression() again. Otherwise, calls to various Node methods may panic
// if the parser has moved on the next expression.
//
// For performance reasons, go-toml doesn't make a copy of the input bytes to
// the parser. Make sure to copy all the bytes you need to outlive the slice
// given to the parser.
type Parser struct {
	KeepComments bool

	data  []byte
	left  []byte
	nodes []Node
	err   error

	// nesting is the current depth of nested arrays and inline tables being
	// parsed. It guards parseVal/parseValArray/parseInlineTable against
	// unbounded mutual recursion, which would otherwise let a deeply nested
	// document overflow the goroutine stack (an unrecoverable fatal error).
	nesting int
}

// maxValueNesting is the maximum depth of nested arrays and inline tables the
// parser accepts. Beyond it, parsing fails with a ParserError instead of
// risking a stack overflow. Real-world TOML documents nest only a handful of
// levels deep, so this bound is generous; it matches the limit used by the
// standard library's encoding/json.
const maxValueNesting = 10000

// Data returns the slice provided to the last call to Reset.
func (p *Parser) Data() []byte {
	return p.data
}

// Range returns a range description that corresponds to a given slice of the
// input. If the argument is not a subslice of the parser input, this function
// panics.
func (p *Parser) Range(b []byte) Range {
	// b is a subslice of p.data if and only if they share the same backing
	// array. In that case, because subslicing cannot extend capacity, the
	// number of bytes between the start of b and the end of the backing array
	// (its capacity) identifies the offset of b within data.
	offset := cap(p.data) - cap(b)
	if offset < 0 || offset+len(b) > len(p.data) {
		panic(errors.New("not a slice of the data slice"))
	}
	return Range{
		Offset: uint32(offset), //nolint:gosec // TOML documents are small
		Length: uint32(len(b)), //nolint:gosec // TOML documents are small
	}
}

// Raw returns the slice corresponding to the bytes in the given range.
func (p *Parser) Raw(raw Range) []byte {
	return p.data[raw.Offset : raw.Offset+raw.Length]
}

// Reset brings the parser to its initial state for a given input. It wipes an
// reuses internal storage to reduce allocation.
func (p *Parser) Reset(b []byte) {
	p.data = b
	p.left = b
	p.nodes = p.nodes[:0]
	p.err = nil
	p.nesting = 0
}

// Error returns any error that has occurred during parsing.
func (p *Parser) Error() error {
	return p.err
}

// Range of bytes in the document.
type Range struct {
	Offset uint32
	Length uint32
}

// Position describes a position in the input.
type Position struct {
	// Number of bytes from the beginning of the input.
	Offset int
	// Line number, starting at 1.
	Line int
	// Column number, starting at 1.
	Column int
}

// Shape describes the position of a range in the input.
type Shape struct {
	Start Position
	End   Position
}

func (p *Parser) position(offset int) Position {
	pos := Position{
		Offset: offset,
		Line:   1,
		Column: 1,
	}
	b := p.data[:offset]
	for {
		idx := bytes.IndexByte(b, '\n')
		if idx < 0 {
			break
		}
		pos.Line++
		b = b[idx+1:]
	}
	pos.Column = len(b) + 1
	return pos
}

// Shape returns the shape of the given range in the input.  Will
// panic if the range is not a subslice of the input.
func (p *Parser) Shape(r Range) Shape {
	raw := p.Raw(r)
	return Shape{
		Start: p.position(int(r.Offset)),
		End:   p.position(int(r.Offset) + len(raw)),
	}
}

// Expression returns a pointer to the node representing the last successfully
// parsed expression.
func (p *Parser) Expression() *Node {
	if len(p.nodes) == 0 {
		return nil
	}
	return &p.nodes[0]
}

// push appends a node to the arena and returns its handle (1-based index).
func (p *Parser) push(n Node) int32 {
	if len(p.nodes) == cap(p.nodes) {
		// Grow by 2x: large expressions (huge arrays) would otherwise grow
		// the arena in small steps, copying it repeatedly.
		newCap := 2 * cap(p.nodes)
		if newCap < 64 {
			newCap = 64
		}
		nodes := make([]Node, len(p.nodes), newCap)
		copy(nodes, p.nodes)
		p.nodes = nodes
	}
	n.parser = p
	p.nodes = append(p.nodes, n)
	return int32(len(p.nodes)) //nolint:gosec // node counts are bounded by document size
}

// at returns a pointer to the node with the given handle. Only valid until
// the next call to push.
func (p *Parser) at(handle int32) *Node {
	return &p.nodes[handle-1]
}

// offsetOf returns the offset of b within the parser's data. b must be a
// subslice of p.data.
func (p *Parser) offsetOf(b []byte) int {
	return cap(p.data) - cap(b)
}

// rangeFrom returns the Range covering bytes from the start of `from` to the
// start of `to`. Both must be subslices of p.data.
func (p *Parser) rangeFrom(from, to []byte) Range {
	start := p.offsetOf(from)
	end := p.offsetOf(to)
	return Range{
		Offset: uint32(start),       //nolint:gosec // TOML documents are small
		Length: uint32(end - start), //nolint:gosec // TOML documents are small
	}
}

// NextExpression parses the next top-level expression. If an expression was
// successfully parsed, it returns true. If the parser is at the end of the
// document or an error occurred, it returns false.
//
// Retrieve the parsed expression with Expression().
func (p *Parser) NextExpression() bool {
	if p.err != nil {
		return false
	}

	p.nodes = p.nodes[:0]

	for {
		b := skipWhitespace(p.left)
		if len(b) == 0 {
			p.left = b
			return false
		}

		var err error
		switch b[0] {
		case '\n':
			p.left = b[1:]
			continue
		case '\r':
			if len(b) > 1 && b[1] == '\n' {
				p.left = b[2:]
				continue
			}
			err = NewParserError(b[:1], "expected newline but got %#U", b[0])
		case '#':
			var comment, rest []byte
			comment, rest, err = scanComment(b)
			if err == nil {
				rest, err = consumeEOL(rest)
			}
			if err == nil {
				if p.KeepComments {
					p.push(Node{
						Kind: Comment,
						Raw:  p.Range(comment),
						Data: comment,
					})
					p.left = rest
					return true
				}
				p.left = rest
				continue
			}
		case '[':
			var rest []byte
			rest, err = p.parseExprTable(b)
			if err == nil {
				p.left = rest
				return true
			}
		default:
			var rest []byte
			rest, err = p.parseExprKeyval(b)
			if err == nil {
				p.left = rest
				return true
			}
		}

		// Errors at the end of the input have an empty highlight. Extend
		// them to the last byte of the input so that they carry a usable
		// position.
		var perr *ParserError
		if errors.As(err, &perr) && len(perr.Highlight) == 0 {
			if offset := p.offsetOf(perr.Highlight); offset > 0 && offset == len(p.data) {
				perr.Highlight = p.data[offset-1 : offset]
			}
		}

		p.err = err
		return false
	}
}

// consumeEOL consumes a newline (LF or CRLF) or end of input.
func consumeEOL(b []byte) ([]byte, error) {
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
	return nil, NewParserError(b[:1], "expected newline but got %#U", b[0])
}

// finishLine handles `ws [comment] (newline|eof)` after a top-level
// expression. If a comment is present and KeepComments is set, it is attached
// as the next sibling of the expression's root node.
func (p *Parser) finishLine(root int32, b []byte) ([]byte, error) {
	b = skipWhitespace(b)
	if len(b) > 0 && b[0] == '#' {
		comment, rest, err := scanComment(b)
		if err != nil {
			return nil, err
		}
		if p.KeepComments {
			h := p.push(Node{
				Kind: Comment,
				Raw:  p.Range(comment),
				Data: comment,
			})
			p.at(root).next = h
		}
		b = rest
	}
	return consumeEOL(b)
}

// parseExprKeyval parses a top-level `key = value` expression, including its
// line termination.
func (p *Parser) parseExprKeyval(b []byte) ([]byte, error) {
	root, rest, err := p.parseKeyval(b)
	if err != nil {
		return nil, err
	}
	return p.finishLine(root, rest)
}

// parseExprTable parses a `[table]` or `[[array table]]` expression,
// including its line termination. b starts at '['.
func (p *Parser) parseExprTable(b []byte) ([]byte, error) {
	var root int32
	var err error
	var rest []byte
	if len(b) > 1 && b[1] == '[' {
		root, rest, err = p.parseArrayTableHeader(b)
	} else {
		root, rest, err = p.parseTableHeader(b)
	}
	if err != nil {
		return nil, err
	}
	return p.finishLine(root, rest)
}

// parseTableHeader parses `[ ws key ws ]`. b starts at '['.
func (p *Parser) parseTableHeader(b []byte) (int32, []byte, error) {
	root := p.push(Node{Kind: Table})

	first, b, err := p.parseKey(skipWhitespace(b[1:]))
	if err != nil {
		return 0, nil, err
	}
	p.at(root).child = first

	if len(b) == 0 || b[0] != ']' {
		return 0, nil, NewParserError(highlight1(b), "expected ']' to close table name")
	}
	return root, b[1:], nil
}

// parseArrayTableHeader parses `[[ ws key ws ]]`. b starts at '[['.
func (p *Parser) parseArrayTableHeader(b []byte) (int32, []byte, error) {
	root := p.push(Node{Kind: ArrayTable})

	first, b, err := p.parseKey(skipWhitespace(b[2:]))
	if err != nil {
		return 0, nil, err
	}
	p.at(root).child = first

	if len(b) < 2 || b[0] != ']' || b[1] != ']' {
		return 0, nil, NewParserError(highlight1(b), "expected ']]' to close array table name")
	}
	return root, b[2:], nil
}

// parseKeyval parses `key keyval-sep val`. Returns the handle to the KeyValue
// node.
func (p *Parser) parseKeyval(b []byte) (int32, []byte, error) {
	root := p.push(Node{Kind: KeyValue})
	start := b

	firstKey, b, err := p.parseKey(b)
	if err != nil {
		return 0, nil, err
	}

	if len(b) == 0 || b[0] != '=' {
		return 0, nil, NewParserError(highlight1(b), "expected '=' after key")
	}
	b = skipWhitespace(b[1:])

	value, b, err := p.parseVal(b)
	if err != nil {
		return 0, nil, err
	}

	p.at(root).child = value
	p.at(value).next = firstKey
	p.at(root).Raw = p.rangeFrom(start, b)
	return root, b, nil
}

// parseKey parses a potentially dotted key. It consumes the whitespace
// following the key, so that the caller can directly check for the next
// expected character ('=', ']', ...). Returns the handle of the first Key
// node; subsequent parts are chained via next.
func (p *Parser) parseKey(b []byte) (int32, []byte, error) {
	var first, last int32
	for {
		h, rest, err := p.parseSimpleKey(b)
		if err != nil {
			return 0, nil, err
		}
		if first == 0 {
			first = h
		} else {
			p.at(last).next = h
		}
		last = h

		b = skipWhitespace(rest)
		if len(b) > 0 && b[0] == '.' {
			b = skipWhitespace(b[1:])
			continue
		}
		return first, b, nil
	}
}

func isUnquotedKeyChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
}

// parseSimpleKey parses one key part: either a bare key or a quoted key.
func (p *Parser) parseSimpleKey(b []byte) (int32, []byte, error) {
	raw, value, rest, err := p.scanSimpleKey(b)
	if err != nil {
		return 0, nil, err
	}
	h := p.push(Node{Kind: Key, Raw: p.Range(raw), Data: value})
	return h, rest, nil
}

// scanSimpleKey scans one key part (bare or quoted) without building an AST
// node. It returns the raw bytes, the decoded key value, and the rest of the
// input.
func (p *Parser) scanSimpleKey(b []byte) (raw, value, rest []byte, err error) {
	if len(b) == 0 {
		return nil, nil, nil, NewParserError(b, "expected key but reached end of input")
	}

	switch b[0] {
	case '\'':
		return p.parseLiteralString(b)
	case '"':
		return p.parseBasicString(b)
	default:
		i := 0
		for i < len(b) && isUnquotedKeyChar(b[i]) {
			i++
		}
		if i == 0 {
			return nil, nil, nil, NewParserError(b[:1], "invalid character at start of key: %#U", b[0])
		}
		return b[:i], b[:i], b[i:], nil
	}
}

// parseVal parses a TOML value and returns the handle to its node.
func (p *Parser) parseVal(b []byte) (int32, []byte, error) {
	if len(b) == 0 {
		return 0, nil, NewParserError(b, "expected value, not end of input")
	}

	c := b[0]
	switch {
	case c == '"':
		var raw, value, rest []byte
		var err error
		if len(b) > 2 && b[1] == '"' && b[2] == '"' {
			raw, value, rest, err = p.parseMultilineBasicString(b)
		} else {
			raw, value, rest, err = p.parseBasicString(b)
		}
		if err != nil {
			return 0, nil, err
		}
		h := p.push(Node{Kind: String, Raw: p.Range(raw), Data: value})
		return h, rest, nil
	case c == '\'':
		var raw, value, rest []byte
		var err error
		if len(b) > 2 && b[1] == '\'' && b[2] == '\'' {
			raw, value, rest, err = p.parseMultilineLiteralString(b)
		} else {
			raw, value, rest, err = p.parseLiteralString(b)
		}
		if err != nil {
			return 0, nil, err
		}
		h := p.push(Node{Kind: String, Raw: p.Range(raw), Data: value})
		return h, rest, nil
	case c == 't':
		return p.parseKeyword(b, "true", Bool)
	case c == 'f':
		return p.parseKeyword(b, "false", Bool)
	case c == 'i':
		return p.parseKeyword(b, "inf", Float)
	case c == 'n':
		return p.parseKeyword(b, "nan", Float)
	case c == '[' || c == '{':
		// Arrays and inline tables recurse back into parseVal for each of
		// their elements. Bound that recursion so a document with millions of
		// nested brackets or braces cannot overflow the goroutine stack.
		if p.nesting >= maxValueNesting {
			return 0, nil, NewParserError(b[:1], "arrays and inline tables are nested more than the maximum of %d levels deep", maxValueNesting)
		}
		p.nesting++
		var h int32
		var rest []byte
		var err error
		if c == '[' {
			h, rest, err = p.parseValArray(b)
		} else {
			h, rest, err = p.parseInlineTable(b)
		}
		p.nesting--
		return h, rest, err
	case c == '+' || c == '-':
		return p.parseIntOrFloat(b)
	case c >= '0' && c <= '9':
		if isDateTimeStart(b) {
			return p.parseDateTime(b)
		}
		return p.parseIntOrFloat(b)
	default:
		return 0, nil, NewParserError(b[:1], "unexpected character %#U at start of value", c)
	}
}

// scanScalar scans a single scalar TOML value (string, integer, float,
// boolean, or date/time) without building any AST node. It returns the kind of
// the value, its raw bytes, its decoded value bytes (for strings: quotes
// removed and escapes resolved; identical to raw for the other kinds), and the
// rest of the input. Arrays and inline tables are not scalars and produce an
// error: use parseValue for those.
//
// It is exposed to the root toml package through internal/parserbridge for the
// fused generic-decode path; it is not part of the public API.
func (p *Parser) scanScalar(b []byte) (kind Kind, raw, value, rest []byte, err error) {
	if len(b) == 0 {
		return Invalid, nil, nil, nil, NewParserError(b, "expected value, not end of input")
	}

	c := b[0]
	switch {
	case c == '"':
		if len(b) > 2 && b[1] == '"' && b[2] == '"' {
			raw, value, rest, err = p.parseMultilineBasicString(b)
		} else {
			raw, value, rest, err = p.parseBasicString(b)
		}
		return String, raw, value, rest, err
	case c == '\'':
		if len(b) > 2 && b[1] == '\'' && b[2] == '\'' {
			raw, value, rest, err = p.parseMultilineLiteralString(b)
		} else {
			raw, value, rest, err = p.parseLiteralString(b)
		}
		return String, raw, value, rest, err
	case c == 't':
		return scanKeyword(b, "true", Bool)
	case c == 'f':
		return scanKeyword(b, "false", Bool)
	case c == 'i':
		return scanKeyword(b, "inf", Float)
	case c == 'n':
		return scanKeyword(b, "nan", Float)
	case c == '+' || c == '-':
		return scanIntOrFloat(b)
	case c >= '0' && c <= '9':
		if isDateTimeStart(b) {
			return scanDateTime(b)
		}
		return scanIntOrFloat(b)
	default:
		return Invalid, nil, nil, nil, NewParserError(b[:1], "unexpected character %#U at start of value", c)
	}
}

// scanKey scans a potentially dotted key without building AST nodes,
// appending the decoded value of each part to dst (pass dst[:0] to reuse a
// buffer). It consumes the whitespace following the key, so the caller can
// directly check for the next expected character ('=', ']', ...). It returns
// the parts, the raw bytes spanning the whole key (from the first part to the
// end of the last one, excluding trailing whitespace, usable as an error
// highlight), the rest of the input, and any error.
//
// It is exposed to the root toml package through internal/parserbridge for the
// fused generic-decode path; it is not part of the public API.
func (p *Parser) scanKey(b []byte, dst [][]byte) (parts [][]byte, raw, rest []byte, err error) {
	parts = dst
	start := b
	for {
		_, value, r, err := p.scanSimpleKey(b)
		if err != nil {
			return nil, nil, nil, err
		}
		parts = append(parts, value)

		// r points just past the current part: the key spans from start to
		// here, ignoring any whitespace that follows.
		raw = start[:len(start)-len(r)]

		b = skipWhitespace(r)
		if len(b) > 0 && b[0] == '.' {
			b = skipWhitespace(b[1:])
			continue
		}
		return parts, raw, b, nil
	}
}

// parseValue parses a single TOML value, which may be an array or inline table,
// into the parser's arena. It returns the root node of the value and the rest
// of the input. It resets the arena, so any node returned by a previous call to
// parseValue, Expression, or NextExpression is invalidated.
//
// It is exposed to the root toml package through internal/parserbridge for the
// fused generic-decode path; it is not part of the public API.
func (p *Parser) parseValue(b []byte) (*Node, []byte, error) {
	p.nodes = p.nodes[:0]
	h, rest, err := p.parseVal(b)
	if err != nil {
		return nil, nil, err
	}
	return &p.nodes[h-1], rest, nil
}

func (p *Parser) parseKeyword(b []byte, kw string, kind Kind) (int32, []byte, error) {
	k, raw, _, rest, err := scanKeyword(b, kw, kind)
	if err != nil {
		return 0, nil, err
	}
	h := p.push(Node{Kind: k, Raw: p.Range(raw), Data: raw})
	return h, rest, nil
}

// scanKeyword scans a keyword value (true, false, inf, nan) without building
// an AST node. raw and value are identical (the keyword bytes).
func scanKeyword(b []byte, kw string, kind Kind) (Kind, []byte, []byte, []byte, error) {
	if len(b) < len(kw) || string(b[:len(kw)]) != kw {
		n := len(kw)
		if len(b) < n {
			n = len(b)
		}
		return Invalid, nil, nil, nil, NewParserError(b[:n], "expected keyword %q", kw)
	}
	return kind, b[:len(kw)], b[:len(kw)], b[len(kw):], nil
}

// parseValArray parses an array value. b starts at '['.
func (p *Parser) parseValArray(b []byte) (int32, []byte, error) {
	arr := p.push(Node{Kind: Array})
	b = b[1:]

	var lastChild int32
	appendChild := func(h int32) {
		if lastChild == 0 {
			p.at(arr).child = h
		} else {
			p.at(lastChild).next = h
		}
		lastChild = h
	}

	// Comments inside the array are attached as follows: the first comment
	// of a "run" (consecutive comments with no value in between) becomes a
	// child of the array, interleaved with values; subsequent comments of the
	// run are attached as children of the first one.
	var runFirst, runLast int32

	// afterValue is true when a value has been parsed and a comma (or the
	// closing bracket) is expected before the next one.
	afterValue := false
	for {
		b = skipWhitespace(b)
		if len(b) == 0 {
			return 0, nil, NewParserError(b, "array is incomplete")
		}

		switch b[0] {
		case ']':
			return arr, b[1:], nil
		case '\n':
			b = b[1:]
			continue
		case '\r':
			if len(b) > 1 && b[1] == '\n' {
				b = b[2:]
				continue
			}
			return 0, nil, NewParserError(b[:1], "expected newline but got %#U", b[0])
		case '#':
			comment, rest, err := scanComment(b)
			if err != nil {
				return 0, nil, err
			}
			if p.KeepComments {
				h := p.push(Node{Kind: Comment, Raw: p.Range(comment), Data: comment})
				switch {
				case runFirst == 0:
					appendChild(h)
					runFirst = h
				case runLast == runFirst:
					p.at(runFirst).child = h
				default:
					p.at(runLast).next = h
				}
				runLast = h
			}
			b = rest
			continue
		case ',':
			if !afterValue {
				return 0, nil, NewParserError(b[:1], "expected value but got %#U", b[0])
			}
			afterValue = false
			b = b[1:]
			continue
		default:
			if afterValue {
				return 0, nil, NewParserError(b[:1], "expected ',' or ']' after array value")
			}
			h, rest, err := p.parseVal(b)
			if err != nil {
				return 0, nil, err
			}
			appendChild(h)
			afterValue = true
			runFirst, runLast = 0, 0
			b = rest
			continue
		}
	}
}

// parseInlineTable parses an inline table value. b starts at '{'.
//
// Per TOML v1.1.0, inline tables may span multiple lines (whitespace,
// comments and newlines are allowed between elements) and may contain a
// trailing comma.
func (p *Parser) parseInlineTable(b []byte) (int32, []byte, error) {
	tbl := p.push(Node{Kind: InlineTable, Raw: p.Range(b[:1])})
	b = b[1:]

	var lastChild int32
	appendChild := func(h int32) {
		if lastChild == 0 {
			p.at(tbl).child = h
		} else {
			p.at(lastChild).next = h
		}
		lastChild = h
	}

	// Comments are attached as in arrays: the first comment of a "run"
	// (consecutive comments with no key-value in between) becomes a child of
	// the table, interleaved with key-values; subsequent comments of the run
	// hang off the first one.
	var runFirst, runLast int32

	// afterValue is true when a key-value has been parsed and a comma (or the
	// closing brace) is expected before the next one.
	afterValue := false
	for {
		b = skipWhitespace(b)
		if len(b) == 0 {
			return 0, nil, NewParserError(b, "inline table is incomplete")
		}

		switch b[0] {
		case '}':
			return tbl, b[1:], nil
		case '\n':
			b = b[1:]
			continue
		case '\r':
			if len(b) > 1 && b[1] == '\n' {
				b = b[2:]
				continue
			}
			return 0, nil, NewParserError(b[:1], "expected newline but got %#U", b[0])
		case '#':
			comment, rest, err := scanComment(b)
			if err != nil {
				return 0, nil, err
			}
			if p.KeepComments {
				h := p.push(Node{Kind: Comment, Raw: p.Range(comment), Data: comment})
				switch {
				case runFirst == 0:
					appendChild(h)
					runFirst = h
				case runLast == runFirst:
					p.at(runFirst).child = h
				default:
					p.at(runLast).next = h
				}
				runLast = h
			}
			b = rest
			continue
		case ',':
			if !afterValue {
				return 0, nil, NewParserError(b[:1], "unexpected comma in inline table")
			}
			afterValue = false
			b = b[1:]
			continue
		default:
			if afterValue {
				return 0, nil, NewParserError(b[:1], "expected ',' or '}' after inline table key-value")
			}
			h, rest, err := p.parseKeyval(b)
			if err != nil {
				return 0, nil, err
			}
			appendChild(h)
			afterValue = true
			runFirst, runLast = 0, 0
			b = rest
			continue
		}
	}
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// isDateTimeStart reports whether b looks like the start of a date or time
// value instead of a number. Values starting with two digits followed by a
// colon are times; values starting with four digits followed by a dash are
// dates.
func isDateTimeStart(b []byte) bool {
	if len(b) >= 3 && isDigit(b[1]) && b[2] == ':' {
		return true
	}
	if len(b) >= 5 && isDigit(b[1]) && isDigit(b[2]) && isDigit(b[3]) && b[4] == '-' {
		return true
	}
	return false
}

// expectDigits checks that the n first bytes of b are digits.
// parseDateTime parses date and/or time values. b starts with a digit.
//
// The parser is lenient: it scans the characters that can be part of a date
// and/or time value to delimit and classify the token, but leaves the
// validation of its contents to the document consumer. This keeps the value
// in one piece, so that errors about its content can point at the right
// place.
func (p *Parser) parseDateTime(b []byte) (int32, []byte, error) {
	kind, raw, _, rest, err := scanDateTime(b)
	if err != nil {
		return 0, nil, err
	}
	h := p.push(Node{Kind: kind, Raw: p.Range(raw), Data: raw})
	return h, rest, nil
}

// scanDateTime classifies and delimits a date/time value without building an
// AST node. raw and value are identical (the token bytes).
func scanDateTime(b []byte) (Kind, []byte, []byte, []byte, error) {
	// Greedily scan the characters that may compose a date/time value. A
	// space is part of the value only when it serves as the delimiter
	// between the date and the time, which is approximated by requiring a
	// digit right after it.
	i := 0
	delim := -1
	for i < len(b) {
		c := b[i]
		if isDigit(c) || c == ':' || c == '-' || c == '+' || c == '.' || c == 'Z' || c == 'z' {
			i++
			continue
		}
		if c == 'T' || c == 't' || (c == ' ' && i+1 < len(b) && isDigit(b[i+1])) {
			if delim < 0 {
				delim = i
			}
			i++
			continue
		}
		break
	}
	tok := b[:i]

	var kind Kind
	switch {
	case tok[2] == ':':
		kind = LocalTime
	case delim < 0:
		kind = LocalDate
	case bytes.ContainsAny(tok[delim+1:], "Zz+-"):
		kind = DateTime
	default:
		kind = LocalDateTime
	}

	return kind, tok, tok, b[i:], nil
}

// scanDigitsWithUnderscores scans a run of digits potentially separated by
// underscores. b starts right after the first digit of the run. isInRange
// selects the kind of digits. Returns the index after the run.
func scanDigitsWithUnderscores(b []byte, i int, isInRange func(byte) bool) (int, error) {
	for i < len(b) {
		c := b[i]
		if isInRange(c) {
			i++
			continue
		}
		if c == '_' {
			if i+1 >= len(b) || !isInRange(b[i+1]) {
				end := i + 2
				if end > len(b) {
					end = len(b)
				}
				return 0, NewParserError(b[i:end], "number must have at least one digit between underscores")
			}
			i += 2
			continue
		}
		break
	}
	return i, nil
}

// parseIntOrFloat parses integer and float values, including the special
// values inf and nan with an optional sign.
func (p *Parser) parseIntOrFloat(b []byte) (int32, []byte, error) {
	kind, raw, _, rest, err := scanIntOrFloat(b)
	if err != nil {
		return 0, nil, err
	}
	h := p.push(Node{Kind: kind, Raw: p.Range(raw), Data: raw})
	return h, rest, nil
}

// scanIntOrFloat delimits and classifies an integer or float value (including
// the special floats inf and nan with an optional sign) without building an
// AST node. raw and value are identical (the token bytes).
func scanIntOrFloat(b []byte) (Kind, []byte, []byte, []byte, error) {
	i := 0
	if b[i] == '+' || b[i] == '-' {
		i++
	}
	if i >= len(b) {
		return Invalid, nil, nil, nil, NewParserError(b, "expected number after sign")
	}

	// special floats
	if b[i] == 'i' || b[i] == 'n' {
		kw := "inf"
		if b[i] == 'n' {
			kw = "nan"
		}
		if len(b) < i+3 || string(b[i:i+3]) != kw {
			return Invalid, nil, nil, nil, NewParserError(b[i:i+1], "expected %q", kw)
		}
		i += 3
		return Float, b[:i], b[:i], b[i:], nil
	}

	if !isDigit(b[i]) {
		return Invalid, nil, nil, nil, NewParserError(b[i:i+1], "expected digit but got %#U", b[i])
	}

	// radix prefixes
	if b[i] == '0' && i+1 < len(b) && (b[i+1] == 'x' || b[i+1] == 'o' || b[i+1] == 'b') {
		if i != 0 {
			return Invalid, nil, nil, nil, NewParserError(b[:2], "sign is not allowed on numbers with a radix prefix")
		}
		var isInRange func(byte) bool
		switch b[1] {
		case 'x':
			isInRange = isHexDigit
		case 'o':
			isInRange = func(c byte) bool { return c >= '0' && c <= '7' }
		case 'b':
			isInRange = func(c byte) bool { return c == '0' || c == '1' }
		}
		i = 2
		if i >= len(b) || !isInRange(b[i]) {
			return Invalid, nil, nil, nil, NewParserError(b[:2], "radix prefix must be followed by at least one digit")
		}
		i++
		var err error
		i, err = scanDigitsWithUnderscores(b, i, isInRange)
		if err != nil {
			return Invalid, nil, nil, nil, err
		}
		return Integer, b[:i], b[:i], b[i:], nil
	}

	// decimal integer part
	leadingZero := b[i] == '0'
	digitsStart := i
	i++
	var err error
	i, err = scanDigitsWithUnderscores(b, i, isDigit)
	if err != nil {
		return Invalid, nil, nil, nil, err
	}
	if leadingZero && i > digitsStart+1 {
		return Invalid, nil, nil, nil, NewParserError(b[digitsStart:digitsStart+2], "integers cannot have leading zeroes")
	}

	kind := Integer

	// fractional part
	if i < len(b) && b[i] == '.' {
		i++
		if i >= len(b) || !isDigit(b[i]) {
			return Invalid, nil, nil, nil, NewParserError(highlight1(b[i:]), "decimal point must be followed by a digit")
		}
		i++
		i, err = scanDigitsWithUnderscores(b, i, isDigit)
		if err != nil {
			return Invalid, nil, nil, nil, err
		}
		kind = Float
	}

	// exponent
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		if i >= len(b) || !isDigit(b[i]) {
			return Invalid, nil, nil, nil, NewParserError(highlight1(b[i:]), "exponent must contain at least one digit")
		}
		i++
		i, err = scanDigitsWithUnderscores(b, i, isDigit)
		if err != nil {
			return Invalid, nil, nil, nil, err
		}
		kind = Float
	}

	// A letter right after the number means it was meant to be a string that
	// was left unquoted (e.g. "20s"). Report that instead of the misleading
	// "expected newline" raised later (issue #413).
	if i < len(b) {
		if c := b[i]; (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return Invalid, nil, nil, nil, NewParserError(b[i:i+1], "strings must be quoted")
		}
	}

	return kind, b[:i], b[:i], b[i:], nil
}

// highlight1 returns a 1-byte highlight at the start of b, or b itself if it
// is empty.
func highlight1(b []byte) []byte {
	if len(b) > 0 {
		return b[:1]
	}
	return b
}

func skipWhitespace(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	return b
}

// Word-at-a-time byte scanning helpers. These detect, within an 8-byte word,
// the presence of bytes that need special handling, so that runs of plain
// ASCII characters can be skipped 8 bytes at a time.
const (
	lsb = 0x0101010101010101
	msb = 0x8080808080808080
)

// hasByteBelow reports whether any byte of the word x is strictly below n.
// Only meaningful when combined with a check that no byte has its high bit
// set.
func hasByteBelow(x uint64, n uint64) uint64 {
	return (x - n*lsb) & ^x & msb
}

// hasByteEqual reports whether any byte of the word x equals c. Only
// meaningful for bytes without their high bit set.
func hasByteEqual(x uint64, c uint64) uint64 {
	y := x ^ (c * lsb)
	return (y - lsb) & ^y & msb
}

// scanComment parses a comment, starting at the '#' character. It returns the
// comment bytes (including '#', excluding the line ending) and the rest of
// the input.
func scanComment(b []byte) ([]byte, []byte, error) {
	i := 1
	for i < len(b) {
		// Fast path: skip 8 bytes at a time as long as they are all plain
		// printable ASCII.
		for i+8 <= len(b) {
			x := binary.LittleEndian.Uint64(b[i:])
			if (x&msb)|hasByteBelow(x, 0x20)|hasByteEqual(x, 0x7f) != 0 {
				break
			}
			i += 8
		}
		if i >= len(b) {
			break
		}

		c := b[i]
		if c >= 0x80 {
			var ok bool
			i, ok = scanUtf8Run(b, i)
			if !ok {
				return nil, nil, NewParserError(b[i:i+1], "invalid UTF-8 character in comment")
			}
			continue
		}
		switch {
		case c >= 0x20 && c < 0x7f:
			i++
		case c == '\n':
			return b[:i], b[i:], nil
		case c == '\r':
			if i+1 < len(b) && b[i+1] == '\n' {
				return b[:i], b[i:], nil
			}
			return nil, nil, NewParserError(b[i:i+1], "carriage returns are not allowed in comments")
		case c == '\t':
			i++
		default:
			return nil, nil, NewParserError(b[i:i+1], "control characters are not allowed in comments")
		}
	}
	return b[:i], b[i:], nil
}

// parseLiteralString parses a single-line literal string, starting at the
// opening quote. Returns the raw bytes (with quotes), the string value
// (without quotes) and the rest of the input.
func (p *Parser) parseLiteralString(b []byte) ([]byte, []byte, []byte, error) {
	i := 1
	for {
		// Fast path over plain ASCII.
		for i+8 <= len(b) {
			x := binary.LittleEndian.Uint64(b[i:])
			if (x&msb)|hasByteBelow(x, 0x20)|hasByteEqual(x, '\'')|hasByteEqual(x, 0x7f) != 0 {
				break
			}
			i += 8
		}
		if i >= len(b) {
			return nil, nil, nil, NewParserError(b[len(b):], "unterminated literal string")
		}

		c := b[i]
		switch {
		case c == '\'':
			return b[:i+1], b[1:i], b[i+1:], nil
		case c >= 0x20 && c < 0x7f:
			i++
		case c == '\t':
			i++
		case c == '\n' || c == '\r':
			return nil, nil, nil, NewParserError(b[i:i+1], "literal strings cannot have new lines")
		case c < 0x80:
			return nil, nil, nil, NewParserError(b[i:i+1], "literal strings cannot have control characters")
		default:
			var ok bool
			i, ok = scanUtf8Run(b, i)
			if !ok {
				return nil, nil, nil, NewParserError(b[i:i+1], "invalid UTF-8 character in literal string")
			}
		}
	}
}

// parseMultilineLiteralString parses a multi-line literal string, starting at
// the opening triple quote.
func (p *Parser) parseMultilineLiteralString(b []byte) ([]byte, []byte, []byte, error) {
	i := 3
	// trim the newline right after the opening delimiter
	if i < len(b) && b[i] == '\n' {
		i++
	} else if i+1 < len(b) && b[i] == '\r' && b[i+1] == '\n' {
		i += 2
	}
	contentStart := i

	for i < len(b) {
		c := b[i]
		switch {
		case c == '\'':
			// count consecutive quotes
			j := i
			for j < len(b) && b[j] == '\'' {
				j++
			}
			n := j - i
			if n >= 3 {
				if n > 5 {
					return nil, nil, nil, NewParserError(b[i:j], "too many quotes at the end of a multiline literal string")
				}
				// n-3 quotes belong to the content; the last 3 close the
				// string.
				contentEnd := i + n - 3
				return b[:j], b[contentStart:contentEnd], b[j:], nil
			}
			i = j
		case c >= 0x20 && c < 0x7f:
			i++
		case c == '\t' || c == '\n':
			i++
		case c == '\r':
			if i+1 < len(b) && b[i+1] == '\n' {
				i += 2
				continue
			}
			return nil, nil, nil, NewParserError(b[i:i+1], "carriage returns must be followed by a newline character")
		case c < 0x80:
			return nil, nil, nil, NewParserError(b[i:i+1], "multiline literal strings cannot have control characters")
		default:
			var ok bool
			i, ok = scanUtf8Run(b, i)
			if !ok {
				return nil, nil, nil, NewParserError(b[i:i+1], "invalid UTF-8 character in multiline literal string")
			}
		}
	}
	return nil, nil, nil, NewParserError(b[len(b):], "multiline literal string not terminated by '''")
}

// parseBasicString parses a single-line basic string, starting at the opening
// quote. The value is a subslice of the input if the string contains no
// escape sequence, or a new allocation otherwise.
func (p *Parser) parseBasicString(b []byte) ([]byte, []byte, []byte, error) {
	i := 1
	// First pass: handle strings without escape sequences without allocating.
	for {
		for i+8 <= len(b) {
			x := binary.LittleEndian.Uint64(b[i:])
			if (x&msb)|hasByteBelow(x, 0x20)|hasByteEqual(x, '"')|hasByteEqual(x, '\\')|hasByteEqual(x, 0x7f) != 0 {
				break
			}
			i += 8
		}
		if i >= len(b) {
			return nil, nil, nil, NewParserError(b[len(b):], "unterminated basic string")
		}

		c := b[i]
		switch {
		case c == '"':
			return b[:i+1], b[1:i], b[i+1:], nil
		case c == '\\':
			// switch to the escape-aware parser, copying what has been
			// scanned so far
			return p.parseBasicStringEscaped(b, i)
		case c >= 0x20 && c < 0x7f:
			i++
		case c == '\t':
			i++
		case c == '\n' || c == '\r':
			return nil, nil, nil, NewParserError(b[i:i+1], "basic strings cannot have new lines")
		case c < 0x80:
			return nil, nil, nil, NewParserError(b[i:i+1], "basic strings cannot have control characters")
		default:
			var ok bool
			i, ok = scanUtf8Run(b, i)
			if !ok {
				return nil, nil, nil, NewParserError(b[i:i+1], "invalid UTF-8 character in basic string")
			}
		}
	}
}

// findBasicStringEnd returns the index of the quote closing a basic string,
// or -1 if the string is not terminated. i is the index of the first
// character after the opening quote. It does not validate the content: it
// only skips over escape sequences so that escaped quotes do not terminate
// the string.
func findBasicStringEnd(b []byte, i int) int {
	for i < len(b) {
		switch b[i] {
		case '"':
			return i
		case '\\':
			i += 2
		default:
			i++
		}
	}
	return -1
}

// parseBasicStringEscaped continues parsing a basic string that contains
// escape sequences. i is the index of the first backslash.
func (p *Parser) parseBasicStringEscaped(b []byte, i int) ([]byte, []byte, []byte, error) {
	// Escape sequences only ever shrink, so the content length before
	// unescaping is enough to never reallocate.
	bufCap := len(b) - 1
	if end := findBasicStringEnd(b, i); end >= 0 {
		bufCap = end - 1
	}
	buf := make([]byte, i-1, bufCap)
	copy(buf, b[1:i])

	for i < len(b) {
		c := b[i]
		switch {
		case c == '"':
			return b[:i+1], buf, b[i+1:], nil
		case c == '\\':
			i++
			if i >= len(b) {
				return nil, nil, nil, NewParserError(b[i-1:], `need a character after \`)
			}
			var err error
			buf, i, err = unescape(buf, b, i)
			if err != nil {
				return nil, nil, nil, err
			}
		case c >= 0x20 && c < 0x7f:
			buf = append(buf, c)
			i++
		case c == '\t':
			buf = append(buf, c)
			i++
		case c == '\n' || c == '\r':
			return nil, nil, nil, NewParserError(b[i:i+1], "basic strings cannot have new lines")
		case c < 0x80:
			return nil, nil, nil, NewParserError(b[i:i+1], "basic strings cannot have control characters")
		default:
			j, ok := scanUtf8Run(b, i)
			if !ok {
				return nil, nil, nil, NewParserError(b[i:i+1], "invalid UTF-8 character in basic string")
			}
			buf = append(buf, b[i:j]...)
			i = j
		}
	}
	return nil, nil, nil, NewParserError(b[len(b):], "unterminated basic string")
}

// unescape processes one escape sequence. i is the index of the character
// right after the backslash. It returns the updated buffer and index.
func unescape(buf []byte, b []byte, i int) ([]byte, int, error) {
	c := b[i]
	switch c {
	case '"':
		return append(buf, '"'), i + 1, nil
	case '\\':
		return append(buf, '\\'), i + 1, nil
	case 'b':
		return append(buf, '\b'), i + 1, nil
	case 'f':
		return append(buf, '\f'), i + 1, nil
	case 'n':
		return append(buf, '\n'), i + 1, nil
	case 'r':
		return append(buf, '\r'), i + 1, nil
	case 't':
		return append(buf, '\t'), i + 1, nil
	case 'e':
		// TOML v1.1.0: \e is the escape character (U+001B).
		return append(buf, 0x1B), i + 1, nil
	case 'x':
		// TOML v1.1.0: \xHH is a two-digit hexadecimal code point.
		return unescapeUnicode(buf, b, i+1, 2)
	case 'u':
		return unescapeUnicode(buf, b, i+1, 4)
	case 'U':
		return unescapeUnicode(buf, b, i+1, 8)
	default:
		return nil, 0, NewParserError(b[i-1:i+1], "invalid escape character %#U", c)
	}
}

// unescapeUnicode handles \uXXXX and \UXXXXXXXX escape sequences. i is the
// index of the first hex digit.
func unescapeUnicode(buf []byte, b []byte, i int, n int) ([]byte, int, error) {
	if i+n > len(b) {
		return nil, 0, NewParserError(b[i-2:], "unicode escape sequence is too short")
	}
	var r uint32
	for k := 0; k < n; k++ {
		c := b[i+k]
		var d uint32
		switch {
		case c >= '0' && c <= '9':
			d = uint32(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint32(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = uint32(c-'A') + 10
		default:
			return nil, 0, NewParserError(b[i+k:i+k+1], "invalid hexadecimal digit in unicode escape sequence")
		}
		r = r<<4 | d
	}
	if r > utf8.MaxRune || (r >= 0xD800 && r <= 0xDFFF) {
		return nil, 0, NewParserError(b[i-2:i+n], "escape sequence is not a valid unicode code point")
	}
	return utf8.AppendRune(buf, rune(r)), i + n, nil
}

// parseMultilineBasicString parses a multi-line basic string, starting at the
// opening triple quote.
func (p *Parser) parseMultilineBasicString(b []byte) ([]byte, []byte, []byte, error) {
	i := 3
	// trim the newline right after the opening delimiter
	if i < len(b) && b[i] == '\n' {
		i++
	} else if i+1 < len(b) && b[i] == '\r' && b[i+1] == '\n' {
		i += 2
	}
	contentStart := i

	// First pass without allocating, until an escape sequence is found.
	for i < len(b) {
		c := b[i]
		switch {
		case c == '"':
			j := i
			for j < len(b) && b[j] == '"' {
				j++
			}
			n := j - i
			if n >= 3 {
				if n > 5 {
					return nil, nil, nil, NewParserError(b[i:j], "too many quotes at the end of a multiline basic string")
				}
				contentEnd := i + n - 3
				return b[:j], b[contentStart:contentEnd], b[j:], nil
			}
			i = j
		case c == '\\':
			return p.parseMultilineBasicStringEscaped(b, contentStart, i)
		case c >= 0x20 && c < 0x7f:
			i++
		case c == '\t' || c == '\n':
			i++
		case c == '\r':
			if i+1 < len(b) && b[i+1] == '\n' {
				i += 2
				continue
			}
			return nil, nil, nil, NewParserError(b[i:i+1], "carriage returns must be followed by a newline character")
		case c < 0x80:
			return nil, nil, nil, NewParserError(b[i:i+1], "multiline basic strings cannot have control characters")
		default:
			var ok bool
			i, ok = scanUtf8Run(b, i)
			if !ok {
				return nil, nil, nil, NewParserError(b[i:i+1], "invalid UTF-8 character in multiline basic string")
			}
		}
	}
	return nil, nil, nil, NewParserError(b[len(b):], `multiline basic string not terminated by """`)
}

// findMultilineBasicStringEnd returns the index of the first quote of the
// run of quotes closing a multi-line basic string, or -1 if the string is
// not terminated. It does not validate the content: it only skips over
// escape sequences so that escaped quotes do not terminate the string.
func findMultilineBasicStringEnd(b []byte, i int) int {
	for {
		j := bytes.IndexAny(b[i:], "\"\\")
		if j < 0 {
			return -1
		}
		i += j
		if b[i] == '\\' {
			i += 2
			if i > len(b) {
				return -1
			}
			continue
		}
		j = i
		for j < len(b) && b[j] == '"' {
			j++
		}
		if j-i >= 3 {
			return i
		}
		i = j
	}
}

// parseMultilineBasicStringEscaped continues parsing a multi-line basic
// string that contains escape sequences. i is the index of the first
// backslash; content starts at contentStart.
func (p *Parser) parseMultilineBasicStringEscaped(b []byte, contentStart, i int) ([]byte, []byte, []byte, error) {
	// Escape sequences only ever shrink, so the content length before
	// unescaping is enough to never reallocate. The closing run of quotes
	// can lend up to two quotes to the content.
	bufCap := len(b) - contentStart
	if end := findMultilineBasicStringEnd(b, i); end >= 0 {
		bufCap = end + 2 - contentStart
	}
	buf := make([]byte, i-contentStart, bufCap)
	copy(buf, b[contentStart:i])

	for i < len(b) {
		c := b[i]
		switch {
		case c == '"':
			j := i
			for j < len(b) && b[j] == '"' {
				j++
			}
			n := j - i
			if n >= 3 {
				if n > 5 {
					return nil, nil, nil, NewParserError(b[i:j], "too many quotes at the end of a multiline basic string")
				}
				buf = append(buf, b[i:i+n-3]...)
				return b[:j], buf, b[j:], nil
			}
			buf = append(buf, b[i:j]...)
			i = j
		case c == '\\':
			i++
			if i >= len(b) {
				return nil, nil, nil, NewParserError(b[i-1:], `need a character after \`)
			}
			// Escaped newline: backslash, optional whitespace, newline,
			// then all following whitespace and newlines are trimmed.
			if b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r' {
				j := i
				for j < len(b) && (b[j] == ' ' || b[j] == '\t') {
					j++
				}
				if j < len(b) && b[j] == '\r' {
					j++
				}
				if j >= len(b) || b[j] != '\n' {
					return nil, nil, nil, NewParserError(b[i-1:i+1], "invalid escape character %#U", b[i])
				}
				j++
				for j < len(b) && (b[j] == ' ' || b[j] == '\t' || b[j] == '\n' || b[j] == '\r') {
					// note: a lone \r not followed by \n will be caught on
					// the next iteration of the outer loop.
					if b[j] == '\r' {
						if j+1 >= len(b) || b[j+1] != '\n' {
							break
						}
						j++
					}
					j++
				}
				i = j
				continue
			}
			var err error
			buf, i, err = unescape(buf, b, i)
			if err != nil {
				return nil, nil, nil, err
			}
		case c >= 0x20 && c < 0x7f:
			buf = append(buf, c)
			i++
		case c == '\t' || c == '\n':
			buf = append(buf, c)
			i++
		case c == '\r':
			if i+1 < len(b) && b[i+1] == '\n' {
				buf = append(buf, '\r', '\n')
				i += 2
				continue
			}
			return nil, nil, nil, NewParserError(b[i:i+1], "carriage returns must be followed by a newline character")
		case c < 0x80:
			return nil, nil, nil, NewParserError(b[i:i+1], "multiline basic strings cannot have control characters")
		default:
			j, ok := scanUtf8Run(b, i)
			if !ok {
				return nil, nil, nil, NewParserError(b[i:i+1], "invalid UTF-8 character in multiline basic string")
			}
			buf = append(buf, b[i:j]...)
			i = j
		}
	}
	return nil, nil, nil, NewParserError(b[len(b):], `multiline basic string not terminated by """`)
}

// scanUtf8Run consumes a run of valid non-ASCII UTF-8 runes starting at
// b[i]. It returns the index of the first byte after the run, and whether
// the run was entirely valid. Processing whole runs amortizes the cost of
// the call compared to validating rune by rune.
func scanUtf8Run(b []byte, i int) (int, bool) {
	for i < len(b) {
		c := b[i]
		switch {
		case c < 0x80:
			return i, true
		case c < 0xC2:
			return i, false
		case c < 0xE0:
			if i+1 >= len(b) || b[i+1]&0xC0 != 0x80 {
				return i, false
			}
			i += 2
		case c < 0xF0:
			if i+2 >= len(b) || b[i+2]&0xC0 != 0x80 {
				return i, false
			}
			b1 := b[i+1]
			switch c {
			case 0xE0:
				if b1 < 0xA0 || b1 > 0xBF {
					return i, false
				}
			case 0xED:
				// exclude surrogates
				if b1 < 0x80 || b1 > 0x9F {
					return i, false
				}
			default:
				if b1&0xC0 != 0x80 {
					return i, false
				}
			}
			i += 3
		case c < 0xF5:
			if i+3 >= len(b) || b[i+2]&0xC0 != 0x80 || b[i+3]&0xC0 != 0x80 {
				return i, false
			}
			b1 := b[i+1]
			switch c {
			case 0xF0:
				if b1 < 0x90 || b1 > 0xBF {
					return i, false
				}
			case 0xF4:
				if b1 < 0x80 || b1 > 0x8F {
					return i, false
				}
			default:
				if b1&0xC0 != 0x80 {
					return i, false
				}
			}
			i += 4
		default:
			return i, false
		}
	}
	return i, true
}
