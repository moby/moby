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
	"strings"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_superfluous_ku_encoding",
		Description:   "RFC 5280 Section 4.2.1.3 describes the value of a KeyUsage to be a DER encoded BitString, which itself must not have unnecessary trailing 00 bytes.",
		Citation:      "1.2.2 Where Rec. ITU-T X.680 | ISO/IEC 8824-1, 22.7, applies, the bitstring shall have all trailing 0 bits removed before it is encoded.",
		Source:        lint.RFC5280,
		EffectiveDate: util.ZeroDate,
		Lint:          func() lint.LintInterface { return &superfluousKuEncoding{} },
	})
}

type superfluousKuEncoding struct{}

func NewSuperfluousKuEncoding() lint.LintInterface {
	return &superfluousKuEncoding{}
}

func (l *superfluousKuEncoding) CheckApplies(c *x509.Certificate) bool {
	ku := util.GetExtFromCert(c, util.KeyUsageOID)
	return ku != nil && len(ku.Value) > 0
}

func (l *superfluousKuEncoding) Execute(c *x509.Certificate) *lint.LintResult {
	ku := util.GetExtFromCert(c, util.KeyUsageOID).Value
	if ku[len(ku)-1] != 0 {
		return &lint.LintResult{Status: lint.Pass}
	}
	binary := make([]string, len(ku))
	for i, b := range ku {
		binary[i] = fmt.Sprintf("%08b", b)
	}
	// E.G. KeyUsage contains superfluous trailing 00 byte. Bytes: [3 3 7 6 0], Binary: [00000011 00000011 00000111 00000110 00000000]
	return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf(
		"KeyUsage contains superfluous trailing 00 byte. Bytes: %v, Binary: [%s]", ku, strings.Join(binary, " "),
	)}
}
