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

type SerialNumberNotPositive struct{}

/************************************************
4.1.2.2.  Serial Number
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
		Name:          "e_serial_number_not_positive",
		Description:   "Certificates must have a positive serial number",
		Citation:      "RFC 5280: 4.1.2.2",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC3280Date,
		Lint:          &SerialNumberNotPositive{},
	})
}

func (l *SerialNumberNotPositive) Initialize() error {
	return nil
}

func (l *SerialNumberNotPositive) CheckApplies(cert *x509.Certificate) bool {
	return true
}

func (l *SerialNumberNotPositive) Execute(cert *x509.Certificate) *lint.LintResult {
	if cert.SerialNumber.Sign() == -1 { // -1 Means negative when using big.Sign()
		return &lint.LintResult{Status: lint.Error}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}
