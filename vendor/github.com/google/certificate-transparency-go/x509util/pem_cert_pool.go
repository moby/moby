// Copyright 2016 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package x509util

import (
	"crypto/sha256"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"github.com/google/certificate-transparency-go/x509"
)

// String for certificate blocks in BEGIN / END PEM headers
const pemCertificateBlockType string = "CERTIFICATE"

// PEMCertPool is a wrapper / extension to x509.CertPool. It allows us to access the
// raw certs, which we need to serve get-roots request and has stricter handling on loading
// certs into the pool. CertPool ignores errors if at least one cert loads correctly but
// PEMCertPool requires all certs to load.
type PEMCertPool struct {
	// maps from sha-256 to certificate, used for dup detection
	fingerprintToCertMap map[[sha256.Size]byte]x509.Certificate
	rawCerts             []*x509.Certificate
	certPool             *x509.CertPool
}

// NewPEMCertPool creates a new, empty, instance of PEMCertPool.
func NewPEMCertPool() *PEMCertPool {
	return &PEMCertPool{fingerprintToCertMap: make(map[[sha256.Size]byte]x509.Certificate), certPool: x509.NewCertPool()}
}

// AddCert adds a certificate to a pool. Uses fingerprint to weed out duplicates.
// cert must not be nil.
func (p *PEMCertPool) AddCert(cert *x509.Certificate) {
	fingerprint := sha256.Sum256(cert.Raw)
	_, ok := p.fingerprintToCertMap[fingerprint]

	if !ok {
		p.fingerprintToCertMap[fingerprint] = *cert
		p.certPool.AddCert(cert)
		p.rawCerts = append(p.rawCerts, cert)
	}
}

// Included indicates whether the given cert is included in the pool.
func (p *PEMCertPool) Included(cert *x509.Certificate) bool {
	fingerprint := sha256.Sum256(cert.Raw)
	_, ok := p.fingerprintToCertMap[fingerprint]
	return ok
}

// AppendCertsFromPEM adds certs to the pool from a byte slice assumed to contain PEM encoded data.
// Skips over non certificate blocks in the data. Returns true if all certificates in the
// data were parsed and added to the pool successfully and at least one certificate was found.
func (p *PEMCertPool) AppendCertsFromPEM(pemCerts []byte) (ok bool) {
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		if block.Type != pemCertificateBlockType || len(block.Headers) != 0 {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if x509.IsFatal(err) {
			return false
		}

		p.AddCert(cert)
		ok = true
	}

	return
}

// AppendCertsFromPEMFile adds certs from a file that contains concatenated PEM data.
func (p *PEMCertPool) AppendCertsFromPEMFile(pemFile string) error {
	pemData, err := os.ReadFile(pemFile)
	if err != nil {
		return fmt.Errorf("failed to load PEM certs file: %v", err)
	}

	if !p.AppendCertsFromPEM(pemData) {
		return errors.New("failed to parse PEM certs file")
	}
	return nil
}

// Subjects returns a list of the DER-encoded subjects of all of the certificates in the pool.
func (p *PEMCertPool) Subjects() (res [][]byte) {
	return p.certPool.Subjects()
}

// CertPool returns the underlying CertPool.
func (p *PEMCertPool) CertPool() *x509.CertPool {
	return p.certPool
}

// RawCertificates returns a list of the raw bytes of certificates that are in this pool
func (p *PEMCertPool) RawCertificates() []*x509.Certificate {
	return p.rawCerts
}
