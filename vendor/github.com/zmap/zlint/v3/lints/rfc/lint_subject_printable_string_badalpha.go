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
	"errors"
	"fmt"
	"regexp"

	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_subject_printable_string_badalpha",
		Description:   "PrintableString type's alphabet only includes a-z, A-Z, 0-9, and 11 special characters",
		Citation:      "RFC 5280: Appendix B. ASN.1 Notes",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC2459Date,
		Lint:          NewSubjectPrintableStringBadAlpha,
	})
}

func NewSubjectPrintableStringBadAlpha() lint.LintInterface {
	return &subjectPrintableStringBadAlpha{}
}

var (
	// Per RFC 5280, Appendix B. ASN.1 Notes:
	//   The character string type PrintableString supports a very basic Latin
	//   character set: the lowercase letters 'a' through 'z', uppercase
	//   letters 'A' through 'Z', the digits '0' through '9', eleven special
	//   characters ' = ( ) + , - . / : ? and space.
	printableStringRegex = regexp.MustCompile(`^[a-zA-Z0-9\=\(\)\+,\-.\/:\? ']+$`)
)

// validatePrintableString returns an error if the provided encoded printable
// string doesn't adhere to the character set defined in RFC 5280.
func validatePrintableString(rawPS []byte) error {
	if !printableStringRegex.Match(rawPS) {
		return errors.New("encoded PrintableString contained illegal characters")
	}
	return nil
}

type subjectPrintableStringBadAlpha struct {
}

// CheckApplies returns true for any certificate with a non-empty RawSubject.
func (l *subjectPrintableStringBadAlpha) CheckApplies(c *x509.Certificate) bool {
	return len(c.RawSubject) > 0
}

// Execute checks the certificate's RawSubject to ensure that any
// PrintableString attribute/value pairs in the Subject match the character set
// defined for this type in RFC 5280. An lint.Error level lint.LintResult is returned if any
// of the PrintableString attributes do not match a regular expression for the
// allowed character set.
func (l *subjectPrintableStringBadAlpha) Execute(c *x509.Certificate) *lint.LintResult {
	rdnSequence := util.RawRDNSequence{}
	rest, err := asn1.Unmarshal(c.RawSubject, &rdnSequence)
	if err != nil {
		return &lint.LintResult{
			Status:  lint.Fatal,
			Details: "Failed to Unmarshal RawSubject into RawRDNSequence",
		}
	}
	if len(rest) > 0 {
		return &lint.LintResult{
			Status:  lint.Fatal,
			Details: "Trailing data after RawSubject RawRDNSequence",
		}
	}

	for _, attrTypeAndValueSet := range rdnSequence {
		for _, attrTypeAndValue := range attrTypeAndValueSet {
			// If the attribute type is a PrintableString the bytes of the attribute
			// value must match the printable string alphabet.
			if attrTypeAndValue.Value.Tag == asn1.TagPrintableString {
				if err := validatePrintableString(attrTypeAndValue.Value.Bytes); err != nil {
					return &lint.LintResult{
						Status: lint.Error,
						Details: fmt.Sprintf("RawSubject attr oid %s %s",
							attrTypeAndValue.Type, err.Error()),
					}
				}
			}
		}
	}

	return &lint.LintResult{
		Status: lint.Pass,
	}
}
