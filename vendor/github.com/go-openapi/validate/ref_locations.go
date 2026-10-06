// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/go-openapi/analysis"
)

// refLocations tells where a $ref value is declared in a document.
//
// The analyzer indexes references the other way around, by the location each
// was found at, so the index is inverted here. Only declarations are indexed:
// a "$ref" member sitting in an example or in a default value is data, and the
// analyzer never walks into it.
type refLocations map[string]pathSegments

// newRefLocations inverts the analyzer's reference index.
//
// A reference declared in several places keeps the smallest declaration, so
// that the answer does not depend on map iteration order.
func newRefLocations(analyzer *analysis.Spec) refLocations {
	declarations := make(map[string]string)
	for location, ref := range analyzer.AllRefsByLocation() {
		value := ref.String()
		if value == "" {
			continue
		}

		if known, isKnown := declarations[value]; isKnown && known <= location {
			continue
		}

		declarations[value] = location
	}

	locations := make(refLocations, len(declarations))
	for value, location := range declarations {
		locations[value] = localRefPath(location)
	}

	return locations
}

// at returns where a reference is declared, or the document root when the
// reference is not one the analyzer indexed.
func (l refLocations) at(ref string) pathSegments {
	return l[ref]
}
