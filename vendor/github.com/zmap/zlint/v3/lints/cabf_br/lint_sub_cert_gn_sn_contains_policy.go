package cabf_br

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

type subCertSubjectGnOrSnContainsPolicy struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_sub_cert_given_name_surname_contains_correct_policy",
		Description:   "Subscriber Certificate: A certificate containing a subject:givenName field or subject:surname field MUST contain the (2.23.140.1.2.3) certPolicy OID.",
		Citation:      "BRs: 7.1.4.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABGivenNameDate,
		Lint:          NewSubCertSubjectGnOrSnContainsPolicy,
	})
}

func NewSubCertSubjectGnOrSnContainsPolicy() lint.LintInterface {
	return &subCertSubjectGnOrSnContainsPolicy{}
}

func (l *subCertSubjectGnOrSnContainsPolicy) CheckApplies(c *x509.Certificate) bool {
	//Check if GivenName or Surname fields are filled out
	return util.IsSubscriberCert(c) && (len(c.Subject.GivenName) != 0 || len(c.Subject.Surname) != 0)
}

func (l *subCertSubjectGnOrSnContainsPolicy) Execute(c *x509.Certificate) *lint.LintResult {
	for _, policyIds := range c.PolicyIdentifiers {
		if policyIds.Equal(util.BRIndividualValidatedOID) {
			return &lint.LintResult{Status: lint.Pass}
		}
	}
	return &lint.LintResult{Status: lint.Error}
}
