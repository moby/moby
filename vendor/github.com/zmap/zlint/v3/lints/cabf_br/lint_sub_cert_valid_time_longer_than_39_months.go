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

type subCertValidTimeLongerThan39Months struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_sub_cert_valid_time_longer_than_39_months",
		Description:   "Subscriber Certificates issued after 1 July 2016 but prior to 1 March 2018 MUST have a Validity Period no greater than 39 months.",
		Citation:      "BRs: 6.3.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.SubCert39Month,
		Lint:          NewSubCertValidTimeLongerThan39Months,
	})
}

func NewSubCertValidTimeLongerThan39Months() lint.LintInterface {
	return &subCertValidTimeLongerThan39Months{}
}

func (l *subCertValidTimeLongerThan39Months) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c)
}

func (l *subCertValidTimeLongerThan39Months) Execute(c *x509.Certificate) *lint.LintResult {
	if c.NotBefore.AddDate(0, 39, 0).Before(c.NotAfter) {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
