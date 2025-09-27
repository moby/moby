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

package util

import (
	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509/pkix"
)

type empty struct{}

var nameAttributePrefix = asn1.ObjectIdentifier{2, 5, 4}
var nameAttributeLeaves = map[int]empty{
	// Name attributes defined in RFC 5280 appendix A
	3:  {}, // id-at-commonName	AttributeType ::= { id-at 3 }
	4:  {}, // id-at-surname	AttributeType ::= { id-at  4 }
	5:  {}, // id-at-serialNumber	AttributeType ::= { id-at 5 }
	6:  {}, // id-at-countryName	AttributeType ::= { id-at 6 }
	7:  {}, // id-at-localityName	AttributeType ::= { id-at 7 }
	8:  {}, // id-at-stateOrProvinceName	AttributeType ::= { id-at 8 }
	10: {}, // id-at-organizationName	AttributeType ::= { id-at 10 }
	11: {}, // id-at-organizationalUnitName	AttributeType ::= { id-at 11 }
	12: {}, // id-at-title	AttributeType ::= { id-at 12 }
	41: {}, // id-at-name	AttributeType ::= { id-at 41 }
	42: {}, // id-at-givenName	AttributeType ::= { id-at 42 }
	43: {}, // id-at-initials	AttributeType ::= { id-at 43 }
	44: {}, // id-at-generationQualifier	AttributeType ::= { id-at 44 }
	46: {}, // id-at-dnQualifier	AttributeType ::= { id-at 46 }

	// Name attributes not present in RFC 5280, but appeared in Go's crypto/x509/pkix.go
	9:  {}, // id-at-streetName	AttributeType ::= { id-at 9 }
	17: {}, // id-at-postalCodeName	AttributeType ::= { id-at 17 }
}

// IsNameAttribute returns true if the given ObjectIdentifier corresponds with
// the type of any name attribute for PKIX.
func IsNameAttribute(oid asn1.ObjectIdentifier) bool {
	if len(oid) != 4 {
		return false
	}
	if !nameAttributePrefix.Equal(oid[0:3]) {
		return false
	}
	_, ok := nameAttributeLeaves[oid[3]]
	return ok
}

func NotAllNameFieldsAreEmpty(name *pkix.Name) bool {
	//Return true if at least one field is non-empty
	return len(name.Names) >= 1
}
