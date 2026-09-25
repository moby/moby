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

type sigAlgTestsSHA1 struct{}

/**************************************************************************************************
BRs: 7.1.3
SHA‐1 MAY be used with RSA keys in accordance with the criteria defined in Section 7.1.3.
**************************************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_sub_cert_or_sub_ca_using_sha1",
		Description:   "CAs MUST NOT issue any new Subscriber certificates or Subordinate CA certificates using SHA-1 after 1 January 2016",
		Citation:      "BRs: 7.1.3",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.NO_SHA1,
		Lint:          NewSigAlgTestsSHA1,
	})
}

func NewSigAlgTestsSHA1() lint.LintInterface {
	return &sigAlgTestsSHA1{}
}

func (l *sigAlgTestsSHA1) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *sigAlgTestsSHA1) Execute(c *x509.Certificate) *lint.LintResult {
	if c.SignatureAlgorithm == x509.SHA1WithRSA || c.SignatureAlgorithm == x509.DSAWithSHA1 || c.SignatureAlgorithm == x509.ECDSAWithSHA1 {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
