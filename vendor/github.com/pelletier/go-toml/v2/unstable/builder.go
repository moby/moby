package unstable

// root contains a full AST.
//
// It is immutable once constructed with Builder.
type root struct {
	nodes []Node
}

func (r *root) at(idx reference) *Node {
	return &r.nodes[idx]
}

type reference int

const invalidReference reference = -1

func (r reference) Valid() bool {
	return r != invalidReference
}

type builder struct {
	tree    root
	lastIdx int
}

func (b *builder) NodeAt(ref reference) *Node {
	n := b.tree.at(ref)
	n.nodes = &b.tree.nodes
	return n
}

func (b *builder) Reset() {
	b.tree.nodes = b.tree.nodes[:0]
	b.lastIdx = 0
}

func (b *builder) Push(n Node) reference {
	b.lastIdx = len(b.tree.nodes)
	n.next = -1
	n.child = -1
	b.tree.nodes = append(b.tree.nodes, n)
	return reference(b.lastIdx)
}

func (b *builder) PushAndChain(n Node) reference {
	newIdx := len(b.tree.nodes)
	n.next = -1
	n.child = -1
	b.tree.nodes = append(b.tree.nodes, n)
	if b.lastIdx >= 0 {
		b.tree.nodes[b.lastIdx].next = int32(newIdx) //nolint:gosec // TOML ASTs are small
	}
	b.lastIdx = newIdx
	return reference(b.lastIdx)
}

func (b *builder) AttachChild(parent reference, child reference) {
	b.tree.nodes[parent].child = int32(child) //nolint:gosec // TOML ASTs are small
}

func (b *builder) Chain(from reference, to reference) {
	b.tree.nodes[from].next = int32(to) //nolint:gosec // TOML ASTs are small
}
