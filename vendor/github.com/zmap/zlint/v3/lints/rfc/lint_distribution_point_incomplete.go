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
	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zcrypto/x509/pkix"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type distributionPoint struct {
	DistributionPoint distributionPointName `asn1:"optional,tag:0"`
	Reason            asn1.BitString        `asn1:"optional,tag:1"`
	CRLIssuer         asn1.RawValue         `asn1:"optional,tag:2"`
}

type distributionPointName struct {
	FullName     asn1.RawValue    `asn1:"optional,tag:0"`
	RelativeName pkix.RDNSequence `asn1:"optional,tag:1"`
}

type dpIncomplete struct{}

/********************************************************************
The cRLDistributionPoints extension is a SEQUENCE of
DistributionPoint.  A DistributionPoint consists of three fields,
each of which is optional: distributionPoint, reasons, and cRLIssuer.
While each of these fields is optional, a DistributionPoint MUST NOT
consist of only the reasons field; either distributionPoint or
cRLIssuer MUST be present.  If the certificate issuer is not the CRL
issuer, then the cRLIssuer field MUST be present and contain the Name
of the CRL issuer.  If the certificate issuer is also the CRL issuer,
then conforming CAs MUST omit the cRLIssuer field and MUST include
the distributionPoint field.
********************************************************************/

func init() {
	lint.RegisterLint(&lint.Lint{
		Name:          "e_distribution_point_incomplete",
		Description:   "A DistributionPoint from the CRLDistributionPoints extension MUST NOT consist of only the reasons field; either distributionPoint or CRLIssuer must be present",
		Citation:      "RFC 5280: 4.2.1.13",
		Source:        lint.RFC5280,
		EffectiveDate: util.RFC3280Date,
		Lint:          NewDpIncomplete,
	})
}

func NewDpIncomplete() lint.LintInterface {
	return &dpIncomplete{}
}

func (l *dpIncomplete) CheckApplies(c *x509.Certificate) bool {
	return util.IsExtInCert(c, util.CrlDistOID)
}

func (l *dpIncomplete) Execute(c *x509.Certificate) *lint.LintResult {
	dp := util.GetExtFromCert(c, util.CrlDistOID)
	var cdp []distributionPoint
	_, err := asn1.Unmarshal(dp.Value, &cdp)
	if err != nil {
		return &lint.LintResult{Status: lint.Fatal}
	}
	for _, dp := range cdp {
		if dp.Reason.BitLength != 0 && len(dp.DistributionPoint.FullName.Bytes) == 0 &&
			dp.DistributionPoint.RelativeName == nil && len(dp.CRLIssuer.Bytes) == 0 {
			return &lint.LintResult{Status: lint.Error}
		}
	}
	return &lint.LintResult{Status: lint.Pass}
}
