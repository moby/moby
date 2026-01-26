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
	"regexp"

	"golang.org/x/net/idna"
)

var reservedLabelPrefix = regexp.MustCompile(`^..--`)

var xnLabelPrefix = regexp.MustCompile(`(?i)^xn--`)

// HasReservedLabelPrefix checks whether the given string (presumably
// a domain label) has hyphens ("-") as the third and fourth characters. Domain
// labels with hyphens in these positions are considered to be "Reserved Labels"
// per RFC 5890, section 2.3.1.
// (https://datatracker.ietf.org/doc/html/rfc5890#section-2.3.1)
func HasReservedLabelPrefix(s string) bool {
	return reservedLabelPrefix.MatchString(s)
}

// HasXNLabelPrefix checks whether the given string (presumably a
// domain label) is prefixed with the case-insensitive string "xn--" (the
// IDNA ACE prefix).
//
// This check is useful given the bug following bug report for IDNA wherein
// the ACE prefix incorrectly taken to be case-sensitive.
//
// https://github.com/golang/go/issues/48778
func HasXNLabelPrefix(s string) bool {
	return xnLabelPrefix.MatchString(s)
}

// IdnaToUnicode is a wrapper around idna.ToUnicode.
//
// If the provided string starts with the IDNA ACE prefix ("xn--", case
// insensitive), then that ACE prefix is coerced to a lowercase "xn--" before
// processing by the idna package.
//
// This is only necessary due to the bug at https://github.com/golang/go/issues/48778
func IdnaToUnicode(s string) (string, error) {
	if HasXNLabelPrefix(s) {
		s = "xn--" + s[4:]
	}
	return idna.ToUnicode(s)
}
