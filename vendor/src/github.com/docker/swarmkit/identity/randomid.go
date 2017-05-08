package identity

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
)

var (
	// idReader is used for random id generation. This declaration allows us to
	// replace it for testing.
	idReader = rand.Reader
)

// parameters for random identifier generation. We can tweak this when there is
// time for further analysis.
const (
	randomIDEntropyBytes = 16
	randomIDBase         = 36

	// To ensure that all identifiers are fixed length, we make sure they
	// get padded out to 25 characters, which is the maximum for the base36
	// representation of 128-bit identifiers.
	//
	// For academics,  f5lxx1zz5pnorynqglhzmsp33  == 2^128 - 1. This value
	// was calculated from floor(log(2^128-1, 36)) + 1.
	//
	// See http://mathworld.wolfram.com/NumberLength.html for more information.
	maxRandomIDLength = 25
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
