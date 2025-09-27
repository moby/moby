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
	"fmt"
	"math/big"
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_incorrect_ku_encoding",
		Description:   "RFC 5280 Section 4.2.1.3 describes the value of a KeyUsage to be a DER encoded BitString, which itself defines that all trailing 0 bits be counted as being \"unused\".",
		Citation:      "Where ITU-T Rec. X.680 | ISO/IEC 8824-1, 21.7, applies, the bitstring shall have all trailing 0 bits removed before it is encoded.",
		Source:        lint.RFC5280,
		EffectiveDate: util.ZeroDate,
		Lint:          func() lint.LintInterface { return &incorrectKuEncoding{} },
	})
}

type incorrectKuEncoding struct{}

func NewIncorrectKuEncoding() lint.LintInterface {
	return &incorrectKuEncoding{}
}

func (l *incorrectKuEncoding) CheckApplies(c *x509.Certificate) bool {
	ku := util.GetExtFromCert(c, util.KeyUsageOID)
	return ku != nil && len(ku.Value) > 0
}

func (l *incorrectKuEncoding) Execute(c *x509.Certificate) *lint.LintResult {
	ku := util.GetExtFromCert(c, util.KeyUsageOID).Value
	if len(ku) < 4 {
		return &lint.LintResult{
			Status:  lint.Fatal,
			Details: fmt.Sprintf("KeyUsage encodings must be at least four bytes long. Got %d bytes", len(ku)),
		}
	}
	// Byte 0: Tag
	// Byte 1: Length
	// Byte 2: Unused bits
	// Bytes 3..n: KeyUsage
	declaredUnused := uint(ku[2])
	actualUnused := big.NewInt(0).SetBytes(ku[3:]).TrailingZeroBits()
	if declaredUnused == actualUnused {
		return &lint.LintResult{Status: lint.Pass}
	}
	// Just a bit of formatting to a visualized binary form so
	// it's easier for users to see what the exact binary that
	// we're referring to so that they can debug their own certs.
	binary := make([]string, len(ku))
	for i, b := range ku {
		binary[i] = fmt.Sprintf("%08b", b)
	}
	return &lint.LintResult{
		Status: lint.Error,
		Details: fmt.Sprintf(
			"KeyUsage contains an inefficient encoding wherein the number of 'unused bits' is declared to be "+
				"%d, but it should be %d. Raw Bytes: %v, Raw Binary: [%s]",
			declaredUnused, actualUnused, ku, strings.Join(binary, " "),
		)}
}
