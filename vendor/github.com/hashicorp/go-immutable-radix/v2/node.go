package iradix

import (
	"bytes"
	"sort"
)

// WalkFn is used when walking the tree. Takes a
// key and value, returning if iteration should
// be terminated.
type WalkFn[T any] func(k []byte, v T) bool

// leafNode is used to represent a value
type leafNode[T any] struct {
	mutateCh chan struct{}
	key      []byte
	val      T
}

// edge is used to represent an edge node
type edge[T any] struct {
	label byte
	node  *Node[T]
}

// Node is an immutable node in the radix tree
type Node[T any] struct {
	// mutateCh is closed if this node is modified
	mutateCh chan struct{}

	// leaf is used to store possible leaf
	leaf *leafNode[T]

	// prefix is the common prefix we ignore
	prefix []byte

	// Edges should be stored in-order for iteration.
	// We avoid a fully materialized slice to save memory,
	// since in most cases we expect to be sparse
	edges edges[T]
}

func (n *Node[T]) isLeaf() bool {
	return n.leaf != nil
}

func (n *Node[T]) addEdge(e edge[T]) {
	num := len(n.edges)
	idx := sort.Search(num, func(i int) bool {
		return n.edges[i].label >= e.label
	})
	n.edges = append(n.edges, e)
	if idx != num {
		copy(n.edges[idx+1:], n.edges[idx:num])
		n.edges[idx] = e
	}
}

func (n *Node[T]) replaceEdge(e edge[T]) {
	num := len(n.edges)
	idx := sort.Search(num, func(i int) bool {
		return n.edges[i].label >= e.label
	})
	if idx < num && n.edges[idx].label == e.label {
		n.edges[idx].node = e.node
		return
	}
	panic("replacing missing edge")
}

func (n *Node[T]) getEdge(label byte) (int, *Node[T]) {
	num := len(n.edges)
	idx := sort.Search(num, func(i int) bool {
		return n.edges[i].label >= label
	})
	if idx < num && n.edges[idx].label == label {
		return idx, n.edges[idx].node
	}
	return -1, nil
}

func (n *Node[T]) getLowerBoundEdge(label byte) (int, *Node[T]) {
	num := len(n.edges)
	idx := sort.Search(num, func(i int) bool {
		return n.edges[i].label >= label
	})
	// we want lower bound behavior so return even if it's not an exact match
	if idx < num {
		return idx, n.edges[idx].node
	}
	return -1, nil
}

func (n *Node[T]) delEdge(label byte) {
	num := len(n.edges)
	idx := sort.Search(num, func(i int) bool {
		return n.edges[i].label >= label
	})
	if idx < num && n.edges[idx].label == label {
		copy(n.edges[idx:], n.edges[idx+1:])
		n.edges[len(n.edges)-1] = edge[T]{}
		n.edges = n.edges[:len(n.edges)-1]
	}
}

func (n *Node[T]) GetWatch(k []byte) (<-chan struct{}, T, bool) {
	search := k
	watch := n.mutateCh
	for {
		// Check for key exhaustion
		if len(search) == 0 {
			if n.isLeaf() {
				return n.leaf.mutateCh, n.leaf.val, true
			}
			break
		}

		// Look for an edge
		_, n = n.getEdge(search[0])
		if n == nil {
			break
		}

		// Update to the finest granularity as the search makes progress
		watch = n.mutateCh

		// Consume the search prefix
		if bytes.HasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]
		} else {
			break
		}
	}
	var zero T
	return watch, zero, false
}

func (n *Node[T]) Get(k []byte) (T, bool) {
	_, val, ok := n.GetWatch(k)
	return val, ok
}

// LongestPrefix is like Get, but instead of an
// exact match, it will return the longest prefix match.
func (n *Node[T]) LongestPrefix(k []byte) ([]byte, T, bool) {
	var last *leafNode[T]
	search := k
	for {
		// Look for a leaf node
		if n.isLeaf() {
			last = n.leaf
		}

		// Check for key exhaustion
		if len(search) == 0 {
			break
		}

		// Look for an edge
		_, n = n.getEdge(search[0])
		if n == nil {
			break
		}

		// Consume the search prefix
		if bytes.HasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]
		} else {
			break
		}
	}
	if last != nil {
		return last.key, last.val, true
	}
	var zero T
	return nil, zero, false
}

// Minimum is used to return the minimum value in the tree
func (n *Node[T]) Minimum() ([]byte, T, bool) {
	for {
		if n.isLeaf() {
			return n.leaf.key, n.leaf.val, true
		}
		if len(n.edges) > 0 {
			n = n.edges[0].node
		} else {
			break
		}
	}
	var zero T
	return nil, zero, false
}

// Maximum is used to return the maximum value in the tree
func (n *Node[T]) Maximum() ([]byte, T, bool) {
	for {
		if num := len(n.edges); num > 0 {
			n = n.edges[num-1].node // bug?
			continue
		}
		if n.isLeaf() {
			return n.leaf.key, n.leaf.val, true
		} else {
			break
		}
	}
	var zero T
	return nil, zero, false
}

// Iterator is used to return an iterator at
// the given node to walk the tree
func (n *Node[T]) Iterator() *Iterator[T] {
	return &Iterator[T]{node: n}
}

// ReverseIterator is used to return an iterator at
// the given node to walk the tree backwards
func (n *Node[T]) ReverseIterator() *ReverseIterator[T] {
	return NewReverseIterator(n)
}

// Iterator is used to return an iterator at
// the given node to walk the tree
func (n *Node[T]) PathIterator(path []byte) *PathIterator[T] {
	return &PathIterator[T]{node: n, path: path}
}

// rawIterator is used to return a raw iterator at the given node to walk the
// tree.
func (n *Node[T]) rawIterator() *rawIterator[T] {
	iter := &rawIterator[T]{node: n}
	iter.Next()
	return iter
}

// Walk is used to walk the tree
func (n *Node[T]) Walk(fn WalkFn[T]) {
	recursiveWalk(n, fn)
}

// WalkBackwards is used to walk the tree in reverse order
func (n *Node[T]) WalkBackwards(fn WalkFn[T]) {
	reverseRecursiveWalk(n, fn)
}

// WalkPrefix is used to walk the tree under a prefix
func (n *Node[T]) WalkPrefix(prefix []byte, fn WalkFn[T]) {
	search := prefix
	for {
		// Check for key exhaustion
		if len(search) == 0 {
			recursiveWalk(n, fn)
			return
		}

		// Look for an edge
		_, n = n.getEdge(search[0])
		if n == nil {
			break
		}

		// Consume the search prefix
		if bytes.HasPrefix(search, n.prefix) {
			search = search[len(n.prefix):]

		} else if bytes.HasPrefix(n.prefix, search) {
			// Child may be under our search prefix
			recursiveWalk(n, fn)
			return
		} else {
			break
		}
	}
}

// WalkPath is used to walk the tree, but only visiting nodes
// from the root down to a given leaf. Where WalkPrefix walks
// all the entries *under* the given prefix, this walks the
// entries *above* the given prefix.
func (n *Node[T]) WalkPath(path []byte, fn WalkFn[T]) {
	i := n.PathIterator(path)

	for path, val, ok := i.Next(); ok; path, val, ok = i.Next() {
		if fn(path, val) {
			return
		}
	}
}

// recursiveWalk is used to do a pre-order walk of a node
// recursively. Returns true if the walk should be aborted
func recursiveWalk[T any](n *Node[T], fn WalkFn[T]) bool {
	// Visit the leaf values if any
	if n.leaf != nil && fn(n.leaf.key, n.leaf.val) {
		return true
	}

	// Recurse on the children
	for _, e := range n.edges {
		if recursiveWalk(e.node, fn) {
			return true
		}
	}
	return false
}

// reverseRecursiveWalk is used to do a reverse pre-order
// walk of a node recursively. Returns true if the walk
// should be aborted
func reverseRecursiveWalk[T any](n *Node[T], fn WalkFn[T]) bool {
	// Visit the leaf values if any
	if n.leaf != nil && fn(n.leaf.key, n.leaf.val) {
		return true
	}

	// Recurse on the children in reverse order
	for i := len(n.edges) - 1; i >= 0; i-- {
		e := n.edges[i]
		if reverseRecursiveWalk(e.node, fn) {
			return true
		}
	}
	return false
}
