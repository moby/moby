package signed

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"reflect"

	"github.com/Sirupsen/logrus"
	"github.com/agl/ed25519"
	"github.com/docker/notary/tuf/data"
)

const (
	minRSAKeySizeBit  = 2048 // 2048 bits = 256 bytes
	minRSAKeySizeByte = minRSAKeySizeBit / 8
)

// Verifiers serves as a map of all verifiers available on the system and
// can be injected into a verificationService. For testing and configuration
// purposes, it will not be used by default.
var Verifiers = map[data.SigAlgorithm]Verifier{
	data.RSAPSSSignature:      RSAPSSVerifier{},
	data.RSAPKCS1v15Signature: RSAPKCS1v15Verifier{},
	data.PyCryptoSignature:    RSAPyCryptoVerifier{},
	data.ECDSASignature:       ECDSAVerifier{},
	data.EDDSASignature:       Ed25519Verifier{},
}

// RegisterVerifier provides a convenience function for init() functions
// to register additional verifiers or replace existing ones.
func RegisterVerifier(algorithm data.SigAlgorithm, v Verifier) {
	curr, ok := Verifiers[algorithm]
	if ok {
		typOld := reflect.TypeOf(curr)
		typNew := reflect.TypeOf(v)
		logrus.Debugf(
			"replacing already loaded verifier %s:%s with %s:%s",
			typOld.PkgPath(), typOld.Name(),
			typNew.PkgPath(), typNew.Name(),
		)
	} else {
		logrus.Debug("adding verifier for: ", algorithm)
	}
	Verifiers[algorithm] = v
}

// Ed25519Verifier used to verify Ed25519 signatures
type Ed25519Verifier struct{}

// Verify checks that an ed25519 signature is valid
func (v Ed25519Verifier) Verify(key data.PublicKey, sig []byte, msg []byte) error {
	if key.Algorithm() != data.ED25519Key {
		return ErrInvalidKeyType{}
	}
	var sigBytes [ed25519.SignatureSize]byte
	if len(sig) != ed25519.SignatureSize {
		logrus.Debugf("signature length is incorrect, must be %d, was %d.", ed25519.SignatureSize, len(sig))
		return ErrInvalid
	}
	copy(sigBytes[:], sig)

	var keyBytes [ed25519.PublicKeySize]byte
	pub := key.Public()
	if len(pub) != ed25519.PublicKeySize {
		logrus.Errorf("public key is incorrect size, must be %d, was %d.", ed25519.PublicKeySize, len(pub))
		return ErrInvalidKeyLength{msg: fmt.Sprintf("ed25519 public key must be %d bytes.", ed25519.PublicKeySize)}
	}
	n := copy(keyBytes[:], key.Public())
	if n < ed25519.PublicKeySize {
		logrus.Errorf("failed to copy the key, must have %d bytes, copied %d bytes.", ed25519.PublicKeySize, n)
		return ErrInvalid
	}

	if !ed25519.Verify(&keyBytes, msg, &sigBytes) {
		logrus.Debugf("failed ed25519 verification")
		return ErrInvalid
	}
	return nil
}

func verifyPSS(key interface{}, digest, sig []byte) error {
	rsaPub, ok := key.(*rsa.PublicKey)
	if !ok {
		logrus.Debugf("value was not an RSA public key")
		return ErrInvalid
	}

	if rsaPub.N.BitLen() < minRSAKeySizeBit {
		logrus.Debugf("RSA keys less than 2048 bits are not acceptable, provided key has length %d.", rsaPub.N.BitLen())
		return ErrInvalidKeyLength{msg: fmt.Sprintf("RSA key must be at least %d bits.", minRSAKeySizeBit)}
	}

	if len(sig) < minRSAKeySizeByte {
		logrus.Debugf("RSA keys less than 2048 bits are not acceptable, provided signature has length %d.", len(sig))
		return ErrInvalid
	}

	opts := rsa.PSSOptions{SaltLength: sha256.Size, Hash: crypto.SHA256}
	if err := rsa.VerifyPSS(rsaPub, crypto.SHA256, digest[:], sig, &opts); err != nil {
		logrus.Debugf("failed RSAPSS verification: %s", err)
		return ErrInvalid
	}
	return nil
}

func getRSAPubKey(key data.PublicKey) (crypto.PublicKey, error) {
	algorithm := key.Algorithm()
	var pubKey crypto.PublicKey

	switch algorithm {
	case data.RSAx509Key:
		pemCert, _ := pem.Decode([]byte(key.Public()))
		if pemCert == nil {
			logrus.Debugf("failed to decode PEM-encoded x509 certificate")
			return nil, ErrInvalid
		}
		cert, err := x509.ParseCertificate(pemCert.Bytes)
		if err != nil {
			logrus.Debugf("failed to parse x509 certificate: %s\n", err)
			return nil, ErrInvalid
		}
		pubKey = cert.PublicKey
	case data.RSAKey:
		var err error
		pubKey, err = x509.ParsePKIXPublicKey(key.Public())
		if err != nil {
			logrus.Debugf("failed to parse public key: %s\n", err)
			return nil, ErrInvalid
		}
	default:
		// only accept RSA keys
		logrus.Debugf("invalid key type for RSAPSS verifier: %s", algorithm)
		return nil, ErrInvalidKeyType{}
	}

	return pubKey, nil
}

// RSAPSSVerifier checks RSASSA-PSS signatures
type RSAPSSVerifier struct{}

// Verify does the actual check.
func (v RSAPSSVerifier) Verify(key data.PublicKey, sig []byte, msg []byte) error {
	// will return err if keytype is not a recognized RSA type
	pubKey, err := getRSAPubKey(key)
	if err != nil {
		return err
	}

	digest := sha256.Sum256(msg)

	return verifyPSS(pubKey, digest[:], sig)
}

// RSAPKCS1v15Verifier checks RSA PKCS1v15 signatures
type RSAPKCS1v15Verifier struct{}

// Verify does the actual verification
func (v RSAPKCS1v15Verifier) Verify(key data.PublicKey, sig []byte, msg []byte) error {
	// will return err if keytype is not a recognized RSA type
	pubKey, err := getRSAPubKey(key)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(msg)

	rsaPub, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		logrus.Debugf("value was not an RSA public key")
		return ErrInvalid
	}

	if rsaPub.N.BitLen() < minRSAKeySizeBit {
		logrus.Debugf("RSA keys less than 2048 bits are not acceptable, provided key has length %d.", rsaPub.N.BitLen())
		return ErrInvalidKeyLength{msg: fmt.Sprintf("RSA key must be at least %d bits.", minRSAKeySizeBit)}
	}

	if len(sig) < minRSAKeySizeByte {
		logrus.Debugf("RSA keys less than 2048 bits are not acceptable, provided signature has length %d.", len(sig))
		return ErrInvalid
	}

	if err = rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, digest[:], sig); err != nil {
		logrus.Errorf("Failed verification: %s", err.Error())
		return ErrInvalid
	}
	return nil
}

// RSAPyCryptoVerifier checks RSASSA-PSS signatures
type RSAPyCryptoVerifier struct{}

// Verify does the actual check.
// N.B. We have not been able to make this work in a way that is compatible
// with PyCrypto.
func (v RSAPyCryptoVerifier) Verify(key data.PublicKey, sig []byte, msg []byte) error {
	digest := sha256.Sum256(msg)
	if key.Algorithm() != data.RSAKey {
		return ErrInvalidKeyType{}
	}

	k, _ := pem.Decode([]byte(key.Public()))
	if k == nil {
		logrus.Debugf("failed to decode PEM-encoded x509 certificate")
		return ErrInvalid
	}

	pub, err := x509.ParsePKIXPublicKey(k.Bytes)
	if err != nil {
		logrus.Debugf("failed to parse public key: %s\n", err)
		return ErrInvalid
	}

	return verifyPSS(pub, digest[:], sig)
}

// ECDSAVerifier checks ECDSA signatures, decoding the keyType appropriately
type ECDSAVerifier struct{}

// Verify does the actual check.
func (v ECDSAVerifier) Verify(key data.PublicKey, sig []byte, msg []byte) error {
	algorithm := key.Algorithm()
	var pubKey crypto.PublicKey

	switch algorithm {
	case data.ECDSAx509Key:
		pemCert, _ := pem.Decode([]byte(key.Public()))
		if pemCert == nil {
			logrus.Debugf("failed to decode PEM-encoded x509 certificate for keyID: %s", key.ID())
			logrus.Debugf("certificate bytes: %s", string(key.Public()))
			return ErrInvalid
		}
		cert, err := x509.ParseCertificate(pemCert.Bytes)
		if err != nil {
			logrus.Debugf("failed to parse x509 certificate: %s\n", err)
			return ErrInvalid
		}
		pubKey = cert.PublicKey
	case data.ECDSAKey:
		var err error
		pubKey, err = x509.ParsePKIXPublicKey(key.Public())
		if err != nil {
			logrus.Debugf("Failed to parse private key for keyID: %s, %s\n", key.ID(), err)
			return ErrInvalid
		}
	default:
		// only accept ECDSA keys.
		logrus.Debugf("invalid key type for ECDSA verifier: %s", algorithm)
		return ErrInvalidKeyType{}
	}

	ecdsaPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		logrus.Debugf("value isn't an ECDSA public key")
		return ErrInvalid
	}

	sigLength := len(sig)
	expectedOctetLength := 2 * ((ecdsaPubKey.Params().BitSize + 7) >> 3)
	if sigLength != expectedOctetLength {
		logrus.Debugf("signature had an unexpected length")
		return ErrInvalid
	}

	rBytes, sBytes := sig[:sigLength/2], sig[sigLength/2:]
	r := new(big.Int).SetBytes(rBytes)
	s := new(big.Int).SetBytes(sBytes)

	digest := sha256.Sum256(msg)

	if !ecdsa.Verify(ecdsaPubKey, digest[:], r, s) {
		logrus.Debugf("failed ECDSA signature validation")
		return ErrInvalid
	}

	return nil
}
