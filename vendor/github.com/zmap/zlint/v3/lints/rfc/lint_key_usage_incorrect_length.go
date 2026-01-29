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
	"encoding/asn1"
	"fmt"
	"math/big"

	"golang.org/x/crypto/cryptobyte"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type keyUsageIncorrectLength struct{}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_key_usage_incorrect_length",
		Description:   "The key usage is a bit string with exactly nine possible flags",
		Citation:      "RFC 5280: 4.2.1.3",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC5280Date,
		Lint:          NewKeyUsageIncorrectLength,
	})
}

func NewKeyUsageIncorrectLength() lint.LintInterface {
	return &keyUsageIncorrectLength{}
}

func (l *keyUsageIncorrectLength) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.KeyUsageOID)
}

func keyUsageIncorrectLengthBytes(kuBytes []byte) *lint.LintResult {
	keyUsageExt := cryptobyte.String(kuBytes)
	var keyUsageVal asn1.BitString
	ok := keyUsageExt.ReadASN1BitString(&keyUsageVal)
	if !ok {
		return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("the key usage (%v) extension is not parseable.", kuBytes)}
	}
	unused := kuBytes[2]
	kuBig := big.NewInt(0).SetBytes(keyUsageVal.Bytes)
	if !kuBig.IsInt64() || kuBig.Int64()>>unused >= 512 {
		return &lint.LintResult{Status: lint.Error, Details: fmt.Sprintf("the key usage (%v) contains a value that is out of bounds of the range of possible KU values. (raw ASN: %v)", keyUsageVal.Bytes, kuBytes)}
	}
	return &lint.LintResult{Status: lint.Pass}
}

func (l *keyUsageIncorrectLength) Execute(c *x509.Certificate) *lint.LintResult {
	return keyUsageIncorrectLengthBytes(util.GetExtFromCert(c, util.KeyUsageOID).Value)
}
