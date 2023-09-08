// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"encoding/pem"
)

// CertPool is a set of certificates.
type CertPool struct {
	bySubjectKeyId map[string][]int
	byName         map[string][]int
	bySHA256       map[string]int
	certs          []*Certificate
}

// NewCertPool returns a new, empty CertPool.
func NewCertPool() *CertPool {
	return &CertPool{
		bySubjectKeyId: make(map[string][]int),
		byName:         make(map[string][]int),
		bySHA256:       make(map[string]int),
	}
}

// findVerifiedParents attempts to find certificates in s which have signed the
// given certificate. If any candidates were rejected then errCert will be set
// to one of them, arbitrarily, and err will contain the reason that it was
// rejected.
func (s *CertPool) findVerifiedParents(cert *Certificate) (parents []int, errCert *Certificate, err error) {
	if s == nil {
		return
	}
	var candidates []int

	if len(cert.AuthorityKeyId) > 0 {
		candidates, _ = s.bySubjectKeyId[string(cert.AuthorityKeyId)]
	}
	if len(candidates) == 0 {
		candidates, _ = s.byName[string(cert.RawIssuer)]
	}

	for _, c := range candidates {
		if err = cert.CheckSignatureFrom(s.certs[c]); err == nil {
			cert.validSignature = true
			parents = append(parents, c)
		} else {
			errCert = s.certs[c]
		}
	}

	return
}

// Contains returns true if c is in s.
func (s *CertPool) Contains(c *Certificate) bool {
	if s == nil {
		return false
	}
	_, ok := s.bySHA256[string(c.FingerprintSHA256)]
	return ok
}

// Covers returns true if all certs in pool are in s.
func (s *CertPool) Covers(pool *CertPool) bool {
	if pool == nil {
		return true
	}
	for _, c := range pool.certs {
		if !s.Contains(c) {
			return false
		}
	}
	return true
}

// Certificates returns a list of parsed certificates in the pool.
func (s *CertPool) Certificates() []*Certificate {
	out := make([]*Certificate, 0, len(s.certs))
	out = append(out, s.certs...)
	return out
}

// Size returns the number of unique certificates in the CertPool.
func (s *CertPool) Size() int {
	if s == nil {
		return 0
	}
	return len(s.certs)
}

// Sum returns the union of two certificate pools as a new certificate pool.
func (s *CertPool) Sum(other *CertPool) (sum *CertPool) {
	sum = NewCertPool()
	if s != nil {
		for _, c := range s.certs {
			sum.AddCert(c)
		}
	}
	if other != nil {
		for _, c := range other.certs {
			sum.AddCert(c)
		}
	}
	return
}

// AddCert adds a certificate to a pool.
func (s *CertPool) AddCert(cert *Certificate) {
	if cert == nil {
		panic("adding nil Certificate to CertPool")
	}

	// Check that the certificate isn't being added twice.
	sha256fp := string(cert.FingerprintSHA256)
	if _, ok := s.bySHA256[sha256fp]; ok {
		return
	}

	n := len(s.certs)
	s.certs = append(s.certs, cert)

	if len(cert.SubjectKeyId) > 0 {
		keyId := string(cert.SubjectKeyId)
		s.bySubjectKeyId[keyId] = append(s.bySubjectKeyId[keyId], n)
	}
	name := string(cert.RawSubject)
	s.byName[name] = append(s.byName[name], n)
	s.bySHA256[sha256fp] = n
}

// AppendCertsFromPEM attempts to parse a series of PEM encoded certificates.
// It appends any certificates found to s and reports whether any certificates
// were successfully parsed.
//
// On many Linux systems, /etc/ssl/cert.pem will contain the system wide set
// of root CAs in a format suitable for this function.
func (s *CertPool) AppendCertsFromPEM(pemCerts []byte) (ok bool) {
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		cert, err := ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}

		s.AddCert(cert)
		ok = true
	}

	return
}

// Subjects returns a list of the DER-encoded subjects of
// all of the certificates in the pool.
func (s *CertPool) Subjects() [][]byte {
	res := make([][]byte, len(s.certs))
	for i, c := range s.certs {
		res[i] = c.RawSubject
	}
	return res
}
