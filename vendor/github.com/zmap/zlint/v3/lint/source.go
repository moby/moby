package lint

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

// LintSource is a type representing a known lint source that lints cite
// requirements from.
type LintSource string

const (
	UnknownLintSource        LintSource = "Unknown"
	RFC5280                  LintSource = "RFC5280"
	RFC5480                  LintSource = "RFC5480"
	RFC5891                  LintSource = "RFC5891"
	CABFBaselineRequirements LintSource = "CABF_BR"
	CABFEVGuidelines         LintSource = "CABF_EV"
	MozillaRootStorePolicy   LintSource = "Mozilla"
	AppleRootStorePolicy     LintSource = "Apple"
	Community                LintSource = "Community"
	EtsiEsi                  LintSource = "ETSI_ESI"
)

// UnmarshalJSON implements the json.Unmarshaler interface. It ensures that the
// unmarshaled value is a known LintSource.
func (s *LintSource) UnmarshalJSON(data []byte) error {
	var throwAway string
	if err := json.Unmarshal(data, &throwAway); err != nil {
		return err
	}

	switch LintSource(throwAway) {
	case RFC5280, RFC5480, RFC5891, CABFBaselineRequirements, CABFEVGuidelines, MozillaRootStorePolicy, AppleRootStorePolicy, Community, EtsiEsi:
		*s = LintSource(throwAway)
		return nil
	default:
		*s = UnknownLintSource
		return fmt.Errorf("unknown LintSource value %q", throwAway)
	}
}

// FromString sets the LintSource value based on the source string provided
// (case sensitive). If the src string does not match any of the known
// LintSource's then s is set to the UnknownLintSource.
func (s *LintSource) FromString(src string) {
	// Start with the unknown lint source
	*s = UnknownLintSource
	// Trim space and try to match a known value
	src = strings.TrimSpace(src)
	switch LintSource(src) {
	case RFC5280:
		*s = RFC5280
	case RFC5480:
		*s = RFC5480
	case RFC5891:
		*s = RFC5891
	case CABFBaselineRequirements:
		*s = CABFBaselineRequirements
	case CABFEVGuidelines:
		*s = CABFEVGuidelines
	case MozillaRootStorePolicy:
		*s = MozillaRootStorePolicy
	case AppleRootStorePolicy:
		*s = AppleRootStorePolicy
	case Community:
		*s = Community
	case EtsiEsi:
		*s = EtsiEsi
	}
}

// SourceList is a slice of LintSources that can be sorted.
type SourceList []LintSource

// Len returns the length of the list.
func (l SourceList) Len() int {
	return len(l)
}

// Swap swaps the LintSource at index i and j in the list.
func (l SourceList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// Less compares the LintSources at index i and j lexicographically.
func (l SourceList) Less(i, j int) bool {
	return l[i] < l[j]
}

// FromString populates a SourceList (replacing any existing content) with the
// comma separated list of sources provided in raw. If any of the comma
// separated values are not known LintSource's an error is returned.
func (l *SourceList) FromString(raw string) error {
	// Start with an empty list
	*l = SourceList{}

	values := strings.Split(raw, ",")
	for _, val := range values {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		// Populate the LintSource with the trimmed value.
		var src LintSource
		src.FromString(val)
		// If the LintSource is UnknownLintSource then return an error.
		if src == UnknownLintSource {
			return fmt.Errorf("unknown lint source in list: %q", val)
		}
		*l = append(*l, src)
	}
	return nil
}
