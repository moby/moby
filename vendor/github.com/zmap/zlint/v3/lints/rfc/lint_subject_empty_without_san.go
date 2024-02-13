package rfc

/*
 * ZLint Copyright 2021 Regents of the University of Michigan
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

type emptyWithoutSAN struct{}

/*************************************************************************
RFC 5280: 4.2 & 4.2.1.6
Further, if the only subject identity included in the certificate is
an alternative name form (e.g., an electronic mail address), then the
subject distinguished name MUST be empty (an empty sequence), and the
subjectAltName extension MUST be present.  If the subject field
contains an empty sequence, then the issuing CA MUST include a
subjectAltName extension that is marked as critical.  When including
the subjectAltName extension in a certificate that has a non-empty
subject distinguished name, conforming CAs SHOULD mark the
subjectAltName extension as non-critical.
*************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_empty_without_san",
		Description:   "CAs MUST support subject alternative name if the subject field is an empty sequence",
		Citation:      "RFC 5280: 4.2 & 4.2.1.6",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          &emptyWithoutSAN{},
	})
}

func (l *emptyWithoutSAN) Initialize() error {
	return nil
}

func (l *emptyWithoutSAN) CheckApplies(cert *x509.Certificate) bool {
	return true
}

func (l *emptyWithoutSAN) Execute(cert *x509.Certificate) *lint.LintResult {
	if subjectIsEmpty(cert) && !util.IsExtInCert(cert, util.SubjectAlternateNameOID) {
		return &lint.LintResult{Status: lint.Error}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}

func subjectIsEmpty(cert *x509.Certificate) bool {
	return len(cert.Subject.Names) == 0
}
