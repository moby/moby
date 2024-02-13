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

type ExtCertPolicyDuplicate struct{}

/************************************************
  The certificate policies extension contains a sequence of one or more
  policy information terms, each of which consists of an object identifier
  (OID) and optional qualifiers. Optional qualifiers, which MAY be present,
  are not expected to change the definition of the policy. A certificate
  policy OID MUST NOT appear more than once in a certificate policies extension.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_cert_policy_duplicate",
		Description:   "A certificate policy OID must not appear more than once in the extension",
		Citation:      "RFC 5280: 4.2.1.4",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          &ExtCertPolicyDuplicate{},
	})
}

func (l *ExtCertPolicyDuplicate) Initialize() error {
	return nil
}

func (l *ExtCertPolicyDuplicate) CheckApplies(cert *x509.Certificate) bool {
	return util.IsExtInCert(cert, util.CertPolicyOID)
}

func (l *ExtCertPolicyDuplicate) Execute(cert *x509.Certificate) *lint.LintResult {
	// O(n^2) is not terrible here because n is small
	for i := 0; i < len(cert.PolicyIdentifiers); i++ {
		for j := i + 1; j < len(cert.PolicyIdentifiers); j++ {
			if i != j && cert.PolicyIdentifiers[i].Equal(cert.PolicyIdentifiers[j]) {
				// Any one duplicate fails the test, so return here
				return &lint.LintResult{Status: lint.Error}
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
