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

package mozilla

import (
	"fmt"

	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type keyIdentifier struct {
	KeyIdentifier             asn1.RawValue `asn1:"optional,tag:0"`
	AuthorityCertIssuer       asn1.RawValue `asn1:"optional,tag:1"`
	AuthorityCertSerialNumber asn1.RawValue `asn1:"optional,tag:2"`
}

type authorityKeyIdentifierCorrect struct{}

/********************************************************************
Section 5.2 - Forbidden and Required Practices
CAs MUST NOT issue certificates that have:
- incorrect extensions (e.g., SSL certificates that exclude SSL usage, or authority key IDs
  that include both the key ID and the issuer’s issuer name and serial number);
********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_mp_authority_key_identifier_correct",
		Description:   "CAs MUST NOT issue certificates that have authority key IDs that include both the key ID and the issuer's issuer name and serial number",
		Citation:      "Mozilla Root Store Policy / Section 5.2",
		Source:        lint.MozillaRootStorePolicy,
		EffectiveDate: util.MozillaPolicy22Date,
		Lint:          NewAuthorityKeyIdentifierCorrect,
	})
}

func NewAuthorityKeyIdentifierCorrect() lint.LintInterface {
	return &authorityKeyIdentifierCorrect{}
}

func (l *authorityKeyIdentifierCorrect) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.AuthkeyOID)
}

func (l *authorityKeyIdentifierCorrect) Execute(c *x509.Certificate) *lint.LintResult {
	var keyID keyIdentifier

	// ext is assumed not-nil based on CheckApplies.
	ext := util.GetExtFromCert(c, util.AuthkeyOID)
	if _, err := asn1.Unmarshal(ext.Value, &keyID); err != nil {
		return &lint.LintResult{
			Status:  lint.Fatal,
			Details: fmt.Sprintf("error unmarshalling authority key identifier extension: %v", err),
		}
	}

	hasKeyID := len(keyID.KeyIdentifier.Bytes) > 0
	hasCertIssuer := len(keyID.AuthorityCertIssuer.Bytes) > 0
	if hasKeyID && hasCertIssuer {
		return &lint.LintResult{Status: lint.Error}
	}
	return &lint.LintResult{Status: lint.Pass}
}
