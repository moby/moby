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

type crlHasNextUpdate struct{}

/************************************************
RFC 5280: 5.1.2.5
Conforming CRL issuers MUST include the nextUpdate field in all CRLs.
************************************************/

func init() {
	lint.RegisterRevocationListLint(&lint.RevocationListLint{
		LintMetadata: lint.LintMetadata{
			Name:          "e_crl_has_next_update",
			Description:   "Conforming CRL issuers MUST include the nextUpdate field in all CRLs.",
			Citation:      "RFC 5280: 5.1.2.5",
			Source:        lint.RFC5280,
			EffectiveDate: util.RFC5280Date,
		},
		Lint: NewCrlHasNextUpdate,
	})
}

func NewCrlHasNextUpdate() lint.RevocationListLintInterface {
	return &crlHasNextUpdate{}
}

func (l *crlHasNextUpdate) CheckApplies(c *x509.RevocationList) bool {
	return true
}

func (l *crlHasNextUpdate) Execute(c *x509.RevocationList) *lint.LintResult {
	if c.NextUpdate.IsZero() {
		return &lint.LintResult{Status: lint.Error, Details: "Confoming CRL issuers MUST include the nextUpdate field in all CRLs."}
	}
	return &lint.LintResult{Status: lint.Pass}
}
