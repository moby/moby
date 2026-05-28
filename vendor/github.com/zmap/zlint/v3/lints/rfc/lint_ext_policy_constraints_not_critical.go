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
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type policyConstraintsCritical struct{}

/************************************************
RFC 5280: 4.2.1.11
Conforming CAs MUST mark this extension as critical.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_policy_constraints_not_critical",
		Description:   "Conforming CAs MUST mark the policy constraints extension as critical",
		Citation:      "RFC 5280: 4.2.1.11",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          NewPolicyConstraintsCritical,
	})
}

func NewPolicyConstraintsCritical() lint.LintInterface {
	return &policyConstraintsCritical{}
}

func (l *policyConstraintsCritical) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.PolicyConstOID)
}

func (l *policyConstraintsCritical) Execute(c *x509.Certificate) *lint.LintResult {
	pc := util.GetExtFromCert(c, util.PolicyConstOID)
	if !pc.Critical {
		return &lint.LintResult{Status: lint.Error}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
