package cabf_br

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
	"regexp"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type DNSNameProperCharacters struct {
	CompiledExpression *regexp.Regexp
}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_dnsname_bad_character_in_label",
		Description:   "Characters in labels of DNSNames MUST be alphanumeric, - , _ or *",
		Citation:      "BRs: 7.1.4.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          &DNSNameProperCharacters{},
	})
}

func (l *DNSNameProperCharacters) Initialize() error {
	const dnsNameRegexp = `^(\*\.)?(\?\.)*([A-Za-z0-9*_-]+\.)*[A-Za-z0-9*_-]*$`
	var err error
	l.CompiledExpression, err = regexp.Compile(dnsNameRegexp)

	return err
}

func (l *DNSNameProperCharacters) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubscriberCert(c) && util.DNSNamesExist(c)
}

func (l *DNSNameProperCharacters) Execute(c *x509.Certificate) *lint.LintResult {
	if c.Subject.CommonName != "" && !util.CommonNameIsIP(c) {
		if !l.CompiledExpression.MatchString(c.Subject.CommonName) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	for _, dns := range c.DNSNames {
		if !l.CompiledExpression.MatchString(dns) {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
