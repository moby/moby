package signerverifier

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"os"
)

const (
	ECDSAKeyType   = "ecdsa"
	ECDSAKeyScheme = "ecdsa-sha2-nistp256"
)

// ECDSASignerVerifier is a dsse.SignerVerifier compliant interface to sign and
// verify signatures using ECDSA keys.
type ECDSASignerVerifier struct {
	keyID     string
	curveSize int
	private   *ecdsa.PrivateKey
	public    *ecdsa.PublicKey
}

// NewECDSASignerVerifierFromSSLibKey creates an ECDSASignerVerifier from an
// SSLibKey.
func NewECDSASignerVerifierFromSSLibKey(key *SSLibKey) (*ECDSASignerVerifier, error) {
	if len(key.KeyVal.Public) == 0 {
		return nil, ErrInvalidKey
	}

	_, publicParsedKey, err := decodeAndParsePEM([]byte(key.KeyVal.Public))
	if err != nil {
		return nil, fmt.Errorf("unable to create ECDSA signerverifier: %w", err)
	}

	sv := &ECDSASignerVerifier{
		keyID:     key.KeyID,
		curveSize: publicParsedKey.(*ecdsa.PublicKey).Params().BitSize,
		public:    publicParsedKey.(*ecdsa.PublicKey),
		private:   nil,
	}

	if len(key.KeyVal.Private) > 0 {
		_, privateParsedKey, err := decodeAndParsePEM([]byte(key.KeyVal.Private))
		if err != nil {
			return nil, fmt.Errorf("unable to create ECDSA signerverifier: %w", err)
		}

		sv.private = privateParsedKey.(*ecdsa.PrivateKey)
	}

	return sv, nil
}

// Sign creates a signature for `data`.
func (sv *ECDSASignerVerifier) Sign(_ context.Context, data []byte) ([]byte, error) {
	if sv.private == nil {
		return nil, ErrNotPrivateKey
	}

	hashedData := getECDSAHashedData(data, sv.curveSize)

	return ecdsa.SignASN1(rand.Reader, sv.private, hashedData)
}

// Verify verifies the `sig` value passed in against `data`.
func (sv *ECDSASignerVerifier) Verify(_ context.Context, data []byte, sig []byte) error {
	hashedData := getECDSAHashedData(data, sv.curveSize)

	if ok := ecdsa.VerifyASN1(sv.public, hashedData, sig); !ok {
		return ErrSignatureVerificationFailed
	}

	return nil
}

// KeyID returns the identifier of the key used to create the
// ECDSASignerVerifier instance.
func (sv *ECDSASignerVerifier) KeyID() (string, error) {
	return sv.keyID, nil
}

// Public returns the public portion of the key used to create the
// ECDSASignerVerifier instance.
func (sv *ECDSASignerVerifier) Public() crypto.PublicKey {
	return sv.public
}

// LoadECDSAKeyFromFile returns an SSLibKey instance for an ECDSA key stored in
// a file in the custom securesystemslib format.
//
// Deprecated: use LoadKey(). The custom serialization format is deprecated. Use
// https://github.com/secure-systems-lab/securesystemslib/blob/main/docs/migrate_key.py
// to convert your key.
func LoadECDSAKeyFromFile(path string) (*SSLibKey, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to load ECDSA key from file: %w", err)
	}

	return LoadKeyFromSSLibBytes(contents)
}

func getECDSAHashedData(data []byte, curveSize int) []byte {
	switch {
	case curveSize <= 256:
		return hashBeforeSigning(data, sha256.New())
	case 256 < curveSize && curveSize <= 384:
		return hashBeforeSigning(data, sha512.New384())
	case curveSize > 384:
		return hashBeforeSigning(data, sha512.New())
	}
	return []byte{}
}
