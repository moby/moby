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

type DNSNameEmptyLabel struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_rfc_dnsname_empty_label",
		Description:   "DNSNames should not have an empty label.",
		Citation:      "RFC5280: 4.2.1.6",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          NewDNSNameEmptyLabel,
	})
}

func NewDNSNameEmptyLabel() lint.LintInterface {
	return &DNSNameEmptyLabel{}
}

func (l *DNSNameEmptyLabel) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c)
}

func domainHasEmptyLabel(domain string) bool {
	labels := strings.Split(domain, ".")
	for _, elem := range labels {
		if elem == "" {
			return true
		}
	}
	return false
}

func (l *DNSNameEmptyLabel) Execute(c *x509.Certificate) *lint.LintResult {
	for _, dns := range c.DNSNames {
		if domainHasEmptyLabel(dns) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
