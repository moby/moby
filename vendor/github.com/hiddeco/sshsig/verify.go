package sshsig

import (
	"errors"
	"io"

	"golang.org/x/crypto/ssh"
)

var (
	// ErrPublicKeyMismatch is returned by Verify if the public key in the signature
	// does not match the public key used to verify the signature.
	ErrPublicKeyMismatch = errors.New("public key does not match")

	// ErrNamespaceMismatch is returned by Verify if the namespace in the signature
	// does not match the namespace used to verify the signature.
	ErrNamespaceMismatch = errors.New("namespace does not match")
)

// Verify verifies the message from the io.Reader matches the Signature using
// the given ssh.PublicKey and HashAlgorithm.
//
// The purpose of the namespace value is to specify an unambiguous interpretation
// domain for the signature, e.g. file signing. This prevents cross-protocol
// attacks caused by signatures intended for one intended domain being accepted
// in another. Unlike Sign, the namespace value is not required to allow
// verification of signatures created by looser implementations.
//
// Verify returns an error if the verification process fails.
func Verify(m io.Reader, sig *Signature, pub ssh.PublicKey, h HashAlgorithm, namespace string) error {
	// Check that the public key in the signature matches the public key used to
	// verify the signature. If this is e.g. tricked in to a hash collision, it
	// will still be caught by the verification.
	if ssh.FingerprintSHA256(pub) != ssh.FingerprintSHA256(sig.PublicKey) {
		return ErrPublicKeyMismatch
	}

	// Check that namespace matches the namespace in the Signature.
	// If this is malformed, it will still be caught by the verification.
	if sig.Namespace != namespace {
		return ErrNamespaceMismatch
	}

	if err := h.Available(); err != nil {
		return err
	}

	hf := h.Hash()
	if _, err := io.Copy(hf, m); err != nil {
		return err
	}
	mh := hf.Sum(nil)

	return pub.Verify(signedData{
		Namespace:     namespace,
		HashAlgorithm: h.String(),
		Hash:          string(mh),
	}.Marshal(), sig.Signature)
}
