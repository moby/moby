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
	"unicode/utf8"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type subjectPostalCodeMaxLength struct{}

/************************************************
RFC 5280: A.1
	* In this Appendix, there is a list of upperbounds
	for fields in a x509 Certificate. *
	ub-postal-code-length INTEGER ::= 16

************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_postal_code_max_length",
		Description:   "The 'PostalCode' field of the subject MUST be less than 17 characters",
		Citation:      "RFC 5280: A.1",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewSubjectPostalCodeMaxLength,
	})
}

func NewSubjectPostalCodeMaxLength() lint.LintInterface {
	return &subjectPostalCodeMaxLength{}
}

func (l *subjectPostalCodeMaxLength) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *subjectPostalCodeMaxLength) Execute(c *x509.Certificate) *lint.LintResult {
	for _, j := range c.Subject.PostalCode {
		if utf8.RuneCountInString(j) > 16 {
			return &lint.LintResult{Status: lint.Error}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
