// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"encoding/hex"
	"encoding/json"
	"net"
	"strconv"
	"strings"

	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509/ct"
	"github.com/zmap/zcrypto/x509/pkix"
)

var (
	oidExtKeyUsage           = asn1.ObjectIdentifier{2, 5, 29, 15}
	oidExtBasicConstraints   = asn1.ObjectIdentifier{2, 5, 29, 19}
	oidExtSubjectAltName     = asn1.ObjectIdentifier{2, 5, 29, 17}
	oidExtIssuerAltName      = asn1.ObjectIdentifier{2, 5, 29, 18}
	oidExtNameConstraints    = asn1.ObjectIdentifier{2, 5, 29, 30}
	oidCRLDistributionPoints = asn1.ObjectIdentifier{2, 5, 29, 31}
	oidExtAuthKeyId          = asn1.ObjectIdentifier{2, 5, 29, 35}
	oidExtSubjectKeyId       = asn1.ObjectIdentifier{2, 5, 29, 14}
	oidExtExtendedKeyUsage   = asn1.ObjectIdentifier{2, 5, 29, 37}
	oidExtCertificatePolicy  = asn1.ObjectIdentifier{2, 5, 29, 32}
	oidExtensionCRLNumber    = asn1.ObjectIdentifier{2, 5, 29, 20}
	oidExtensionReasonCode   = asn1.ObjectIdentifier{2, 5, 29, 21}

	oidExtAuthorityInfoAccess            = oidExtensionAuthorityInfoAccess
	oidExtensionCTPrecertificatePoison   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11129, 2, 4, 3}
	oidExtSignedCertificateTimestampList = oidExtensionSignedCertificateTimestampList

	oidExtCABFOrganizationID = asn1.ObjectIdentifier{2, 23, 140, 3, 1}
	oidExtQCStatements       = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 3}
)

type CertificateExtensions struct {
	KeyUsage                       KeyUsage                         `json:"key_usage,omitempty"`
	BasicConstraints               *BasicConstraints                `json:"basic_constraints,omitempty"`
	SubjectAltName                 *GeneralNames                    `json:"subject_alt_name,omitempty"`
	IssuerAltName                  *GeneralNames                    `json:"issuer_alt_name,omitempty"`
	NameConstraints                *NameConstraints                 `json:"name_constraints,omitempty"`
	CRLDistributionPoints          CRLDistributionPoints            `json:"crl_distribution_points,omitempty"`
	AuthKeyID                      SubjAuthKeyId                    `json:"authority_key_id,omitempty"`
	SubjectKeyID                   SubjAuthKeyId                    `json:"subject_key_id,omitempty"`
	ExtendedKeyUsage               *ExtendedKeyUsageExtension       `json:"extended_key_usage,omitempty"`
	CertificatePolicies            *CertificatePoliciesData         `json:"certificate_policies,omitempty"`
	AuthorityInfoAccess            *AuthorityInfoAccess             `json:"authority_info_access,omitempty"`
	IsPrecert                      IsPrecert                        `json:"ct_poison,omitempty"`
	SignedCertificateTimestampList []*ct.SignedCertificateTimestamp `json:"signed_certificate_timestamps,omitempty"`
	TorServiceDescriptors          []*TorServiceDescriptorHash      `json:"tor_service_descriptors,omitempty"`
	CABFOrganizationIdentifier     *CABFOrganizationIdentifier      `json:"cabf_organization_id,omitempty"`
	QCStatements                   *QCStatements                    `json:"qc_statements,omitempty"`
}

type UnknownCertificateExtensions []pkix.Extension

type IsPrecert bool

type BasicConstraints struct {
	IsCA       bool `json:"is_ca"`
	MaxPathLen *int `json:"max_path_len,omitempty"`
}

type NoticeReference struct {
	Organization  string       `json:"organization,omitempty"`
	NoticeNumbers NoticeNumber `json:"notice_numbers,omitempty"`
}

type UserNoticeData struct {
	ExplicitText    string            `json:"explicit_text,omitempty"`
	NoticeReference []NoticeReference `json:"notice_reference,omitempty"`
}

type CertificatePoliciesJSON struct {
	PolicyIdentifier string           `json:"id,omitempty"`
	CPSUri           []string         `json:"cps,omitempty"`
	UserNotice       []UserNoticeData `json:"user_notice,omitempty"`
}

type CertificatePolicies []CertificatePoliciesJSON

type CertificatePoliciesData struct {
	PolicyIdentifiers     []asn1.ObjectIdentifier
	QualifierId           [][]asn1.ObjectIdentifier
	CPSUri                [][]string
	ExplicitTexts         [][]string
	NoticeRefOrganization [][]string
	NoticeRefNumbers      [][]NoticeNumber
}

func (cp *CertificatePoliciesData) MarshalJSON() ([]byte, error) {
	policies := CertificatePolicies{}
	for idx, oid := range cp.PolicyIdentifiers {
		cpsJSON := CertificatePoliciesJSON{}
		cpsJSON.PolicyIdentifier = oid.String()
		for _, uri := range cp.CPSUri[idx] {
			cpsJSON.CPSUri = append(cpsJSON.CPSUri, uri)
		}

		for idx2, explicit_text := range cp.ExplicitTexts[idx] {
			uNoticeData := UserNoticeData{}
			uNoticeData.ExplicitText = explicit_text
			noticeRef := NoticeReference{}
			if len(cp.NoticeRefOrganization[idx]) > 0 {
				organization := cp.NoticeRefOrganization[idx][idx2]
				noticeRef.Organization = organization
				noticeRef.NoticeNumbers = cp.NoticeRefNumbers[idx][idx2]
				uNoticeData.NoticeReference = append(uNoticeData.NoticeReference, noticeRef)
			}
			cpsJSON.UserNotice = append(cpsJSON.UserNotice, uNoticeData)
		}

		policies = append(policies, cpsJSON)
	}
	return json.Marshal(policies)
}

// GeneralNames corresponds an X.509 GeneralName defined in
// Section 4.2.1.6 of RFC 5280.
//
//	GeneralName ::= CHOICE {
//	     otherName                 [0]  AnotherName,
//	     rfc822Name                [1]  IA5String,
//	     dNSName                   [2]  IA5String,
//	     x400Address               [3]  ORAddress,
//	     directoryName             [4]  Name,
//	     ediPartyName              [5]  EDIPartyName,
//	     uniformResourceIdentifier [6]  IA5String,
//	     iPAddress                 [7]  OCTET STRING,
//	     registeredID              [8]  OBJECT IDENTIFIER }
type GeneralNames struct {
	DirectoryNames []pkix.Name
	DNSNames       []string
	EDIPartyNames  []pkix.EDIPartyName
	EmailAddresses []string
	IPAddresses    []net.IP
	OtherNames     []pkix.OtherName
	RegisteredIDs  []asn1.ObjectIdentifier
	URIs           []string
}

type jsonGeneralNames struct {
	DirectoryNames []pkix.Name         `json:"directory_names,omitempty"`
	DNSNames       []string            `json:"dns_names,omitempty"`
	EDIPartyNames  []pkix.EDIPartyName `json:"edi_party_names,omitempty"`
	EmailAddresses []string            `json:"email_addresses,omitempty"`
	IPAddresses    []net.IP            `json:"ip_addresses,omitempty"`
	OtherNames     []pkix.OtherName    `json:"other_names,omitempty"`
	RegisteredIDs  []string            `json:"registered_ids,omitempty"`
	URIs           []string            `json:"uniform_resource_identifiers,omitempty"`
}

func (gn *GeneralNames) MarshalJSON() ([]byte, error) {
	jsan := jsonGeneralNames{
		DirectoryNames: gn.DirectoryNames,
		DNSNames:       gn.DNSNames,
		EDIPartyNames:  gn.EDIPartyNames,
		EmailAddresses: gn.EmailAddresses,
		IPAddresses:    gn.IPAddresses,
		OtherNames:     gn.OtherNames,
		RegisteredIDs:  make([]string, 0, len(gn.RegisteredIDs)),
		URIs:           gn.URIs,
	}
	for _, id := range gn.RegisteredIDs {
		jsan.RegisteredIDs = append(jsan.RegisteredIDs, id.String())
	}
	return json.Marshal(jsan)
}

func (gn *GeneralNames) UnmarshalJSON(b []byte) error {
	var jsan jsonGeneralNames
	err := json.Unmarshal(b, &jsan)
	if err != nil {
		return err
	}

	gn.DirectoryNames = jsan.DirectoryNames
	gn.DNSNames = jsan.DNSNames
	gn.EDIPartyNames = jsan.EDIPartyNames
	gn.EmailAddresses = jsan.EmailAddresses
	gn.IPAddresses = jsan.IPAddresses
	gn.OtherNames = jsan.OtherNames
	gn.RegisteredIDs = make([]asn1.ObjectIdentifier, len(jsan.RegisteredIDs))
	gn.URIs = jsan.URIs

	for i, rID := range jsan.RegisteredIDs {
		arcs := strings.Split(rID, ".")
		oid := make(asn1.ObjectIdentifier, len(arcs))

		for j, s := range arcs {
			tmp, err := strconv.ParseInt(s, 10, 32)
			if err != nil {
				return err
			}
			oid[j] = int(tmp)
		}
		gn.RegisteredIDs[i] = oid
	}
	return nil
}

// TODO: Handle excluded names

type NameConstraints struct {
	Critical bool `json:"critical"`

	PermittedDNSNames       []GeneralSubtreeString
	PermittedEmailAddresses []GeneralSubtreeString
	PermittedURIs           []GeneralSubtreeString
	PermittedIPAddresses    []GeneralSubtreeIP
	PermittedDirectoryNames []GeneralSubtreeName
	PermittedEdiPartyNames  []GeneralSubtreeEdi
	PermittedRegisteredIDs  []GeneralSubtreeOid

	ExcludedEmailAddresses []GeneralSubtreeString
	ExcludedDNSNames       []GeneralSubtreeString
	ExcludedURIs           []GeneralSubtreeString
	ExcludedIPAddresses    []GeneralSubtreeIP
	ExcludedDirectoryNames []GeneralSubtreeName
	ExcludedEdiPartyNames  []GeneralSubtreeEdi
	ExcludedRegisteredIDs  []GeneralSubtreeOid
}

type NameConstraintsJSON struct {
	Critical bool `json:"critical"`

	PermittedDNSNames       []string            `json:"permitted_names,omitempty"`
	PermittedEmailAddresses []string            `json:"permitted_email_addresses,omitempty"`
	PermittedURIs           []string            `json:"permitted_uris,omitempty"`
	PermittedIPAddresses    []GeneralSubtreeIP  `json:"permitted_ip_addresses,omitempty"`
	PermittedDirectoryNames []pkix.Name         `json:"permitted_directory_names,omitempty"`
	PermittedEdiPartyNames  []pkix.EDIPartyName `json:"permitted_edi_party_names,omitempty"`
	PermittedRegisteredIDs  []string            `json:"permitted_registred_id,omitempty"`

	ExcludedDNSNames       []string            `json:"excluded_names,omitempty"`
	ExcludedEmailAddresses []string            `json:"excluded_email_addresses,omitempty"`
	ExcludedURIs           []string            `json:"excluded_uris,omitempty"`
	ExcludedIPAddresses    []GeneralSubtreeIP  `json:"excluded_ip_addresses,omitempty"`
	ExcludedDirectoryNames []pkix.Name         `json:"excluded_directory_names,omitempty"`
	ExcludedEdiPartyNames  []pkix.EDIPartyName `json:"excluded_edi_party_names,omitempty"`
	ExcludedRegisteredIDs  []string            `json:"excluded_registred_id,omitempty"`
}

func (nc *NameConstraints) UnmarshalJSON(b []byte) error {
	var ncJson NameConstraintsJSON
	err := json.Unmarshal(b, &ncJson)
	if err != nil {
		return err
	}
	for _, dns := range ncJson.PermittedDNSNames {
		nc.PermittedDNSNames = append(nc.PermittedDNSNames, GeneralSubtreeString{Data: dns})
	}
	for _, email := range ncJson.PermittedEmailAddresses {
		nc.PermittedEmailAddresses = append(nc.PermittedEmailAddresses, GeneralSubtreeString{Data: email})
	}
	for _, uri := range ncJson.PermittedURIs {
		nc.PermittedURIs = append(nc.PermittedURIs, GeneralSubtreeString{Data: uri})
	}
	for _, constraint := range ncJson.PermittedIPAddresses {
		nc.PermittedIPAddresses = append(nc.PermittedIPAddresses, constraint)
	}
	for _, directory := range ncJson.PermittedDirectoryNames {
		nc.PermittedDirectoryNames = append(nc.PermittedDirectoryNames, GeneralSubtreeName{Data: directory})
	}
	for _, edi := range ncJson.PermittedEdiPartyNames {
		nc.PermittedEdiPartyNames = append(nc.PermittedEdiPartyNames, GeneralSubtreeEdi{Data: edi})
	}
	for _, id := range ncJson.PermittedRegisteredIDs {
		arcs := strings.Split(id, ".")
		oid := make(asn1.ObjectIdentifier, len(arcs))

		for j, s := range arcs {
			tmp, err := strconv.ParseInt(s, 10, 32)
			if err != nil {
				return err
			}
			oid[j] = int(tmp)
		}
		nc.PermittedRegisteredIDs = append(nc.PermittedRegisteredIDs, GeneralSubtreeOid{Data: oid})
	}

	for _, dns := range ncJson.ExcludedDNSNames {
		nc.ExcludedDNSNames = append(nc.ExcludedDNSNames, GeneralSubtreeString{Data: dns})
	}
	for _, email := range ncJson.ExcludedEmailAddresses {
		nc.ExcludedEmailAddresses = append(nc.ExcludedEmailAddresses, GeneralSubtreeString{Data: email})
	}
	for _, uri := range ncJson.ExcludedURIs {
		nc.ExcludedURIs = append(nc.ExcludedURIs, GeneralSubtreeString{Data: uri})
	}
	for _, constraint := range ncJson.ExcludedIPAddresses {
		nc.ExcludedIPAddresses = append(nc.ExcludedIPAddresses, constraint)
	}
	for _, directory := range ncJson.ExcludedDirectoryNames {
		nc.ExcludedDirectoryNames = append(nc.ExcludedDirectoryNames, GeneralSubtreeName{Data: directory})
	}
	for _, edi := range ncJson.ExcludedEdiPartyNames {
		nc.ExcludedEdiPartyNames = append(nc.ExcludedEdiPartyNames, GeneralSubtreeEdi{Data: edi})
	}
	for _, id := range ncJson.ExcludedRegisteredIDs {
		arcs := strings.Split(id, ".")
		oid := make(asn1.ObjectIdentifier, len(arcs))

		for j, s := range arcs {
			tmp, err := strconv.ParseInt(s, 10, 32)
			if err != nil {
				return err
			}
			oid[j] = int(tmp)
		}
		nc.ExcludedRegisteredIDs = append(nc.ExcludedRegisteredIDs, GeneralSubtreeOid{Data: oid})
	}
	return nil
}

func (nc NameConstraints) MarshalJSON() ([]byte, error) {
	var out NameConstraintsJSON
	for _, dns := range nc.PermittedDNSNames {
		out.PermittedDNSNames = append(out.PermittedDNSNames, dns.Data)
	}
	for _, email := range nc.PermittedEmailAddresses {
		out.PermittedEmailAddresses = append(out.PermittedEmailAddresses, email.Data)
	}
	for _, uri := range nc.PermittedURIs {
		out.PermittedURIs = append(out.PermittedURIs, uri.Data)
	}
	out.PermittedIPAddresses = nc.PermittedIPAddresses
	for _, directory := range nc.PermittedDirectoryNames {
		out.PermittedDirectoryNames = append(out.PermittedDirectoryNames, directory.Data)
	}
	for _, edi := range nc.PermittedEdiPartyNames {
		out.PermittedEdiPartyNames = append(out.PermittedEdiPartyNames, edi.Data)
	}
	for _, id := range nc.PermittedRegisteredIDs {
		out.PermittedRegisteredIDs = append(out.PermittedRegisteredIDs, id.Data.String())
	}

	for _, dns := range nc.ExcludedDNSNames {
		out.ExcludedDNSNames = append(out.ExcludedDNSNames, dns.Data)
	}
	for _, email := range nc.ExcludedEmailAddresses {
		out.ExcludedEmailAddresses = append(out.ExcludedEmailAddresses, email.Data)
	}
	for _, uri := range nc.ExcludedURIs {
		out.ExcludedURIs = append(out.ExcludedURIs, uri.Data)
	}
	for _, ip := range nc.ExcludedIPAddresses {
		out.ExcludedIPAddresses = append(out.ExcludedIPAddresses, ip)
	}
	for _, directory := range nc.ExcludedDirectoryNames {
		out.ExcludedDirectoryNames = append(out.ExcludedDirectoryNames, directory.Data)
	}
	for _, edi := range nc.ExcludedEdiPartyNames {
		out.ExcludedEdiPartyNames = append(out.ExcludedEdiPartyNames, edi.Data)
	}
	for _, id := range nc.ExcludedRegisteredIDs {
		out.ExcludedRegisteredIDs = append(out.ExcludedRegisteredIDs, id.Data.String())
	}
	return json.Marshal(out)
}

type CRLDistributionPoints []string

type SubjAuthKeyId []byte

func (kid SubjAuthKeyId) MarshalJSON() ([]byte, error) {
	enc := hex.EncodeToString(kid)
	return json.Marshal(enc)
}

type ExtendedKeyUsage []ExtKeyUsage

type ExtendedKeyUsageExtension struct {
	Known   ExtendedKeyUsage
	Unknown []asn1.ObjectIdentifier
}

// MarshalJSON implements the json.Marshal interface. The output is a struct of
// bools, with an additional `Value` field containing the actual OIDs.
func (e *ExtendedKeyUsageExtension) MarshalJSON() ([]byte, error) {
	aux := new(auxExtendedKeyUsage)
	for _, e := range e.Known {
		aux.populateFromExtKeyUsage(e)
	}
	for _, oid := range e.Unknown {
		aux.Unknown = append(aux.Unknown, oid.String())
	}
	return json.Marshal(aux)
}

func (e *ExtendedKeyUsageExtension) UnmarshalJSON(b []byte) error {
	aux := new(auxExtendedKeyUsage)
	if err := json.Unmarshal(b, aux); err != nil {
		return err
	}
	// TODO: Generate the reverse functions.
	return nil
}

//go:generate go run extended_key_usage_gen.go

// The string functions for CertValidationLevel are auto-generated via
// `go generate <full_path_to_x509_package>` or running `go generate` in the package directory
//
//go:generate stringer -type=CertValidationLevel -output=generated_certvalidationlevel_string.go
type CertValidationLevel int

const (
	UnknownValidationLevel CertValidationLevel = 0
	DV                     CertValidationLevel = 1
	OV                     CertValidationLevel = 2
	EV                     CertValidationLevel = 3
)

func (c *CertValidationLevel) MarshalJSON() ([]byte, error) {
	if *c == UnknownValidationLevel || *c < 0 || *c > EV {
		return json.Marshal("unknown")
	}
	return json.Marshal(c.String())
}

// TODO: All of validation-level maps should be auto-generated from
// https://github.com/zmap/constants.

// ExtendedValidationOIDs contains the UNION of Chromium
// (https://chromium.googlesource.com/chromium/src/net/+/master/cert/ev_root_ca_metadata.cc)
// and Firefox
// (http://hg.mozilla.org/mozilla-central/file/tip/security/certverifier/ExtendedValidation.cpp)
// EV OID lists
var ExtendedValidationOIDs = map[string]interface{}{
	// CA/Browser Forum EV OID standard
	// https://cabforum.org/object-registry/
	"2.23.140.1.1": nil,
	// CA/Browser Forum EV Code Signing
	"2.23.140.1.3": nil,
	// CA/Browser Forum .onion EV Certs
	"2.23.140.1.31": nil,
	// AC Camerfirma S.A. Chambers of Commerce Root - 2008
	// https://www.camerfirma.com
	// AC Camerfirma uses the last two arcs to track how the private key
	// is managed - the effective verification policy is the same.
	"1.3.6.1.4.1.17326.10.14.2.1.2": nil,
	"1.3.6.1.4.1.17326.10.14.2.2.2": nil,
	// AC Camerfirma S.A. Global Chambersign Root - 2008
	// https://server2.camerfirma.com:8082
	// AC Camerfirma uses the last two arcs to track how the private key
	// is managed - the effective verification policy is the same.
	"1.3.6.1.4.1.17326.10.8.12.1.2": nil,
	"1.3.6.1.4.1.17326.10.8.12.2.2": nil,
	// Actalis Authentication Root CA
	// https://ssltest-a.actalis.it:8443
	"1.3.159.1.17.1": nil,
	// AffirmTrust Commercial
	// https://commercial.affirmtrust.com/
	"1.3.6.1.4.1.34697.2.1": nil,
	// AffirmTrust Networking
	// https://networking.affirmtrust.com:4431
	"1.3.6.1.4.1.34697.2.2": nil,
	// AffirmTrust Premium
	// https://premium.affirmtrust.com:4432/
	"1.3.6.1.4.1.34697.2.3": nil,
	// AffirmTrust Premium ECC
	// https://premiumecc.affirmtrust.com:4433/
	"1.3.6.1.4.1.34697.2.4": nil,
	// Autoridad de Certificacion Firmaprofesional CIF A62634068
	// https://publifirma.firmaprofesional.com/
	"1.3.6.1.4.1.13177.10.1.3.10": nil,
	// Buypass Class 3 CA 1
	// https://valid.evident.ca13.ssl.buypass.no/
	"2.16.578.1.26.1.3.3": nil,
	// Certification Authority of WoSign
	// CA 沃通根证书
	// https://root2evtest.wosign.com/
	"1.3.6.1.4.1.36305.2": nil,
	// CertPlus Class 2 Primary CA (KEYNECTIS)
	// https://www.keynectis.com/
	"1.3.6.1.4.1.22234.2.5.2.3.1": nil,
	// Certum Trusted Network CA
	// https://juice.certum.pl/
	"1.2.616.1.113527.2.5.1.1": nil,
	// China Internet Network Information Center EV Certificates Root
	// https://evdemo.cnnic.cn/
	"1.3.6.1.4.1.29836.1.10": nil,
	// COMODO Certification Authority & USERTrust RSA Certification Authority & UTN-USERFirst-Hardware & AddTrust External CA Root
	// https://secure.comodo.com/
	// https://usertrustrsacertificationauthority-ev.comodoca.com/
	// https://addtrustexternalcaroot-ev.comodoca.com
	"1.3.6.1.4.1.6449.1.2.1.5.1": nil,
	// Cybertrust Global Root & GTE CyberTrust Global Root & Baltimore CyberTrust Root
	// https://evup.cybertrust.ne.jp/ctj-ev-upgrader/evseal.gif
	// https://www.cybertrust.ne.jp/
	// https://secure.omniroot.com/repository/
	"1.3.6.1.4.1.6334.1.100.1": nil,
	// DigiCert High Assurance EV Root CA
	// https://www.digicert.com
	"2.16.840.1.114412.2.1": nil,
	// D-TRUST Root Class 3 CA 2 EV 2009
	// https://certdemo-ev-valid.ssl.d-trust.net/
	"1.3.6.1.4.1.4788.2.202.1": nil,
	// Entrust.net Secure Server Certification Authority
	// https://www.entrust.net/
	"2.16.840.1.114028.10.1.2": nil,
	// E-Tugra Certification Authority
	// https://sslev.e-tugra.com.tr
	"2.16.792.3.0.4.1.1.4": nil,
	// GeoTrust Primary Certification Authority
	// https://www.geotrust.com/
	"1.3.6.1.4.1.14370.1.6": nil,
	// GlobalSign Root CA - R2
	// https://www.globalsign.com/
	"1.3.6.1.4.1.4146.1.1": nil,
	// Go Daddy Class 2 Certification Authority & Go Daddy Root Certificate Authority - G2
	// https://www.godaddy.com/
	// https://valid.gdig2.catest.godaddy.com/
	"2.16.840.1.114413.1.7.23.3": nil,
	// Izenpe.com - SHA256 root
	// The first OID is for businesses and the second for government entities.
	// These are the test sites, respectively:
	// https://servicios.izenpe.com
	// https://servicios1.izenpe.com
	// Windows XP finds this, SHA1, root instead. The policy OIDs are the same
	// as for the SHA256 root, above.
	"1.3.6.1.4.1.14777.6.1.1": nil,
	"1.3.6.1.4.1.14777.6.1.2": nil,
	// Network Solutions Certificate Authority
	// https://www.networksolutions.com/website-packages/index.jsp
	"1.3.6.1.4.1.782.1.2.1.8.1": nil,
	// QuoVadis Root CA 2
	// https://www.quovadis.bm/
	"1.3.6.1.4.1.8024.0.2.100.1.2": nil,
	// SecureTrust CA, SecureTrust Corporation
	// https://www.securetrust.com
	// https://www.trustwave.com/
	"2.16.840.1.114404.1.1.2.4.1": nil,
	// Security Communication RootCA1
	// https://www.secomtrust.net/contact/form.html
	"1.2.392.200091.100.721.1": nil,
	// Staat der Nederlanden EV Root CA
	// https://pkioevssl-v.quovadisglobal.com/
	"2.16.528.1.1003.1.2.7": nil,
	// StartCom Certification Authority
	// https://www.startssl.com/
	"1.3.6.1.4.1.23223.1.1.1": nil,
	// Starfield Class 2 Certification Authority
	// https://www.starfieldtech.com/
	"2.16.840.1.114414.1.7.23.3": nil,
	// Starfield Services Root Certificate Authority - G2
	// https://valid.sfsg2.catest.starfieldtech.com/
	"2.16.840.1.114414.1.7.24.3": nil,
	// SwissSign Gold CA - G2
	// https://testevg2.swisssign.net/
	"2.16.756.1.89.1.2.1.1": nil,
	// Swisscom Root EV CA 2
	// https://test-quarz-ev-ca-2.pre.swissdigicert.ch
	"2.16.756.1.83.21.0": nil,
	// thawte Primary Root CA
	// https://www.thawte.com/
	"2.16.840.1.113733.1.7.48.1": nil,
	// TWCA Global Root CA
	// https://evssldemo3.twca.com.tw/index.html
	"1.3.6.1.4.1.40869.1.1.22.3": nil,
	// T-TeleSec GlobalRoot Class 3
	// http://www.telesec.de/ / https://root-class3.test.telesec.de/
	"1.3.6.1.4.1.7879.13.24.1": nil,
	// VeriSign Class 3 Public Primary Certification Authority - G5
	// https://www.verisign.com/
	"2.16.840.1.113733.1.7.23.6": nil,
	// Wells Fargo WellsSecure Public Root Certificate Authority
	// https://nerys.wellsfargo.com/test.html
	"2.16.840.1.114171.500.9": nil,
	// CN=CFCA EV ROOT,O=China Financial Certification Authority,C=CN
	// https://www.cfca.com.cn/
	"2.16.156.112554.3": nil,
	// CN=OISTE WISeKey Global Root GB CA,OU=OISTE Foundation Endorsed,O=WISeKey,C=CH
	// https://www.wisekey.com/repository/cacertificates/
	"2.16.756.5.14.7.4.8": nil,
	// CN=TÜRKTRUST Elektronik Sertifika Hizmet Sağlayıcısı H6,O=TÜRKTRUST Bilgi İletişim ve Bilişim Güvenliği Hizmetleri A...,L=Ankara,C=TR
	// https://www.turktrust.com.tr/
	"2.16.792.3.0.3.1.1.5": nil,
}

// OrganizationValidationOIDs contains CA specific OV OIDs from
// https://cabforum.org/object-registry/
var OrganizationValidationOIDs = map[string]interface{}{
	// CA/Browser Forum OV OID standard
	// https://cabforum.org/object-registry/
	"2.23.140.1.2.2": nil,
	// CA/Browser Forum individually validated
	"2.23.140.1.2.3": nil,
	// Digicert
	"2.16.840.1.114412.1.1": nil,
	// D-Trust
	"1.3.6.1.4.1.4788.2.200.1": nil,
	// GoDaddy
	"2.16.840.1.114413.1.7.23.2": nil,
	// Logius
	"2.16.528.1.1003.1.2.5.6": nil,
	// QuoVadis
	"1.3.6.1.4.1.8024.0.2.100.1.1": nil,
	// Starfield
	"2.16.840.1.114414.1.7.23.2": nil,
	// TurkTrust
	"2.16.792.3.0.3.1.1.2": nil,
}

// DomainValidationOIDs contain OIDs that identify DV certs.
var DomainValidationOIDs = map[string]interface{}{
	// Globalsign
	"1.3.6.1.4.1.4146.1.10.10": nil,
	// Let's Encrypt
	"1.3.6.1.4.1.44947.1.1.1": nil,
	// Comodo (eNom)
	"1.3.6.1.4.1.6449.1.2.2.10": nil,
	// Comodo (WoTrust)
	"1.3.6.1.4.1.6449.1.2.2.15": nil,
	// Comodo (RBC SOFT)
	"1.3.6.1.4.1.6449.1.2.2.16": nil,
	// Comodo (RegisterFly)
	"1.3.6.1.4.1.6449.1.2.2.17": nil,
	// Comodo (Central Security Patrols)
	"1.3.6.1.4.1.6449.1.2.2.18": nil,
	// Comodo (eBiz Networks)
	"1.3.6.1.4.1.6449.1.2.2.19": nil,
	// Comodo (OptimumSSL)
	"1.3.6.1.4.1.6449.1.2.2.21": nil,
	// Comodo (WoSign)
	"1.3.6.1.4.1.6449.1.2.2.22": nil,
	// Comodo (Register.com)
	"1.3.6.1.4.1.6449.1.2.2.24": nil,
	// Comodo (The Code Project)
	"1.3.6.1.4.1.6449.1.2.2.25": nil,
	// Comodo (Gandi)
	"1.3.6.1.4.1.6449.1.2.2.26": nil,
	// Comodo (GlobeSSL)
	"1.3.6.1.4.1.6449.1.2.2.27": nil,
	// Comodo (DreamHost)
	"1.3.6.1.4.1.6449.1.2.2.28": nil,
	// Comodo (TERENA)
	"1.3.6.1.4.1.6449.1.2.2.29": nil,
	// Comodo (GlobalSSL)
	"1.3.6.1.4.1.6449.1.2.2.31": nil,
	// Comodo (IceWarp)
	"1.3.6.1.4.1.6449.1.2.2.35": nil,
	// Comodo (Dotname Korea)
	"1.3.6.1.4.1.6449.1.2.2.37": nil,
	// Comodo (TrustSign)
	"1.3.6.1.4.1.6449.1.2.2.38": nil,
	// Comodo (Formidable)
	"1.3.6.1.4.1.6449.1.2.2.39": nil,
	// Comodo (SSL Blindado)
	"1.3.6.1.4.1.6449.1.2.2.40": nil,
	// Comodo (Dreamscape Networks)
	"1.3.6.1.4.1.6449.1.2.2.41": nil,
	// Comodo (K Software)
	"1.3.6.1.4.1.6449.1.2.2.42": nil,
	// Comodo (FBS)
	"1.3.6.1.4.1.6449.1.2.2.44": nil,
	// Comodo (ReliaSite)
	"1.3.6.1.4.1.6449.1.2.2.45": nil,
	// Comodo (CertAssure)
	"1.3.6.1.4.1.6449.1.2.2.47": nil,
	// Comodo (TrustAsia)
	"1.3.6.1.4.1.6449.1.2.2.49": nil,
	// Comodo (SecureCore)
	"1.3.6.1.4.1.6449.1.2.2.50": nil,
	// Comodo (Western Digital)
	"1.3.6.1.4.1.6449.1.2.2.51": nil,
	// Comodo (cPanel)
	"1.3.6.1.4.1.6449.1.2.2.52": nil,
	// Comodo (BlackCert)
	"1.3.6.1.4.1.6449.1.2.2.53": nil,
	// Comodo (KeyNet Systems)
	"1.3.6.1.4.1.6449.1.2.2.54": nil,
	// Comodo
	"1.3.6.1.4.1.6449.1.2.2.7": nil,
	// Comodo (CSC)
	"1.3.6.1.4.1.6449.1.2.2.8": nil,
	// Digicert
	"2.16.840.1.114412.1.2": nil,
	// GoDaddy
	"2.16.840.1.114413.1.7.23.1": nil,
	// Starfield
	"2.16.840.1.114414.1.7.23.1": nil,
	// CA/B Forum
	"2.23.140.1.2.1": nil,
}

// TODO pull out other types
type AuthorityInfoAccess struct {
	OCSPServer            []string `json:"ocsp_urls,omitempty"`
	IssuingCertificateURL []string `json:"issuer_urls,omitempty"`
}

/*
	    id-CABFOrganizationIdentifier OBJECT IDENTIFIER ::= { joint-iso-itu-t(2) international-organizations(23) ca-browser-forum(140) certificate-extensions(3) cabf-organization-identifier(1) }

	    ext-CABFOrganizationIdentifier EXTENSION ::= { SYNTAX CABFOrganizationIdentifier IDENTIFIED BY id-CABFOrganizationIdentifier }

	    CABFOrganizationIdentifier ::= SEQUENCE {

	        registrationSchemeIdentifier   PrintableString (SIZE(3)),

	        registrationCountry            PrintableString (SIZE(2)),

	        registrationStateOrProvince    [0] IMPLICIT PrintableString OPTIONAL (SIZE(0..128)),

	        registrationReference          UTF8String

		}
*/
type CABFOrganizationIDASN struct {
	RegistrationSchemeIdentifier string `asn1:"printable"`
	RegistrationCountry          string `asn1:"printable"`
	RegistrationStateOrProvince  string `asn1:"printable,optional,tag:0"`
	RegistrationReference        string `asn1:"utf8"`
}

type CABFOrganizationIdentifier struct {
	Scheme    string `json:"scheme,omitempty"`
	Country   string `json:"country,omitempty"`
	State     string `json:"state,omitempty"`
	Reference string `json:"reference,omitempty"`
}

func (c *Certificate) jsonifyExtensions() (*CertificateExtensions, UnknownCertificateExtensions) {
	exts := new(CertificateExtensions)
	unk := make([]pkix.Extension, 0, 2)
	for _, e := range c.Extensions {
		if e.Id.Equal(oidExtKeyUsage) {
			exts.KeyUsage = c.KeyUsage
		} else if e.Id.Equal(oidExtBasicConstraints) {
			exts.BasicConstraints = new(BasicConstraints)
			exts.BasicConstraints.IsCA = c.IsCA
			if c.MaxPathLen > 0 || c.MaxPathLenZero {
				exts.BasicConstraints.MaxPathLen = new(int)
				*exts.BasicConstraints.MaxPathLen = c.MaxPathLen
			}
		} else if e.Id.Equal(oidExtSubjectAltName) {
			exts.SubjectAltName = new(GeneralNames)
			exts.SubjectAltName.DirectoryNames = c.DirectoryNames
			exts.SubjectAltName.DNSNames = c.DNSNames
			exts.SubjectAltName.EDIPartyNames = c.EDIPartyNames
			exts.SubjectAltName.EmailAddresses = c.EmailAddresses
			exts.SubjectAltName.IPAddresses = c.IPAddresses
			exts.SubjectAltName.OtherNames = c.OtherNames
			exts.SubjectAltName.RegisteredIDs = c.RegisteredIDs
			exts.SubjectAltName.URIs = c.URIs
		} else if e.Id.Equal(oidExtIssuerAltName) {
			exts.IssuerAltName = new(GeneralNames)
			exts.IssuerAltName.DirectoryNames = c.IANDirectoryNames
			exts.IssuerAltName.DNSNames = c.IANDNSNames
			exts.IssuerAltName.EDIPartyNames = c.IANEDIPartyNames
			exts.IssuerAltName.EmailAddresses = c.IANEmailAddresses
			exts.IssuerAltName.IPAddresses = c.IANIPAddresses
			exts.IssuerAltName.OtherNames = c.IANOtherNames
			exts.IssuerAltName.RegisteredIDs = c.IANRegisteredIDs
			exts.IssuerAltName.URIs = c.IANURIs
		} else if e.Id.Equal(oidExtNameConstraints) {
			exts.NameConstraints = new(NameConstraints)
			exts.NameConstraints.Critical = c.NameConstraintsCritical

			exts.NameConstraints.PermittedDNSNames = c.PermittedDNSNames
			exts.NameConstraints.PermittedEmailAddresses = c.PermittedEmailAddresses
			exts.NameConstraints.PermittedURIs = c.PermittedURIs
			exts.NameConstraints.PermittedIPAddresses = c.PermittedIPAddresses
			exts.NameConstraints.PermittedDirectoryNames = c.PermittedDirectoryNames
			exts.NameConstraints.PermittedEdiPartyNames = c.PermittedEdiPartyNames
			exts.NameConstraints.PermittedRegisteredIDs = c.PermittedRegisteredIDs

			exts.NameConstraints.ExcludedEmailAddresses = c.ExcludedEmailAddresses
			exts.NameConstraints.ExcludedDNSNames = c.ExcludedDNSNames
			exts.NameConstraints.ExcludedURIs = c.ExcludedURIs
			exts.NameConstraints.ExcludedIPAddresses = c.ExcludedIPAddresses
			exts.NameConstraints.ExcludedDirectoryNames = c.ExcludedDirectoryNames
			exts.NameConstraints.ExcludedEdiPartyNames = c.ExcludedEdiPartyNames
			exts.NameConstraints.ExcludedRegisteredIDs = c.ExcludedRegisteredIDs
		} else if e.Id.Equal(oidCRLDistributionPoints) {
			exts.CRLDistributionPoints = c.CRLDistributionPoints
		} else if e.Id.Equal(oidExtAuthKeyId) {
			exts.AuthKeyID = c.AuthorityKeyId
		} else if e.Id.Equal(oidExtExtendedKeyUsage) {
			exts.ExtendedKeyUsage = new(ExtendedKeyUsageExtension)
			exts.ExtendedKeyUsage.Known = c.ExtKeyUsage
			exts.ExtendedKeyUsage.Unknown = c.UnknownExtKeyUsage
		} else if e.Id.Equal(oidExtCertificatePolicy) {
			exts.CertificatePolicies = new(CertificatePoliciesData)
			exts.CertificatePolicies.PolicyIdentifiers = c.PolicyIdentifiers
			exts.CertificatePolicies.NoticeRefNumbers = c.NoticeRefNumbers
			exts.CertificatePolicies.NoticeRefOrganization = c.ParsedNoticeRefOrganization
			exts.CertificatePolicies.ExplicitTexts = c.ParsedExplicitTexts
			exts.CertificatePolicies.QualifierId = c.QualifierId
			exts.CertificatePolicies.CPSUri = c.CPSuri

		} else if e.Id.Equal(oidExtAuthorityInfoAccess) {
			exts.AuthorityInfoAccess = new(AuthorityInfoAccess)
			exts.AuthorityInfoAccess.OCSPServer = c.OCSPServer
			exts.AuthorityInfoAccess.IssuingCertificateURL = c.IssuingCertificateURL
		} else if e.Id.Equal(oidExtSubjectKeyId) {
			exts.SubjectKeyID = c.SubjectKeyId
		} else if e.Id.Equal(oidExtSignedCertificateTimestampList) {
			exts.SignedCertificateTimestampList = c.SignedCertificateTimestampList
		} else if e.Id.Equal(oidExtensionCTPrecertificatePoison) {
			exts.IsPrecert = true
		} else if e.Id.Equal(oidBRTorServiceDescriptor) {
			exts.TorServiceDescriptors = c.TorServiceDescriptors
		} else if e.Id.Equal(oidExtCABFOrganizationID) {
			exts.CABFOrganizationIdentifier = c.CABFOrganizationIdentifier
		} else if e.Id.Equal(oidExtQCStatements) {
			exts.QCStatements = c.QCStatements
		} else {
			// Unknown extension
			unk = append(unk, e)
		}
	}
	return exts, unk
}
