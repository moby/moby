package rfc

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

type explicitTextTooLong struct{}

/*******************************************************************
An explicitText field includes the textual statement directly in
the certificate.  The explicitText field is a string with a
maximum size of 200 characters.  Conforming CAs SHOULD use the
UTF8String encoding for explicitText.  VisibleString or BMPString
are acceptable but less preferred alternatives.  Conforming CAs
MUST NOT encode explicitText as IA5String.  The explicitText string
SHOULD NOT include any control characters (e.g., U+0000 to U+001F
and U+007F to U+009F).  When the UTF8String or BMPString encoding
is used, all character sequences SHOULD be normalized according
to Unicode normalization form C (NFC) [NFC].
*******************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_ext_cert_policy_explicit_text_too_long",
		Description:   "Explicit text has a maximum size of 200 characters",
		Citation:      "RFC 6818: 3",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC6818Date,
		Lint:          &explicitTextTooLong{},
	})
}

const tagBMPString int = 30

func (l *explicitTextTooLong) Initialize() error {
	return nil
}

func (l *explicitTextTooLong) CheckApplies(c *x509.Certificate) bool {
	for _, text := range c.ExplicitTexts {
		if text != nil {
			return true
		}
	}
	return false
}

func (l *explicitTextTooLong) Execute(c *x509.Certificate) *lint.LintResult {
	for _, firstLvl := range c.ExplicitTexts {
		for _, text := range firstLvl {
			var runes string
			// If the field is a BMPString, we need to parse the bytes out into
			// UTF-16-BE runes in order to check their length accurately
			// The `Bytes` attribute here is the raw representation of the userNotice
			if text.Tag == tagBMPString {
				runes, _ = util.ParseBMPString(text.Bytes)
			} else {
				runes = string(text.Bytes)
			}
			if len(runes) > 200 {
				return &lint.LintResult{Status: lint.Error}
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
