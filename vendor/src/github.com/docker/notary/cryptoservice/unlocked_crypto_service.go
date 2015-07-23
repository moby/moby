package cryptoservice

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"github.com/docker/notary/trustmanager"
	"github.com/endophage/gotuf/data"
	"github.com/endophage/gotuf/signed"
)

// UnlockedCryptoService encapsulates a private key and a cryptoservice that
// uses that private key, providing convinience methods for generation of
// certificates.
type UnlockedCryptoService struct {
	PrivKey       data.PrivateKey
	CryptoService signed.CryptoService
}

// NewUnlockedCryptoService creates an UnlockedCryptoService instance
func NewUnlockedCryptoService(privKey data.PrivateKey, cryptoService signed.CryptoService) *UnlockedCryptoService {
	return &UnlockedCryptoService{
		PrivKey:       privKey,
		CryptoService: cryptoService,
	}
}

// ID gets a consistent ID based on the PrivateKey bytes and algorithm type
func (ucs *UnlockedCryptoService) ID() string {
	return ucs.PublicKey().ID()
}

// PublicKey Returns the public key associated with the private key
func (ucs *UnlockedCryptoService) PublicKey() data.PublicKey {
	return data.PublicKeyFromPrivate(ucs.PrivKey)
}

// GenerateCertificate generates an X509 Certificate from a template, given a GUN
func (ucs *UnlockedCryptoService) GenerateCertificate(gun string) (*x509.Certificate, error) {
	algorithm := ucs.PrivKey.Algorithm()
	var publicKey crypto.PublicKey
	var privateKey crypto.PrivateKey
	var err error
	switch algorithm {
	case data.RSAKey:
		var rsaPrivateKey *rsa.PrivateKey
		rsaPrivateKey, err = x509.ParsePKCS1PrivateKey(ucs.PrivKey.Private())
		privateKey = rsaPrivateKey
		publicKey = rsaPrivateKey.Public()
	case data.ECDSAKey:
		var ecdsaPrivateKey *ecdsa.PrivateKey
		ecdsaPrivateKey, err = x509.ParseECPrivateKey(ucs.PrivKey.Private())
		privateKey = ecdsaPrivateKey
		publicKey = ecdsaPrivateKey.Public()
	default:
		return nil, fmt.Errorf("only RSA or ECDSA keys are currently supported. Found: %s", algorithm)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse root key: %s (%v)", gun, err)
	}

	template, err := trustmanager.NewCertificate(gun)
	if err != nil {
		return nil, fmt.Errorf("failed to create the certificate template for: %s (%v)", gun, err)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create the certificate for: %s (%v)", gun, err)
	}

	// Encode the new certificate into PEM
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the certificate for key: %s (%v)", gun, err)
	}

	return cert, nil
}
