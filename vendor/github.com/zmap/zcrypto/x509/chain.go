// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"bytes"
	"strings"
)

// CertificateChain is a slice of certificates. The 0'th element is the leaf,
// and the last element is a root. Successive elements have a child-parent
// relationship.
type CertificateChain []*Certificate

// Range runs a function on each element of chain. It can modify each
// certificate in place.
func (chain CertificateChain) Range(f func(int, *Certificate)) {
	for i, c := range chain {
		f(i, c)
	}
}

// SubjectAndKeyInChain returns true if the given SubjectAndKey is found in any
// certificate in the chain.
func (chain CertificateChain) SubjectAndKeyInChain(sk *SubjectAndKey) bool {
	for _, cert := range chain {
		if bytes.Equal(sk.RawSubject, cert.RawSubject) && bytes.Equal(sk.RawSubjectPublicKeyInfo, cert.RawSubjectPublicKeyInfo) {
			return true
		}
	}
	return false
}

// CertificateSubjectAndKeyInChain returns true if the SubjectAndKey from c is
// found in any certificate in the chain.
func (chain CertificateChain) CertificateSubjectAndKeyInChain(c *Certificate) bool {
	for _, cert := range chain {
		if bytes.Equal(c.RawSubject, cert.RawSubject) && bytes.Equal(c.RawSubjectPublicKeyInfo, cert.RawSubjectPublicKeyInfo) {
			return true
		}
	}
	return false
}

// CertificateInChain returns true if c is in the chain.
func (chain CertificateChain) CertificateInChain(c *Certificate) bool {
	for _, cert := range chain {
		if bytes.Equal(c.Raw, cert.Raw) {
			return true
		}
	}
	return false
}

func (chain CertificateChain) AppendToFreshChain(c *Certificate) CertificateChain {
	n := make([]*Certificate, len(chain)+1)
	copy(n, chain)
	n[len(chain)] = c
	return n
}

func (chain CertificateChain) chainID() string {
	var parts []string
	for _, c := range chain {
		parts = append(parts, string(c.FingerprintSHA256))
	}
	return strings.Join(parts, "")
}
