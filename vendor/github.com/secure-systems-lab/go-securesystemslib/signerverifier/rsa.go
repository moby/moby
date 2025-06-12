package signerverifier

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
)

const (
	RSAKeyType       = "rsa"
	RSAKeyScheme     = "rsassa-pss-sha256"
	RSAPrivateKeyPEM = "RSA PRIVATE KEY"
)

// RSAPSSSignerVerifier is a dsse.SignerVerifier compliant interface to sign and
// verify signatures using RSA keys following the RSA-PSS scheme.
type RSAPSSSignerVerifier struct {
	keyID   string
	private *rsa.PrivateKey
	public  *rsa.PublicKey
}

// NewRSAPSSSignerVerifierFromSSLibKey creates an RSAPSSSignerVerifier from an
// SSLibKey.
func NewRSAPSSSignerVerifierFromSSLibKey(key *SSLibKey) (*RSAPSSSignerVerifier, error) {
	if len(key.KeyVal.Public) == 0 {
		return nil, ErrInvalidKey
	}

	_, publicParsedKey, err := decodeAndParsePEM([]byte(key.KeyVal.Public))
	if err != nil {
		return nil, fmt.Errorf("unable to create RSA-PSS signerverifier: %w", err)
	}

	if len(key.KeyVal.Private) > 0 {
		_, privateParsedKey, err := decodeAndParsePEM([]byte(key.KeyVal.Private))
		if err != nil {
			return nil, fmt.Errorf("unable to create RSA-PSS signerverifier: %w", err)
		}

		return &RSAPSSSignerVerifier{
			keyID:   key.KeyID,
			public:  publicParsedKey.(*rsa.PublicKey),
			private: privateParsedKey.(*rsa.PrivateKey),
		}, nil
	}

	return &RSAPSSSignerVerifier{
		keyID:   key.KeyID,
		public:  publicParsedKey.(*rsa.PublicKey),
		private: nil,
	}, nil
}

// Sign creates a signature for `data`.
func (sv *RSAPSSSignerVerifier) Sign(ctx context.Context, data []byte) ([]byte, error) {
	if sv.private == nil {
		return nil, ErrNotPrivateKey
	}

	hashedData := hashBeforeSigning(data, sha256.New())

	return rsa.SignPSS(rand.Reader, sv.private, crypto.SHA256, hashedData, &rsa.PSSOptions{SaltLength: sha256.Size, Hash: crypto.SHA256})
}

// Verify verifies the `sig` value passed in against `data`.
func (sv *RSAPSSSignerVerifier) Verify(ctx context.Context, data []byte, sig []byte) error {
	hashedData := hashBeforeSigning(data, sha256.New())

	if err := rsa.VerifyPSS(sv.public, crypto.SHA256, hashedData, sig, &rsa.PSSOptions{SaltLength: sha256.Size, Hash: crypto.SHA256}); err != nil {
		return ErrSignatureVerificationFailed
	}

	return nil
}

// KeyID returns the identifier of the key used to create the
// RSAPSSSignerVerifier instance.
func (sv *RSAPSSSignerVerifier) KeyID() (string, error) {
	return sv.keyID, nil
}

// Public returns the public portion of the key used to create the
// RSAPSSSignerVerifier instance.
func (sv *RSAPSSSignerVerifier) Public() crypto.PublicKey {
	return sv.public
}

// LoadRSAPSSKeyFromFile returns an SSLibKey instance for an RSA key stored in a
// file.
func LoadRSAPSSKeyFromFile(path string) (*SSLibKey, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to load RSA key from file: %w", err)
	}

	pemData, keyObj, err := decodeAndParsePEM(contents)
	if err != nil {
		return nil, fmt.Errorf("unable to load RSA key from file: %w", err)
	}

	key := &SSLibKey{
		KeyType:             RSAKeyType,
		Scheme:              RSAKeyScheme,
		KeyIDHashAlgorithms: KeyIDHashAlgorithms,
		KeyVal:              KeyVal{},
	}

	switch k := keyObj.(type) {
	case *rsa.PublicKey:
		pubKeyBytes, err := x509.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, fmt.Errorf("unable to load RSA key from file: %w", err)
		}
		key.KeyVal.Public = strings.TrimSpace(string(generatePEMBlock(pubKeyBytes, PublicKeyPEM)))

	case *rsa.PrivateKey:
		pubKeyBytes, err := x509.MarshalPKIXPublicKey(k.Public())
		if err != nil {
			return nil, fmt.Errorf("unable to load RSA key from file: %w", err)
		}
		key.KeyVal.Public = strings.TrimSpace(string(generatePEMBlock(pubKeyBytes, PublicKeyPEM)))
		key.KeyVal.Private = strings.TrimSpace(string(generatePEMBlock(pemData.Bytes, RSAPrivateKeyPEM)))
	}

	if len(key.KeyID) == 0 {
		keyID, err := calculateKeyID(key)
		if err != nil {
			return nil, fmt.Errorf("unable to load RSA key from file: %w", err)
		}
		key.KeyID = keyID
	}

	return key, nil
}
