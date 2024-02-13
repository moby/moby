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
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type illegalChar struct{}

/**********************************************************************************************************************
BRs: 7.1.4.2.2
Other Subject Attributes
With the exception of the subject:organizationalUnitName (OU) attribute, optional attributes, when present within
the subject field, MUST contain information that has been verified by the CA. Metadata such as ‘.’, ‘-‘, and ‘ ‘ (i.e.
space) characters, and/or any other indication that the value is absent, incomplete, or not applicable, SHALL NOT
be used.
**********************************************************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_contains_noninformational_value",
		Description:   "Subject name fields must not contain '.','-',' ' or any other indication that the field has been omitted",
		Citation:      "BRs: 7.1.4.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          &illegalChar{},
	})
}

func (l *illegalChar) Initialize() error {
	return nil
}

func (l *illegalChar) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *illegalChar) Execute(c *x509.Certificate) *lint.LintResult {
	for _, j := range c.Subject.Names {
		value, ok := j.Value.(string)
		if !ok {
			continue
		}

		if !checkAlphaNumericOrUTF8Present(value) {
			return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("found only metadata %s in subjectDN attribute %s", value, j.Type.String())}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}

// checkAlphaNumericOrUTF8Present checks if input string contains at least one occurrence of [a-Z0-9] or
// a UTF8 rune outside of ascii table
func checkAlphaNumericOrUTF8Present(input string) bool {
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r > 127 {
			return true
		}
	}

	return false
}
