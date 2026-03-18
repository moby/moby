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

type certPolicyConflictsWithProvince struct{}

/************************************************
BRs: 7.1.6.4
Certificate Policy Identifier: 2.23.140.1.2.1
If the Certificate complies with these requirements and lacks Subject identity information that
has been verified in accordance with Section 3.2.2.1 or Section 3.2.3.
Such Certificates MUST NOT include organizationName, givenName, surname,
streetAddress, localityName, stateOrProvinceName, or postalCode in the Subject
field.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_cab_dv_conflicts_with_province",
		Description:   "If certificate policy 2.23.140.1.2.1 (CA/B BR domain validated) is included, stateOrProvinceName MUST NOT be included in subject",
		Citation:      "BRs: 7.1.6.4",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewCertPolicyConflictsWithProvince,
	})
}

func NewCertPolicyConflictsWithProvince() lint.LintInterface {
	return &certPolicyConflictsWithProvince{}
}

func (l *certPolicyConflictsWithProvince) CheckApplies(cert *x509.Certificate) bool {
	return util.SliceContainsOID(cert.PolicyIdentifiers, util.BRDomainValidatedOID) && !util.IsCACert(cert)
}

func (l *certPolicyConflictsWithProvince) Execute(cert *x509.Certificate) *lint.LintResult {
	var out lint.LintResult
	if util.TypeInName(&cert.Subject, util.StateOrProvinceNameOID) {
		out.Status = lint.Error
	} else {
		out.Status = lint.Pass
	}
	return &out
}
