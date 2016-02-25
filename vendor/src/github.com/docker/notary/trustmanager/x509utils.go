package trustmanager

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/agl/ed25519"
	"github.com/docker/notary"
	"github.com/docker/notary/tuf/data"
)

// GetCertFromURL tries to get a X509 certificate given a HTTPS URL
func GetCertFromURL(urlStr string) (*x509.Certificate, error) {
	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// Check if we are adding via HTTPS
	if url.Scheme != "https" {
		return nil, errors.New("only HTTPS URLs allowed")
	}

	// Download the certificate and write to directory
	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}

	// Copy the content to certBytes
	defer resp.Body.Close()
	certBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Try to extract the first valid PEM certificate from the bytes
	cert, err := LoadCertFromPEM(certBytes)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

// CertToPEM is an utility function returns a PEM encoded x509 Certificate
func CertToPEM(cert *x509.Certificate) []byte {
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})

	return pemCert
}

// LoadCertFromPEM returns the first certificate found in a bunch of bytes or error
// if nothing is found. Taken from https://golang.org/src/crypto/x509/cert_pool.go#L85.
func LoadCertFromPEM(pemBytes []byte) (*x509.Certificate, error) {
	for len(pemBytes) > 0 {
		var block *pem.Block
		block, pemBytes = pem.Decode(pemBytes)
		if block == nil {
			return nil, errors.New("no certificates found in PEM data")
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}

		return cert, nil
	}

	return nil, errors.New("no certificates found in PEM data")
}

// FingerprintCert returns a TUF compliant fingerprint for a X509 Certificate
func FingerprintCert(cert *x509.Certificate) (string, error) {
	certID, err := fingerprintCert(cert)
	if err != nil {
		return "", err
	}

	return string(certID), nil
}

func fingerprintCert(cert *x509.Certificate) (CertID, error) {
	block := pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
	pemdata := pem.EncodeToMemory(&block)

	var tufKey data.PublicKey
	switch cert.PublicKeyAlgorithm {
	case x509.RSA:
		tufKey = data.NewRSAx509PublicKey(pemdata)
	case x509.ECDSA:
		tufKey = data.NewECDSAx509PublicKey(pemdata)
	default:
		return "", fmt.Errorf("got Unknown key type while fingerprinting certificate")
	}

	return CertID(tufKey.ID()), nil
}

// loadCertsFromDir receives a store AddCertFromFile for each certificate found
func loadCertsFromDir(s *X509FileStore) error {
	for _, f := range s.fileStore.ListFiles() {
		// ListFiles returns relative paths
		data, err := s.fileStore.Get(f)
		if err != nil {
			// the filestore told us it had a file that it then couldn't serve.
			// this is a serious problem so error immediately
			return err
		}
		err = s.AddCertFromPEM(data)
		if err != nil {
			if _, ok := err.(*ErrCertValidation); ok {
				logrus.Debugf("ignoring certificate, did not pass validation: %s", f)
				continue
			}
			if _, ok := err.(*ErrCertExists); ok {
				logrus.Debugf("ignoring certificate, already exists in the store: %s", f)
				continue
			}

			return err
		}
	}
	return nil
}

// LoadCertFromFile loads the first certificate from the file provided. The
// data is expected to be PEM Encoded and contain one of more certificates
// with PEM type "CERTIFICATE"
func LoadCertFromFile(filename string) (*x509.Certificate, error) {
	certs, err := LoadCertBundleFromFile(filename)
	if err != nil {
		return nil, err
	}
	return certs[0], nil
}

// LoadCertBundleFromFile loads certificates from the []byte provided. The
// data is expected to be PEM Encoded and contain one of more certificates
// with PEM type "CERTIFICATE"
func LoadCertBundleFromFile(filename string) ([]*x509.Certificate, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return LoadCertBundleFromPEM(b)
}

// LoadCertBundleFromPEM loads certificates from the []byte provided. The
// data is expected to be PEM Encoded and contain one of more certificates
// with PEM type "CERTIFICATE"
func LoadCertBundleFromPEM(pemBytes []byte) ([]*x509.Certificate, error) {
	certificates := []*x509.Certificate{}
	var block *pem.Block
	block, pemBytes = pem.Decode(pemBytes)
	for ; block != nil; block, pemBytes = pem.Decode(pemBytes) {
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			certificates = append(certificates, cert)
		} else {
			return nil, fmt.Errorf("invalid pem block type: %s", block.Type)
		}
	}

	if len(certificates) == 0 {
		return nil, fmt.Errorf("no valid certificates found")
	}

	return certificates, nil
}

// GetLeafCerts parses a list of x509 Certificates and returns all of them
// that aren't CA
func GetLeafCerts(certs []*x509.Certificate) []*x509.Certificate {
	var leafCerts []*x509.Certificate
	for _, cert := range certs {
		if cert.IsCA {
			continue
		}
		leafCerts = append(leafCerts, cert)
	}
	return leafCerts
}

// GetIntermediateCerts parses a list of x509 Certificates and returns all of the
// ones marked as a CA, to be used as intermediates
func GetIntermediateCerts(certs []*x509.Certificate) []*x509.Certificate {
	var intCerts []*x509.Certificate
	for _, cert := range certs {
		if cert.IsCA {
			intCerts = append(intCerts, cert)
		}
	}
	return intCerts
}

// ParsePEMPrivateKey returns a data.PrivateKey from a PEM encoded private key. It
// only supports RSA (PKCS#1) and attempts to decrypt using the passphrase, if encrypted.
func ParsePEMPrivateKey(pemBytes []byte, passphrase string) (data.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no valid private key found")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		var privKeyBytes []byte
		var err error

		if x509.IsEncryptedPEMBlock(block) {
			privKeyBytes, err = x509.DecryptPEMBlock(block, []byte(passphrase))
			if err != nil {
				return nil, errors.New("could not decrypt private key")
			}
		} else {
			privKeyBytes = block.Bytes
		}

		rsaPrivKey, err := x509.ParsePKCS1PrivateKey(privKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse DER encoded key: %v", err)
		}

		tufRSAPrivateKey, err := RSAToPrivateKey(rsaPrivKey)
		if err != nil {
			return nil, fmt.Errorf("could not convert rsa.PrivateKey to data.PrivateKey: %v", err)
		}

		return tufRSAPrivateKey, nil
	case "EC PRIVATE KEY":
		var privKeyBytes []byte
		var err error

		if x509.IsEncryptedPEMBlock(block) {
			privKeyBytes, err = x509.DecryptPEMBlock(block, []byte(passphrase))
			if err != nil {
				return nil, errors.New("could not decrypt private key")
			}
		} else {
			privKeyBytes = block.Bytes
		}

		ecdsaPrivKey, err := x509.ParseECPrivateKey(privKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse DER encoded private key: %v", err)
		}

		tufECDSAPrivateKey, err := ECDSAToPrivateKey(ecdsaPrivKey)
		if err != nil {
			return nil, fmt.Errorf("could not convert ecdsa.PrivateKey to data.PrivateKey: %v", err)
		}

		return tufECDSAPrivateKey, nil
	case "ED25519 PRIVATE KEY":
		// We serialize ED25519 keys by concatenating the private key
		// to the public key and encoding with PEM. See the
		// ED25519ToPrivateKey function.
		var privKeyBytes []byte
		var err error

		if x509.IsEncryptedPEMBlock(block) {
			privKeyBytes, err = x509.DecryptPEMBlock(block, []byte(passphrase))
			if err != nil {
				return nil, errors.New("could not decrypt private key")
			}
		} else {
			privKeyBytes = block.Bytes
		}

		tufECDSAPrivateKey, err := ED25519ToPrivateKey(privKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not convert ecdsa.PrivateKey to data.PrivateKey: %v", err)
		}

		return tufECDSAPrivateKey, nil

	default:
		return nil, fmt.Errorf("unsupported key type %q", block.Type)
	}
}

// ParsePEMPublicKey returns a data.PublicKey from a PEM encoded public key or certificate.
func ParsePEMPublicKey(pubKeyBytes []byte) (data.PublicKey, error) {
	pemBlock, _ := pem.Decode(pubKeyBytes)
	if pemBlock == nil {
		return nil, errors.New("no valid public key found")
	}

	switch pemBlock.Type {
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse provided certificate: %v", err)
		}
		err = ValidateCertificate(cert)
		if err != nil {
			return nil, fmt.Errorf("invalid certificate: %v", err)
		}
		return CertToKey(cert), nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q, expected certificate", pemBlock.Type)
	}
}

// ValidateCertificate returns an error if the certificate is not valid for notary
// Currently this is only a time expiry check, and ensuring the public key has a large enough modulus if RSA
func ValidateCertificate(c *x509.Certificate) error {
	if (c.NotBefore).After(c.NotAfter) {
		return fmt.Errorf("certificate validity window is invalid")
	}
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	// Give one day leeway on creation "before" time, check "after" against today
	if (tomorrow).Before(c.NotBefore) || now.After(c.NotAfter) {
		return fmt.Errorf("certificate is expired")
	}
	// If we have an RSA key, make sure it's long enough
	if c.PublicKeyAlgorithm == x509.RSA {
		rsaKey, ok := c.PublicKey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("unable to parse RSA public key")
		}
		if rsaKey.N.BitLen() < notary.MinRSABitSize {
			return fmt.Errorf("RSA bit length is too short")
		}
	}
	return nil
}

// GenerateRSAKey generates an RSA private key and returns a TUF PrivateKey
func GenerateRSAKey(random io.Reader, bits int) (data.PrivateKey, error) {
	rsaPrivKey, err := rsa.GenerateKey(random, bits)
	if err != nil {
		return nil, fmt.Errorf("could not generate private key: %v", err)
	}

	tufPrivKey, err := RSAToPrivateKey(rsaPrivKey)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("generated RSA key with keyID: %s", tufPrivKey.ID())

	return tufPrivKey, nil
}

// RSAToPrivateKey converts an rsa.Private key to a TUF data.PrivateKey type
func RSAToPrivateKey(rsaPrivKey *rsa.PrivateKey) (data.PrivateKey, error) {
	// Get a DER-encoded representation of the PublicKey
	rsaPubBytes, err := x509.MarshalPKIXPublicKey(&rsaPrivKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %v", err)
	}

	// Get a DER-encoded representation of the PrivateKey
	rsaPrivBytes := x509.MarshalPKCS1PrivateKey(rsaPrivKey)

	pubKey := data.NewRSAPublicKey(rsaPubBytes)
	return data.NewRSAPrivateKey(pubKey, rsaPrivBytes)
}

// GenerateECDSAKey generates an ECDSA Private key and returns a TUF PrivateKey
func GenerateECDSAKey(random io.Reader) (data.PrivateKey, error) {
	ecdsaPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), random)
	if err != nil {
		return nil, err
	}

	tufPrivKey, err := ECDSAToPrivateKey(ecdsaPrivKey)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("generated ECDSA key with keyID: %s", tufPrivKey.ID())

	return tufPrivKey, nil
}

// GenerateED25519Key generates an ED25519 private key and returns a TUF
// PrivateKey. The serialization format we use is just the public key bytes
// followed by the private key bytes
func GenerateED25519Key(random io.Reader) (data.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(random)
	if err != nil {
		return nil, err
	}

	var serialized [ed25519.PublicKeySize + ed25519.PrivateKeySize]byte
	copy(serialized[:], pub[:])
	copy(serialized[ed25519.PublicKeySize:], priv[:])

	tufPrivKey, err := ED25519ToPrivateKey(serialized[:])
	if err != nil {
		return nil, err
	}

	logrus.Debugf("generated ED25519 key with keyID: %s", tufPrivKey.ID())

	return tufPrivKey, nil
}

// ECDSAToPrivateKey converts an ecdsa.Private key to a TUF data.PrivateKey type
func ECDSAToPrivateKey(ecdsaPrivKey *ecdsa.PrivateKey) (data.PrivateKey, error) {
	// Get a DER-encoded representation of the PublicKey
	ecdsaPubBytes, err := x509.MarshalPKIXPublicKey(&ecdsaPrivKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %v", err)
	}

	// Get a DER-encoded representation of the PrivateKey
	ecdsaPrivKeyBytes, err := x509.MarshalECPrivateKey(ecdsaPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %v", err)
	}

	pubKey := data.NewECDSAPublicKey(ecdsaPubBytes)
	return data.NewECDSAPrivateKey(pubKey, ecdsaPrivKeyBytes)
}

// ED25519ToPrivateKey converts a serialized ED25519 key to a TUF
// data.PrivateKey type
func ED25519ToPrivateKey(privKeyBytes []byte) (data.PrivateKey, error) {
	if len(privKeyBytes) != ed25519.PublicKeySize+ed25519.PrivateKeySize {
		return nil, errors.New("malformed ed25519 private key")
	}

	pubKey := data.NewED25519PublicKey(privKeyBytes[:ed25519.PublicKeySize])
	return data.NewED25519PrivateKey(*pubKey, privKeyBytes)
}

func blockType(k data.PrivateKey) (string, error) {
	switch k.Algorithm() {
	case data.RSAKey, data.RSAx509Key:
		return "RSA PRIVATE KEY", nil
	case data.ECDSAKey, data.ECDSAx509Key:
		return "EC PRIVATE KEY", nil
	case data.ED25519Key:
		return "ED25519 PRIVATE KEY", nil
	default:
		return "", fmt.Errorf("algorithm %s not supported", k.Algorithm())
	}
}

// KeyToPEM returns a PEM encoded key from a Private Key
func KeyToPEM(privKey data.PrivateKey, role string) ([]byte, error) {
	bt, err := blockType(privKey)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{}
	if role != "" {
		headers = map[string]string{
			"role": role,
		}
	}

	block := &pem.Block{
		Type:    bt,
		Headers: headers,
		Bytes:   privKey.Private(),
	}

	return pem.EncodeToMemory(block), nil
}

// EncryptPrivateKey returns an encrypted PEM key given a Privatekey
// and a passphrase
func EncryptPrivateKey(key data.PrivateKey, role, passphrase string) ([]byte, error) {
	bt, err := blockType(key)
	if err != nil {
		return nil, err
	}

	password := []byte(passphrase)
	cipherType := x509.PEMCipherAES256

	encryptedPEMBlock, err := x509.EncryptPEMBlock(rand.Reader,
		bt,
		key.Private(),
		password,
		cipherType)
	if err != nil {
		return nil, err
	}

	if encryptedPEMBlock.Headers == nil {
		return nil, fmt.Errorf("unable to encrypt key - invalid PEM file produced")
	}
	encryptedPEMBlock.Headers["role"] = role

	return pem.EncodeToMemory(encryptedPEMBlock), nil
}

// ReadRoleFromPEM returns the value from the role PEM header, if it exists
func ReadRoleFromPEM(pemBytes []byte) string {
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock.Headers == nil {
		return ""
	}
	role, ok := pemBlock.Headers["role"]
	if !ok {
		return ""
	}
	return role
}

// CertToKey transforms a single input certificate into its corresponding
// PublicKey
func CertToKey(cert *x509.Certificate) data.PublicKey {
	block := pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
	pemdata := pem.EncodeToMemory(&block)

	switch cert.PublicKeyAlgorithm {
	case x509.RSA:
		return data.NewRSAx509PublicKey(pemdata)
	case x509.ECDSA:
		return data.NewECDSAx509PublicKey(pemdata)
	default:
		logrus.Debugf("Unknown key type parsed from certificate: %v", cert.PublicKeyAlgorithm)
		return nil
	}
}

// CertsToKeys transforms each of the input certificates into it's corresponding
// PublicKey
func CertsToKeys(certs []*x509.Certificate) map[string]data.PublicKey {
	keys := make(map[string]data.PublicKey)
	for _, cert := range certs {
		newKey := CertToKey(cert)
		keys[newKey.ID()] = newKey
	}
	return keys
}

// NewCertificate returns an X509 Certificate following a template, given a GUN and validity interval.
func NewCertificate(gun string, startTime, endTime time.Time) (*x509.Certificate, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new certificate: %v", err)
	}

	return &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: gun,
		},
		NotBefore: startTime,
		NotAfter:  endTime,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}, nil
}

// X509PublicKeyID returns a public key ID as a string, given a
// data.PublicKey that contains an X509 Certificate
func X509PublicKeyID(certPubKey data.PublicKey) (string, error) {
	cert, err := LoadCertFromPEM(certPubKey.Public())
	if err != nil {
		return "", err
	}
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return "", err
	}

	var key data.PublicKey
	switch certPubKey.Algorithm() {
	case data.ECDSAx509Key:
		key = data.NewECDSAPublicKey(pubKeyBytes)
	case data.RSAx509Key:
		key = data.NewRSAPublicKey(pubKeyBytes)
	}

	return key.ID(), nil
}

// FilterCertsExpiredSha1 can be used as the filter function to cert store
// initializers to filter out all expired or SHA-1 certificate that we
// shouldn't load.
func FilterCertsExpiredSha1(cert *x509.Certificate) bool {
	return !cert.IsCA &&
		time.Now().Before(cert.NotAfter) &&
		cert.SignatureAlgorithm != x509.SHA1WithRSA &&
		cert.SignatureAlgorithm != x509.DSAWithSHA1 &&
		cert.SignatureAlgorithm != x509.ECDSAWithSHA1
}
