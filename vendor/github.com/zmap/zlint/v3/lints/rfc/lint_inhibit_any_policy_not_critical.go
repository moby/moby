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

type InhibitAnyPolicyNotCritical struct{}

/************************************************
4.2.1.14.  Inhibit anyPolicy
   The inhibit anyPolicy extension can be used in certificates issued to CAs.
   The inhibit anyPolicy extension indicates that the special anyPolicy OID,
   with the value { 2 5 29 32 0 }, is not considered an explicit match for other
   certificate policies except when it appears in an intermediate self-issued
   CA certificate. The value indicates the number of additional non-self-issued
   certificates that may appear in the path before anyPolicy is no longer permitted.
   For example, a value of one indicates that anyPolicy may be processed in
   certificates issued by the subject of this certificate, but not in additional
   certificates in the path.

   Conforming CAs MUST mark this extension as critical.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_inhibit_any_policy_not_critical",
		Description:   "CAs MUST mark the inhibitAnyPolicy extension as critical",
		Citation:      "RFC 5280: 4.2.1.14",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC3280Date,
		Lint:          NewInhibitAnyPolicyNotCritical,
	})
}

func NewInhibitAnyPolicyNotCritical() lint.LintInterface {
	return &InhibitAnyPolicyNotCritical{}
}

func (l *InhibitAnyPolicyNotCritical) CheckApplies(cert *x509.Certificate) bool {
	return util.IsExtInCert(cert, util.InhibitAnyPolicyOID)
}

func (l *InhibitAnyPolicyNotCritical) Execute(cert *x509.Certificate) *lint.LintResult {
	if anyPol := util.GetExtFromCert(cert, util.InhibitAnyPolicyOID); !anyPol.Critical {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
