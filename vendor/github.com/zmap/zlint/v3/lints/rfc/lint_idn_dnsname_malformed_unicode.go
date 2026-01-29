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

type IDNMalformedUnicode struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_international_dns_name_not_unicode",
		Description:   "Internationalized DNSNames punycode not valid Unicode",
		Citation:      "RFC 3490",
		EffectiveDate: util.RFC3490Date,
		Source:        lint.RFC5280,
		Lint:          NewIDNMalformedUnicode,
	})
}

func NewIDNMalformedUnicode() lint.LintInterface {
	return &IDNMalformedUnicode{}
}

func (l *IDNMalformedUnicode) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.SubjectAlternateNameOID)
}

func (l *IDNMalformedUnicode) Execute(c *x509.Certificate) *lint.LintResult {
	for _, dns := range c.DNSNames {
		labels := strings.Split(dns, ".")
		for _, label := range labels {
			if util.HasXNLabelPrefix(label) {
				_, err := util.IdnaToUnicode(label)
				if err != nil {
					return &lint.LintResult{Status: lint.Error}
				}
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
