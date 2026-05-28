package sshsig

import (
	"crypto"
	"errors"
	"fmt"
	"hash"
)

// supportedHashAlgorithm is a map of supported hash algorithms, as defined in
// the protocol.
// xref: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#L66-L67
var supportedHashAlgorithms = map[HashAlgorithm]crypto.Hash{
	HashSHA256: crypto.SHA256,
	HashSHA512: crypto.SHA512,
}

var (
	// ErrUnsupportedHashAlgorithm is returned by Sign and Verify if the hash
	// algorithm is not supported.
	ErrUnsupportedHashAlgorithm = errors.New("unsupported hash algorithm")
	// ErrUnavailableHashAlgorithm is returned by Sign and Verify if the hash
	// algorithm is not available.
	ErrUnavailableHashAlgorithm = errors.New("unavailable hash algorithm")
)

const (
	// HashSHA256 is the SHA-256 hash algorithm.
	HashSHA256 HashAlgorithm = "sha256"
	// HashSHA512 is the SHA-512 hash algorithm.
	HashSHA512 HashAlgorithm = "sha512"
)

// HashAlgorithm represents an algorithm used to compute a hash of a
// message.
type HashAlgorithm string

// Supported returns ErrUnsupportedHashAlgorithm if the hash algorithm is not
// supported, nil otherwise. Use Available if the intention is to make use of
// the hash algorithm, as it also checks if the hash algorithm is available.
func (h HashAlgorithm) Supported() error {
	if _, ok := supportedHashAlgorithms[h]; !ok {
		// TODO(hidde): if the number of supported hash algorithms grows, it
		//  might be worth generating the list of supported algorithms.
		return fmt.Errorf("%w %q: must be %q or %q", ErrUnsupportedHashAlgorithm, h.String(), HashSHA256, HashSHA512)
	}
	return nil
}

// Available returns ErrUnsupportedHashAlgorithm if the hash algorithm is not
// supported, ErrUnavailableHashAlgorithm if the hash algorithm is not available,
// nil otherwise.
func (h HashAlgorithm) Available() error {
	if err := h.Supported(); err != nil {
		return err
	}
	if !supportedHashAlgorithms[h].Available() {
		return fmt.Errorf("%w %q", ErrUnavailableHashAlgorithm, h.String())
	}
	return nil
}

// Hash returns a hash.Hash for the hash algorithm. If the hash algorithm is
// not available, it panics. The library itself ensures that the hash algorithm
// is available before calling this function.
func (h HashAlgorithm) Hash() hash.Hash {
	if h == "" {
		panic("sshsig: hash algorithm not specified")
	}
	if err := h.Available(); err != nil {
		panic("sshsig: " + err.Error())
	}
	return supportedHashAlgorithms[h].New()
}

// String returns the string representation of the hash algorithm.
func (h HashAlgorithm) String() string {
	return string(h)
}

// SupportedHashAlgorithms returns a list of supported hash algorithms.
func SupportedHashAlgorithms() []HashAlgorithm {
	var algorithms []HashAlgorithm
	for algorithm := range supportedHashAlgorithms {
		algorithms = append(algorithms, algorithm)
	}
	return algorithms
}
