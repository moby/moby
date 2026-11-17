// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"

	"github.com/go-openapi/analysis"
)

// maxRefHops bounds how many $ref a pointer may be followed through, so that a
// document referring to itself cannot spin here.
const maxRefHops = 10

// refRedirects maps the location of a $ref to the location it points at, for
// the local references of a document.
//
// Checks walk the expanded document, so a finding below a $ref comes out with a
// pointer that descends into a node the authored document does not contain: a
// bare "$ref" member has nothing under it. Following the reference turns such a
// pointer back into one the document addresses.
type refRedirects map[string]string

func newRefRedirects(analyzer *analysis.Spec) refRedirects {
	redirects := make(refRedirects)
	for location, ref := range analyzer.AllRefsByLocation() {
		target := ref.String()
		if !strings.HasPrefix(target, "#/") {
			// only a local reference has a location in this document
			continue
		}

		redirects[strings.TrimPrefix(location, "#")] = strings.TrimPrefix(target, "#")
	}

	return redirects
}

// through rewrites a pointer that descends below a $ref.
//
// A pointer that stops at the $ref itself is left alone: that node exists, and
// it is where a reader has to go to amend the reference.
func (r refRedirects) through(pointer string) string {
	if len(r) == 0 {
		return pointer
	}

	for range maxRefHops {
		prefix, rest, ok := r.crossing(pointer)
		if !ok {
			return pointer
		}

		pointer = prefix + rest
	}

	return pointer
}

// crossing finds the longest prefix of pointer that holds a $ref, and returns
// the location that reference points at together with what is left below it.
func (r refRedirects) crossing(pointer string) (target, rest string, ok bool) {
	for at := strings.LastIndex(pointer, "/"); at > 0; at = strings.LastIndex(pointer[:at], "/") {
		if target, isRef := r[pointer[:at]]; isRef {
			return target, pointer[at:], true
		}
	}

	return "", "", false
}
