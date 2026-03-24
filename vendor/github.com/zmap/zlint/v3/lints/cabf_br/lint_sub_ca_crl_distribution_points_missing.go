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

type subCACRLDistMissing struct{}

/************************************************
BRs: 7.1.2.2b cRLDistributionPoints
This extension MUST be present and MUST NOT be marked critical.
It MUST contain the HTTP URL of the CA’s CRL service.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_sub_ca_crl_distribution_points_missing",
		Description:   "Subordinate CA Certificate: cRLDistributionPoints MUST be present and MUST NOT be marked critical.",
		Citation:      "BRs: 7.1.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewSubCACRLDistMissing,
	})
}

func NewSubCACRLDistMissing() lint.LintInterface {
	return &subCACRLDistMissing{}
}

func (l *subCACRLDistMissing) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubCA(c)
}

func (l *subCACRLDistMissing) Execute(c *x509.Certificate) *lint.LintResult {
	if util.IsExtInCert(c, util.CrlDistOID) {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
