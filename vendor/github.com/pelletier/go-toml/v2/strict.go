package toml

import (
	"github.com/pelletier/go-toml/v2/internal/tracker"
	"github.com/pelletier/go-toml/v2/unstable"
)

type strict struct {
	Enabled bool

	// Tracks the current key being processed.
	key tracker.KeyTracker

	missing []decodeError
}

// decodeError is the information needed to materialize a DecodeError once the
// whole document is available.
type decodeError struct {
	highlight unstable.Range
	key       Key
	message   string
}

// Reset clears the state of the tracker so it can be reused for another
// document.
func (s *strict) Reset() {
	s.key = tracker.KeyTracker{}
	s.missing = s.missing[:0]
}

// EnterTable is called when a new table or array table expression starts
// being processed.
func (s *strict) EnterTable(node *unstable.Node) {
	if !s.Enabled {
		return
	}
	s.key.UpdateTable(node)
}

// MissingTable is called when a table is present in the document but has no
// corresponding field in the target.
func (s *strict) MissingTable(node *unstable.Node) {
	if !s.Enabled {
		return
	}
	s.missing = append(s.missing, decodeError{
		highlight: keyLocation(node),
		key:       s.key.Key(),
		message:   "missing table",
	})
}

// MissingField is called when a key-value is present in the document but has
// no corresponding field in the target.
func (s *strict) MissingField(node *unstable.Node) {
	if !s.Enabled {
		return
	}
	s.key.Push(node)
	s.missing = append(s.missing, decodeError{
		highlight: keyLocation(node),
		key:       s.key.Key(),
		message:   "unknown field",
	})
	s.key.Pop(node)
}

// Error returns the cumulated StrictMissingError for the document, or nil.
func (s *strict) Error(document []byte) error {
	if !s.Enabled || len(s.missing) == 0 {
		return nil
	}

	err := &StrictMissingError{
		Errors: make([]DecodeError, 0, len(s.missing)),
	}

	for _, derr := range s.missing {
		highlight := document[derr.highlight.Offset : derr.highlight.Offset+derr.highlight.Length]
		err.Errors = append(err.Errors, *newDecodeError(document, highlight, derr.key, derr.message))
	}

	return err
}

// keyLocation returns the range of the document covering all the parts of
// the key of the given node.
func keyLocation(node *unstable.Node) unstable.Range {
	k := node.Key()

	hasOne := k.Next()
	if !hasOne {
		panic("should not be called with empty key")
	}

	start := k.Node().Raw
	end := start

	for k.Next() {
		end = k.Node().Raw
	}

	return unstable.Range{
		Offset: start.Offset,
		Length: end.Offset + end.Length - start.Offset,
	}
}
