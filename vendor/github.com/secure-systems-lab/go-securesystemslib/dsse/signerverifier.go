package dsse

import (
	"context"
	"crypto"
)

/*
Signer defines the interface for an abstract signing algorithm. The Signer
interface is used to inject signature algorithm implementations into the
EnvelopeSigner. This decoupling allows for any signing algorithm and key
management system can be used. The full message is provided as the parameter.
If the signature algorithm depends on hashing of the message prior to signature
calculation, the implementor of this interface must perform such hashing. The
function must return raw bytes representing the calculated signature using the
current algorithm, and the key used (if applicable).
*/
type Signer interface {
	Sign(ctx context.Context, data []byte) ([]byte, error)
	KeyID() (string, error)
}

/*
Verifier verifies a complete message against a signature and key. If the message
was hashed prior to signature generation, the verifier must perform the same
steps. If KeyID returns successfully, only signature matching the key ID will be
verified.
*/
type Verifier interface {
	Verify(ctx context.Context, data, sig []byte) error
	KeyID() (string, error)
	Public() crypto.PublicKey
}

// SignerVerifier provides both the signing and verification interface.
type SignerVerifier interface {
	Signer
	Verifier
}

// Deprecated: switch to renamed SignerVerifier. This is currently aliased for
// backwards compatibility.
type SignVerifier = SignerVerifier
