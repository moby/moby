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

package rfc

import (
	"unicode/utf8"

	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type subjectDNNotPrintableCharacters struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_dn_not_printable_characters",
		Description:   "X520 Subject fields MUST only contain printable control characters",
		Citation:      "RFC 5280: Appendix A",
		Source:        lint.RFC5280,
		EffectiveDate: util.ZeroDate,
		Lint:          NewSubjectDNNotPrintableCharacters,
	})
}

func NewSubjectDNNotPrintableCharacters() lint.LintInterface {
	return &subjectDNNotPrintableCharacters{}
}

func (l *subjectDNNotPrintableCharacters) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *subjectDNNotPrintableCharacters) Execute(c *x509.Certificate) *lint.LintResult {
	rdnSequence := util.RawRDNSequence{}
	rest, err := asn1.Unmarshal(c.RawSubject, &rdnSequence)
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal}
	}
	if len(rest) > 0 {
		return &lint.LintResult{Status: lint.Fatal}
	}

	for _, attrTypeAndValueSet := range rdnSequence {
		for _, attrTypeAndValue := range attrTypeAndValueSet {
			bytes := attrTypeAndValue.Value.Bytes
			for len(bytes) > 0 {
				r, size := utf8.DecodeRune(bytes)
				if r < 0x20 {
					return &lint.LintResult{Status: lint.Error}
				}
				if r >= 0x7F && r <= 0x9F {
					return &lint.LintResult{Status: lint.Error}
				}
				bytes = bytes[size:]
			}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
