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

type DNSNameWildcardOnlyInLeftlabel struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_dnsname_wildcard_only_in_left_label",
		Description:   "DNSName should not have wildcards except in the left-most label",
		Citation:      "BRs: 1.6.1, Wildcard Domain Name",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewDNSNameWildcardOnlyInLeftlabel,
	})
}

func NewDNSNameWildcardOnlyInLeftlabel() lint.LintInterface {
	return &DNSNameWildcardOnlyInLeftlabel{}
}

func (l *DNSNameWildcardOnlyInLeftlabel) CheckApplies(c *x509.Certificate) bool {
	return true
}

func wildcardNotInLeftLabel(domain string) bool {
	labels := strings.Split(domain, ".")
	if len(labels) > 1 {
		labels = labels[1:]
		for _, label := range labels {
			if strings.Contains(label, "*") {
				return true
			}
		}
	}
	return false
}

func (l *DNSNameWildcardOnlyInLeftlabel) Execute(c *x509.Certificate) *lint.LintResult {
	if wildcardNotInLeftLabel(c.Subject.CommonName) {
		return &lint.LintResult{Status: lint.Error}
	}
	for _, dns := range c.DNSNames {
		if wildcardNotInLeftLabel(dns) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
