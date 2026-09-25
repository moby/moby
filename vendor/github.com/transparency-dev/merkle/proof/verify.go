// Copyright 2017 Google LLC. All Rights Reserved.
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

package proof

import (
	"bytes"
	"errors"
	"fmt"
	"math/bits"

	"github.com/transparency-dev/merkle"
)

// RootMismatchError occurs when an inclusion proof fails.
type RootMismatchError struct {
	ExpectedRoot   []byte
	CalculatedRoot []byte
}

func (e RootMismatchError) Error() string {
	return fmt.Sprintf("calculated root:\n%v\n does not match expected root:\n%v", e.CalculatedRoot, e.ExpectedRoot)
}

func verifyMatch(calculated, expected []byte) error {
	if !bytes.Equal(calculated, expected) {
		return RootMismatchError{ExpectedRoot: expected, CalculatedRoot: calculated}
	}
	return nil
}

// VerifyInclusion verifies the correctness of the inclusion proof for the leaf
// with the specified hash and index, relatively to the tree of the given size
// and root hash. Requires 0 <= index < size.
func VerifyInclusion(hasher merkle.LogHasher, index, size uint64, leafHash []byte, proof [][]byte, root []byte) error {
	calcRoot, err := RootFromInclusionProof(hasher, index, size, leafHash, proof)
	if err != nil {
		return err
	}
	return verifyMatch(calcRoot, root)
}

// RootFromInclusionProof calculates the expected root hash for a tree of the
// given size, provided a leaf index and hash with the corresponding inclusion
// proof. Requires 0 <= index < size.
func RootFromInclusionProof(hasher merkle.LogHasher, index, size uint64, leafHash []byte, proof [][]byte) ([]byte, error) {
	if index >= size {
		return nil, fmt.Errorf("index is beyond size: %d >= %d", index, size)
	}
	if got, want := len(leafHash), hasher.Size(); got != want {
		return nil, fmt.Errorf("leafHash has unexpected size %d, want %d", got, want)
	}

	inner, border := decompInclProof(index, size)
	if got, want := len(proof), inner+border; got != want {
		return nil, fmt.Errorf("wrong proof size %d, want %d", got, want)
	}

	res := chainInner(hasher, leafHash, proof[:inner], index)
	res = chainBorderRight(hasher, res, proof[inner:])
	return res, nil
}

// VerifyConsistency checks that the passed-in consistency proof is valid
// between the passed in tree sizes, with respect to the corresponding root
// hashes. Requires 0 <= size1 <= size2.
func VerifyConsistency(hasher merkle.LogHasher, size1, size2 uint64, proof [][]byte, root1, root2 []byte) error {
	switch {
	case size2 < size1:
		return fmt.Errorf("size2 (%d) < size1 (%d)", size1, size2)
	case size1 == size2:
		if len(proof) > 0 {
			return errors.New("size1=size2, but proof is not empty")
		}
		return verifyMatch(root1, root2)
	case size1 == 0:
		// Any size greater than 0 is consistent with size 0.
		if len(proof) > 0 {
			return fmt.Errorf("expected empty proof, but got %d components", len(proof))
		}
		return nil // Proof OK.
	case len(proof) == 0:
		return errors.New("empty proof")
	}

	inner, border := decompInclProof(size1-1, size2)
	shift := bits.TrailingZeros64(size1)
	inner -= shift // Note: shift < inner if size1 < size2.

	// The proof includes the root hash for the sub-tree of size 2^shift.
	seed, start := proof[0], 1
	if size1 == 1<<uint(shift) { // Unless size1 is that very 2^shift.
		seed, start = root1, 0
	}
	if got, want := len(proof), start+inner+border; got != want {
		return fmt.Errorf("wrong proof size %d, want %d", got, want)
	}
	proof = proof[start:]
	// Now len(proof) == inner+border, and proof is effectively a suffix of
	// inclusion proof for entry |size1-1| in a tree of size |size2|.

	// Verify the first root.
	mask := (size1 - 1) >> uint(shift) // Start chaining from level |shift|.
	hash1 := chainInnerRight(hasher, seed, proof[:inner], mask)
	hash1 = chainBorderRight(hasher, hash1, proof[inner:])
	if err := verifyMatch(hash1, root1); err != nil {
		return err
	}

	// Verify the second root.
	hash2 := chainInner(hasher, seed, proof[:inner], mask)
	hash2 = chainBorderRight(hasher, hash2, proof[inner:])
	return verifyMatch(hash2, root2)
}

// decompInclProof breaks down inclusion proof for a leaf at the specified
// |index| in a tree of the specified |size| into 2 components. The splitting
// point between them is where paths to leaves |index| and |size-1| diverge.
// Returns lengths of the bottom and upper proof parts correspondingly. The sum
// of the two determines the correct length of the inclusion proof.
func decompInclProof(index, size uint64) (int, int) {
	inner := innerProofSize(index, size)
	border := bits.OnesCount64(index >> uint(inner))
	return inner, border
}

func innerProofSize(index, size uint64) int {
	return bits.Len64(index ^ (size - 1))
}

// chainInner computes a subtree hash for a node on or below the tree's right
// border. Assumes |proof| hashes are ordered from lower levels to upper, and
// |seed| is the initial subtree/leaf hash on the path located at the specified
// |index| on its level.
func chainInner(hasher merkle.LogHasher, seed []byte, proof [][]byte, index uint64) []byte {
	for i, h := range proof {
		if (index>>uint(i))&1 == 0 {
			seed = hasher.HashChildren(seed, h)
		} else {
			seed = hasher.HashChildren(h, seed)
		}
	}
	return seed
}

// chainInnerRight computes a subtree hash like chainInner, but only takes
// hashes to the left from the path into consideration, which effectively means
// the result is a hash of the corresponding earlier version of this subtree.
func chainInnerRight(hasher merkle.LogHasher, seed []byte, proof [][]byte, index uint64) []byte {
	for i, h := range proof {
		if (index>>uint(i))&1 == 1 {
			seed = hasher.HashChildren(h, seed)
		}
	}
	return seed
}

// chainBorderRight chains proof hashes along tree borders. This differs from
// inner chaining because |proof| contains only left-side subtree hashes.
func chainBorderRight(hasher merkle.LogHasher, seed []byte, proof [][]byte) []byte {
	for _, h := range proof {
		seed = hasher.HashChildren(h, seed)
	}
	return seed
}
