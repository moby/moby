package unstable

import (
	"errors"
	"fmt"
)

// Iterator over a sequence of nodes.
//
// Starts uninitialized, you need to call Next() first.
//
// For example:
//
//	it := n.Children()
//	for it.Next() {
//		n := it.Node()
//		// do something with n
//	}
type Iterator struct {
	nodes   *[]Node
	idx     int32
	started bool
}

// Next moves the iterator forward and returns true if points to a
// node, false otherwise.
func (c *Iterator) Next() bool {
	if c.nodes == nil {
		return false
	}
	nodes := *c.nodes
	if !c.started {
		c.started = true
	} else {
		idx := c.idx
		if idx >= 0 && int(idx) < len(nodes) {
			c.idx = nodes[idx].next
		}
	}
	return c.idx >= 0 && int(c.idx) < len(nodes)
}

// IsLast returns true if the current node of the iterator is the last
// one.  Subsequent calls to Next() will return false.
func (c *Iterator) IsLast() bool {
	return c.nodes == nil || c.idx < 0 || (*c.nodes)[c.idx].next < 0
}

// Node returns a pointer to the node pointed at by the iterator.
func (c *Iterator) Node() *Node {
	if c.nodes == nil || c.idx < 0 {
		return nil
	}
	n := &(*c.nodes)[c.idx]
	n.nodes = c.nodes
	return n
}

// Node in a TOML expression AST.
//
// Depending on Kind, its sequence of children should be interpreted
// differently.
//
//   - Array have one child per element in the array.
//   - InlineTable have one child per key-value in the table (each of kind
//     InlineTable).
//   - KeyValue have at least two children. The first one is the value. The rest
//     make a potentially dotted key.
//   - Table and ArrayTable's children represent a dotted key (same as
//     KeyValue, but without the first node being the value).
//
// When relevant, Raw describes the range of bytes this node is referring to in
// the input document. Use Parser.Raw() to retrieve the actual bytes.
type Node struct {
	Kind Kind
	Raw  Range  // Raw bytes from the input.
	Data []byte // Node value (either allocated or referencing the input).

	// Absolute indices into the backing nodes slice. -1 means none.
	next  int32
	child int32

	// Reference to the backing nodes slice for navigation.
	nodes *[]Node
}

// Range of bytes in the document.
type Range struct {
	Offset uint32
	Length uint32
}

// Next returns a pointer to the next node, or nil if there is no next node.
func (n *Node) Next() *Node {
	if n.next < 0 {
		return nil
	}
	next := &(*n.nodes)[n.next]
	next.nodes = n.nodes
	return next
}

// Child returns a pointer to the first child node of this node. Other children
// can be accessed calling Next on the first child.  Returns nil if this Node
// has no child.
func (n *Node) Child() *Node {
	if n.child < 0 {
		return nil
	}
	child := &(*n.nodes)[n.child]
	child.nodes = n.nodes
	return child
}

// Valid returns true if the node's kind is set (not to Invalid).
func (n *Node) Valid() bool {
	return n != nil
}

// Key returns the children nodes making the Key on a supported node. Panics
// otherwise.  They are guaranteed to be all be of the Kind Key. A simple key
// would return just one element.
func (n *Node) Key() Iterator {
	switch n.Kind {
	case KeyValue:
		child := n.child
		if child < 0 {
			panic(errors.New("KeyValue should have at least two children"))
		}
		valueNode := &(*n.nodes)[child]
		return Iterator{nodes: n.nodes, idx: valueNode.next}
	case Table, ArrayTable:
		return Iterator{nodes: n.nodes, idx: n.child}
	default:
		panic(fmt.Errorf("Key() is not supported on a %s", n.Kind))
	}
}

// Value returns a pointer to the value node of a KeyValue.
// Guaranteed to be non-nil.  Panics if not called on a KeyValue node,
// or if the Children are malformed.
func (n *Node) Value() *Node {
	return n.Child()
}

// Children returns an iterator over a node's children.
func (n *Node) Children() Iterator {
	return Iterator{nodes: n.nodes, idx: n.child}
}
