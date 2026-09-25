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

package lint

type Profile struct {
	// Name is a lowercase underscore-separated string describing what a given
	// profile aggregates.
	Name string `json:"name"`

	// A human-readable description of what the Profile checks. Usually copied
	// directly from the CA/B Baseline Requirements, RFC 5280, or other published
	// document.
	Description string `json:"description,omitempty"`

	// The source of the check, e.g. "BRs: 6.1.6" or "RFC 5280: 4.1.2.6".
	Citation string `json:"citation,omitempty"`

	// Programmatic source of the check, BRs, RFC5280, or ZLint
	Source LintSource `json:"source,omitempty"`

	// The names of the lints that compromise this profile. These names
	// MUST be the exact same found within Lint.Name.
	LintNames []string `json:"lints"`
}

var profiles = map[string]Profile{}

// RegisterProfile registered the provided profile into the global profile mapping.
func RegisterProfile(profile Profile) {
	profiles[profile.Name] = profile
}

// GetProfile returns the Profile for which the provided name matches Profile.Name.
// If no such Profile exists then the `ok` returns false, else true.
func GetProfile(name string) (profile Profile, ok bool) {
	profile, ok = profiles[name]
	return profile, ok
}

// AllProfiles returns a slice of all Profiles currently registered globally.
func AllProfiles() []Profile {
	p := make([]Profile, 0)
	for _, profile := range profiles {
		p = append(p, profile)
	}
	return p
}
