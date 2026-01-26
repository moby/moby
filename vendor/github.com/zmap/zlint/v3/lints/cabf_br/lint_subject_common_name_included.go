package cabf_br

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
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type commonNames struct{}

/***************************************************************
BRs: 7.1.4.2.2
Required/Optional: Deprecated (Discouraged, but not prohibited)
***************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "n_subject_common_name_included",
		Description:   "Subscriber Certificate: commonName is deprecated.",
		Citation:      "BRs: 7.1.4.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABEffectiveDate,
		Lint:          NewCommonNames,
	})
}

func NewCommonNames() lint.LintInterface {
	return &commonNames{}
}

func (l *commonNames) CheckApplies(c *x509.Certificate) bool {
	return !util.IsCACert(c)
}

func (l *commonNames) Execute(c *x509.Certificate) *lint.LintResult {
	if c.Subject.CommonName == "" {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Notice}
	}
}
