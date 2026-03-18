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

type subjectGivenNameMaxLength struct{}

/************************************************
RFC 5280: A.1
-- Naming attributes of type X520name

id-at-givenName           AttributeType ::= { id-at 42 }

-- Naming attributes of type X520Name:
--   X520name ::= DirectoryString (SIZE (1..ub-name))
--
-- Expanded to avoid parameterized type:
X520name ::= CHOICE {
      teletexString     TeletexString   (SIZE (1..ub-name)),
      printableString   PrintableString (SIZE (1..ub-name)),
      universalString   UniversalString (SIZE (1..ub-name)),
      utf8String        UTF8String      (SIZE (1..ub-name)),
      bmpString         BMPString       (SIZE (1..ub-name)) }

--  specifications of Upper Bounds MUST be regarded as mandatory
--  from Annex B of ITU-T X.411 Reference Definition of MTS Parameter
--  Upper Bounds

-- Upper Bounds
ub-name INTEGER ::= 32768
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_given_name_max_length",
		Description:   "The 'GivenName' field of the subject MUST be less than 32769 characters",
		Citation:      "RFC 5280: A.1",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewSubjectGivenNameMaxLength,
	})
}

func NewSubjectGivenNameMaxLength() lint.LintInterface {
	return &subjectGivenNameMaxLength{}
}

func (l *subjectGivenNameMaxLength) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *subjectGivenNameMaxLength) Execute(c *x509.Certificate) *lint.LintResult {
	for _, givenName := range c.Subject.GivenName {
		characters := utf8.RuneCountInString(givenName)
		if characters > 32768 {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
