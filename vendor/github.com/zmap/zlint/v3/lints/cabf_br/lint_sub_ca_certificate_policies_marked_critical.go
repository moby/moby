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

type subCACertPolicyCrit struct{}

/************************************************
BRs: 7.1.2.2a certificatePolicies
This extension MUST be present and SHOULD NOT be marked critical.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_sub_ca_certificate_policies_marked_critical",
		Description:   "Subordinate CA certificates certificatePolicies extension should not be marked as critical",
		Citation:      "BRs: 7.1.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewSubCACertPolicyCrit,
	})
}

func NewSubCACertPolicyCrit() lint.LintInterface {
	return &subCACertPolicyCrit{}
}

func (l *subCACertPolicyCrit) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubCA(c) && util.IsExtInCert(c, util.CertPolicyOID)
}

func (l *subCACertPolicyCrit) Execute(c *x509.Certificate) *lint.LintResult {
	if e := util.GetExtFromCert(c, util.CertPolicyOID); e.Critical {
		return &lint.LintResult{Status: lint.Warn}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}

}
