// Copyright 2019 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package compact

import "math/bits"

// NodeID identifies a node of a Merkle tree.
//
// The ID consists of a level and index within this level. Levels are numbered
// from 0, which corresponds to the tree leaves. Within each level, nodes are
// numbered with consecutive indices starting from 0.
//
//	L4:         ┌───────0───────┐                ...
//	L3:     ┌───0───┐       ┌───1───┐       ┌─── ...
//	L2:   ┌─0─┐   ┌─1─┐   ┌─2─┐   ┌─3─┐   ┌─4─┐  ...
//	L1:  ┌0┐ ┌1┐ ┌2┐ ┌3┐ ┌4┐ ┌5┐ ┌6┐ ┌7┐ ┌8┐ ┌9┐ ...
//	L0:  0 1 2 3 4 5 6 7 8 9 ... ... ... ... ... ...
//
// When the tree is not perfect, the nodes that would complement it to perfect
// are called ephemeral. Algorithms that operate with ephemeral nodes still map
// them to the same address space.
type NodeID struct {
	Level uint
	Index uint64
}

// NewNodeID returns a NodeID with the passed in node coordinates.
func NewNodeID(level uint, index uint64) NodeID {
	return NodeID{Level: level, Index: index}
}

// Parent returns the ID of the parent node.
func (id NodeID) Parent() NodeID {
	return NewNodeID(id.Level+1, id.Index>>1)
}

// Sibling returns the ID of the sibling node.
func (id NodeID) Sibling() NodeID {
	return NewNodeID(id.Level, id.Index^1)
}

// Coverage returns the [begin, end) range of leaves covered by the node.
func (id NodeID) Coverage() (uint64, uint64) {
	return id.Index << id.Level, (id.Index + 1) << id.Level
}

// RangeNodes appends the IDs of the nodes that comprise the [begin, end)
// compact range to the given slice, and returns the new slice. The caller may
// pre-allocate space with the help of the RangeSize function.
func RangeNodes(begin, end uint64, ids []NodeID) []NodeID {
	left, right := Decompose(begin, end)

	pos := begin
	// Iterate over perfect subtrees along the left border of the range, ordered
	// from lower to upper levels.
	for bit := uint64(0); left != 0; pos, left = pos+bit, left^bit {
		level := uint(bits.TrailingZeros64(left))
		bit = uint64(1) << level
		ids = append(ids, NewNodeID(level, pos>>level))
	}

	// Iterate over perfect subtrees along the right border of the range, ordered
	// from upper to lower levels.
	for bit := uint64(0); right != 0; pos, right = pos+bit, right^bit {
		level := uint(bits.Len64(right)) - 1
		bit = uint64(1) << level
		ids = append(ids, NewNodeID(level, pos>>level))
	}

	return ids
}

// RangeSize returns the number of nodes in the [begin, end) compact range.
func RangeSize(begin, end uint64) int {
	left, right := Decompose(begin, end)
	return bits.OnesCount64(left) + bits.OnesCount64(right)
}
