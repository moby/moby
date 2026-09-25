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

	"github.com/zmap/zlint/v3/util"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
)

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_underscore_not_permissible_in_dnsname",
		Description:   "DNSNames MUST NOT contain underscore characters",
		Citation:      "BR 7.1.4.2.1",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABFBRs_1_6_2_UnderscorePermissibilitySunsetDate,
		Lint:          func() lint.LintInterface { return &UnderscoreNotPermissibleInDNSName{} },
	})
}

type UnderscoreNotPermissibleInDNSName struct{}

func (l *UnderscoreNotPermissibleInDNSName) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c)
}

func (l *UnderscoreNotPermissibleInDNSName) Execute(c *x509.Certificate) *lint.LintResult {
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
