package identity

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
)

var (
	// idReader is used for random id generation. This declaration allows us to
	// replace it for testing.
	idReader = rand.Reader
)

// parameters for random identifier generation. We can tweak this when there is
// time for further analysis.
const (
	randomIDEntropyBytes     = 16
	randomNodeIDEntropyBytes = 8
	randomIDBase             = 36

	// To ensure that all identifiers are fixed length, we make sure they
	// get padded out to 25 characters, which is the maximum for the base36
	// representation of 128-bit identifiers.
	//
	// For academics,  f5lxx1zz5pnorynqglhzmsp33  == 2^128 - 1. This value
	// was calculated from floor(log(2^128-1, 36)) + 1.
	//
	// See http://mathworld.wolfram.com/NumberLength.html for more information.
	maxRandomIDLength     = 25
	maxRandomNodeIDLength = 13
)

// NewID generates a new identifier for use where random identifiers with low
// collision probability are required.
//
// With the parameters in this package, the generated identifier will provide
// 128 bits of entropy encoded with base36. Leading padding is added if the
// string is less 25 bytes. We do not intend to maintain this interface, so
// identifiers should be treated opaquely.
func NewID() string {
	var p [randomIDEntropyBytes]byte

	if _, err := io.ReadFull(idReader, p[:]); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	var nn big.Int
	nn.SetBytes(p[:])
	return fmt.Sprintf("%0[1]*s", maxRandomIDLength, nn.Text(randomIDBase))
}

// NewNodeID generates a new identifier for identifying a node. These IDs
// are shorter than the IDs returned by NewID, so they can be used directly
// by Raft. Because they are short, they MUST be checked for collisions.
func NewNodeID() string {
	var p [randomNodeIDEntropyBytes]byte

	if _, err := io.ReadFull(idReader, p[:]); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	randomInt := binary.LittleEndian.Uint64(p[:])
	return FormatNodeID(randomInt)
}

// FormatNodeID converts a node ID from uint64 to string format.
// A string-formatted node ID looks like 1w8ynjwhcy4zd.
func FormatNodeID(nodeID uint64) string {
	return fmt.Sprintf("%0[1]*s", maxRandomNodeIDLength, strconv.FormatUint(nodeID, 36))
}

// ParseNodeID converts a node ID from string format to uint64.
func ParseNodeID(nodeID string) (uint64, error) {
	if len(nodeID) != maxRandomNodeIDLength {
		return 0, errors.New("node ID has invalid length")
	}
	return strconv.ParseUint(nodeID, 36, 64)
}
