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

package util

import (
	"encoding/asn1"
	"errors"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zcrypto/x509/pkix"
)

var (
	//extension OIDs
	AiaOID                  = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 1}        // Authority Information Access
	AuthkeyOID              = asn1.ObjectIdentifier{2, 5, 29, 35}                     // Authority Key Identifier
	BasicConstOID           = asn1.ObjectIdentifier{2, 5, 29, 19}                     // Basic Constraints
	CertPolicyOID           = asn1.ObjectIdentifier{2, 5, 29, 32}                     // Certificate Policies
	CrlDistOID              = asn1.ObjectIdentifier{2, 5, 29, 31}                     // CRL Distribution Points
	CtPoisonOID             = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 4, 3} // CT Poison
	EkuSynOid               = asn1.ObjectIdentifier{2, 5, 29, 37}                     // Extended Key Usage Syntax
	FreshCRLOID             = asn1.ObjectIdentifier{2, 5, 29, 46}                     // Freshest CRL
	InhibitAnyPolicyOID     = asn1.ObjectIdentifier{2, 5, 29, 54}                     // Inhibit Any Policy
	IssuerAlternateNameOID  = asn1.ObjectIdentifier{2, 5, 29, 18}                     // Issuer Alt Name
	KeyUsageOID             = asn1.ObjectIdentifier{2, 5, 29, 15}                     // Key Usage
	LogoTypeOID             = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 12}       // Logo Type Ext
	NameConstOID            = asn1.ObjectIdentifier{2, 5, 29, 30}                     // Name Constraints
	OscpNoCheckOID          = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 48, 1, 5}    // OSCP No Check
	PolicyConstOID          = asn1.ObjectIdentifier{2, 5, 29, 36}                     // Policy Constraints
	PolicyMapOID            = asn1.ObjectIdentifier{2, 5, 29, 33}                     // Policy Mappings
	PrivKeyUsageOID         = asn1.ObjectIdentifier{2, 5, 29, 16}                     // Private Key Usage Period
	QcStateOid              = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 3}        // QC Statements
	TimestampOID            = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 4, 2} // Signed Certificate Timestamp List
	SmimeOID                = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 15}      // Smime Capabilities
	SubjectAlternateNameOID = asn1.ObjectIdentifier{2, 5, 29, 17}                     // Subject Alt Name
	SubjectDirAttrOID       = asn1.ObjectIdentifier{2, 5, 29, 9}                      // Subject Directory Attributes
	SubjectInfoAccessOID    = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 11}       // Subject Info Access Syntax
	SubjectKeyIdentityOID   = asn1.ObjectIdentifier{2, 5, 29, 14}                     // Subject Key Identifier
	// CA/B reserved policies
	BRDomainValidatedOID                = asn1.ObjectIdentifier{2, 23, 140, 1, 2, 1} // CA/B BR Domain-Validated
	BROrganizationValidatedOID          = asn1.ObjectIdentifier{2, 23, 140, 1, 2, 2} // CA/B BR Organization-Validated
	BRIndividualValidatedOID            = asn1.ObjectIdentifier{2, 23, 140, 1, 2, 3} // CA/B BR Individual-Validated
	BRTorServiceDescriptor              = asn1.ObjectIdentifier{2, 23, 140, 1, 31}   // CA/B BR Tor Service Descriptor
	CabfExtensionOrganizationIdentifier = asn1.ObjectIdentifier{2, 23, 140, 3, 1}    // CA/B EV 9.8.2 cabfOrganizationIdentifier
	//X.500 attribute types
	CommonNameOID             = asn1.ObjectIdentifier{2, 5, 4, 3}
	SurnameOID                = asn1.ObjectIdentifier{2, 5, 4, 4}
	SerialOID                 = asn1.ObjectIdentifier{2, 5, 4, 5}
	CountryNameOID            = asn1.ObjectIdentifier{2, 5, 4, 6}
	LocalityNameOID           = asn1.ObjectIdentifier{2, 5, 4, 7}
	StateOrProvinceNameOID    = asn1.ObjectIdentifier{2, 5, 4, 8}
	StreetAddressOID          = asn1.ObjectIdentifier{2, 5, 4, 9}
	OrganizationNameOID       = asn1.ObjectIdentifier{2, 5, 4, 10}
	OrganizationalUnitNameOID = asn1.ObjectIdentifier{2, 5, 4, 11}
	BusinessOID               = asn1.ObjectIdentifier{2, 5, 4, 15}
	PostalCodeOID             = asn1.ObjectIdentifier{2, 5, 4, 17}
	GivenNameOID              = asn1.ObjectIdentifier{2, 5, 4, 42}
	// Hash algorithms - see https://golang.org/src/crypto/x509/x509.go
	SHA256OID = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	SHA384OID = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	SHA512OID = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}
	// other OIDs
	OidRSAEncryption           = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	OidRSASSAPSS               = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	OidMD2WithRSAEncryption    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 2}
	OidMD5WithRSAEncryption    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 4}
	OidSHA1WithRSAEncryption   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 5}
	OidSHA224WithRSAEncryption = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 14}
	OidSHA256WithRSAEncryption = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
	OidSHA384WithRSAEncryption = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12}
	OidSHA512WithRSAEncryption = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13}
	AnyPolicyOID               = asn1.ObjectIdentifier{2, 5, 29, 32, 0}
	UserNoticeOID              = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 2, 2}
	CpsOID                     = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 2, 1}
	IdEtsiQcsQcCompliance      = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 1}
	IdEtsiQcsQcLimitValue      = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 2}
	IdEtsiQcsQcRetentionPeriod = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 3}
	IdEtsiQcsQcSSCD            = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 4}
	IdEtsiQcsQcEuPDS           = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 5}
	IdEtsiQcsQcType            = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6}
	IdEtsiQcsQctEsign          = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6, 1}
	IdEtsiQcsQctEseal          = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6, 2}
	IdEtsiQcsQctWeb            = asn1.ObjectIdentifier{0, 4, 0, 1862, 1, 6, 3}
)

const (
	// Tags
	DNSNameTag = 2
)

// IsExtInCert is equivalent to GetExtFromCert() != nil.
func IsExtInCert(cert *x509.Certificate, oid asn1.ObjectIdentifier) bool {
	if cert != nil && GetExtFromCert(cert, oid) != nil {
		return true
	}
	return false
}

// GetExtFromCert returns the extension with the matching OID, if present. If
// the extension if not present, it returns nil.
//nolint:interfacer
func GetExtFromCert(cert *x509.Certificate, oid asn1.ObjectIdentifier) *pkix.Extension {
	// Since this function is called by many Lint CheckApplies functions we use
	// the x509.Certificate.ExtensionsMap field added by zcrypto to check for
	// the extension in O(1) instead of looping through the
	// `x509.Certificate.Extensions` in O(n).
	if ext, found := cert.ExtensionsMap[oid.String()]; found {
		return &ext
	}
	return nil
}

// Helper function that checks if an []asn1.ObjectIdentifier slice contains an asn1.ObjectIdentifier
func SliceContainsOID(list []asn1.ObjectIdentifier, oid asn1.ObjectIdentifier) bool {
	for _, v := range list {
		if oid.Equal(v) {
			return true
		}
	}
	return false
}

// Helper function that checks for a name type in a pkix.Name
func TypeInName(name *pkix.Name, oid asn1.ObjectIdentifier) bool {
	for _, v := range name.Names {
		if oid.Equal(v.Type) {
			return true
		}
	}
	return false
}

//helper function to parse policyMapping extensions, returns slices of CertPolicyIds separated by domain
func GetMappedPolicies(polMap *pkix.Extension) ([][2]asn1.ObjectIdentifier, error) {
	if polMap == nil {
		return nil, errors.New("policyMap: null pointer")
	}
	var outSeq, inSeq asn1.RawValue

	empty, err := asn1.Unmarshal(polMap.Value, &outSeq) //strip outer sequence tag/length should be nothing extra
	if err != nil || len(empty) != 0 || outSeq.Class != 0 || outSeq.Tag != 16 || !outSeq.IsCompound {
		return nil, errors.New("policyMap: Could not unmarshal outer sequence.")
	}

	var out [][2]asn1.ObjectIdentifier
	for done := false; !done; { //loop through SEQUENCE OF
		outSeq.Bytes, err = asn1.Unmarshal(outSeq.Bytes, &inSeq) //extract next inner SEQUENCE (OID pair)
		if err != nil || inSeq.Class != 0 || inSeq.Tag != 16 || !inSeq.IsCompound {
			return nil, errors.New("policyMap: Could not unmarshal inner sequence.")
		}
		if len(outSeq.Bytes) == 0 { //nothing remaining to parse, stop looping after
			done = true
		}

		var oidIssue, oidSubject asn1.ObjectIdentifier
		var restIn asn1.RawContent
		restIn, err = asn1.Unmarshal(inSeq.Bytes, &oidIssue) //extract first inner CertPolicyId (issuer domain)
		if err != nil || len(restIn) == 0 {
			return nil, errors.New("policyMap: Could not unmarshal inner sequence.")
		}

		empty, err = asn1.Unmarshal(restIn, &oidSubject) //extract second inner CertPolicyId (subject domain)
		if err != nil || len(empty) != 0 {
			return nil, errors.New("policyMap: Could not unmarshal inner sequence.")
		}

		//append found OIDs
		out = append(out, [2]asn1.ObjectIdentifier{oidIssue, oidSubject})
	}

	return out, nil
}
