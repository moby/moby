package cabf_br

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

type subCertPostalCodeMustNotAppear struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_sub_cert_postal_code_must_not_appear",
		Description:   "Subscriber Certificate: subject:postalCode MUST NOT appear if the subject:organizationName field, subject:givenName field, or subject:surname fields are absent.",
		Citation:      "BRs: 7.1.4.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABGivenNameDate,
		Lint:          &subCertPostalCodeMustNotAppear{},
	})
}

func (l *subCertPostalCodeMustNotAppear) Initialize() error {
	return nil
}

func (l *subCertPostalCodeMustNotAppear) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c)
}

func (l *subCertPostalCodeMustNotAppear) Execute(c *x509.Certificate) *lint.LintResult {
	// BR 7.1.4.2.2 uses "or" and "and" interchangeably when they mean "and".
	if len(c.Subject.Organization) == 0 && len(c.Subject.GivenName) == 0 && len(c.Subject.Surname) == 0 {
		if len(c.Subject.PostalCode) > 0 {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
