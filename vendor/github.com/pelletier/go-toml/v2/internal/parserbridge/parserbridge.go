// Package parserbridge exposes the unstable parser's non-AST scanners to the
// root toml package without making them part of the unstable public API.
//
// The fused generic-decode fast path needs to scan keys, scalars and comments
// (and parse container values into the arena) without going through the
// AST-pushing NextExpression/Expression methods. Those scanners depend on
// Parser internals (the string-unescape scratch buffer and the node arena), so
// they have to live in the unstable package; but they are an implementation
// detail of the decoder, not something we want to commit to in the public API.
//
// The unstable package populates these variables in its init; the toml package
// reads them. The parser is passed as an any (it is always an *unstable.Parser)
// and the scalar kind is an int (it is always an unstable.Kind) so that this
// package imports neither unstable nor toml, avoiding an import cycle. Passing
// a pointer through an interface does not allocate, so the fused path keeps its
// allocation profile.
package parserbridge

var (
	// ScanScalar scans a single scalar value (string, integer, float, bool or
	// date/time) without building an AST node. kind is an unstable.Kind.
	ScanScalar func(p any, b []byte) (kind int, raw, value, rest []byte, err error)

	// ScanKey scans a (possibly dotted) key without building AST nodes,
	// appending each decoded part to dst.
	ScanKey func(p any, b []byte, dst [][]byte) (parts [][]byte, raw, rest []byte, err error)

	// ScanComment scans a comment starting at '#', returning the comment bytes
	// (including '#', excluding the line ending) and the rest of the input. It
	// needs no parser state.
	ScanComment func(b []byte) (comment, rest []byte, err error)

	// ParseValue parses a single value (including arrays and inline tables) into
	// the parser arena, returning the root *unstable.Node and the rest of the
	// input.
	ParseValue func(p any, b []byte) (node any, rest []byte, err error)
)
