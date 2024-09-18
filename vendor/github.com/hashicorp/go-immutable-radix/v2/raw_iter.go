package iradix

// rawIterator visits each of the nodes in the tree, even the ones that are not
// leaves. It keeps track of the effective path (what a leaf at a given node
// would be called), which is useful for comparing trees.
type rawIterator[T any] struct {
	// node is the starting node in the tree for the iterator.
	node *Node[T]

	// stack keeps track of edges in the frontier.
	stack []rawStackEntry[T]

	// pos is the current position of the iterator.
	pos *Node[T]

	// path is the effective path of the current iterator position,
	// regardless of whether the current node is a leaf.
	path string
}

// rawStackEntry is used to keep track of the cumulative common path as well as
// its associated edges in the frontier.
type rawStackEntry[T any] struct {
	path  string
	edges edges[T]
}

// Front returns the current node that has been iterated to.
func (i *rawIterator[T]) Front() *Node[T] {
	return i.pos
}

// Path returns the effective path of the current node, even if it's not actually
// a leaf.
func (i *rawIterator[T]) Path() string {
	return i.path
}

// Next advances the iterator to the next node.
func (i *rawIterator[T]) Next() {
	// Initialize our stack if needed.
	if i.stack == nil && i.node != nil {
		i.stack = []rawStackEntry[T]{
			{
				edges: edges[T]{
					edge[T]{node: i.node},
				},
			},
		}
	}

	for len(i.stack) > 0 {
		// Inspect the last element of the stack.
		n := len(i.stack)
		last := i.stack[n-1]
		elem := last.edges[0].node

		// Update the stack.
		if len(last.edges) > 1 {
			i.stack[n-1].edges = last.edges[1:]
		} else {
			i.stack = i.stack[:n-1]
		}

		// Push the edges onto the frontier.
		if len(elem.edges) > 0 {
			path := last.path + string(elem.prefix)
			i.stack = append(i.stack, rawStackEntry[T]{path, elem.edges})
		}

		i.pos = elem
		i.path = last.path + string(elem.prefix)
		return
	}

	i.pos = nil
	i.path = ""
}
