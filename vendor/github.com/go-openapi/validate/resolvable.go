// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strconv"
	"strings"

	"github.com/go-openapi/jsonpointer"
)

// resolvable trims a pointer down to the deepest node the document holds.
//
// Checks walk an expanded, model-level view of a specification, which holds
// members the document itself never wrote: a parameter merged in from a path
// item, a member of a schema reached through a $ref. A pointer built along the
// way may therefore end on something a reader cannot go to.
//
// Trimming is the last word on a location, applied once every check has had its
// say: it only ever shortens, so a pointer that already addressed a node comes
// back untouched, and one that did not still says as much as it truthfully can.
// This is what makes [Located.Pointer] always resolve.
func (s *SpecValidator) resolvable(pointer string) string {
	if pointer == "" || s.document == nil {
		return pointer
	}

	node := s.document
	for at := 0; at < len(pointer); {
		end := strings.IndexByte(pointer[at+1:], '/')
		token := pointer[at+1:]
		if end >= 0 {
			token = pointer[at+1 : at+1+end]
		}

		member, isHeld := memberOf(node, jsonpointer.Unescape(token))
		if !isHeld {
			return pointer[:at]
		}
		node = member

		if end < 0 {
			break
		}
		at += end + 1
	}

	return pointer
}

// memberOf returns the member a reference token addresses in a decoded JSON
// node, and whether the node holds one at all.
func memberOf(node any, token string) (any, bool) {
	switch held := node.(type) {
	case map[string]any:
		member, isHeld := held[token]

		return member, isHeld
	case []any:
		index, err := strconv.Atoi(token)
		if err != nil || index < 0 || index >= len(held) {
			return nil, false
		}

		return held[index], true
	default:
		return nil, false
	}
}
