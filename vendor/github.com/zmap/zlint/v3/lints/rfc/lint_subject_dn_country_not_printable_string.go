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
	"encoding/asn1"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type SubjectDNCountryNotPrintableString struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_dn_country_not_printable_string",
		Description:   "X520 Distinguished Name Country MUST be encoded as PrintableString",
		Citation:      "RFC 5280: Appendix A",
		Source:        lint.RFC5280,
		EffectiveDate: util.ZeroDate,
		Lint:          &SubjectDNCountryNotPrintableString{},
	})
}

func (l *SubjectDNCountryNotPrintableString) Initialize() error {
	return nil
}

func (l *SubjectDNCountryNotPrintableString) CheckApplies(c *x509.Certificate) bool {
	return len(c.Subject.Country) > 0
}

func (l *SubjectDNCountryNotPrintableString) Execute(c *x509.Certificate) *lint.LintResult {
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
			if attrTypeAndValue.Type.Equal(util.CountryNameOID) && attrTypeAndValue.Value.Tag != asn1.TagPrintableString {
				return &lint.LintResult{Status: lint.Error}
			}
		}
	}

	return &lint.LintResult{Status: lint.Pass}
}
