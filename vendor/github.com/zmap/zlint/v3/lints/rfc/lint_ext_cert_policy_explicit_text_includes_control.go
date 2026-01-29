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

type controlChar struct{}

/*********************************************************************
An explicitText field includes the textual statement directly in
the certificate.  The explicitText field is a string with a
maximum size of 200 characters.  Conforming CAs SHOULD use the
UTF8String encoding for explicitText, but MAY use IA5String.
Conforming CAs MUST NOT encode explicitText as VisibleString or
BMPString.  The explicitText string SHOULD NOT include any control
characters (e.g., U+0000 to U+001F and U+007F to U+009F).  When
the UTF8String encoding is used, all character sequences SHOULD be
normalized according to Unicode normalization form C (NFC) [NFC].
*********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_ext_cert_policy_explicit_text_includes_control",
		Description:   "Explicit text should not include any control characters",
		Citation:      "RFC 6818: 3",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC6818Date,
		Lint:          NewControlChar,
	})
}

func NewControlChar() lint.LintInterface {
	return &controlChar{}
}

func (l *controlChar) CheckApplies(c *x509.Certificate) bool {
	for _, text := range c.ExplicitTexts {
		if text != nil {
			return true
		}
	}
	return false
}

//nolint:nestif
//nolint:cyclop
func (l *controlChar) Execute(c *x509.Certificate) *lint.LintResult {
	for _, firstLvl := range c.ExplicitTexts {
		for _, text := range firstLvl {
			if text.Tag == 12 {
				for i := 0; i < len(text.Bytes); i++ {
					if text.Bytes[i]&0x80 == 0 {
						if text.Bytes[i] < 0x20 || text.Bytes[i] == 0x7f {
							return &lint.LintResult{Status: lint.Warn}
						}
					} else if text.Bytes[i]&0x20 == 0 {
						if text.Bytes[i] == 0xc2 && text.Bytes[i+1] >= 0x80 && text.Bytes[i+1] <= 0x9f {
							return &lint.LintResult{Status: lint.Warn}
						}
						i += 1
					} else if text.Bytes[i]&0x10 == 0 {
						i += 2
					} else if text.Bytes[i]&0x08 == 0 {
						i += 3
					} else if text.Bytes[i]&0x04 == 0 {
						i += 4
					} else if text.Bytes[i]&0x02 == 0 {
						i += 5
					}
				}
			}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
