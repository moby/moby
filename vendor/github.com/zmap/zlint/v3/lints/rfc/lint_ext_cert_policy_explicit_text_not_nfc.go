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
	"golang.org/x/text/unicode/norm"
)

type ExtCertPolicyExplicitTextNotNFC struct{}

/************************************************
  When the UTF8String encoding is used, all character sequences SHOULD be
  normalized according to Unicode normalization form C (NFC) [NFC].
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "w_ext_cert_policy_explicit_text_not_nfc",
		Description:   "When utf8string or bmpstring encoding is used for explicitText field in certificate policy, it SHOULD be normalized by NFC format",
		Citation:      "RFC6181 3",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC6818Date,
		Lint:          &ExtCertPolicyExplicitTextNotNFC{},
	})
}

func (l *ExtCertPolicyExplicitTextNotNFC) Initialize() error {
	return nil
}

func (l *ExtCertPolicyExplicitTextNotNFC) CheckApplies(c *x509.Certificate) bool {
	for _, text := range c.ExplicitTexts {
		if text != nil {
			return true
		}
	}
	return false
}

func (l *ExtCertPolicyExplicitTextNotNFC) Execute(c *x509.Certificate) *lint.LintResult {
	for _, firstLvl := range c.ExplicitTexts {
		for _, text := range firstLvl {
			if text.Tag == 12 || text.Tag == 30 {
				if !norm.NFC.IsNormal(text.Bytes) {
					return &lint.LintResult{Status: lint.Warn}
				}
			}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
