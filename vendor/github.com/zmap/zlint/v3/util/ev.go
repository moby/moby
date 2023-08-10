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
)

var evoids = map[string]bool{
	"1.3.159.1.17.1":                   true,
	"1.3.6.1.4.1.34697.2.1":            true,
	"1.3.6.1.4.1.34697.2.2":            true,
	"1.3.6.1.4.1.34697.2.3":            true,
	"1.3.6.1.4.1.34697.2.4":            true,
	"1.2.40.0.17.1.22":                 true,
	"2.16.578.1.26.1.3.3":              true,
	"1.3.6.1.4.1.17326.10.14.2.1.2":    true,
	"1.3.6.1.4.1.17326.10.8.2.1.2":     true,
	"1.3.6.1.4.1.6449.1.2.1.5.1":       true,
	"2.16.840.1.114412.2.1":            true,
	"2.16.840.1.114412.1.3.0.2":        true,
	"2.16.528.1.1001.1.1.1.12.6.1.1.1": true,
	"2.16.792.3.0.4.1.1.4":             true,
	"2.16.840.1.114028.10.1.2":         true,
	"0.4.0.2042.1.4":                   true,
	"0.4.0.2042.1.5":                   true,
	"1.3.6.1.4.1.13177.10.1.3.10":      true,
	"1.3.6.1.4.1.14370.1.6":            true,
	"1.3.6.1.4.1.4146.1.1":             true,
	"2.16.840.1.114413.1.7.23.3":       true,
	"1.3.6.1.4.1.14777.6.1.1":          true,
	"2.16.792.1.2.1.1.5.7.1.9":         true,
	"1.3.6.1.4.1.782.1.2.1.8.1":        true,
	"1.3.6.1.4.1.22234.2.5.2.3.1":      true,
	"1.3.6.1.4.1.8024.0.2.100.1.2":     true,
	"1.2.392.200091.100.721.1":         true,
	"2.16.840.1.114414.1.7.23.3":       true,
	"1.3.6.1.4.1.23223.2":              true,
	"1.3.6.1.4.1.23223.1.1.1":          true,
	"2.16.756.1.83.21.0":               true,
	"2.16.756.1.89.1.2.1.1":            true,
	"1.3.6.1.4.1.7879.13.24.1":         true,
	"2.16.840.1.113733.1.7.48.1":       true,
	"2.16.840.1.114404.1.1.2.4.1":      true,
	"2.16.840.1.113733.1.7.23.6":       true,
	"1.3.6.1.4.1.6334.1.100.1":         true,
	"2.16.840.1.114171.500.9":          true,
	"1.3.6.1.4.1.36305.2":              true,
}

// IsEV returns true if the input is a known Extended Validation OID.
func IsEV(in []asn1.ObjectIdentifier) bool {
	for _, oid := range in {
		if _, ok := evoids[oid.String()]; ok {
			return true
		}
	}
	return false
}

const OnionTLD = ".onion"
