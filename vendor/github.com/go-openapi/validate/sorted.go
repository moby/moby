// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/go-openapi/spec"
)

// sortedKeys returns the keys of a map in ascending order.
//
// Findings are reported in the order the checks walk the document, and several
// of them walk a map: definitions, paths, response codes, headers, properties.
// Go randomises map iteration, so the same bytes validated twice would list
// their findings in a different order — and where a check stops on the first
// fault it meets, would name a different offender altogether.
//
// Walking keys in sorted order makes both defined: findings come out in
// definition-name (or path, or status-code) order, on every run.
func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	return slices.Sorted(maps.Keys(m))
}

// sortedRefs orders references by the location they point at.
//
// The analyzer gathers references in a map, so the slice it hands back comes
// out in a different order on every run. See [sortedKeys].
func sortedRefs(refs []spec.Ref) []spec.Ref {
	slices.SortFunc(refs, func(a, b spec.Ref) int {
		return strings.Compare(a.String(), b.String())
	})

	return refs
}
