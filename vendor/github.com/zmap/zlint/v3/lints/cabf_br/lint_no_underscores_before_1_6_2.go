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

package cabf_br

import (
	"fmt"
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:            "e_no_underscores_before_1_6_2",
		Description:     "Before explicitly stating as such in CABF 1.6.2, the stance of RFC5280 is adopted that DNSNames MUST NOT contain an underscore character.",
		Citation:        "BR 7.1.4.2.1",
		Source:          lint.CABFBaselineRequirements,
		EffectiveDate:   util.ZeroDate,
		IneffectiveDate: util.CABFBRs_1_6_2_Date,
		Lint:            func() lint.LintInterface { return &NoUnderscoreBefore1_6_2{} },
	})
}

type NoUnderscoreBefore1_6_2 struct{}

func NewNoUnderscoreBefore1_6_2() lint.LintInterface {
	return &NoUnderscoreBefore1_6_2{}
}

func (l *NoUnderscoreBefore1_6_2) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c)
}

func (l *NoUnderscoreBefore1_6_2) Execute(c *x509.Certificate) *lint.LintResult {
	for _, dns := range c.DNSNames {
		if strings.Contains(dns, "_") {
			return &lint.LintResult{
				Status:  lint.Error,
				Details: fmt.Sprintf("The DNS name '%s' contains an underscore (_) character", dns),
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
