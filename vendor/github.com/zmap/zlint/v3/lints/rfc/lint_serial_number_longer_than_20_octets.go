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
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type serialNumberTooLong struct{}

/************************************************
RFC 5280: 4.1.2.2.  Serial Number
   The serial number MUST be a positive integer assigned by the CA to each
   certificate. It MUST be unique for each certificate issued by a given CA
   (i.e., the issuer name and serial number identify a unique certificate).
   CAs MUST force the serialNumber to be a non-negative integer.

   Given the uniqueness requirements above, serial numbers can be expected to
   contain long integers.  Certificate users MUST be able to handle serialNumber
   values up to 20 octets.  Conforming CAs MUST NOT use serialNumber values longer
   than 20 octets.

   Note: Non-conforming CAs may issue certificates with serial numbers that are
   negative or zero.  Certificate users SHOULD be prepared togracefully handle
   such certificates.
************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_serial_number_longer_than_20_octets",
		Description:   "Certificates must not have a DER encoded serial number longer than 20 octets",
		Citation:      "RFC 5280: 4.1.2.2",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC3280Date,
		Lint:          &serialNumberTooLong{},
	})
}

func (l *serialNumberTooLong) Initialize() error {
	return nil
}

func (l *serialNumberTooLong) CheckApplies(c *x509.Certificate) bool {
	return true
}

func (l *serialNumberTooLong) Execute(c *x509.Certificate) *lint.LintResult {
	// Re-encode the certificate serial number and decode it back into
	// an ASN1 raw value (which does little more than perform length computations,
	// figures out the tag, etc.) so that we can easily see what the actual
	// DER encoded lengths are without having to guess.
	encoding, err := asn1.Marshal(c.SerialNumber)
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal, Details: fmt.Sprint(err)}
	}
	serial := new(asn1.RawValue)
	_, err = asn1.Unmarshal(encoding, serial)
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal, Details: fmt.Sprint(err)}
	}
	length := len(serial.Bytes)
	if length > 20 {
		details := fmt.Sprintf("The DER encoded certificate serial number is %d octets long. "+
			"If this is surprising to you, note that DER integers are signed and that SNs that are "+
			"20 octets long with an MSB of 1 will be automatically prefixed with 0x00, thus bumping "+
			"it up to 21 octets long. "+
			"SN: %X", length, serial.Bytes)
		return &lint.LintResult{Status: lint.Error, Details: details}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
