package mozilla

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

type rsaPssInSPKI struct{}

/************************************************
https://www.mozilla.org/en-US/about/governance/policies/security-group/certs/policy/

Section 5.1.1 RSA

CAs MUST NOT use the id-RSASSA-PSS OID (1.2.840.113549.1.1.10) within a SubjectPublicKeyInfo to represent a RSA key.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_mp_rsassa-pss_in_spki",
		Description:   "CAs MUST NOT use the id-RSASSA-PSS OID (1.2.840.113549.1.1.10) within a SubjectPublicKeyInfo to represent a RSA key.",
		Citation:      "Mozilla Root Store Policy / Section 5.1.1",
		Source:        lint.MozillaRootStorePolicy,
		EffectiveDate: util.MozillaPolicy27Date,
		Lint:          &rsaPssInSPKI{},
	})
}

func (l *rsaPssInSPKI) Initialize() error {
	return nil
}

func (l *rsaPssInSPKI) CheckApplies(c *x509.Certificate) bool {
	// always check, no certificate is allowed to contain the PSS OID in public key
	return true
}

func (l *rsaPssInSPKI) Execute(c *x509.Certificate) *lint.LintResult {
	publicKeyOID, err := util.GetPublicKeyOID(c)
	if err != nil {
		return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("error reading OID in certificate SubjectPublicKeyInfo: %v", err)}
	}

	if publicKeyOID.Equal(util.OidRSASSAPSS) {
		return &lint.LintResult{Status: lint.Error, Details: "id-RSASSA-PSS OID found in certificate SubjectPublicKeyInfo"}
	}

	return &lint.LintResult{Status: lint.Pass}
}
