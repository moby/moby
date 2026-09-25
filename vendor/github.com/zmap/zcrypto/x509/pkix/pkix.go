// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pkix contains shared, low level structures used for ASN.1 parsing
// and serialization of X.509 certificates, CRL and OCSP.
package pkix

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/zmap/zcrypto/encoding/asn1"
)

// LegacyNameString allows to specify legacy ZCrypto behaviour
// for X509Name.String() in reverse order
var LegacyNameString = false

// AlgorithmIdentifier represents the ASN.1 structure of the same name. See RFC
// 5280, section 4.1.1.2.
type AlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type RDNSequence []RelativeDistinguishedNameSET

type RelativeDistinguishedNameSET []AttributeTypeAndValue

// AttributeTypeAndValue mirrors the ASN.1 structure of the same name in
// RFC 5280, Section 4.1.2.4.
type AttributeTypeAndValue struct {
	Type  asn1.ObjectIdentifier `json:"type"`
	Value interface{}           `json:"value"`
}

// AttributeTypeAndValueSET represents a set of ASN.1 sequences of
// AttributeTypeAndValue sequences from RFC 2986 (PKCS #10).
type AttributeTypeAndValueSET struct {
	Type  asn1.ObjectIdentifier
	Value [][]AttributeTypeAndValue `asn1:"set"`
}

// Extension represents the ASN.1 structure of the same name. See RFC
// 5280, section 4.2.
type Extension struct {
	Id       asn1.ObjectIdentifier
	Critical bool `asn1:"optional"`
	Value    []byte
}

// Name represents an X.509 distinguished name. This only includes the common
// elements of a DN. Note that Name is only an approximation of the X.509
// structure. If an accurate representation is needed, asn1.Unmarshal the raw
// subject or issuer as an RDNSequence.
type Name struct {
	Country, Organization, OrganizationalUnit  []string
	Locality, Province                         []string
	StreetAddress, PostalCode, DomainComponent []string
	EmailAddress                               []string
	SerialNumber, CommonName                   string
	SerialNumbers, CommonNames                 []string
	GivenName, Surname                         []string
	OrganizationIDs                            []string
	// EV Components
	JurisdictionLocality, JurisdictionProvince, JurisdictionCountry []string

	// Names contains all parsed attributes. When parsing distinguished names,
	// this can be used to extract non-standard attributes that are not parsed
	// by this package. When marshaling to RDNSequences, the Names field is
	// ignored, see ExtraNames.
	Names []AttributeTypeAndValue

	// ExtraNames contains attributes to be copied, raw, into any marshaled
	// distinguished names. Values override any attributes with the same OID.
	// The ExtraNames field is not populated when parsing, see Names.
	ExtraNames []AttributeTypeAndValue
	// OriginalRDNS is saved if the name is populated using FillFromRDNSequence.
	// Additionally, if OriginalRDNS is non-nil, the String and ToRDNSequence
	// methods will simply use this.
	OriginalRDNS RDNSequence
}

// FillFromRDNSequence populates n from the provided RDNSequence.
// Multi-entry RDNs are flattened, all entries are added to the
// relevant n fields, and the grouping is not preserved.
func (n *Name) FillFromRDNSequence(rdns *RDNSequence) {
	n.OriginalRDNS = *rdns
	for _, rdn := range *rdns {
		if len(rdn) == 0 {
			continue
		}

		for _, atv := range rdn {
			n.Names = append(n.Names, atv)
			value, ok := atv.Value.(string)
			if !ok {
				continue
			}

			t := atv.Type
			if len(t) == 4 && t[0] == 2 && t[1] == 5 && t[2] == 4 {
				switch t[3] {
				case 3:
					n.CommonName = value
					n.CommonNames = append(n.CommonNames, value)
				case 4:
					n.Surname = append(n.Surname, value)
				case 5:
					n.SerialNumber = value
					n.SerialNumbers = append(n.SerialNumbers, value)
				case 6:
					n.Country = append(n.Country, value)
				case 7:
					n.Locality = append(n.Locality, value)
				case 8:
					n.Province = append(n.Province, value)
				case 9:
					n.StreetAddress = append(n.StreetAddress, value)
				case 10:
					n.Organization = append(n.Organization, value)
				case 11:
					n.OrganizationalUnit = append(n.OrganizationalUnit, value)
				case 17:
					n.PostalCode = append(n.PostalCode, value)
				case 42:
					n.GivenName = append(n.GivenName, value)
				case 97:
					n.OrganizationIDs = append(n.OrganizationIDs, value)
				}
			} else if t.Equal(oidDomainComponent) {
				n.DomainComponent = append(n.DomainComponent, value)
			} else if t.Equal(oidDNEmailAddress) {
				// Deprecated, see RFC 5280 Section 4.1.2.6
				n.EmailAddress = append(n.EmailAddress, value)
			} else if t.Equal(oidJurisdictionLocality) {
				n.JurisdictionLocality = append(n.JurisdictionLocality, value)
			} else if t.Equal(oidJurisdictionProvince) {
				n.JurisdictionProvince = append(n.JurisdictionProvince, value)
			} else if t.Equal(oidJurisdictionCountry) {
				n.JurisdictionCountry = append(n.JurisdictionCountry, value)
			}
		}
	}
}

var (
	oidCountry            = []int{2, 5, 4, 6}
	oidOrganization       = []int{2, 5, 4, 10}
	oidOrganizationalUnit = []int{2, 5, 4, 11}
	oidCommonName         = []int{2, 5, 4, 3}
	oidSurname            = []int{2, 5, 4, 4}
	oidSerialNumber       = []int{2, 5, 4, 5}
	oidLocality           = []int{2, 5, 4, 7}
	oidProvince           = []int{2, 5, 4, 8}
	oidStreetAddress      = []int{2, 5, 4, 9}
	oidPostalCode         = []int{2, 5, 4, 17}
	oidGivenName          = []int{2, 5, 4, 42}
	oidDomainComponent    = []int{0, 9, 2342, 19200300, 100, 1, 25}
	oidDNEmailAddress     = []int{1, 2, 840, 113549, 1, 9, 1}
	// EV
	oidJurisdictionLocality = []int{1, 3, 6, 1, 4, 1, 311, 60, 2, 1, 1}
	oidJurisdictionProvince = []int{1, 3, 6, 1, 4, 1, 311, 60, 2, 1, 2}
	oidJurisdictionCountry  = []int{1, 3, 6, 1, 4, 1, 311, 60, 2, 1, 3}
	// QWACS
	oidOrganizationID = []int{2, 5, 4, 97}
)

// appendRDNs appends a relativeDistinguishedNameSET to the given RDNSequence
// and returns the new value. The relativeDistinguishedNameSET contains an
// attributeTypeAndValue for each of the given values. See RFC 5280, A.1, and
// search for AttributeTypeAndValue.
func (n Name) appendRDNs(in RDNSequence, values []string, oid asn1.ObjectIdentifier) RDNSequence {
	// NOTE: stdlib prevents adding if the oid is already present in n.ExtraNames
	//if len(values) == 0 || oidInAttributeTypeAndValue(oid, n.ExtraNames) {
	if len(values) == 0 {
		return in
	}

	s := make([]AttributeTypeAndValue, len(values))
	for i, value := range values {
		s[i].Type = oid
		s[i].Value = value
	}

	return append(in, s)
}

// String returns a string representation of the sequence r,
// roughly following the RFC 2253 Distinguished Names syntax.
func (r RDNSequence) String() string {
	s := ""
	for i := 0; i < len(r); i++ {
		idx := len(r) - 1 - i
		if LegacyNameString {
			idx = i
		}
		rdn := r[idx]
		if i > 0 {
			s += ", "
		}
		for j, tv := range rdn {
			if j > 0 {
				s += ", "
			}

			oidString := tv.Type.String()
			var typeName string
			if oidName, ok := oidDotNotationToNames[oidString]; ok {
				typeName = oidName.ShortName
			}
			if typeName == "" {
				derBytes, err := asn1.Marshal(tv.Value)
				if err == nil {
					s += oidString + "=#" + hex.EncodeToString(derBytes)
					continue // No value escaping necessary.
				}

				typeName = oidString
			}

			valueString := fmt.Sprint(tv.Value)
			escaped := make([]rune, 0, len(valueString))

			for k, c := range valueString {
				escape := false

				switch c {
				case ',', '+', '"', '\\', '<', '>', ';':
					escape = true

				case ' ':
					escape = k == 0 || k == len(valueString)-1

				case '#':
					escape = k == 0
				}

				if escape {
					escaped = append(escaped, '\\', c)
				} else {
					escaped = append(escaped, c)
				}
			}

			s += typeName + "=" + string(escaped)
		}
	}

	return s
}

// ToRDNSequence converts n into a single RDNSequence. The following
// attributes are encoded as multi-value RDNs:
//
//   - Country
//   - Organization
//   - OrganizationalUnit
//   - Locality
//   - Province
//   - StreetAddress
//   - PostalCode
//
// Each ExtraNames entry is encoded as an individual RDN.
func (n Name) ToRDNSequence() (ret RDNSequence) {
	if n.OriginalRDNS != nil {
		return n.OriginalRDNS
	}
	if len(n.CommonName) > 0 {
		ret = n.appendRDNs(ret, []string{n.CommonName}, oidCommonName)
	}
	ret = n.appendRDNs(ret, n.OrganizationalUnit, oidOrganizationalUnit)
	ret = n.appendRDNs(ret, n.Organization, oidOrganization)
	ret = n.appendRDNs(ret, n.StreetAddress, oidStreetAddress)
	ret = n.appendRDNs(ret, n.Locality, oidLocality)
	ret = n.appendRDNs(ret, n.Province, oidProvince)
	ret = n.appendRDNs(ret, n.PostalCode, oidPostalCode)
	ret = n.appendRDNs(ret, n.Country, oidCountry)
	ret = n.appendRDNs(ret, n.DomainComponent, oidDomainComponent)
	// EV Components
	ret = n.appendRDNs(ret, n.JurisdictionLocality, oidJurisdictionLocality)
	ret = n.appendRDNs(ret, n.JurisdictionProvince, oidJurisdictionProvince)
	ret = n.appendRDNs(ret, n.JurisdictionCountry, oidJurisdictionCountry)
	// QWACS
	ret = n.appendRDNs(ret, n.OrganizationIDs, oidOrganizationID)
	if len(n.SerialNumber) > 0 {
		ret = n.appendRDNs(ret, []string{n.SerialNumber}, oidSerialNumber)
	}
	for _, atv := range n.ExtraNames {
		ret = append(ret, []AttributeTypeAndValue{atv})
	}

	return ret
}

// CertificateList represents the ASN.1 structure of the same name. See RFC
// 5280, section 5.1. Use Certificate.CheckCRLSignature to verify the
// signature.
type CertificateList struct {
	TBSCertList        TBSCertificateList
	SignatureAlgorithm AlgorithmIdentifier
	SignatureValue     asn1.BitString
}

// HasExpired reports whether certList should have been updated by now.
func (certList *CertificateList) HasExpired(now time.Time) bool {
	return !now.Before(certList.TBSCertList.NextUpdate)
}

// String returns the string form of n, roughly following
// the RFC 2253 Distinguished Names syntax.
func (n Name) String() string {
	var rdns RDNSequence
	// If there are no ExtraNames, surface the parsed value (all entries in
	// Names) instead.
	if n.ExtraNames == nil {
		for _, atv := range n.Names {
			t := atv.Type
			if len(t) == 4 && t[0] == 2 && t[1] == 5 && t[2] == 4 {
				switch t[3] {
				case 3, 5, 6, 7, 8, 9, 10, 11, 17:
					// These attributes were already parsed into named fields.
					continue
				}
			}
			// Place non-standard parsed values at the beginning of the sequence
			// so they will be at the end of the string. See Issue 39924.
			rdns = append(rdns, []AttributeTypeAndValue{atv})
		}
	}
	rdns = append(rdns, n.ToRDNSequence()...)
	return rdns.String()
}

// OtherName represents the ASN.1 structure of the same name. See RFC
// 5280, section 4.2.1.6.
type OtherName struct {
	TypeID asn1.ObjectIdentifier
	Value  asn1.RawValue `asn1:"explicit"`
}

// EDIPartyName represents the ASN.1 structure of the same name. See RFC
// 5280, section 4.2.1.6.
type EDIPartyName struct {
	NameAssigner string `asn1:"tag:0,optional,explicit" json:"name_assigner,omitempty"`
	PartyName    string `asn1:"tag:1,explicit" json:"party_name"`
}

// TBSCertificateList represents the ASN.1 structure of the same name. See RFC
// 5280, section 5.1.
type TBSCertificateList struct {
	Raw                 asn1.RawContent
	Version             int `asn1:"optional,default:0"`
	Signature           AlgorithmIdentifier
	Issuer              RDNSequence
	ThisUpdate          time.Time
	NextUpdate          time.Time            `asn1:"optional"`
	RevokedCertificates []RevokedCertificate `asn1:"optional"`
	Extensions          []Extension          `asn1:"tag:0,optional,explicit"`
}

// RevokedCertificate represents the ASN.1 structure of the same name. See RFC
// 5280, section 5.1.
type RevokedCertificate struct {
	SerialNumber   *big.Int
	RevocationTime time.Time
	Extensions     []Extension `asn1:"optional"`
}
