package rfc

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

type DNSNameUnderscoreInSLD struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_rfc_dnsname_underscore_in_sld",
		Description:   "DNSName MUST NOT contain underscore characters",
		Citation:      "RFC5280: 4.2.1.6",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          NewDNSNameUnderscoreInSLD,
	})
}

func NewDNSNameUnderscoreInSLD() lint.LintInterface {
	return &DNSNameUnderscoreInSLD{}
}

func (l *DNSNameUnderscoreInSLD) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c)
}

func (l *DNSNameUnderscoreInSLD) Execute(c *x509.Certificate) *lint.LintResult {
	parsedSANDNSNames := c.GetParsedDNSNames(false)
	for i := range c.GetParsedDNSNames(false) {
		if parsedSANDNSNames[i].ParseError != nil {
			return &lint.LintResult{Status: lint.NA}
		}
		if strings.Contains(parsedSANDNSNames[i].ParsedDomain.SLD, "_") {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
