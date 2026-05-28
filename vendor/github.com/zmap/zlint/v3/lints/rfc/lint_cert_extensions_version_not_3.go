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

type CertExtensionsVersonNot3 struct{}

/************************************************
4.1.2.1.  Version
   This field describes the version of the encoded certificate. When
   extensions are used, as expected in this profile, version MUST be 3
   (value is 2). If no extensions are present, but a UniqueIdentifier
   is present, the version SHOULD be 2 (value is 1); however, the version
   MAY be 3.  If only basic fields are present, the version SHOULD be 1
   (the value is omitted from the certificate as the default value);
   however, the version MAY be 2 or 3.

   Implementations SHOULD be prepared to accept any version certificate.
   At a minimum, conforming implementations MUST recognize version 3 certificates.
4.1.2.9.  Extensions
   This field MUST only appear if the version is 3 (Section 4.1.2.1).
   If present, this field is a SEQUENCE of one or more certificate
   extensions. The format and content of certificate extensions in the
   Internet PKI are defined in Section 4.2.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_cert_extensions_version_not_3",
		Description:   "The extensions field MUST only appear in version 3 certificates",
		Citation:      "RFC 5280: 4.1.2.9",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewCertExtensionsVersonNot3,
	})
}

func NewCertExtensionsVersonNot3() lint.LintInterface {
	return &CertExtensionsVersonNot3{}
}

func (l *CertExtensionsVersonNot3) CheckApplies(cert *x509.Certificate) bool {
	return true
}

func (l *CertExtensionsVersonNot3) Execute(cert *x509.Certificate) *lint.LintResult {
	if cert.Version != 3 && len(cert.Extensions) != 0 {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
