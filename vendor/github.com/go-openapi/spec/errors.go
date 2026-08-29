// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import "errors"

// Error codes.
var (
	// ErrUnknownTypeForReference indicates that a resolved reference was found in an unsupported container type.
	ErrUnknownTypeForReference = errors.New("unknown type for the resolved reference")

	// ErrResolveRefNeedsAPointer indicates that a $ref target must be a valid JSON pointer.
	ErrResolveRefNeedsAPointer = errors.New("resolve ref: target needs to be a pointer")

	// ErrDerefUnsupportedType indicates that a resolved reference was found in an unsupported container type.
	// At the moment, $ref are supported only inside: schemas, parameters, responses, path items.
	ErrDerefUnsupportedType = errors.New("deref: unsupported type")

	// ErrExpandUnsupportedType indicates that $ref expansion is attempted on some invalid type.
	ErrExpandUnsupportedType = errors.New("expand: unsupported type. Input should be of type *Parameter or *Response")

	// ErrExpandTooManyNodes indicates that $ref expansion exceeded the maximum number of schema nodes
	// allowed for a single expansion (see ExpandOptions.MaxExpansionNodes).
	//
	// This is a safeguard against maliciously crafted specifications that expand to an exponential
	// number of nodes from a small input (a $ref amplification / "billion laughs" style attack).
	ErrExpandTooManyNodes = errors.New("expand: too many schema nodes: expansion budget exceeded (see ExpandOptions.MaxExpansionNodes)")

	// ErrSpec is an error raised by the spec package.
	ErrSpec = errors.New("spec error")
)
