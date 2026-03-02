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

type DNSNameValidTLD struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_dnsname_not_valid_tld",
		Description:   "DNSNames must have a valid TLD.",
		Citation:      "BRs: 3.2.2.4",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewDNSNameValidTLD,
	})
}

func NewDNSNameValidTLD() lint.LintInterface {
	return &DNSNameValidTLD{}
}

func (l *DNSNameValidTLD) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c)
}

func (l *DNSNameValidTLD) Execute(c *x509.Certificate) *lint.LintResult {
	if c.Subject.CommonName != "" && !util.CommonNameIsIP(c) {
		if !util.HasValidTLD(c.Subject.CommonName, c.NotBefore) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	for _, dns := range c.DNSNames {
		if !util.HasValidTLD(dns, c.NotBefore) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
