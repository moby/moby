package tracker

import (
	"bytes"
	"fmt"

	"github.com/pelletier/go-toml/v2/unstable"
)

type keyKind uint8

const (
	invalidKind keyKind = iota
	// valueKind is a regular value (scalar, array, or inline table). It
	// cannot be extended.
	valueKind
	// kvTableKind is a table created implicitly by a dotted key. It can only
	// be extended by other dotted keys.
	kvTableKind
	// tableKind is a table created by a [header]. The explicit flag tells
	// whether the table was created by its own header (true) or as an
	// intermediate step of a longer key (false).
	tableKind
	// arrayTableKind is an array of tables created by [[header]].
	arrayTableKind
	// anonymousKind is an entry that cannot be looked up by name. It serves
	// as the parent of the content of inline tables stored inside arrays.
	anonymousKind
)

func (k keyKind) String() string {
	switch k {
	case invalidKind:
		return "invalid"
	case valueKind:
		return "value"
	case kvTableKind:
		return "kv-table"
	case tableKind:
		return "table"
	case arrayTableKind:
		return "array-table"
	case anonymousKind:
		return "anonymous"
	}
	panic("missing keyKind string mapping")
}

// entry represents a node that has been seen in the document. Its size has a
// direct impact on the performance of unmarshaling documents: keep it as
// small as possible.
type entry struct {
	parent   int32
	kind     keyKind
	explicit bool
	name     []byte
}

// SeenTracker tracks which keys have been seen with which TOML type to flag
// duplicates and mismatches according to the spec.
//
// Each node in the visited tree is represented by an entry. Each entry has
// an identifier, which is provided by a counter. Entries are stored in the
// array entries. As new nodes are discovered (referenced for the first time
// in the TOML document), entries are created and appended to the array. An
// entry points to its parent using its id.
//
// To find whether a given key (sequence of []byte) has already been visited,
// the entries are linearly searched, looking for one with the right name and
// parent id.
//
// Given that all keys appear in the document after their parent, it is
// guaranteed that all descendants of a node are stored after the node, this
// speeds up the search process.
//
// When encountering [[array tables]], the descendants of that node are removed
// to allow that branch of the tree to be "rediscovered". To maintain the
// invariant above, the deletion process needs to keep the order of entries.
// This results in more copies in that case.
type SeenTracker struct {
	entries      []entry
	currentTable int32

	// scratch buffers for clear()
	removedBuf []bool
	remapBuf   []int32
}

// Reset brings the tracker to its initial state, with just a root table, so
// that it can be reused across documents.
func (s *SeenTracker) Reset() {
	s.reset()
}

// reset brings the tracker to its initial state, with just a root table.
func (s *SeenTracker) reset() {
	s.entries = append(s.entries[:0], entry{
		parent: -1,
		kind:   tableKind,
	})
	s.currentTable = 0
}

// find returns the id of the entry with the given parent and name, or -1.
// Anonymous entries are never returned.
func (s *SeenTracker) find(parent int32, name []byte) int32 {
	// Children always appear after their parent.
	for i := int(parent) + 1; i < len(s.entries); i++ {
		e := &s.entries[i]
		if e.parent == parent && e.kind != anonymousKind && bytes.Equal(e.name, name) {
			return int32(i) //nolint:gosec // entry counts are bounded by document size
		}
	}
	return -1
}

// create appends a new entry and returns its id.
func (s *SeenTracker) create(parent int32, name []byte, kind keyKind, explicit bool) int32 {
	id := int32(len(s.entries)) //nolint:gosec // entry counts are bounded by document size
	s.entries = append(s.entries, entry{
		parent:   parent,
		kind:     kind,
		explicit: explicit,
		name:     name,
	})
	return id
}

// clear removes all the descendants of the entry with the given id, keeping
// the order of the remaining entries.
func (s *SeenTracker) clear(id int32) {
	// Compute which entries are removed. Given that children always appear
	// after their parent, a single forward pass is enough.
	if cap(s.removedBuf) < len(s.entries) {
		s.removedBuf = make([]bool, len(s.entries))
		s.remapBuf = make([]int32, len(s.entries))
	}
	removed := s.removedBuf[:len(s.entries)]
	remap := s.remapBuf[:len(s.entries)]
	for i := range removed {
		removed[i] = false
	}

	n := int32(0)
	for i := 0; i < len(s.entries); i++ {
		parent := s.entries[i].parent
		if parent >= 0 && (parent == id && s.entries[i].kind != invalidKind || removed[parent]) {
			removed[i] = true
			continue
		}
		remap[i] = n
		if int32(i) != n { //nolint:gosec // entry counts are bounded by document size
			e := s.entries[i]
			e.parent = remap[e.parent]
			s.entries[n] = e
		}
		n++
	}
	s.entries = s.entries[:n]
}

// CheckExpression takes a top-level node and checks that it does not contain
// keys that have been seen in previous calls, and validates that types are
// consistent. It returns true if it is the first time this node's key is
// seen. Useful to clear array tables on first use.
func (s *SeenTracker) CheckExpression(node *unstable.Node) (bool, error) {
	if len(s.entries) == 0 {
		s.reset()
	}
	switch node.Kind {
	case unstable.KeyValue:
		return false, s.checkKeyValue(s.currentTable, node)
	case unstable.Table:
		return s.checkTable(node)
	case unstable.ArrayTable:
		return s.checkArrayTable(node)
	default:
		return false, fmt.Errorf("toml: unexpected expression kind %s", node.Kind)
	}
}

// CheckTable validates a [table] header given the decoded parts of its key.
// It mirrors checkTable but is driven directly from the key parts instead of
// an AST, for callers that decode without building one. It returns whether the
// table is seen for the first time.
func (s *SeenTracker) CheckTable(parts [][]byte) (bool, error) {
	parent := int32(0)
	for k := 0; k < len(parts); k++ {
		name := parts[k]
		if k == len(parts)-1 {
			// Final part of the key.
			i := s.find(parent, name)
			if i < 0 {
				i = s.create(parent, name, tableKind, true)
				s.currentTable = i
				return true, nil
			}
			e := &s.entries[i]
			switch e.kind {
			case tableKind:
				if e.explicit {
					return false, fmt.Errorf("toml: table %s already exists", name)
				}
				e.explicit = true
				s.currentTable = i
				return false, nil
			case kvTableKind:
				return false, fmt.Errorf("toml: table %s already exists as defined by a dotted key", name)
			case arrayTableKind:
				return false, fmt.Errorf("toml: table %s already exists as an array of tables", name)
			default:
				return false, fmt.Errorf("toml: key %s should be a table, not a %s", name, e.kind)
			}
		}

		i := s.find(parent, name)
		if i < 0 {
			i = s.create(parent, name, tableKind, false)
		} else {
			switch s.entries[i].kind {
			case tableKind, arrayTableKind, kvTableKind:
				// Tables created by dotted keys can receive new sub-tables,
				// but cannot be redefined (handled by the last-part case).
			default:
				return false, fmt.Errorf("toml: key %s already exists as a value", name)
			}
		}
		parent = i
	}
	panic("unreachable: table expression without key")
}

// CheckArrayTable validates a [[array table]] header given the decoded parts
// of its key. It mirrors checkArrayTable but is driven directly from the key
// parts. It returns whether the array table is seen for the first time.
func (s *SeenTracker) CheckArrayTable(parts [][]byte) (bool, error) {
	parent := int32(0)
	for k := 0; k < len(parts); k++ {
		name := parts[k]
		if k == len(parts)-1 {
			i := s.find(parent, name)
			if i < 0 {
				i = s.create(parent, name, arrayTableKind, true)
				s.currentTable = i
				return true, nil
			}
			if s.entries[i].kind != arrayTableKind {
				return false, fmt.Errorf("toml: key %s already exists as a %s, but should be an array table", name, s.entries[i].kind)
			}
			// Make the descendants of this array table re-discoverable for
			// the new element.
			s.clear(i)
			s.currentTable = i
			return false, nil
		}

		i := s.find(parent, name)
		if i < 0 {
			i = s.create(parent, name, tableKind, false)
		} else {
			switch s.entries[i].kind {
			case tableKind, arrayTableKind, kvTableKind:
				// Tables created by dotted keys can receive new sub-tables,
				// but cannot be redefined (handled by the last-part case).
			default:
				return false, fmt.Errorf("toml: key %s already exists as a value", name)
			}
		}
		parent = i
	}
	panic("unreachable: array table expression without key")
}

// CheckKeyValue validates the (possibly dotted) key of a key-value under the
// current table, WITHOUT validating its value. It returns the id of the leaf
// entry, so the caller can validate a container value with CheckValueUnder.
func (s *SeenTracker) CheckKeyValue(parts [][]byte) (int32, error) {
	parent := s.currentTable
	for k := 0; k < len(parts); k++ {
		name := parts[k]
		if k == len(parts)-1 {
			if i := s.find(parent, name); i >= 0 {
				return -1, fmt.Errorf("toml: key %s is already defined", name)
			}
			return s.create(parent, name, valueKind, false), nil
		}

		i := s.find(parent, name)
		if i < 0 {
			i = s.create(parent, name, kvTableKind, false)
		} else if s.entries[i].kind != kvTableKind {
			return -1, fmt.Errorf("toml: key %s is already defined", name)
		}
		parent = i
	}
	panic("unreachable: key-value expression without key")
}

// CheckValueUnder validates the content of a value stored under the given
// entry (typically the leaf returned by CheckKeyValue): inline tables cannot
// contain duplicate keys, including in the inline tables and arrays they
// contain.
func (s *SeenTracker) CheckValueUnder(parent int32, value *unstable.Node) error {
	return s.checkValue(parent, value)
}

func (s *SeenTracker) checkTable(node *unstable.Node) (bool, error) {
	parent := int32(0)

	it := node.Key()
	// Handle the intermediate parts of the key.
	for it.Next() {
		part := it.Node()
		name := part.Data
		if it.IsLast() {
			// Final part of the key.
			i := s.find(parent, name)
			if i < 0 {
				i = s.create(parent, name, tableKind, true)
				s.currentTable = i
				return true, nil
			}
			e := &s.entries[i]
			switch e.kind {
			case tableKind:
				if e.explicit {
					return false, fmt.Errorf("toml: table %s already exists", name)
				}
				e.explicit = true
				s.currentTable = i
				return false, nil
			case kvTableKind:
				return false, fmt.Errorf("toml: table %s already exists as defined by a dotted key", name)
			case arrayTableKind:
				return false, fmt.Errorf("toml: table %s already exists as an array of tables", name)
			default:
				return false, fmt.Errorf("toml: key %s should be a table, not a %s", name, e.kind)
			}
		}

		i := s.find(parent, name)
		if i < 0 {
			i = s.create(parent, name, tableKind, false)
		} else {
			switch s.entries[i].kind {
			case tableKind, arrayTableKind, kvTableKind:
				// Tables created by dotted keys can receive new sub-tables,
				// but cannot be redefined (handled by the last-part case).
			default:
				return false, fmt.Errorf("toml: key %s already exists as a value", name)
			}
		}
		parent = i
	}
	panic("unreachable: table expression without key")
}

func (s *SeenTracker) checkArrayTable(node *unstable.Node) (bool, error) {
	parent := int32(0)

	it := node.Key()
	for it.Next() {
		part := it.Node()
		name := part.Data
		if it.IsLast() {
			i := s.find(parent, name)
			if i < 0 {
				i = s.create(parent, name, arrayTableKind, true)
				s.currentTable = i
				return true, nil
			}
			if s.entries[i].kind != arrayTableKind {
				return false, fmt.Errorf("toml: key %s already exists as a %s, but should be an array table", name, s.entries[i].kind)
			}
			// Make the descendants of this array table re-discoverable for
			// the new element.
			s.clear(i)
			// Note: clear cannot move i because i comes before all its
			// descendants.
			s.currentTable = i
			return false, nil
		}

		i := s.find(parent, name)
		if i < 0 {
			i = s.create(parent, name, tableKind, false)
		} else {
			switch s.entries[i].kind {
			case tableKind, arrayTableKind, kvTableKind:
				// Tables created by dotted keys can receive new sub-tables,
				// but cannot be redefined (handled by the last-part case).
			default:
				return false, fmt.Errorf("toml: key %s already exists as a value", name)
			}
		}
		parent = i
	}
	panic("unreachable: array table expression without key")
}

func (s *SeenTracker) checkKeyValue(parent int32, node *unstable.Node) error {
	it := node.Key()
	for it.Next() {
		part := it.Node()
		name := part.Data
		if it.IsLast() {
			if i := s.find(parent, name); i >= 0 {
				return fmt.Errorf("toml: key %s is already defined", name)
			}
			id := s.create(parent, name, valueKind, false)
			return s.checkValue(id, node.Value())
		}

		i := s.find(parent, name)
		if i < 0 {
			i = s.create(parent, name, kvTableKind, false)
		} else if s.entries[i].kind != kvTableKind {
			return fmt.Errorf("toml: key %s is already defined", name)
		}
		parent = i
	}
	panic("unreachable: key-value expression without key")
}

// checkValue verifies the content of a value: inline tables cannot contain
// duplicate keys, including in the inline tables and arrays they contain.
func (s *SeenTracker) checkValue(id int32, value *unstable.Node) error {
	switch value.Kind {
	case unstable.InlineTable:
		it := value.Children()
		for it.Next() {
			if err := s.checkKeyValue(id, it.Node()); err != nil {
				return err
			}
		}
	case unstable.Array:
		it := value.Children()
		for it.Next() {
			elem := it.Node()
			if elem.Kind == unstable.InlineTable || elem.Kind == unstable.Array {
				elemID := s.create(id, nil, anonymousKind, false)
				if err := s.checkValue(elemID, elem); err != nil {
					return err
				}
			}
		}
	default:
	}
	return nil
}
