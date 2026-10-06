// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"strconv"

	"github.com/go-openapi/spec"
)

// validateCollectionFormats warns about a collectionFormat that has no effect.
//
// collectionFormat says how to join the members of an array into one value on the wire — csv, ssv,
// tsv, pipes, or multi for a parameter repeated once per member:
//
//	parameters:
//	  - name: tags
//	    in: query
//	    type: array
//	    items: { type: string }
//	    collectionFormat: pipes
//
// There is nothing to join when the type is not array, so a collectionFormat written on a string or
// an integer does nothing. Swagger 2.0 says the member "determines the format of the array if type
// array is used" and stops there — it never forbids writing it elsewhere, so this is a warning and
// the specification stays valid.
//
// The meta-schema already covers the rest of the collectionFormat rules, and covers them at every
// location: the value must be one of csv, ssv, tsv or pipes, widened with multi for a query or
// formData parameter, where repeating the parameter is possible. A body parameter cannot carry the
// member at all. None of that needs a rule here.
//
// Only a parameter, a header and an items carry a collectionFormat — [spec.SimpleSchema] holds it,
// and [spec.Schema] has no such member. So this walks what the operations of the document declare,
// the way [SpecValidator.validateItems] does, and never looks at a schema. A JSON schema validated
// on its own is untouched by this rule.
func (s *SpecValidator) validateCollectionFormats() *Result {
	res := validatorPools.results.Borrow()

	operations := s.analyzer.Operations()
	for _, method := range sortedKeys(operations) {
		byPath := operations[method]
		for _, path := range sortedKeys(byPath) {
			op := byPath[path]

			for _, param := range paramHelp.safeExpandedParamsFor(path, method, op.ID, res, s) {
				if param.In == swaggerBody {
					// a body parameter describes itself with a schema, and the meta-schema
					// forbids it a collectionFormat outright
					continue
				}

				at := s.parameterPath(path, method, param.In, param.Name)
				in := fmt.Sprintf("parameter %q", param.Name)
				checkCollectionFormat(at, param.CollectionFormat, param.Type, in, res)
				checkItemsCollectionFormats(at, param.Items, in, res)
			}

			for _, response := range responsesOf(op) {
				at := responsePath(path, method, response.code)
				for _, name := range sortedKeys(response.resp.Headers) {
					header := response.resp.Headers[name]
					headerAt := at.children(swaggerHeaders, name)
					in := fmt.Sprintf("header %q", name)
					checkCollectionFormat(headerAt, header.CollectionFormat, header.Type, in, res)
					checkItemsCollectionFormats(headerAt, header.Items, in, res)
				}
			}
		}
	}

	return res
}

// codedResponse is a response together with the code the operation files it under, "default"
// included.
type codedResponse struct {
	code string
	resp spec.Response
}

// responsesOf lists the responses an operation declares, in a settled order: the default response
// first, then the status codes in ascending order.
func responsesOf(op *spec.Operation) []codedResponse {
	if op == nil || op.Responses == nil {
		return nil
	}

	var responses []codedResponse
	if op.Responses.Default != nil {
		responses = append(responses, codedResponse{code: jsonDefault, resp: *op.Responses.Default})
	}

	for _, code := range sortedKeys(op.Responses.StatusCodeResponses) {
		responses = append(responses, codedResponse{
			code: strconv.Itoa(code),
			resp: op.Responses.StatusCodeResponses[code],
		})
	}

	return responses
}

// checkItemsCollectionFormats checks the items of a parameter or header, then the items of those
// items, as deep as the document nests them.
//
// Every level is named the same way in a message: an array of arrays that writes a pointless
// collectionFormat twice is one thing to fix, and the deeper location is reported only when the
// shallower one is sound.
func checkItemsCollectionFormats(at pathSegments, items *spec.Items, in string, res *Result) {
	for items != nil {
		at = at.child(jsonItems)
		checkCollectionFormat(at, items.CollectionFormat, items.Type, "items of "+in, res)
		items = items.Items
	}
}

// checkCollectionFormat warns when a collectionFormat is written on something that is not an array.
//
// A missing collectionFormat has nothing to answer for, and neither has a missing type: a document
// that leaves the type out is already reported by the meta-schema, and guessing what it meant here
// would only add noise.
func checkCollectionFormat(at pathSegments, collectionFormat, typ, in string, res *Result) {
	if collectionFormat == "" || typ == "" || typ == arrayType {
		return
	}

	res.addWarningsAt(at.child(swaggerCollectionFormat), collectionFormatIgnoredMsg(collectionFormat, in, typ))
}
