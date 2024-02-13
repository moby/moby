package rfc

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
	"unicode/utf8"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type SubjectDNSerialNumberMaxLength struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_dn_serial_number_max_length",
		Description:   "The 'Serial Number' field of the subject MUST be less than 65 characters",
		Citation:      "RFC 5280: Appendix A",
		Source:        lint.RFC5280,
		EffectiveDate: util.ZeroDate,
		Lint:          &SubjectDNSerialNumberMaxLength{},
	})
}

func (l *SubjectDNSerialNumberMaxLength) Initialize() error {
	return nil
}

func (l *SubjectDNSerialNumberMaxLength) CheckApplies(c *x509.Certificate) bool {
	return len(c.Subject.SerialNumber) > 0
}

func (l *SubjectDNSerialNumberMaxLength) Execute(c *x509.Certificate) *lint.LintResult {
	if utf8.RuneCountInString(c.Subject.SerialNumber) > 64 {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
