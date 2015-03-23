package digest

import (
	"crypto/sha256"
	"hash"
)

// Digester calculates the digest of written data. It is functionally
// equivalent to hash.Hash but provides methods for returning the Digest type
// rather than raw bytes.
type Digester struct {
	alg  string
	hash hash.Hash
}

// NewDigester create a new Digester with the given hashing algorithm and instance
// of that algo's hasher.
func NewDigester(alg string, h hash.Hash) Digester {
	return Digester{
		alg:  alg,
		hash: h,
	}
}

// NewCanonicalDigester is a convenience function to create a new Digester with
// out default settings.
func NewCanonicalDigester() Digester {
	return NewDigester("sha256", sha256.New())
}

// Write data to the digester. These writes cannot fail.
func (d *Digester) Write(p []byte) (n int, err error) {
	return d.hash.Write(p)
}

// Digest returns the current digest for this digester.
func (d *Digester) Digest() Digest {
	return NewDigest(d.alg, d.hash)
}

// Reset the state of the digester.
func (d *Digester) Reset() {
	d.hash.Reset()
}
