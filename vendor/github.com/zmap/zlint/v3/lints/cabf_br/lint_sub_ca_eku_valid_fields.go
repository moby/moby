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
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type subCAEKUValidFields struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "n_sub_ca_eku_not_technically_constrained",
		Description:   "Subordinate CA extkeyUsage, either id-kp-serverAuth or id-kp-clientAuth or both values MUST be present to be technically constrained.",
		Citation:      "BRs: 7.1.2.2",
		Source:        lint.CABFBaselineRequirements,
		EffectiveDate: util.CABV116Date,
		Lint:          &subCAEKUValidFields{},
	})
}

func (l *subCAEKUValidFields) Initialize() error {
	return nil
}

func (l *subCAEKUValidFields) CheckApplies(c *x509.Certificate) bool {
	return util.IsSubCA(c) && util.IsExtInCert(c, util.EkuSynOid)
}

func (l *subCAEKUValidFields) Execute(c *x509.Certificate) *lint.LintResult {
	validFieldsPresent := false
	for _, ekuValue := range c.ExtKeyUsage {
		if ekuValue == x509.ExtKeyUsageServerAuth ||
			ekuValue == x509.ExtKeyUsageClientAuth {
			validFieldsPresent = true
		}
	}
	if validFieldsPresent {
		return &lint.LintResult{Status: lint.Pass}
	} else {
		return &lint.LintResult{Status: lint.Notice}
	}
}
