package jwt

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"math/big"
)

var (
	// Sadly this is missing from crypto/ecdsa compared to crypto/rsa
	ErrECDSAVerification = errors.New("crypto/ecdsa: verification error")
)

// SigningMethodECDSA implements the ECDSA family of signing methods.
// Expects *ecdsa.PrivateKey for signing and *ecdsa.PublicKey for verification
type SigningMethodECDSA struct {
	Name      string
	Hash      crypto.Hash
	KeySize   int
	CurveBits int
}

// Specific instances for EC256 and company
var (
	SigningMethodES256 *SigningMethodECDSA
	SigningMethodES384 *SigningMethodECDSA
	SigningMethodES512 *SigningMethodECDSA
)

func init() {
	// ES256
	SigningMethodES256 = &SigningMethodECDSA{"ES256", crypto.SHA256, 32, 256}
	RegisterSigningMethod(SigningMethodES256.Alg(), func() SigningMethod {
		return SigningMethodES256
	})

	// ES384
	SigningMethodES384 = &SigningMethodECDSA{"ES384", crypto.SHA384, 48, 384}
	RegisterSigningMethod(SigningMethodES384.Alg(), func() SigningMethod {
		return SigningMethodES384
	})

	// ES512
	SigningMethodES512 = &SigningMethodECDSA{"ES512", crypto.SHA512, 66, 521}
	RegisterSigningMethod(SigningMethodES512.Alg(), func() SigningMethod {
		return SigningMethodES512
	})
}

func (m *SigningMethodECDSA) Alg() string {
	return m.Name
}

// Verify implements token verification for the SigningMethod.
// For this verify method, key must be an ecdsa.PublicKey struct
func (m *SigningMethodECDSA) Verify(signingString, signature string, key interface{}) error {
	var err error

	// Decode the signature
	var sig []byte
	if sig, err = DecodeSegment(signature); err != nil {
		return err
	}

	// Get the key
	var ecdsaKey *ecdsa.PublicKey
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		ecdsaKey = k
	default:
		return ErrInvalidKeyType
	}

	if len(sig) != 2*m.KeySize {
		return ErrECDSAVerification
	}

	r := big.NewInt(0).SetBytes(sig[:m.KeySize])
	s := big.NewInt(0).SetBytes(sig[m.KeySize:])

	// Create hasher
	if !m.Hash.Available() {
		return ErrHashUnavailable
	}
	hasher := m.Hash.New()
	hasher.Write([]byte(signingString))

	// Verify the signature
	if verifystatus := ecdsa.Verify(ecdsaKey, hasher.Sum(nil), r, s); verifystatus {
		return nil
	}

	return ErrECDSAVerification
}

// Sign implements token signing for the SigningMethod.
// For this signing method, key must be an ecdsa.PrivateKey struct
func (m *SigningMethodECDSA) Sign(signingString string, key interface{}) (string, error) {
	// Get the key
	var ecdsaKey *ecdsa.PrivateKey
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		ecdsaKey = k
	default:
		return "", ErrInvalidKeyType
	}

	// Create the hasher
	if !m.Hash.Available() {
		return "", ErrHashUnavailable
	}

	hasher := m.Hash.New()
	hasher.Write([]byte(signingString))

	// Sign the string and return r, s
	if r, s, err := ecdsa.Sign(rand.Reader, ecdsaKey, hasher.Sum(nil)); err == nil {
		curveBits := ecdsaKey.Curve.Params().BitSize

		if m.CurveBits != curveBits {
			return "", ErrInvalidKey
		}

		keyBytes := curveBits / 8
		if curveBits%8 > 0 {
			keyBytes += 1
		}

		// We serialize the outputs (r and s) into big-endian byte arrays
		// padded with zeros on the left to make sure the sizes work out.
		// Output must be 2*keyBytes long.
		out := make([]byte, 2*keyBytes)
		r.FillBytes(out[0:keyBytes]) // r is assigned to the first half of output.
		s.FillBytes(out[keyBytes:])  // s is assigned to the second half of output.

		return EncodeSegment(out), nil
	} else {
		return "", err
	}
}
