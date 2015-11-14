package trustmanager

import (
	"crypto/x509"
	"errors"
	"os"
	"path"

	"github.com/Sirupsen/logrus"
)

// X509FileStore implements X509Store that persists on disk
type X509FileStore struct {
	validate       Validator
	fileMap        map[CertID]string
	fingerprintMap map[CertID]*x509.Certificate
	nameMap        map[string][]CertID
	fileStore      FileStore
}

// NewX509FileStore returns a new X509FileStore.
func NewX509FileStore(directory string) (*X509FileStore, error) {
	validate := ValidatorFunc(func(cert *x509.Certificate) bool { return true })
	return newX509FileStore(directory, validate)
}

// NewX509FilteredFileStore returns a new X509FileStore that validates certificates
// that are added.
func NewX509FilteredFileStore(directory string, validate func(*x509.Certificate) bool) (*X509FileStore, error) {
	return newX509FileStore(directory, validate)
}

func newX509FileStore(directory string, validate func(*x509.Certificate) bool) (*X509FileStore, error) {
	fileStore, err := NewSimpleFileStore(directory, certExtension)
	if err != nil {
		return nil, err
	}

	s := &X509FileStore{
		validate:       ValidatorFunc(validate),
		fileMap:        make(map[CertID]string),
		fingerprintMap: make(map[CertID]*x509.Certificate),
		nameMap:        make(map[string][]CertID),
		fileStore:      fileStore,
	}

	err = loadCertsFromDir(s)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// AddCert creates a filename for a given cert and adds a certificate with that name
func (s *X509FileStore) AddCert(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("adding nil Certificate to X509Store")
	}

	// Check if this certificate meets our validation criteria
	if !s.validate.Validate(cert) {
		return &ErrCertValidation{}
	}
	// Attempt to write the certificate to the file
	if err := s.addNamedCert(cert); err != nil {
		return err
	}

	return nil
}

// addNamedCert allows adding a certificate while controlling the filename it gets
// stored under. If the file does not exist on disk, saves it.
func (s *X509FileStore) addNamedCert(cert *x509.Certificate) error {
	fileName, certID, err := fileName(cert)
	if err != nil {
		return err
	}

	logrus.Debug("Adding cert with certID: ", certID)
	// Validate if we already added this certificate before
	if _, ok := s.fingerprintMap[certID]; ok {
		return &ErrCertExists{}
	}

	// Convert certificate to PEM
	certBytes := CertToPEM(cert)

	// Save the file to disk if not already there.
	filePath, err := s.fileStore.GetPath(fileName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := s.fileStore.Add(fileName, certBytes); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// We wrote the certificate succcessfully, add it to our in-memory storage
	s.fingerprintMap[certID] = cert
	s.fileMap[certID] = fileName

	name := string(cert.Subject.CommonName)
	s.nameMap[name] = append(s.nameMap[name], certID)

	return nil
}

// RemoveCert removes a certificate from a X509FileStore.
func (s *X509FileStore) RemoveCert(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("removing nil Certificate from X509Store")
	}

	certID, err := fingerprintCert(cert)
	if err != nil {
		return err
	}
	delete(s.fingerprintMap, certID)
	filename := s.fileMap[certID]
	delete(s.fileMap, certID)

	name := string(cert.Subject.CommonName)

	// Filter the fingerprint out of this name entry
	fpList := s.nameMap[name]
	newfpList := fpList[:0]
	for _, x := range fpList {
		if x != certID {
			newfpList = append(newfpList, x)
		}
	}

	s.nameMap[name] = newfpList

	if err := s.fileStore.Remove(filename); err != nil {
		return err
	}

	return nil
}

// RemoveAll removes all the certificates from the store
func (s *X509FileStore) RemoveAll() error {
	for _, filename := range s.fileMap {
		if err := s.fileStore.Remove(filename); err != nil {
			return err
		}
	}
	s.fileMap = make(map[CertID]string)
	s.fingerprintMap = make(map[CertID]*x509.Certificate)
	s.nameMap = make(map[string][]CertID)

	return nil
}

// AddCertFromPEM adds the first certificate that it finds in the byte[], returning
// an error if no Certificates are found
func (s X509FileStore) AddCertFromPEM(pemBytes []byte) error {
	cert, err := LoadCertFromPEM(pemBytes)
	if err != nil {
		return err
	}
	return s.AddCert(cert)
}

// AddCertFromFile tries to adds a X509 certificate to the store given a filename
func (s *X509FileStore) AddCertFromFile(filename string) error {
	cert, err := LoadCertFromFile(filename)
	if err != nil {
		return err
	}

	return s.AddCert(cert)
}

// GetCertificates returns an array with all of the current X509 Certificates.
func (s *X509FileStore) GetCertificates() []*x509.Certificate {
	certs := make([]*x509.Certificate, len(s.fingerprintMap))
	i := 0
	for _, v := range s.fingerprintMap {
		certs[i] = v
		i++
	}
	return certs
}

// GetCertificatePool returns an x509 CertPool loaded with all the certificates
// in the store.
func (s *X509FileStore) GetCertificatePool() *x509.CertPool {
	pool := x509.NewCertPool()

	for _, v := range s.fingerprintMap {
		pool.AddCert(v)
	}
	return pool
}

// GetCertificateByCertID returns the certificate that matches a certain certID
func (s *X509FileStore) GetCertificateByCertID(certID string) (*x509.Certificate, error) {
	return s.getCertificateByCertID(CertID(certID))
}

// getCertificateByCertID returns the certificate that matches a certain certID
func (s *X509FileStore) getCertificateByCertID(certID CertID) (*x509.Certificate, error) {
	// If it does not look like a hex encoded sha256 hash, error
	if len(certID) != 64 {
		return nil, errors.New("invalid Subject Key Identifier")
	}

	// Check to see if this subject key identifier exists
	if cert, ok := s.fingerprintMap[CertID(certID)]; ok {
		return cert, nil

	}
	return nil, &ErrNoCertificatesFound{query: string(certID)}
}

// GetCertificatesByCN returns all the certificates that match a specific
// CommonName
func (s *X509FileStore) GetCertificatesByCN(cn string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	if ids, ok := s.nameMap[cn]; ok {
		for _, v := range ids {
			cert, err := s.getCertificateByCertID(v)
			if err != nil {
				// This error should never happen. This would mean that we have
				// an inconsistent X509FileStore
				return nil, &ErrBadCertificateStore{}
			}
			certs = append(certs, cert)
		}
	}
	if len(certs) == 0 {
		return nil, &ErrNoCertificatesFound{query: cn}
	}

	return certs, nil
}

// GetVerifyOptions returns VerifyOptions with the certificates within the KeyStore
// as part of the roots list. This never allows the use of system roots, returning
// an error if there are no root CAs.
func (s *X509FileStore) GetVerifyOptions(dnsName string) (x509.VerifyOptions, error) {
	// If we have no Certificates loaded return error (we don't want to rever to using
	// system CAs).
	if len(s.fingerprintMap) == 0 {
		return x509.VerifyOptions{}, errors.New("no root CAs available")
	}

	opts := x509.VerifyOptions{
		DNSName: dnsName,
		Roots:   s.GetCertificatePool(),
	}

	return opts, nil
}

// Empty returns true if there are no certificates in the X509FileStore, false
// otherwise.
func (s *X509FileStore) Empty() bool {
	return len(s.fingerprintMap) == 0
}

func fileName(cert *x509.Certificate) (string, CertID, error) {
	certID, err := fingerprintCert(cert)
	if err != nil {
		return "", "", err
	}

	return path.Join(cert.Subject.CommonName, string(certID)), certID, nil
}
