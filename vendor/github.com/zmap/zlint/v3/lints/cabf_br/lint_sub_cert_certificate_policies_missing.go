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

type subCertPolicy struct{}

/******************************************************************************
BRs: 7.1.2.3
certificatePolicies
This extension MUST be present and SHOULD NOT be marked critical.
******************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_sub_cert_certificate_policies_missing",
		Description:   "Subscriber Certificate: certificatePolicies MUST be present and SHOULD NOT be marked critical.",
		Citation:      "BRs: 7.1.2.3",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewSubCertPolicy,
	})
}

func NewSubCertPolicy() lint.LintInterface {
	return &subCertPolicy{}
}

func (l *subCertPolicy) CheckApplies(c *x509.Certificate) bool {
	return !util.IsCACert(c)
}

func (l *subCertPolicy) Execute(c *x509.Certificate) *lint.LintResult {
	if util.IsExtInCert(c, util.CertPolicyOID) {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
