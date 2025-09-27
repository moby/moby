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
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_dnsname_contains_prohibited_reserved_label",
		Description:   "FQDNs MUST consist solely of Domain Labels that are P‐Labels or Non‐Reserved LDH Labels",
		Citation:      "BRs: 7.1.4.2.1",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.NoReservedDomainLabelsDate,
		Lint:          NewDNSNameContainsProhibitedReservedLabel,
	})
}

type DNSNameContainsProhibitedReservedLabel struct{}

func NewDNSNameContainsProhibitedReservedLabel() lint.LintInterface {
	return &DNSNameContainsProhibitedReservedLabel{}
}

func (l *DNSNameContainsProhibitedReservedLabel) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c)
}

func (l *DNSNameContainsProhibitedReservedLabel) Execute(c *x509.Certificate) *lint.LintResult {
	for _, dns := range c.DNSNames {
		labels := strings.Split(dns, ".")

		for _, label := range labels {
			if util.HasReservedLabelPrefix(label) && !util.HasXNLabelPrefix(label) {
				return &lint.LintResult{Status: lint.Error}
			}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
