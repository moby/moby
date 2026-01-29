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
	"bytes"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509/pkix"
)

// CheckRDNSequenceWhiteSpace returns true if there is leading or trailing
// whitespace in any name attribute in the sequence, respectively.
func CheckRDNSequenceWhiteSpace(raw []byte) (leading, trailing bool, err error) {
	var seq pkix.RDNSequence
	if _, err = asn1.Unmarshal(raw, &seq); err != nil {
		return
	}
	for _, rdn := range seq {
		for _, atv := range rdn {
			if !IsNameAttribute(atv.Type) {
				continue
			}
			value, ok := atv.Value.(string)
			if !ok {
				continue
			}
			if leftStrip := strings.TrimLeftFunc(value, unicode.IsSpace); leftStrip != value {
				leading = true
			}
			if rightStrip := strings.TrimRightFunc(value, unicode.IsSpace); rightStrip != value {
				trailing = true
			}
		}
	}
	return
}

// IsIA5String returns true if raw is an IA5String, and returns false otherwise.
func IsIA5String(raw []byte) bool {
	for _, b := range raw {
		i := int(b)
		if i > 127 || i < 0 {
			return false
		}
	}
	return true
}

func IsInPrefSyn(name string) bool {
	// If the DNS name is just a space, it is valid
	if name == " " {
		return true
	}
	// This is the expression that matches the ABNF syntax from RFC 1034: Sec 3.5, specifically for subdomain since the " " case for domain is covered above
	prefsyn := regexp.MustCompile(`^([[:alpha:]]{1}(([[:alnum:]]|[-])*[[:alnum:]]{1})*){1}([.][[:alpha:]]{1}(([[:alnum:]]|[-])*[[:alnum:]]{1})*)*$`)
	return prefsyn.MatchString(name)
}

// AllAlternateNameWithTagAreIA5 returns true if all sequence members with the
// given tag are encoded as IA5 strings, and false otherwise. If it encounters
// errors parsing asn1, err will be non-nil.
func AllAlternateNameWithTagAreIA5(ext *pkix.Extension, tag int) (bool, error) {
	var seq asn1.RawValue
	var err error
	// Unmarshal the extension as a sequence
	if _, err = asn1.Unmarshal(ext.Value, &seq); err != nil {
		return false, err
	}
	// Ensure the sequence matches what we expect for SAN/IAN
	if !seq.IsCompound || seq.Tag != asn1.TagSequence || seq.Class != asn1.ClassUniversal {
		err = asn1.StructuralError{Msg: "bad alternate name sequence"}
		return false, err
	}

	// Iterate over the sequence and look for items tagged with tag
	rest := seq.Bytes
	for len(rest) > 0 {
		var v asn1.RawValue
		rest, err = asn1.Unmarshal(rest, &v)
		if err != nil {
			return false, err
		}
		if v.Tag == tag {
			if !IsIA5String(v.Bytes) {
				return false, nil
			}
		}
	}

	return true, nil
}

// IsEmptyASN1Sequence returns true if
// *input is an empty sequence (0x30, 0x00) or
// *len(inout) < 2
// This check covers more cases than just empty sequence checks but it makes sense from the usage perspective
var emptyASN1Sequence = []byte{0x30, 0x00}

func IsEmptyASN1Sequence(input []byte) bool {
	return len(input) < 2 || bytes.Equal(input, emptyASN1Sequence)
}

// ParseBMPString returns a uint16 encoded string following the specification for a BMPString type
func ParseBMPString(bmpString []byte) (string, error) {
	if len(bmpString)%2 != 0 {
		return "", errors.New("odd-length BMP string")
	}
	// strip terminator if present
	if l := len(bmpString); l >= 2 && bmpString[l-1] == 0 && bmpString[l-2] == 0 {
		bmpString = bmpString[:l-2]
	}
	s := make([]uint16, 0, len(bmpString)/2)
	for len(bmpString) > 0 {
		s = append(s, uint16(bmpString[0])<<8+uint16(bmpString[1]))
		bmpString = bmpString[2:]
	}
	return string(utf16.Decode(s)), nil
}
