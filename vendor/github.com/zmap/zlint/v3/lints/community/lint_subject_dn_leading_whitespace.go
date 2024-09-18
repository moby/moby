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
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type SubjectDNLeadingSpace struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_subject_dn_leading_whitespace",
		Description:   "AttributeValue in subject RelativeDistinguishedName sequence SHOULD NOT have leading whitespace",
		Citation:      "lint.AWSLabs certlint",
		Source:        lint.Community,
		EffectiveDate: util.ZeroDate,
		Lint:          &SubjectDNLeadingSpace{},
	})
}

func (l *SubjectDNLeadingSpace) Initialize() error {
	return nil
}

func (l *SubjectDNLeadingSpace) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *SubjectDNLeadingSpace) Execute(c *x509.Certificate) *lint.LintResult {
	leading, _, err := util.CheckRDNSequenceWhiteSpace(c.RawSubject)
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal}
	}
	if leading {
		return &lint.LintResult{Status: lint.Warn}
	}
	return &lint.LintResult{Status: lint.Pass}
}
