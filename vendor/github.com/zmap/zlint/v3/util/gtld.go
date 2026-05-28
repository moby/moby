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
	"fmt"
	"strings"
	"time"

	"github.com/zmap/zcrypto/x509"
)

// This package uses the `zlint-gtld-update` command to generate a `tldMap` map.
//go:generate zlint-gtld-update ./gtld_map.go

const (
	GTLDPeriodDateFormat = "2006-01-02"
)

// GTLDPeriod is a struct representing a gTLD's validity period. The field names
// are chosen to match the data returned by the ICANN gTLD v2 JSON registry[0].
// See the `zlint-gtld-update` command for more information.
// [0] - https://www.icann.org/resources/registries/gtlds/v2/gtlds.json
type GTLDPeriod struct {
	// GTLD is the GTLD the period corresponds to. It is used only for friendly
	// error messages from `Valid`
	GTLD string
	// DelegationDate is the date at which ICANN delegated the gTLD into existence
	// from the root DNS, or is empty if the gTLD was never delegated.
	DelegationDate string
	// RemovalDate is the date at which ICANN removed the gTLD delegation from the
	// root DNS, or is empty if the gTLD is still delegated and has not been
	// removed.
	RemovalDate string
}

// Valid determines if the provided `when` time is within the GTLDPeriod for the
// gTLD. E.g. whether a certificate issued at `when` with a subject identifier
// using the specified gTLD can be considered a valid use of the gTLD.
func (p GTLDPeriod) Valid(when time.Time) error {
	// NOTE: We can throw away the errors from time.Parse in this function because
	// the zlint-gtld-update command only writes entries to the generated gTLD map
	// after the dates have been verified as parseable
	notBefore, _ := time.Parse(GTLDPeriodDateFormat, p.DelegationDate)
	if when.Before(notBefore) {
		return fmt.Errorf(`gTLD ".%s" is not valid until %s`,
			p.GTLD, p.DelegationDate)
	}
	// The removal date may be empty. We only need to check `when` against the
	// removal when it isn't empty
	if p.RemovalDate != "" {
		notAfter, _ := time.Parse(GTLDPeriodDateFormat, p.RemovalDate)
		if when.After(notAfter) {
			return fmt.Errorf(`gTLD ".%s" is not valid after %s`,
				p.GTLD, p.RemovalDate)
		}
	}
	return nil
}

// HasValidTLD checks that a domain ends in a valid TLD that was delegated in
// the root DNS at the time specified.
func HasValidTLD(domain string, when time.Time) bool {
	labels := strings.Split(strings.ToLower(domain), ".")
	rightLabel := labels[len(labels)-1]
	// if the rightmost label is not present in the tldMap, it isn't valid and
	// never was.
	if tldPeriod, present := tldMap[rightLabel]; !present {
		return false
	} else if tldPeriod.Valid(when) != nil {
		// If the TLD exists but the date is outside of the gTLD's validity period
		// then it is not a valid TLD.
		return false
	}
	// Otherwise the TLD exists, and was a valid TLD delegated in the root DNS
	// at the time of the given date.
	return true
}

// IsInTLDMap checks that a label is present in the TLD map. It does not
// consider the TLD's validity period and whether the TLD may have been removed,
// only whether it was ever a TLD that was delegated.
func IsInTLDMap(label string) bool {
	label = strings.ToLower(label)
	if _, ok := tldMap[label]; ok {
		return true
	} else {
		return false
	}
}

// CertificateSubjContainsTLD checks whether the provided Certificate has
// a Subject Common Name or DNS Subject Alternate Name that ends in the provided
// TLD label. If IsInTLDMap(label) returns false then CertificateSubjInTLD will
// return false.
func CertificateSubjInTLD(c *x509.Certificate, label string) bool {
	label = strings.ToLower(label)
	label = strings.TrimPrefix(label, ".")
	if !IsInTLDMap(label) {
		return false
	}
	for _, name := range append(c.DNSNames, c.Subject.CommonName) {
		if strings.HasSuffix(name, "."+label) {
			return true
		}
	}
	return false
}
