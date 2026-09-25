package rfc

/*
 * ZLint Copyright 2023 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

import (
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type CertContainsUniqueIdentifier struct{}

/************************************************
 These fields MUST only appear if the version is 2 or 3 (Section 4.1.2.1).
 These fields MUST NOT appear if the version is 1. The subject and issuer
 unique identifiers are present in the certificate to handle the possibility
 of reuse of subject and/or issuer names over time. This profile RECOMMENDS
 that names not be reused for different entities and that Internet certificates
 not make use of unique identifiers. CAs conforming to this profile MUST NOT
 generate certificates with unique identifiers. Applications conforming to
 this profile SHOULD be capable of parsing certificates that include unique
 identifiers, but there are no processing requirements associated with the
 unique identifiers.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_cert_contains_unique_identifier",
		Description:   "CAs MUST NOT generate certificate with unique identifiers",
		Source:        lint.RFC5280,
		Citation:      "RFC 5280: 4.1.2.8",
		EffectiveDate: util.RFC5280Date,
		Lint:          NewCertContainsUniqueIdentifier,
	})
}

func NewCertContainsUniqueIdentifier() lint.LintInterface {
	return &CertContainsUniqueIdentifier{}
}

func (l *CertContainsUniqueIdentifier) CheckApplies(cert *x509.Certificate) bool {
	return true
}

func (l *CertContainsUniqueIdentifier) Execute(cert *x509.Certificate) *lint.LintResult {
	if cert.IssuerUniqueId.Bytes == nil && cert.SubjectUniqueId.Bytes == nil {
		return &lint.LintResult{Status: lint.Pass}
	}
	return &lint.LintResult{Status: lint.Error}
}
