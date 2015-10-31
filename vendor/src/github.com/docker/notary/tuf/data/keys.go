package data

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"io"
	"math/big"

	"github.com/Sirupsen/logrus"
	"github.com/agl/ed25519"
	"github.com/jfrazelle/go/canonical/json"
)

// PublicKey is the necessary interface for public keys
type PublicKey interface {
	ID() string
	Algorithm() string
	Public() []byte
}

// PrivateKey adds the ability to access the private key
type PrivateKey interface {
	PublicKey
	Sign(rand io.Reader, msg []byte, opts crypto.SignerOpts) (signature []byte, err error)
	Private() []byte
	CryptoSigner() crypto.Signer
	SignatureAlgorithm() SigAlgorithm
}

// KeyPair holds the public and private key bytes
type KeyPair struct {
	Public  []byte `json:"public"`
	Private []byte `json:"private"`
}

// Keys represents a map of key ID to PublicKey object. It's necessary
// to allow us to unmarshal into an interface via the json.Unmarshaller
// interface
type Keys map[string]PublicKey

// UnmarshalJSON implements the json.Unmarshaller interface
func (ks *Keys) UnmarshalJSON(data []byte) error {
	parsed := make(map[string]tufKey)
	err := json.Unmarshal(data, &parsed)
	if err != nil {
		return err
	}
	final := make(map[string]PublicKey)
	for k, tk := range parsed {
		final[k] = typedPublicKey(tk)
	}
	*ks = final
	return nil
}

// KeyList represents a list of keys
type KeyList []PublicKey

// UnmarshalJSON implements the json.Unmarshaller interface
func (ks *KeyList) UnmarshalJSON(data []byte) error {
	parsed := make([]tufKey, 0, 1)
	err := json.Unmarshal(data, &parsed)
	if err != nil {
		return err
	}
	final := make([]PublicKey, 0, len(parsed))
	for _, tk := range parsed {
		final = append(final, typedPublicKey(tk))
	}
	*ks = final
	return nil
}

func typedPublicKey(tk tufKey) PublicKey {
	switch tk.Algorithm() {
	case ECDSAKey:
		return &ECDSAPublicKey{tufKey: tk}
	case ECDSAx509Key:
		return &ECDSAx509PublicKey{tufKey: tk}
	case RSAKey:
		return &RSAPublicKey{tufKey: tk}
	case RSAx509Key:
		return &RSAx509PublicKey{tufKey: tk}
	case ED25519Key:
		return &ED25519PublicKey{tufKey: tk}
	}
	return &UnknownPublicKey{tufKey: tk}
}

func typedPrivateKey(tk tufKey) (PrivateKey, error) {
	private := tk.Value.Private
	tk.Value.Private = nil
	switch tk.Algorithm() {
	case ECDSAKey:
		return NewECDSAPrivateKey(
			&ECDSAPublicKey{
				tufKey: tk,
			},
			private,
		)
	case ECDSAx509Key:
		return NewECDSAPrivateKey(
			&ECDSAx509PublicKey{
				tufKey: tk,
			},
			private,
		)
	case RSAKey:
		return NewRSAPrivateKey(
			&RSAPublicKey{
				tufKey: tk,
			},
			private,
		)
	case RSAx509Key:
		return NewRSAPrivateKey(
			&RSAx509PublicKey{
				tufKey: tk,
			},
			private,
		)
	case ED25519Key:
		return NewED25519PrivateKey(
			ED25519PublicKey{
				tufKey: tk,
			},
			private,
		)
	}
	return &UnknownPrivateKey{
		tufKey:     tk,
		privateKey: privateKey{private: private},
	}, nil
}

// NewPublicKey creates a new, correctly typed PublicKey, using the
// UnknownPublicKey catchall for unsupported ciphers
func NewPublicKey(alg string, public []byte) PublicKey {
	tk := tufKey{
		Type: alg,
		Value: KeyPair{
			Public: public,
		},
	}
	return typedPublicKey(tk)
}

// NewPrivateKey creates a new, correctly typed PrivateKey, using the
// UnknownPrivateKey catchall for unsupported ciphers
func NewPrivateKey(pubKey PublicKey, private []byte) (PrivateKey, error) {
	tk := tufKey{
		Type: pubKey.Algorithm(),
		Value: KeyPair{
			Public:  pubKey.Public(),
			Private: private, // typedPrivateKey moves this value
		},
	}
	return typedPrivateKey(tk)
}

// UnmarshalPublicKey is used to parse individual public keys in JSON
func UnmarshalPublicKey(data []byte) (PublicKey, error) {
	var parsed tufKey
	err := json.Unmarshal(data, &parsed)
	if err != nil {
		return nil, err
	}
	return typedPublicKey(parsed), nil
}

// UnmarshalPrivateKey is used to parse individual private keys in JSON
func UnmarshalPrivateKey(data []byte) (PrivateKey, error) {
	var parsed tufKey
	err := json.Unmarshal(data, &parsed)
	if err != nil {
		return nil, err
	}
	return typedPrivateKey(parsed)
}

// tufKey is the structure used for both public and private keys in TUF.
// Normally it would make sense to use a different structures for public and
// private keys, but that would change the key ID algorithm (since the canonical
// JSON would be different). This structure should normally be accessed through
// the PublicKey or PrivateKey interfaces.
type tufKey struct {
	id    string
	Type  string  `json:"keytype"`
	Value KeyPair `json:"keyval"`
}

// Algorithm returns the algorithm of the key
func (k tufKey) Algorithm() string {
	return k.Type
}

// ID efficiently generates if necessary, and caches the ID of the key
func (k *tufKey) ID() string {
	if k.id == "" {
		pubK := tufKey{
			Type: k.Algorithm(),
			Value: KeyPair{
				Public:  k.Public(),
				Private: nil,
			},
		}
		data, err := json.MarshalCanonical(&pubK)
		if err != nil {
			logrus.Error("Error generating key ID:", err)
		}
		digest := sha256.Sum256(data)
		k.id = hex.EncodeToString(digest[:])
	}
	return k.id
}

// Public returns the public bytes
func (k tufKey) Public() []byte {
	return k.Value.Public
}

// Public key types

// ECDSAPublicKey represents an ECDSA key using a raw serialization
// of the public key
type ECDSAPublicKey struct {
	tufKey
}

// ECDSAx509PublicKey represents an ECDSA key using an x509 cert
// as the serialized format of the public key
type ECDSAx509PublicKey struct {
	tufKey
}

// RSAPublicKey represents an RSA key using a raw serialization
// of the public key
type RSAPublicKey struct {
	tufKey
}

// RSAx509PublicKey represents an RSA key using an x509 cert
// as the serialized format of the public key
type RSAx509PublicKey struct {
	tufKey
}

// ED25519PublicKey represents an ED25519 key using a raw serialization
// of the public key
type ED25519PublicKey struct {
	tufKey
}

// UnknownPublicKey is a catchall for key types that are not supported
type UnknownPublicKey struct {
	tufKey
}

// NewECDSAPublicKey initializes a new public key with the ECDSAKey type
func NewECDSAPublicKey(public []byte) *ECDSAPublicKey {
	return &ECDSAPublicKey{
		tufKey: tufKey{
			Type: ECDSAKey,
			Value: KeyPair{
				Public:  public,
				Private: nil,
			},
		},
	}
}

// NewECDSAx509PublicKey initializes a new public key with the ECDSAx509Key type
func NewECDSAx509PublicKey(public []byte) *ECDSAx509PublicKey {
	return &ECDSAx509PublicKey{
		tufKey: tufKey{
			Type: ECDSAx509Key,
			Value: KeyPair{
				Public:  public,
				Private: nil,
			},
		},
	}
}

// NewRSAPublicKey initializes a new public key with the RSA type
func NewRSAPublicKey(public []byte) *RSAPublicKey {
	return &RSAPublicKey{
		tufKey: tufKey{
			Type: RSAKey,
			Value: KeyPair{
				Public:  public,
				Private: nil,
			},
		},
	}
}

// NewRSAx509PublicKey initializes a new public key with the RSAx509Key type
func NewRSAx509PublicKey(public []byte) *RSAx509PublicKey {
	return &RSAx509PublicKey{
		tufKey: tufKey{
			Type: RSAx509Key,
			Value: KeyPair{
				Public:  public,
				Private: nil,
			},
		},
	}
}

// NewED25519PublicKey initializes a new public key with the ED25519Key type
func NewED25519PublicKey(public []byte) *ED25519PublicKey {
	return &ED25519PublicKey{
		tufKey: tufKey{
			Type: ED25519Key,
			Value: KeyPair{
				Public:  public,
				Private: nil,
			},
		},
	}
}

// Private key types
type privateKey struct {
	private []byte
}

type signer struct {
	signer crypto.Signer
}

// ECDSAPrivateKey represents a private ECDSA key
type ECDSAPrivateKey struct {
	PublicKey
	privateKey
	signer
}

// RSAPrivateKey represents a private RSA key
type RSAPrivateKey struct {
	PublicKey
	privateKey
	signer
}

// ED25519PrivateKey represents a private ED25519 key
type ED25519PrivateKey struct {
	ED25519PublicKey
	privateKey
}

// UnknownPrivateKey is a catchall for unsupported key types
type UnknownPrivateKey struct {
	tufKey
	privateKey
}

// NewECDSAPrivateKey initializes a new ECDSA private key
func NewECDSAPrivateKey(public PublicKey, private []byte) (*ECDSAPrivateKey, error) {
	switch public.(type) {
	case *ECDSAPublicKey, *ECDSAx509PublicKey:
	default:
		return nil, errors.New("Invalid public key type provided to NewECDSAPrivateKey")
	}
	ecdsaPrivKey, err := x509.ParseECPrivateKey(private)
	if err != nil {
		return nil, err
	}
	return &ECDSAPrivateKey{
		PublicKey:  public,
		privateKey: privateKey{private: private},
		signer:     signer{signer: ecdsaPrivKey},
	}, nil
}

// NewRSAPrivateKey initialized a new RSA private key
func NewRSAPrivateKey(public PublicKey, private []byte) (*RSAPrivateKey, error) {
	switch public.(type) {
	case *RSAPublicKey, *RSAx509PublicKey:
	default:
		return nil, errors.New("Invalid public key type provided to NewRSAPrivateKey")
	}
	rsaPrivKey, err := x509.ParsePKCS1PrivateKey(private)
	if err != nil {
		return nil, err
	}
	return &RSAPrivateKey{
		PublicKey:  public,
		privateKey: privateKey{private: private},
		signer:     signer{signer: rsaPrivKey},
	}, nil
}

// NewED25519PrivateKey initialized a new ED25519 private key
func NewED25519PrivateKey(public ED25519PublicKey, private []byte) (*ED25519PrivateKey, error) {
	return &ED25519PrivateKey{
		ED25519PublicKey: public,
		privateKey:       privateKey{private: private},
	}, nil
}

// Private return the serialized private bytes of the key
func (k privateKey) Private() []byte {
	return k.private
}

// CryptoSigner returns the underlying crypto.Signer for use cases where we need the default
// signature or public key functionality (like when we generate certificates)
func (s signer) CryptoSigner() crypto.Signer {
	return s.signer
}

// CryptoSigner returns the ED25519PrivateKey which already implements crypto.Signer
func (k ED25519PrivateKey) CryptoSigner() crypto.Signer {
	return nil
}

// CryptoSigner returns the UnknownPrivateKey which already implements crypto.Signer
func (k UnknownPrivateKey) CryptoSigner() crypto.Signer {
	return nil
}

type ecdsaSig struct {
	R *big.Int
	S *big.Int
}

// Sign creates an ecdsa signature
func (k ECDSAPrivateKey) Sign(rand io.Reader, msg []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	ecdsaPrivKey, ok := k.CryptoSigner().(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("Signer was based on the wrong key type")
	}
	hashed := sha256.Sum256(msg)
	sigASN1, err := ecdsaPrivKey.Sign(rand, hashed[:], opts)
	if err != nil {
		return nil, err
	}

	sig := ecdsaSig{}
	_, err = asn1.Unmarshal(sigASN1, &sig)
	if err != nil {
		return nil, err
	}
	rBytes, sBytes := sig.R.Bytes(), sig.S.Bytes()
	octetLength := (ecdsaPrivKey.Params().BitSize + 7) >> 3

	// MUST include leading zeros in the output
	rBuf := make([]byte, octetLength-len(rBytes), octetLength)
	sBuf := make([]byte, octetLength-len(sBytes), octetLength)

	rBuf = append(rBuf, rBytes...)
	sBuf = append(sBuf, sBytes...)
	return append(rBuf, sBuf...), nil
}

// Sign creates an rsa signature
func (k RSAPrivateKey) Sign(rand io.Reader, msg []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	hashed := sha256.Sum256(msg)
	if opts == nil {
		opts = &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       crypto.SHA256,
		}
	}
	return k.CryptoSigner().Sign(rand, hashed[:], opts)
}

// Sign creates an ed25519 signature
func (k ED25519PrivateKey) Sign(rand io.Reader, msg []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	priv := [ed25519.PrivateKeySize]byte{}
	copy(priv[:], k.private[ed25519.PublicKeySize:])
	return ed25519.Sign(&priv, msg)[:], nil
}

// Sign on an UnknownPrivateKey raises an error because the client does not
// know how to sign with this key type.
func (k UnknownPrivateKey) Sign(rand io.Reader, msg []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	return nil, errors.New("Unknown key type, cannot sign.")
}

// SignatureAlgorithm returns the SigAlgorithm for a ECDSAPrivateKey
func (k ECDSAPrivateKey) SignatureAlgorithm() SigAlgorithm {
	return ECDSASignature
}

// SignatureAlgorithm returns the SigAlgorithm for a RSAPrivateKey
func (k RSAPrivateKey) SignatureAlgorithm() SigAlgorithm {
	return RSAPSSSignature
}

// SignatureAlgorithm returns the SigAlgorithm for a ED25519PrivateKey
func (k ED25519PrivateKey) SignatureAlgorithm() SigAlgorithm {
	return EDDSASignature
}

// SignatureAlgorithm returns the SigAlgorithm for an UnknownPrivateKey
func (k UnknownPrivateKey) SignatureAlgorithm() SigAlgorithm {
	return ""
}

// PublicKeyFromPrivate returns a new tufKey based on a private key, with
// the private key bytes guaranteed to be nil.
func PublicKeyFromPrivate(pk PrivateKey) PublicKey {
	return typedPublicKey(tufKey{
		Type: pk.Algorithm(),
		Value: KeyPair{
			Public:  pk.Public(),
			Private: nil,
		},
	})
}
