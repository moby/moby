// Copyright 2017 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package x509util

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509/pkix"
)

// RevocationReasonToString generates a string describing a revocation reason code.
func RevocationReasonToString(reason x509.RevocationReasonCode) string {
	switch reason {
	case x509.Unspecified:
		return "Unspecified"
	case x509.KeyCompromise:
		return "Key Compromise"
	case x509.CACompromise:
		return "CA Compromise"
	case x509.AffiliationChanged:
		return "Affiliation Changed"
	case x509.Superseded:
		return "Superseded"
	case x509.CessationOfOperation:
		return "Cessation Of Operation"
	case x509.CertificateHold:
		return "Certificate Hold"
	case x509.RemoveFromCRL:
		return "Remove From CRL"
	case x509.PrivilegeWithdrawn:
		return "Privilege Withdrawn"
	case x509.AACompromise:
		return "AA Compromise"
	default:
		return strconv.Itoa(int(reason))
	}
}

// CRLToString generates a string describing the given certificate revocation list.
// The output roughly resembles that from openssl crl -text.
func CRLToString(crl *x509.CertificateList) string {
	var result bytes.Buffer
	var showCritical = func(critical bool) {
		if critical {
			result.WriteString(" critical")
		}
		result.WriteString("\n")
	}
	result.WriteString("Certificate Revocation List (CRL):\n")
	result.WriteString(fmt.Sprintf("        Version: %d (%#x)\n", crl.TBSCertList.Version+1, crl.TBSCertList.Version))
	result.WriteString(fmt.Sprintf("    Signature Algorithm: %v\n", x509.SignatureAlgorithmFromAI(crl.TBSCertList.Signature)))
	var issuer pkix.Name
	issuer.FillFromRDNSequence(&crl.TBSCertList.Issuer)
	result.WriteString(fmt.Sprintf("        Issuer: %v\n", NameToString(issuer)))
	result.WriteString(fmt.Sprintf("        Last Update: %v\n", crl.TBSCertList.ThisUpdate))
	result.WriteString(fmt.Sprintf("        Next Update: %v\n", crl.TBSCertList.NextUpdate))

	if len(crl.TBSCertList.Extensions) > 0 {
		result.WriteString("        CRL extensions:\n")
	}

	count, critical := OIDInExtensions(x509.OIDExtensionAuthorityKeyId, crl.TBSCertList.Extensions)
	if count > 0 {
		result.WriteString("            X509v3 Authority Key Identifier:")
		showCritical(critical)
		result.WriteString(fmt.Sprintf("                keyid:%v\n", hex.EncodeToString(crl.TBSCertList.AuthorityKeyID)))
	}
	count, critical = OIDInExtensions(x509.OIDExtensionIssuerAltName, crl.TBSCertList.Extensions)
	if count > 0 {
		result.WriteString("            X509v3 Issuer Alt Name:")
		showCritical(critical)
		result.WriteString(fmt.Sprintf("                %s\n", GeneralNamesToString(&crl.TBSCertList.IssuerAltNames)))
	}
	count, critical = OIDInExtensions(x509.OIDExtensionCRLNumber, crl.TBSCertList.Extensions)
	if count > 0 {
		result.WriteString("            X509v3 CRLNumber:")
		showCritical(critical)
		result.WriteString(fmt.Sprintf("                %d\n", crl.TBSCertList.CRLNumber))
	}
	count, critical = OIDInExtensions(x509.OIDExtensionDeltaCRLIndicator, crl.TBSCertList.Extensions)
	if count > 0 {
		result.WriteString("            X509v3 Delta CRL Indicator:")
		showCritical(critical)
		result.WriteString(fmt.Sprintf("                %d\n", crl.TBSCertList.BaseCRLNumber))
	}
	count, critical = OIDInExtensions(x509.OIDExtensionIssuingDistributionPoint, crl.TBSCertList.Extensions)
	if count > 0 {
		result.WriteString("            X509v3 Issuing Distribution Point:")
		showCritical(critical)
		result.WriteString(fmt.Sprintf("                %s\n", GeneralNamesToString(&crl.TBSCertList.IssuingDPFullNames)))
	}
	count, critical = OIDInExtensions(x509.OIDExtensionFreshestCRL, crl.TBSCertList.Extensions)
	if count > 0 {
		result.WriteString("            X509v3 Freshest CRL:")
		showCritical(critical)
		result.WriteString("                Full Name:\n")
		var buf bytes.Buffer
		for _, pt := range crl.TBSCertList.FreshestCRLDistributionPoint {
			commaAppend(&buf, "URI:"+pt)
		}
		result.WriteString(fmt.Sprintf("                    %v\n", buf.String()))
	}
	count, critical = OIDInExtensions(x509.OIDExtensionAuthorityInfoAccess, crl.TBSCertList.Extensions)
	if count > 0 {
		result.WriteString("            Authority Information Access:")
		showCritical(critical)
		var issuerBuf bytes.Buffer
		for _, issuer := range crl.TBSCertList.IssuingCertificateURL {
			commaAppend(&issuerBuf, "URI:"+issuer)
		}
		if issuerBuf.Len() > 0 {
			result.WriteString(fmt.Sprintf("                CA Issuers - %v\n", issuerBuf.String()))
		}
		var ocspBuf bytes.Buffer
		for _, ocsp := range crl.TBSCertList.OCSPServer {
			commaAppend(&ocspBuf, "URI:"+ocsp)
		}
		if ocspBuf.Len() > 0 {
			result.WriteString(fmt.Sprintf("                OCSP - %v\n", ocspBuf.String()))
		}
		// TODO(drysdale): Display other GeneralName types
	}

	result.WriteString("\n")
	result.WriteString("Revoked Certificates:\n")
	for _, c := range crl.TBSCertList.RevokedCertificates {
		result.WriteString(fmt.Sprintf("    Serial Number: %s (0x%s)\n", c.SerialNumber.Text(10), c.SerialNumber.Text(16)))
		result.WriteString(fmt.Sprintf("        Revocation Date : %v\n", c.RevocationTime))
		count, critical = OIDInExtensions(x509.OIDExtensionCRLReasons, c.Extensions)
		if count > 0 {
			result.WriteString("            X509v3 CRL Reason Code:")
			showCritical(critical)
			result.WriteString(fmt.Sprintf("                %s\n", RevocationReasonToString(c.RevocationReason)))
		}
		count, critical = OIDInExtensions(x509.OIDExtensionInvalidityDate, c.Extensions)
		if count > 0 {
			result.WriteString("        Invalidity Date:")
			showCritical(critical)
			result.WriteString(fmt.Sprintf("                %s\n", c.InvalidityDate))
		}
		count, critical = OIDInExtensions(x509.OIDExtensionCertificateIssuer, c.Extensions)
		if count > 0 {
			result.WriteString("        Issuer:")
			showCritical(critical)
			result.WriteString(fmt.Sprintf("                %s\n", GeneralNamesToString(&c.Issuer)))
		}
	}
	result.WriteString(fmt.Sprintf("    Signature Algorithm: %v\n", x509.SignatureAlgorithmFromAI(crl.SignatureAlgorithm)))
	appendHexData(&result, crl.SignatureValue.Bytes, 18, "         ")
	result.WriteString("\n")

	return result.String()
}
