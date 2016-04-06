// Copyright (c) 2014 The go-patricia AUTHORS
//
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package patricia

import (
	"errors"
)

//------------------------------------------------------------------------------
// Trie
//------------------------------------------------------------------------------

const (
	DefaultMaxPrefixPerNode         = 10
	DefaultMaxChildrenPerSparseNode = 8
)

type (
	Prefix      []byte
	Item        interface{}
	VisitorFunc func(prefix Prefix, item Item) error
)

// Trie is a generic patricia trie that allows fast retrieval of items by prefix.
// and other funky stuff.
//
// Trie is not thread-safe.
type Trie struct {
	prefix Prefix
	item   Item

	maxPrefixPerNode         int
	maxChildrenPerSparseNode int

	children childList
}

// Public API ------------------------------------------------------------------

type Option func(*Trie)

// Trie constructor.
func NewTrie(options ...Option) *Trie {
	trie := &Trie{}

	for _, opt := range options {
		opt(trie)
	}

	if trie.maxPrefixPerNode <= 0 {
		trie.maxPrefixPerNode = DefaultMaxPrefixPerNode
	}
	if trie.maxChildrenPerSparseNode <= 0 {
		trie.maxChildrenPerSparseNode = DefaultMaxChildrenPerSparseNode
	}

	trie.children = newSparseChildList(trie.maxChildrenPerSparseNode)
	return trie
}

func MaxPrefixPerNode(value int) Option {
	return func(trie *Trie) {
		trie.maxPrefixPerNode = value
	}
}

func MaxChildrenPerSparseNode(value int) Option {
	return func(trie *Trie) {
		trie.maxChildrenPerSparseNode = value
	}
}

// Item returns the item stored in the root of this trie.
func (trie *Trie) Item() Item {
	return trie.item
}

// Insert inserts a new item into the trie using the given prefix. Insert does
// not replace existing items. It returns false if an item was already in place.
func (trie *Trie) Insert(key Prefix, item Item) (inserted bool) {
	return trie.put(key, item, false)
}

// Set works much like Insert, but it always sets the item, possibly replacing
// the item previously inserted.
func (trie *Trie) Set(key Prefix, item Item) {
	trie.put(key, item, true)
}

// Get returns the item located at key.
//
// This method is a bit dangerous, because Get can as well end up in an internal
// node that is not really representing any user-defined value. So when nil is
// a valid value being used, it is not possible to tell if the value was inserted
// into the tree by the user or not. A possible workaround for this is not to use
// nil interface as a valid value, even using zero value of any type is enough
// to prevent this bad behaviour.
func (trie *Trie) Get(key Prefix) (item Item) {
	_, node, found, leftover := trie.findSubtree(key)
	if !found || len(leftover) != 0 {
		return nil
	}
	return node.item
}

// Match returns what Get(prefix) != nil would return. The same warning as for
// Get applies here as well.
func (trie *Trie) Match(prefix Prefix) (matchedExactly bool) {
	return trie.Get(prefix) != nil
}

// MatchSubtree returns true when there is a subtree representing extensions
// to key, that is if there are any keys in the tree which have key as prefix.
func (trie *Trie) MatchSubtree(key Prefix) (matched bool) {
	_, _, matched, _ = trie.findSubtree(key)
	return
}

// Visit calls visitor on every node containing a non-nil item
// in alphabetical order.
//
// If an error is returned from visitor, the function stops visiting the tree
// and returns that error, unless it is a special error - SkipSubtree. In that
// case Visit skips the subtree represented by the current node and continues
// elsewhere.
func (trie *Trie) Visit(visitor VisitorFunc) error {
	return trie.walk(nil, visitor)
}

// VisitSubtree works much like Visit, but it only visits nodes matching prefix.
func (trie *Trie) VisitSubtree(prefix Prefix, visitor VisitorFunc) error {
	// Nil prefix not allowed.
	if prefix == nil {
		panic(ErrNilPrefix)
	}

	// Empty trie must be handled explicitly.
	if trie.prefix == nil {
		return nil
	}

	// Locate the relevant subtree.
	_, root, found, leftover := trie.findSubtree(prefix)
	if !found {
		return nil
	}
	prefix = append(prefix, leftover...)

	// Visit it.
	return root.walk(prefix, visitor)
}

// VisitPrefixes visits only nodes that represent prefixes of key.
// To say the obvious, returning SkipSubtree from visitor makes no sense here.
func (trie *Trie) VisitPrefixes(key Prefix, visitor VisitorFunc) error {
	// Nil key not allowed.
	if key == nil {
		panic(ErrNilPrefix)
	}

	// Empty trie must be handled explicitly.
	if trie.prefix == nil {
		return nil
	}

	// Walk the path matching key prefixes.
	node := trie
	prefix := key
	offset := 0
	for {
		// Compute what part of prefix matches.
		common := node.longestCommonPrefixLength(key)
		key = key[common:]
		offset += common

		// Partial match means that there is no subtree matching prefix.
		if common < len(node.prefix) {
			return nil
		}

		// Call the visitor.
		if item := node.item; item != nil {
			if err := visitor(prefix[:offset], item); err != nil {
				return err
			}
		}

		if len(key) == 0 {
			// This node represents key, we are finished.
			return nil
		}

		// There is some key suffix left, move to the children.
		child := node.children.next(key[0])
		if child == nil {
			// There is nowhere to continue, return.
			return nil
		}

		node = child
	}
}

// Delete deletes the item represented by the given prefix.
//
// True is returned if the matching node was found and deleted.
func (trie *Trie) Delete(key Prefix) (deleted bool) {
	// Nil prefix not allowed.
	if key == nil {
		panic(ErrNilPrefix)
	}

	// Empty trie must be handled explicitly.
	if trie.prefix == nil {
		return false
	}

	// Find the relevant node.
	parent, node, _, leftover := trie.findSubtree(key)
	if len(leftover) != 0 {
		return false
	}

	// If the item is already set to nil, there is nothing to do.
	if node.item == nil {
		return false
	}

	// Delete the item.
	node.item = nil

	// Compact since that might be possible now.
	if compacted := node.compact(); compacted != node {
		if parent == nil {
			*node = *compacted
		} else {
			parent.children.replace(node.prefix[0], compacted)
			*parent = *parent.compact()
		}
	}

	return true
}

// DeleteSubtree finds the subtree exactly matching prefix and deletes it.
//
// True is returned if the subtree was found and deleted.
func (trie *Trie) DeleteSubtree(prefix Prefix) (deleted bool) {
	// Nil prefix not allowed.
	if prefix == nil {
		panic(ErrNilPrefix)
	}

	// Empty trie must be handled explicitly.
	if trie.prefix == nil {
		return false
	}

	// Locate the relevant subtree.
	parent, root, found, _ := trie.findSubtree(prefix)
	if !found {
		return false
	}

	// If we are in the root of the trie, reset the trie.
	if parent == nil {
		root.prefix = nil
		root.children = newSparseChildList(trie.maxPrefixPerNode)
		return true
	}

	// Otherwise remove the root node from its parent.
	parent.children.remove(root)
	return true
}

// Internal helper methods -----------------------------------------------------

func (trie *Trie) put(key Prefix, item Item, replace bool) (inserted bool) {
	// Nil prefix not allowed.
	if key == nil {
		panic(ErrNilPrefix)
	}

	var (
		common int
		node   *Trie = trie
		child  *Trie
	)

	if node.prefix == nil {
		if len(key) <= trie.maxPrefixPerNode {
			node.prefix = key
			goto InsertItem
		}
		node.prefix = key[:trie.maxPrefixPerNode]
		key = key[trie.maxPrefixPerNode:]
		goto AppendChild
	}

	for {
		// Compute the longest common prefix length.
		common = node.longestCommonPrefixLength(key)
		key = key[common:]

		// Only a part matches, split.
		if common < len(node.prefix) {
			goto SplitPrefix
		}

		// common == len(node.prefix) since never (common > len(node.prefix))
		// common == len(former key) <-> 0 == len(key)
		// -> former key == node.prefix
		if len(key) == 0 {
			goto InsertItem
		}

		// Check children for matching prefix.
		child = node.children.next(key[0])
		if child == nil {
			goto AppendChild
		}
		node = child
	}

SplitPrefix:
	// Split the prefix if necessary.
	child = new(Trie)
	*child = *node
	*node = *NewTrie()
	node.prefix = child.prefix[:common]
	child.prefix = child.prefix[common:]
	child = child.compact()
	node.children = node.children.add(child)

AppendChild:
	// Keep appending children until whole prefix is inserted.
	// This loop starts with empty node.prefix that needs to be filled.
	for len(key) != 0 {
		child := NewTrie()
		if len(key) <= trie.maxPrefixPerNode {
			child.prefix = key
			node.children = node.children.add(child)
			node = child
			goto InsertItem
		} else {
			child.prefix = key[:trie.maxPrefixPerNode]
			key = key[trie.maxPrefixPerNode:]
			node.children = node.children.add(child)
			node = child
		}
	}

InsertItem:
	// Try to insert the item if possible.
	if replace || node.item == nil {
		node.item = item
		return true
	}
	return false
}

func (trie *Trie) compact() *Trie {
	// Only a node with a single child can be compacted.
	if trie.children.length() != 1 {
		return trie
	}

	child := trie.children.head()

	// If any item is set, we cannot compact since we want to retain
	// the ability to do searching by key. This makes compaction less usable,
	// but that simply cannot be avoided.
	if trie.item != nil || child.item != nil {
		return trie
	}

	// Make sure the combined prefixes fit into a single node.
	if len(trie.prefix)+len(child.prefix) > trie.maxPrefixPerNode {
		return trie
	}

	// Concatenate the prefixes, move the items.
	child.prefix = append(trie.prefix, child.prefix...)
	if trie.item != nil {
		child.item = trie.item
	}

	return child
}

func (trie *Trie) findSubtree(prefix Prefix) (parent *Trie, root *Trie, found bool, leftover Prefix) {
	// Find the subtree matching prefix.
	root = trie
	for {
		// Compute what part of prefix matches.
		common := root.longestCommonPrefixLength(prefix)
		prefix = prefix[common:]

		// We used up the whole prefix, subtree found.
		if len(prefix) == 0 {
			found = true
			leftover = root.prefix[common:]
			return
		}

		// Partial match means that there is no subtree matching prefix.
		if common < len(root.prefix) {
			leftover = root.prefix[common:]
			return
		}

		// There is some prefix left, move to the children.
		child := root.children.next(prefix[0])
		if child == nil {
			// There is nowhere to continue, there is no subtree matching prefix.
			return
		}

		parent = root
		root = child
	}
}

func (trie *Trie) walk(actualRootPrefix Prefix, visitor VisitorFunc) error {
	var prefix Prefix
	// Allocate a bit more space for prefix at the beginning.
	if actualRootPrefix == nil {
		prefix = make(Prefix, 32+len(trie.prefix))
		copy(prefix, trie.prefix)
		prefix = prefix[:len(trie.prefix)]
	} else {
		prefix = make(Prefix, 32+len(actualRootPrefix))
		copy(prefix, actualRootPrefix)
		prefix = prefix[:len(actualRootPrefix)]
	}

	// Visit the root first. Not that this works for empty trie as well since
	// in that case item == nil && len(children) == 0.
	if trie.item != nil {
		if err := visitor(prefix, trie.item); err != nil {
			if err == SkipSubtree {
				return nil
			}
			return err
		}
	}

	// Then continue to the children.
	return trie.children.walk(&prefix, visitor)
}

func (trie *Trie) longestCommonPrefixLength(prefix Prefix) (i int) {
	for ; i < len(prefix) && i < len(trie.prefix) && prefix[i] == trie.prefix[i]; i++ {
	}
	return
}

// Errors ----------------------------------------------------------------------

var (
	SkipSubtree  = errors.New("Skip this subtree")
	ErrNilPrefix = errors.New("Nil prefix passed into a method call")
)
