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

type subjectStreetAddressMaxLength struct{}

/************************************************
ITU-T X.520 (02/2001) UpperBounds
ub-street-address INTEGER ::= 128

************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_street_address_max_length",
		Description:   "The 'StreetAddress' field of the subject MUST be less than 129 characters",
		Citation:      "ITU-T X.520 (02/2001) UpperBounds",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewSubjectStreetAddressMaxLength,
	})
}

func NewSubjectStreetAddressMaxLength() lint.LintInterface {
	return &subjectStreetAddressMaxLength{}
}

func (l *subjectStreetAddressMaxLength) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *subjectStreetAddressMaxLength) Execute(c *x509.Certificate) *lint.LintResult {
	for _, j := range c.Subject.StreetAddress {
		if utf8.RuneCountInString(j) > 128 {
			return &lint.LintResult{Status: lint.Error}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
