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

type subExtKeyUsage struct{}

/*******************************************************************************************************
BRs: 7.1.2.3
extKeyUsage (required)
Either the value id-kp-serverAuth [RFC5280] or id-kp-clientAuth [RFC5280] or
both values MUST be present. id-kp-emailProtection [RFC5280] MAY be present.
Other values SHOULD NOT be present. The value anyExtendedKeyUsage MUST NOT be
present.
*******************************************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_sub_cert_eku_missing",
		Description:   "Subscriber certificates MUST have the extended key usage extension present",
		Citation:      "BRs: 7.1.2.3",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          &subExtKeyUsage{},
	})
}

func (l *subExtKeyUsage) Initialize() error {
	return nil
}

func (l *subExtKeyUsage) CheckApplies(c *x509.Certificate) bool {
	return !util.IsCACert(c)
}

func (l *subExtKeyUsage) Execute(c *x509.Certificate) *lint.LintResult {
	if util.IsExtInCert(c, util.EkuSynOid) {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Error}
	}
}
