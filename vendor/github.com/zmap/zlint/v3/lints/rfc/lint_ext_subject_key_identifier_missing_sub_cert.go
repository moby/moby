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

type subjectKeyIdMissingSubscriber struct{}

/**********************************************************************
   To facilitate certification path construction, this extension MUST
   appear in all conforming CA certificates, that is, all certificates
   including the basic constraints extension (Section 4.2.1.9) where the
   value of cA is TRUE.  In conforming CA certificates, the value of the
   subject key identifier MUST be the value placed in the key identifier
   field of the authority key identifier extension (Section 4.2.1.1) of
   certificates issued by the subject of this certificate.  Applications
   are not required to verify that key identifiers match when performing
   certification path validation.
   ...
   For end entity certificates, the subject key identifier extension provides
   a means for identifying certificates containing the particular public key
   used in an application. Where an end entity has obtained multiple certificates,
   especially from multiple CAs, the subject key identifier provides a means to
   quickly identify the set of certificates containing a particular public key.
   To assist applications in identifying the appropriate end entity certificate,
   this extension SHOULD be included in all end entity certificates.
**********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_ext_subject_key_identifier_missing_sub_cert",
		Description:   "Sub certificates SHOULD include Subject Key Identifier in end entity certs",
		Citation:      "RFC 5280: 4.2 & 4.2.1.2",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewSubjectKeyIdMissingSubscriber,
	})
}

func NewSubjectKeyIdMissingSubscriber() lint.LintInterface {
	return &subjectKeyIdMissingSubscriber{}
}

func (l *subjectKeyIdMissingSubscriber) CheckApplies(cert *x509.Certificate) bool {
	return !util.IsCACert(cert)
}

func (l *subjectKeyIdMissingSubscriber) Execute(cert *x509.Certificate) *lint.LintResult {
	if util.IsExtInCert(cert, util.SubjectKeyIdentityOID) {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Warn}
	}
}
