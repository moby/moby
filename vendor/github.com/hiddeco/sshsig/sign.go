package sshsig

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// ErrMissingNamespace is returned by Sign if the namespace value is missing.
var ErrMissingNamespace = errors.New("missing namespace")

// Signature represents the SSH signature of a message. It can be marshaled
// into an SSH wire format using Marshal, or into an armored (PEM) format using
// Armor.
//
// Manually construction of this type is not recommended. Use ParseSignature or
// Unarmor instead to retrieve a Signature from a wire or armored (PEM) format.
type Signature struct {
	// Version is the version of the signature format.
	// It currently supports version 1, any other value will be rejected with
	// ErrUnsupportedSignatureVersion.
	Version uint32
	// PublicKey is the public key used to create the Signature.
	PublicKey ssh.PublicKey
	// Namespace is the domain of the signature, and is used to prevent signature
	// reuse across different applications.
	Namespace string
	// HashAlgorithm is the hash algorithm used to hash the Signature message.
	HashAlgorithm HashAlgorithm
	// Signature is the SSH signature of the hash of the message.
	Signature *ssh.Signature
}

// Marshal returns the Signature in SSH wire format.
func (s Signature) Marshal() []byte {
	return blob{
		Version:       s.Version,
		PublicKey:     string(s.PublicKey.Marshal()),
		Namespace:     s.Namespace,
		HashAlgorithm: s.HashAlgorithm.String(),
		Signature:     string(ssh.Marshal(s.Signature)),
	}.Marshal()
}

// ParseSignature parses a signature in SSH wire format into a Signature.
// It returns an error if the signature is invalid.
func ParseSignature(b []byte) (*Signature, error) {
	var sig blob
	if err := ssh.Unmarshal(b, &sig); err != nil {
		return nil, err
	}
	if err := sig.Validate(); err != nil {
		return nil, err
	}

	sshSig := ssh.Signature{}
	if err := ssh.Unmarshal([]byte(sig.Signature), &sshSig); err != nil {
		return nil, err
	}

	pub, err := ssh.ParsePublicKey([]byte(sig.PublicKey))
	if err != nil {
		return nil, err
	}

	// For RSA signatures, the signature algorithm must be "rsa-sha2-512" or
	// "rsa-sha2-256".
	// xref: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#L69-L72
	if pub.Type() == ssh.KeyAlgoRSA && sshSig.Format != ssh.KeyAlgoRSASHA256 && sshSig.Format != ssh.KeyAlgoRSASHA512 {
		return nil, fmt.Errorf("invalid signature format %q: expected %q or %q", sshSig.Format, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512)
	}

	return &Signature{
		Version:       sig.Version,
		PublicKey:     pub,
		Namespace:     sig.Namespace,
		HashAlgorithm: HashAlgorithm(sig.HashAlgorithm),
		Signature:     &sshSig,
	}, nil
}

// Sign generates a signature of the message from the io.Reader using the
// given ssh.Signer private key. The signature hash is computed using the provided
// HashAlgorithm.
//
// The purpose of the namespace value is to specify an unambiguous interpretation
// domain for the signature, e.g. file signing. This prevents cross-protocol
// attacks caused by signatures intended for one intended domain being accepted
// in another. The namespace must not be empty, or ErrMissingNamespace will be
// returned.
//
// When the signer is an RSA key, the signature algorithm will always be
// "rsa-sha2-512". This is the same default used by OpenSSH, and is required by
// the SSH signature wire protocol.
//
// Sign returns a Signature containing the signed message and metadata, or an
// error if the signing process fails.
func Sign(m io.Reader, signer ssh.Signer, h HashAlgorithm, namespace string) (*Signature, error) {
	return SignWithRand(m, crand.Reader, signer, h, namespace)
}

// SignWithRand is like Sign, but uses the provided rand io.Reader to create any
// necessary random values. Most callers likely want to use Sign instead.
func SignWithRand(m, rand io.Reader, signer ssh.Signer, h HashAlgorithm, namespace string) (*Signature, error) {
	if namespace == "" {
		// xref: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#LL57C13-L57C13
		return nil, ErrMissingNamespace
	}

	if err := h.Available(); err != nil {
		return nil, err
	}

	hf := h.Hash()
	if _, err := io.Copy(hf, m); err != nil {
		return nil, err
	}
	mh := hf.Sum(nil)

	var (
		sd = signedData{
			Namespace:     namespace,
			HashAlgorithm: h.String(),
			Hash:          string(mh),
		}
		sig *ssh.Signature
		err error
	)

	switch signer.PublicKey().Type() {
	case ssh.KeyAlgoRSA:
		// For RSA signatures, the signature algorithm must be "rsa-sha2-512" or
		// "rsa-sha2-256". We use the same "rsa-sha2-512" default as OpenSSH.
		// xref: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#L69-L72
		// xref: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/ssh-keygen.c#L1804-L1805
		algo := ssh.KeyAlgoRSASHA512

		// This should always succeed as an SSH signer must implement the
		// AlgorithmSigner, but we check anyway.
		as, ok := signer.(ssh.AlgorithmSigner)
		if !ok {
			return nil, fmt.Errorf("signer does not support non-default signature algorithm %q", algo)
		}

		if sig, err = as.SignWithAlgorithm(rand, sd.Marshal(), algo); err != nil {
			return nil, err
		}
	default:
		if sig, err = signer.Sign(rand, sd.Marshal()); err != nil {
			return nil, err
		}
	}

	return &Signature{
		Version:       sigVersion,
		PublicKey:     signer.PublicKey(),
		Namespace:     namespace,
		HashAlgorithm: h,
		Signature:     sig,
	}, nil
}
