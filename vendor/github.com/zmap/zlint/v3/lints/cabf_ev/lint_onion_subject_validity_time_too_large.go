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

package cabf_ev

import (
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

const (
	// Ballot 144 specified:
	// CAs MUST NOT issue a Certificate that includes a Domain Name where .onion
	// is in the right-most label of the Domain Name with a validity period longer
	// than 15 months
	maxOnionValidityMonths = 15
)

type torValidityTooLarge struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name: "e_onion_subject_validity_time_too_large",
		Description: fmt.Sprintf(
			"certificates with .onion names can not be valid for more than %d months",
			maxOnionValidityMonths),
		Citation:      "EVGs: Appendix F",
		Source:        lint.CABFEVGuidelines,
		EffectiveDate: util.OnionOnlyEVDate,
		Lint:          NewTorValidityTooLarge,
	})
}

func NewTorValidityTooLarge() lint.LintInterface {
	return &torValidityTooLarge{}
}

// Initialize for a torValidityTooLarge linter is a NOP.

// CheckApplies returns true if the certificate is a subscriber certificate that
// contains a subject name ending in `.onion`.
func (l *torValidityTooLarge) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.CertificateSubjInTLD(c, util.OnionTLD)
}

// Execute will return an lint.Error lint.LintResult if the provided certificate has
// a validity period longer than the maximum allowed validity for a certificate
// with a .onion subject.
func (l *torValidityTooLarge) Execute(c *x509.Certificate) *lint.LintResult {
	if c.NotBefore.AddDate(0, maxOnionValidityMonths, 0).Before(c.NotAfter) {
		return &lint.LintResult{
			Status: lint.Error,
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
