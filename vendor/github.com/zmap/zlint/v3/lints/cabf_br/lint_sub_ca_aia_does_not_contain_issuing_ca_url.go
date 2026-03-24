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
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type subCaIssuerUrl struct{}

/***********************************************
BRs: 7.1.2.2c
This extension SHOULD be present. It MUST NOT be marked critical.
It SHOULD contain the HTTP URL of the Issuing CA’s certificate (accessMethod =
1.3.6.1.5.5.7.48.2). It MAY contain the HTTP URL of the Issuing CA’s OCSP responder
(accessMethod = 1.3.6.1.5.5.7.48.1).
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_sub_ca_aia_does_not_contain_issuing_ca_url",
		Description:   "Subordinate CA Certificate: authorityInformationAccess SHOULD also contain the HTTP URL of the Issuing CA's certificate.",
		Citation:      "BRs: 7.1.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewSubCaIssuerUrl,
	})
}

func NewSubCaIssuerUrl() lint.LintInterface {
	return &subCaIssuerUrl{}
}

func (l *subCaIssuerUrl) CheckApplies(c *x509.Certificate) bool {
	return util.IsCACert(c) && !util.IsRootCA(c)
}

func (l *subCaIssuerUrl) Execute(c *x509.Certificate) *lint.LintResult {
	for _, url := range c.IssuingCertificateURL {
		if strings.HasPrefix(url, "http://") {
			return &lint.LintResult{Status: lint.Pass}
		}
	}
	return &lint.LintResult{Status: lint.Warn}
}
