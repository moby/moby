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

type validityNegative struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_validity_time_not_positive",
		Description:   "Certificates MUST have a positive time for which they are valid",
		Citation:      "lint.AWSLabs certlint",
		Source:        lint.Community,
		EffectiveDate: util.ZeroDate,
		Lint:          &validityNegative{},
	})
}

func (l *validityNegative) Initialize() error {
	return nil
}

func (l *validityNegative) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *validityNegative) Execute(c *x509.Certificate) *lint.LintResult {
	if c.NotBefore.After(c.NotAfter) {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
