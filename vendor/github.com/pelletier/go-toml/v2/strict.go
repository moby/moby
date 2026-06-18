package toml

import (
	"github.com/pelletier/go-toml/v2/internal/tracker"
	"github.com/pelletier/go-toml/v2/unstable"
)

type strict struct {
	Enabled bool

	// Tracks the current key being processed.
	key tracker.KeyTracker

	missing []unstable.ParserError

	// Reference to the document for computing key ranges.
	doc []byte
}

func (s *strict) EnterTable(node *unstable.Node) {
	if !s.Enabled {
		return
	}

	s.key.UpdateTable(node)
}

func (s *strict) EnterArrayTable(node *unstable.Node) {
	if !s.Enabled {
		return
	}

	s.key.UpdateArrayTable(node)
}

func (s *strict) EnterKeyValue(node *unstable.Node) {
	if !s.Enabled {
		return
	}

	s.key.Push(node)
}

func (s *strict) ExitKeyValue(node *unstable.Node) {
	if !s.Enabled {
		return
	}

	s.key.Pop(node)
}

func (s *strict) MissingTable(node *unstable.Node) {
	if !s.Enabled {
		return
	}

	s.missing = append(s.missing, unstable.ParserError{
		Highlight: s.keyLocation(node),
		Message:   "missing table",
		Key:       s.key.Key(),
	})
}

func (s *strict) MissingField(node *unstable.Node) {
	if !s.Enabled {
		return
	}

	s.missing = append(s.missing, unstable.ParserError{
		Highlight: s.keyLocation(node),
		Message:   "unknown field",
		Key:       s.key.Key(),
	})
}

func (s *strict) Error(doc []byte) error {
	if !s.Enabled || len(s.missing) == 0 {
		return nil
	}

	err := &StrictMissingError{
		Errors: make([]DecodeError, 0, len(s.missing)),
	}

	for _, derr := range s.missing {
		derr := derr
		err.Errors = append(err.Errors, *wrapDecodeError(doc, &derr))
	}

	return err
}

func (s *strict) keyLocation(node *unstable.Node) []byte {
	k := node.Key()

	hasOne := k.Next()
	if !hasOne {
		panic("should not be called with empty key")
	}

	// Get the range from the first key to the last key.
	firstRaw := k.Node().Raw
	lastRaw := firstRaw

	for k.Next() {
		lastRaw = k.Node().Raw
	}

	// Compute the slice from the document using the ranges.
	start := firstRaw.Offset
	end := lastRaw.Offset + lastRaw.Length

	return s.doc[start:end]
}
