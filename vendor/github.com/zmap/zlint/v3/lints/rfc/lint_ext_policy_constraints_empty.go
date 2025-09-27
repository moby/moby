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
	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type policyConstraintsContents struct{}

/*************************************************************************
RFC 5280: 4.2.1.11
Conforming CAs MUST NOT issue certificates where policy constraints
   is an empty sequence.  That is, either the inhibitPolicyMapping field
   or the requireExplicitPolicy field MUST be present.  The behavior of
   clients that encounter an empty policy constraints field is not
   addressed in this profile.
*************************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_policy_constraints_empty",
		Description:   "Conforming CAs MUST NOT issue certificates where policy constraints is an empty sequence. That is, either the inhibitPolicyMapping field or the requireExplicityPolicy field MUST be present",
		Citation:      "RFC 5280: 4.2.1.11",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewPolicyConstraintsContents,
	})
}

func NewPolicyConstraintsContents() lint.LintInterface {
	return &policyConstraintsContents{}
}

func (l *policyConstraintsContents) CheckApplies(c *x509.Certificate) bool {
	if !(util.IsExtInCert(c, util.PolicyConstOID)) {
		return false
	}
	pc := util.GetExtFromCert(c, util.PolicyConstOID)
	var seq asn1.RawValue
	rest, err := asn1.Unmarshal(pc.Value, &seq) //only one sequence, so rest should be empty
	if err != nil || len(rest) != 0 || seq.Tag != 16 || seq.Class != 0 || !seq.IsCompound {
		return false
	}
	return true
}

func (l *policyConstraintsContents) Execute(c *x509.Certificate) *lint.LintResult {
	pc := util.GetExtFromCert(c, util.PolicyConstOID)
	var seq asn1.RawValue
	_, err := asn1.Unmarshal(pc.Value, &seq) //only one sequence, so rest should be empty
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal}
	}
	if len(seq.Bytes) == 0 {
		return &lint.LintResult{Status: lint.Error}
	}

	return &lint.LintResult{Status: lint.Pass}
}
