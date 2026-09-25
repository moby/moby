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

package rfc

import (
	"unicode/utf8"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

/************************************************
RFC 5280: A.1
--  specifications of Upper Bounds MUST be regarded as mandatory
--  from Annex B of ITU-T X.411 Reference Definition of MTS Parameter
--  Upper Bounds
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name: "w_subject_given_name_recommended_max_length",
		Description: "X.411 (1988) describes ub-common-name-length to be 64 bytes long. As systems may have " +
			"targeted this length, for compatibility purposes it may be prudent to limit given names to this length.",
		Citation:      "ITU-T Rec. X.411 (11/1988), Annex B Reference Definition of MTS Parameter Upper Bounds",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewSubjectGivenNameRecommendedMaxLength,
	})
}

func NewSubjectGivenNameRecommendedMaxLength() lint.LintInterface {
	return &SubjectGivenNameRecommendedMaxLength{}
}

type SubjectGivenNameRecommendedMaxLength struct{}

func (l *SubjectGivenNameRecommendedMaxLength) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *SubjectGivenNameRecommendedMaxLength) Execute(c *x509.Certificate) *lint.LintResult {
	for _, givenName := range c.Subject.GivenName {
		characters := utf8.RuneCountInString(givenName)
		if characters > 64 {
			return &lint.LintResult{Status: lint.Warn}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
