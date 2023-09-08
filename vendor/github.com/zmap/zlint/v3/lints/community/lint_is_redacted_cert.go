package community

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
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type DNSNameRedacted struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "n_contains_redacted_dnsname",
		Description:   "Some precerts are redacted and of the form ?.?.a.com or *.?.a.com",
		Source:        lint.Community,
		Citation:      "IETF Draft: https://tools.ietf.org/id/draft-strad-trans-redaction-00.html",
		EffectiveDate: util.ZeroDate,
		Lint:          &DNSNameRedacted{},
	})
}

func (l *DNSNameRedacted) Initialize() error {
	return nil
}

func (l *DNSNameRedacted) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c)
}

func isRedactedCertificate(domain string) bool {
	domain = util.RemovePrependedWildcard(domain)
	return strings.HasPrefix(domain, "?.")
}

func (l *DNSNameRedacted) Execute(c *x509.Certificate) *lint.LintResult {
	if c.Subject.CommonName != "" {
		if isRedactedCertificate(c.Subject.CommonName) {
			return &lint.LintResult{Status: lint.Notice}
		}
	}
	for _, domain := range c.DNSNames {
		if isRedactedCertificate(domain) {
			return &lint.LintResult{Status: lint.Notice}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
