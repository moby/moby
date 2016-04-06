package trustmanager

import (
	"crypto/x509"
	"errors"
	"fmt"
)

const certExtension string = "crt"

// ErrNoCertificatesFound is returned when no certificates are found for a
// GetCertificatesBy*
type ErrNoCertificatesFound struct {
	query string
}

// ErrNoCertificatesFound is returned when no certificates are found for a
// GetCertificatesBy*
func (err ErrNoCertificatesFound) Error() string {
	return fmt.Sprintf("error, no certificates found in the keystore match: %s", err.query)
}

// ErrCertValidation is returned when a certificate doesn't pass the store specific
// validations
type ErrCertValidation struct {
}

// ErrCertValidation is returned when a certificate doesn't pass the store specific
// validations
func (err ErrCertValidation) Error() string {
	return fmt.Sprintf("store-specific certificate validations failed")
}

// ErrCertExists is returned when a Certificate already exists in the key store
type ErrCertExists struct {
}

// ErrCertExists is returned when a Certificate already exists in the key store
func (err ErrCertExists) Error() string {
	return fmt.Sprintf("certificate already in the store")
}

// ErrBadCertificateStore is returned when there is an internal inconsistency
// in our x509 store
type ErrBadCertificateStore struct {
}

// ErrBadCertificateStore is returned when there is an internal inconsistency
// in our x509 store
func (err ErrBadCertificateStore) Error() string {
	return fmt.Sprintf("inconsistent certificate store")
}

// X509Store is the interface for all X509Stores
type X509Store interface {
	AddCert(cert *x509.Certificate) error
	AddCertFromPEM(pemCerts []byte) error
	AddCertFromFile(filename string) error
	RemoveCert(cert *x509.Certificate) error
	RemoveAll() error
	GetCertificateByCertID(certID string) (*x509.Certificate, error)
	GetCertificatesByCN(cn string) ([]*x509.Certificate, error)
	GetCertificates() []*x509.Certificate
	GetCertificatePool() *x509.CertPool
	GetVerifyOptions(dnsName string) (x509.VerifyOptions, error)
}

// CertID represent the ID used to identify certificates
type CertID string

// Validator is a convenience type to create validating function that filters
// certificates that get added to the store
type Validator interface {
	Validate(cert *x509.Certificate) bool
}

// ValidatorFunc is a convenience type to create functions that implement
// the Validator interface
type ValidatorFunc func(cert *x509.Certificate) bool

// Validate implements the Validator interface to allow for any func() bool method
// to be passed as a Validator
func (vf ValidatorFunc) Validate(cert *x509.Certificate) bool {
	return vf(cert)
}

// Verify operates on an X509Store and validates the existence of a chain of trust
// between a leafCertificate and a CA present inside of the X509 Store.
// It requires at least two certificates in certList, a leaf Certificate and an
// intermediate CA certificate.
func Verify(s X509Store, dnsName string, certList []*x509.Certificate) error {
	// If we have no Certificates loaded return error (we don't want to revert to using
	// system CAs).
	if len(s.GetCertificates()) == 0 {
		return errors.New("no root CAs available")
	}

	// At a minimum we should be provided a leaf cert and an intermediate.
	if len(certList) < 2 {
		return errors.New("certificate and at least one intermediate needed")
	}

	// Get the VerifyOptions from the keystore for a base dnsName
	opts, err := s.GetVerifyOptions(dnsName)
	if err != nil {
		return err
	}

	// Create a Certificate Pool for our intermediate certificates
	intPool := x509.NewCertPool()
	var leafCert *x509.Certificate

	// Iterate through all the certificates
	for _, c := range certList {
		// If the cert is a CA, we add it to the intermediates pool. If not, we call
		// it the leaf cert
		if c.IsCA {
			intPool.AddCert(c)
			continue
		}
		// Certificate is not a CA, it must be our leaf certificate.
		// If we already found one, bail with error
		if leafCert != nil {
			return errors.New("more than one leaf certificate found")
		}
		leafCert = c
	}

	// We exited the loop with no leaf certificates
	if leafCert == nil {
		return errors.New("no leaf certificates found")
	}

	// We have one leaf certificate and at least one intermediate. Lets add this
	// Cert Pool as the Intermediates list on our VerifyOptions
	opts.Intermediates = intPool

	// Finally, let's call Verify on our leafCert with our fully configured options
	chains, err := leafCert.Verify(opts)
	if len(chains) == 0 || err != nil {
		return fmt.Errorf("certificate verification failed: %v", err)
	}
	return nil
}
