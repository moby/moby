// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package certtostore handles storage for certificates.
package certtostore

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	// BEGIN-INTERNAL
	// internal content #1
	// END-INTERNAL
)

const (
	createMode = os.FileMode(0600)
)

// Algorithm indicates an asymmetric algorithm used by the credential.
type Algorithm string

// Algorithms types supported by this package.
const (
	EC  Algorithm = "EC"
	RSA Algorithm = "RSA"
)

func (a Algorithm) Tox509() x509.PublicKeyAlgorithm {
	switch a {
	case EC:
		return x509.ECDSA
	case RSA:
		return x509.RSA
	default:
		return 0
	}
}

// GenerateOpts holds parameters used to generate a private key.
type GenerateOpts struct {
	// Algorithm to be used, either RSA or EC.
	Algorithm Algorithm
	// Size is used to specify the bit size of the RSA key or curve for EC keys.
	Size int
}

// CertStorage exposes the different backend storage options for certificates.
type CertStorage interface {
	// Cert returns the current X509 certificate or nil if no certificate is installed.
	Cert() (*x509.Certificate, error)
	// Intermediate returns the current intermediate X509 certificate or nil if no certificate is installed.
	Intermediate() (*x509.Certificate, error)
	// CertificateChain returns the leaf and subsequent certificates.
	CertificateChain() ([][]*x509.Certificate, error)
	// Generate generates a new private key in the storage and returns a signer that can be used
	// to perform signatures with the new key and read the public portion of the key. CertStorage
	// implementations should strive to ensure a Generate call doesn't actually destroy any current
	// key or cert material and to only install the new key for clients once Store is called.
	Generate(opts GenerateOpts) (crypto.Signer, error)
	// Store finishes the cert installation started by the last Generate call with the given cert and
	// intermediate.
	Store(cert *x509.Certificate, intermediate *x509.Certificate) error
	// Key returns the certificate as a Credential (crypto.Signer and crypto.Decrypter).
	Key() (Credential, error)
}

// certificateChain returns chains of the leaf and subsequent certificates.
func certificateChain(cert, intermediate *x509.Certificate) ([][]*x509.Certificate, error) {
	var intermediates *x509.CertPool
	if intermediate != nil {
		intermediates = x509.NewCertPool()
		intermediates.AddCert(intermediate)
	}
	vo := x509.VerifyOptions{
		Roots:         nil, // Use system roots
		Intermediates: intermediates,
	}
	chains, err := cert.Verify(vo)
	if err != nil {
		// Fall back to a basic chain if the system cannot verify our chain (eg. if it is self signed).
		return [][]*x509.Certificate{{cert, intermediate}}, nil
	}
	return chains, nil
}

// Credential provides access to a certificate and is a crypto.Signer and crypto.Decrypter.
type Credential interface {
	// Public returns the public key corresponding to the leaf certificate.
	Public() crypto.PublicKey
	// Sign signs digest with the private key.
	Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error)
	// Decrypt decrypts msg. Returns an error if not implemented.
	Decrypt(rand io.Reader, msg []byte, opts crypto.DecrypterOpts) (plaintext []byte, err error)
}

// FileStorage exposes the file storage (on disk) backend type for certificates.
// The certificate id is used as the base of the filename within the basepath.
type FileStorage struct {
	certFile   string
	caCertFile string
	keyFile    string
	key        crypto.Signer
}

// NewFileStorage sets up a new file storage struct for use by StoreCert.
func NewFileStorage(basepath string) *FileStorage {
	certFile := filepath.Join(basepath, "cert.crt")
	caCertFile := filepath.Join(basepath, "cacert.crt")
	keyFile := filepath.Join(basepath, "cert.key")
	return &FileStorage{certFile: certFile, caCertFile: caCertFile, keyFile: keyFile}
}

// Key returns a Credential for the current FileStorage.
func (f FileStorage) Key() (Credential, error) {
	// Not every storage type implements Credential directly, this is an opportunity
	// to construct and return a Credential in that case.  Here it is a no-op.
	return f, nil
}

// Cert returns the FileStorage's current cert or nil if there is none.
func (f FileStorage) Cert() (*x509.Certificate, error) {
	return certFromDisk(f.certFile)
}

// Public returns the public key corresponding to the leaf certificate or nil if there is none.
func (f FileStorage) Public() crypto.PublicKey {
	c, err := f.Cert()
	if err != nil {
		return nil
	}
	if c == nil {
		return nil
	}
	pk, ok := c.PublicKey.(crypto.PublicKey)
	if !ok {
		return nil
	}
	return pk
}

// Intermediate returns the FileStorage's current intermediate cert or nil if there is none.
func (f FileStorage) Intermediate() (*x509.Certificate, error) {
	return certFromDisk(f.caCertFile)
}

// CertificateChain returns chains of the leaf and subsequent certificates.
func (f FileStorage) CertificateChain() ([][]*x509.Certificate, error) {
	cert, err := f.Cert()
	if err != nil {
		return nil, fmt.Errorf("CertificateChain load leaf: %v", err)
	}
	if cert == nil {
		return nil, fmt.Errorf("CertificateChain load leaf: no certificate")
	}
	intermediate, err := f.Intermediate()
	if err != nil {
		return nil, fmt.Errorf("CertificateChain load intermediate: %v", err)
	}
	return certificateChain(cert, intermediate)
}

var ecdsaCurves = map[int]elliptic.Curve{
	256: elliptic.P256(),
	384: elliptic.P384(),
	521: elliptic.P521(),
}

// Generate creates a new ECDSA or RSA private key and returns a signer that can be used to make a CSR for the key.
func (f *FileStorage) Generate(opts GenerateOpts) (crypto.Signer, error) {
	var err error
	switch opts.Algorithm {
	case RSA:
		f.key, err = rsa.GenerateKey(rand.Reader, opts.Size)
		return f.key, err
	case EC:
		curve, ok := ecdsaCurves[opts.Size]
		if !ok {
			return nil, fmt.Errorf("invalid ecdsa curve size: %d", opts.Size)
		}
		f.key, err = ecdsa.GenerateKey(curve, rand.Reader)
		return f.key, err
	default:

		return nil, fmt.Errorf("unsupported key type: %q", opts.Algorithm)
	}
}

// Store finishes our cert installation by PEM encoding the cert, intermediate, and key and storing them to disk.
func (f *FileStorage) Store(cert *x509.Certificate, intermediate *x509.Certificate) error {
	// Make sure our directory exists
	if err := os.MkdirAll(filepath.Dir(f.certFile), createMode|0111); err != nil {
		return err
	}

	// Encode our certs and key
	var certBuf, intermediateBuf, keyBuf bytes.Buffer
	if err := pem.Encode(&certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
		return fmt.Errorf("could not encode cert to PEM: %v", err)
	}
	if err := pem.Encode(&intermediateBuf, &pem.Block{Type: "CERTIFICATE", Bytes: intermediate.Raw}); err != nil {
		return fmt.Errorf("could not encode intermediate to PEM: %v", err)
	}

	// Write the certificates out to files
	if err := ioutil.WriteFile(f.certFile, certBuf.Bytes(), createMode); err != nil {
		return err
	}
	if err := ioutil.WriteFile(f.caCertFile, intermediateBuf.Bytes(), createMode); err != nil {
		return err
	}

	// Return early if no private key is available
	if f.key == nil {
		return nil
	}
	// Write our private key out to a file
	b, err := x509.MarshalPKCS8PrivateKey(f.key)
	if err != nil {
		return err
	}
	if err := pem.Encode(&keyBuf, &pem.Block{Type: "PRIVATE KEY", Bytes: b}); err != nil {
		return fmt.Errorf("could not encode key to PEM: %v", err)
	}

	return ioutil.WriteFile(f.keyFile, keyBuf.Bytes(), createMode)
}

// Sign returns a signature for the provided digest.
// The opts are passed to the private key's Sign method, as per the crypto.Signer interface.
// https://pkg.go.dev/crypto#Signer
func (f FileStorage) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	tlsCert, err := tls.LoadX509KeyPair(f.certFile, f.keyFile)
	if err != nil {
		return nil, err
	}
	s, ok := tlsCert.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("x509 private key does not implement crypto.Signer")
	}
	return s.Sign(rand, digest, opts)
}

// Decrypt decrypts msg. Returns an error if not implemented.
// The opts are passed to the private key's Decrypt method, as per the crypto.Decrypter interface.
// https://pkg.go.dev/crypto#Decrypter
// Only RSA keys are supported for decryption.
func (f FileStorage) Decrypt(rand io.Reader, msg []byte, opts crypto.DecrypterOpts) ([]byte, error) {
	tlsCert, err := tls.LoadX509KeyPair(f.certFile, f.keyFile)
	if err != nil {
		return nil, err
	}
	s, ok := tlsCert.PrivateKey.(crypto.Decrypter)
	if !ok {
		return nil, fmt.Errorf("x509 private key does not implement crypto.Decrypter")
	}
	return s.Decrypt(rand, msg, opts)
}

// certFromDisk reads a x509.Certificate from a location on disk and
// validates it as a certificate. If the filename doesn't exist it returns
// (nil, nil) to indicate a non-fatal failure to read the cert.
func certFromDisk(filename string) (*x509.Certificate, error) {
	certPEM, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	xc, err := PEMToX509(certPEM)
	if err != nil {
		return nil, fmt.Errorf("file %q is not recognized as a certificate", filename)
	}
	return xc, nil
}

// PEMToX509 takes a raw PEM certificate and decodes it to an x509.Certificate.
func PEMToX509(b []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("unable to parse PEM certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

// Verify interface conformance.
var _ CertStorage = &FileStorage{}
var _ Credential = &FileStorage{}
