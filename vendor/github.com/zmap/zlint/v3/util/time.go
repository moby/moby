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
	"time"

	"github.com/zmap/zcrypto/x509"
)

var (
	ZeroDate                   = time.Date(0000, time.January, 1, 0, 0, 0, 0, time.UTC)
	RFC1035Date                = time.Date(1987, time.January, 1, 0, 0, 0, 0, time.UTC)
	RFC2459Date                = time.Date(1999, time.January, 1, 0, 0, 0, 0, time.UTC)
	RFC3280Date                = time.Date(2002, time.April, 1, 0, 0, 0, 0, time.UTC)
	RFC3490Date                = time.Date(2003, time.March, 1, 0, 0, 0, 0, time.UTC)
	RFC8399Date                = time.Date(2018, time.May, 1, 0, 0, 0, 0, time.UTC)
	RFC4325Date                = time.Date(2005, time.December, 1, 0, 0, 0, 0, time.UTC)
	RFC4630Date                = time.Date(2006, time.August, 1, 0, 0, 0, 0, time.UTC)
	RFC5280Date                = time.Date(2008, time.May, 1, 0, 0, 0, 0, time.UTC)
	RFC6818Date                = time.Date(2013, time.January, 1, 0, 0, 0, 0, time.UTC)
	CABEffectiveDate           = time.Date(2012, time.July, 1, 0, 0, 0, 0, time.UTC)
	CABReservedIPDate          = time.Date(2016, time.October, 1, 0, 0, 0, 0, time.UTC)
	CABGivenNameDate           = time.Date(2016, time.September, 7, 0, 0, 0, 0, time.UTC)
	CABSerialNumberEntropyDate = time.Date(2016, time.September, 30, 0, 0, 0, 0, time.UTC)
	CABV102Date                = time.Date(2012, time.June, 8, 0, 0, 0, 0, time.UTC)
	CABV113Date                = time.Date(2013, time.February, 21, 0, 0, 0, 0, time.UTC)
	CABV114Date                = time.Date(2013, time.May, 3, 0, 0, 0, 0, time.UTC)
	CABV116Date                = time.Date(2013, time.July, 29, 0, 0, 0, 0, time.UTC)
	CABV130Date                = time.Date(2015, time.April, 16, 0, 0, 0, 0, time.UTC)
	CABV131Date                = time.Date(2015, time.September, 28, 0, 0, 0, 0, time.UTC)
	// https://cabforum.org/wp-content/uploads/CA-Browser-Forum-EV-Guidelines-v1.7.0.pdf
	CABV170Date                 = time.Date(2020, time.January, 31, 0, 0, 0, 0, time.UTC)
	NO_SHA1                     = time.Date(2016, time.January, 1, 0, 0, 0, 0, time.UTC)
	NoRSA1024RootDate           = time.Date(2011, time.January, 1, 0, 0, 0, 0, time.UTC)
	NoRSA1024Date               = time.Date(2014, time.January, 1, 0, 0, 0, 0, time.UTC)
	GeneralizedDate             = time.Date(2050, time.January, 1, 0, 0, 0, 0, time.UTC)
	NoReservedIP                = time.Date(2015, time.November, 1, 0, 0, 0, 0, time.UTC)
	SubCert39Month              = time.Date(2016, time.July, 2, 0, 0, 0, 0, time.UTC)
	SubCert825Days              = time.Date(2018, time.March, 2, 0, 0, 0, 0, time.UTC)
	CABV148Date                 = time.Date(2017, time.June, 8, 0, 0, 0, 0, time.UTC)
	EtsiEn319_412_5_V2_2_1_Date = time.Date(2017, time.November, 1, 0, 0, 0, 0, time.UTC)
	OnionOnlyEVDate             = time.Date(2015, time.May, 1, 0, 0, 0, 0, time.UTC)
	CABV201Date                 = time.Date(2017, time.July, 28, 0, 0, 0, 0, time.UTC)
	AppleCTPolicyDate           = time.Date(2018, time.October, 15, 0, 0, 0, 0, time.UTC)
	MozillaPolicy22Date         = time.Date(2013, time.July, 26, 0, 0, 0, 0, time.UTC)
	MozillaPolicy24Date         = time.Date(2017, time.February, 28, 0, 0, 0, 0, time.UTC)
	MozillaPolicy27Date         = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	CABFBRs_1_6_9_Date          = time.Date(2020, time.March, 27, 0, 0, 0, 0, time.UTC)
	AppleReducedLifetimeDate    = time.Date(2020, time.September, 1, 0, 0, 0, 0, time.UTC)
)

var (
	CABFEV_9_8_2 = CABV170Date
)

func FindTimeType(firstDate, secondDate asn1.RawValue) (int, int) {
	return firstDate.Tag, secondDate.Tag
}

// TODO(@cpu): This function is a little bit rough around the edges (especially
// after my quick fixes for the ineffassigns) and would be a good candidate for
// clean-up/refactoring.
func GetTimes(cert *x509.Certificate) (asn1.RawValue, asn1.RawValue) {
	var outSeq, firstDate, secondDate asn1.RawValue
	// Unmarshal into the sequence
	_, err := asn1.Unmarshal(cert.RawTBSCertificate, &outSeq)
	if err != nil {
		return asn1.RawValue{}, asn1.RawValue{}
	}
	// Start unmarshalling the bytes
	rest, err := asn1.Unmarshal(outSeq.Bytes, &outSeq)
	if err != nil {
		return asn1.RawValue{}, asn1.RawValue{}
	}
	// This is here to account for if version is not included
	if outSeq.Tag == 0 {
		rest, err = asn1.Unmarshal(rest, &outSeq)
		if err != nil {
			return asn1.RawValue{}, asn1.RawValue{}
		}
	}
	rest, err = asn1.Unmarshal(rest, &outSeq)
	if err != nil {
		return asn1.RawValue{}, asn1.RawValue{}
	}
	rest, err = asn1.Unmarshal(rest, &outSeq)
	if err != nil {
		return asn1.RawValue{}, asn1.RawValue{}
	}
	_, err = asn1.Unmarshal(rest, &outSeq)
	if err != nil {
		return asn1.RawValue{}, asn1.RawValue{}
	}
	// Finally at the validity date, load them into a different RawValue
	rest, err = asn1.Unmarshal(outSeq.Bytes, &firstDate)
	if err != nil {
		return asn1.RawValue{}, asn1.RawValue{}
	}
	_, err = asn1.Unmarshal(rest, &secondDate)
	if err != nil {
		return asn1.RawValue{}, asn1.RawValue{}
	}
	return firstDate, secondDate
}
