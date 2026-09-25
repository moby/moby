package platform

import (
	"io"
	"math/rand"
)

// seed is a fixed seed value for NewFakeRandSource.
//
// Trivia: While arbitrary, 42 was chosen as it is the "Ultimate Answer" in
// the Douglas Adams novel "The Hitchhiker's Guide to the Galaxy."
const seed = int64(42)

// NewFakeRandSource returns a deterministic source of random values.
func NewFakeRandSource() io.Reader {
	return rand.New(rand.NewSource(seed))
}
