// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/go-openapi/spec"
)

// ExampleValidator validates example values defined in a spec.
type exampleValidator struct {
	SpecValidator  *SpecValidator
	visitedSchemas map[string]struct{}
	schemaOptions  *SchemaValidatorOptions
}

// Validate validates the example values declared in the swagger spec
// Example values MUST conform to their schema.
//
// With Swagger 2.0, examples are supported in:
//   - schemas
//   - individual property
//   - responses
func (ex *exampleValidator) Validate() *Result {
	errs := validatorPools.results.Borrow()

	if ex == nil || ex.SpecValidator == nil {
		return errs
	}
	ex.resetVisited()
	errs.Merge(ex.validateExampleValueValidAgainstSchema()) // error -

	return errs
}

// resetVisited resets the internal state of visited schemas.
func (ex *exampleValidator) resetVisited() {
	if ex.visitedSchemas == nil {
		ex.visitedSchemas = make(map[string]struct{})

		return
	}

	// NOTE(go1.21): clear(ex.visitedSchemas)
	for k := range ex.visitedSchemas {
		delete(ex.visitedSchemas, k)
	}
}

// beingVisited asserts a schema is being visited.
func (ex *exampleValidator) beingVisited(path pathSegments) {
	ex.visitedSchemas[path.pointer()] = struct{}{}
}

// isVisited tells if a path has already been visited.
func (ex *exampleValidator) isVisited(path pathSegments) bool {
	return isVisited(path, ex.visitedSchemas)
}

//nolint:gocognit // refactor in a forthcoming PR
func (ex *exampleValidator) validateExampleValueValidAgainstSchema() *Result {
	// every example value that is specified must validate against the schema for that property
	// in: schemas, properties, object, items
	// not in: headers, parameters without schema

	res := validatorPools.results.Borrow()
	s := ex.SpecValidator

	operations := s.expandedAnalyzer().Operations()
	for _, method := range sortedKeys(operations) {
		pathItem := operations[method]
		for _, path := range sortedKeys(pathItem) {
			op := pathItem[path]
			// parameters
			for _, param := range paramHelp.safeExpandedParamsFor(path, method, op.ID, res, s) {

				// As of swagger 2.0, Examples are not supported in simple parameters
				// However, it looks like it is supported by go-openapi

				// reset explored schemas to get depth-first recursive-proof exploration
				ex.resetVisited()

				// Check simple parameters first
				// default values provided must validate against their inline definition (no explicit schema)
				if param.Example != nil && param.Schema == nil {
					// check param default value is valid
					red := newParamValidator(&param, s.KnownFormats, ex.schemaOptions).Validate(param.Example) //#nosec
					red.relocate(s.parameterPath(path, method, param.In, param.Name).child(swaggerExample))
					if red.HasErrorsOrWarnings() {
						res.addWarningsAt(s.parameterPath(path, method, param.In, param.Name), exampleValueDoesNotValidateMsg(param.Name, param.In))
						res.MergeAsWarnings(red)
					} else if red.wantsRedeemOnMerge {
						redeemResult(red)
					}
				}

				// Recursively follows Items and Schemas
				if param.Items != nil {
					red := ex.validateExampleValueItemsAgainstSchema(s.parameterPath(path, method, param.In, param.Name), param.In, &param, param.Items) //#nosec
					if red.HasErrorsOrWarnings() {
						res.addWarningsAt(s.parameterPath(path, method, param.In, param.Name), exampleValueItemsDoesNotValidateMsg(param.Name, param.In))
						res.Merge(red)
					} else if red.wantsRedeemOnMerge {
						redeemResult(red)
					}
				}

				if param.Schema != nil {
					// Validate example value against schema
					red := ex.validateExampleValueSchemaAgainstSchema(s.parameterPath(path, method, param.In, param.Name).structuralChild(jsonSchema), param.In, param.Schema)
					if red.HasErrorsOrWarnings() {
						res.addWarningsAt(s.parameterPath(path, method, param.In, param.Name), exampleValueDoesNotValidateMsg(param.Name, param.In))
						res.Merge(red)
					} else if red.wantsRedeemOnMerge {
						redeemResult(red)
					}
				}
			}

			if op.Responses != nil {
				if op.Responses.Default != nil {
					// Same constraint on default Response
					res.Merge(ex.validateExampleInResponse(op.Responses.Default, jsonDefault, path, method, 0, op.ID))
				}
				// Same constraint on regular Responses
				if op.Responses.StatusCodeResponses != nil { // Safeguard
					for _, code := range sortedKeys(op.Responses.StatusCodeResponses) {
						r := op.Responses.StatusCodeResponses[code]
						res.Merge(ex.validateExampleInResponse(&r, "response", path, method, code, op.ID))
					}
				}
			} else if op.ID != "" {
				// Empty op.ID means there is no meaningful operation: no need to report a specific message
				res.addErrorsAt(operationPath(path, method), noValidResponseMsg(op.ID))
			}
		}
	}
	if s.spec.Spec().Definitions != nil { // Safeguard
		// reset explored schemas to get depth-first recursive-proof exploration
		ex.resetVisited()
		definitions := s.spec.Spec().Definitions
		for _, nm := range sortedKeys(definitions) {
			sch := definitions[nm]
			res.Merge(ex.validateExampleValueSchemaAgainstSchema(newPathSegments(swaggerDefinitions, nm), "body", &sch))
		}
	}
	return res
}

func (ex *exampleValidator) validateExampleInResponse(
	resp *spec.Response, responseType, path, method string, responseCode int, operationID string,
) *Result {
	s := ex.SpecValidator

	responseName, responseCodeAsStr := responseHelp.responseMsgVariants(responseType, responseCode)
	response, res := responseHelp.expandResponseRef(resp, path, responsePath(path, method, responseCodeAsStr), s)
	if !res.IsValid() { // Safeguard
		return res
	}

	if response.Headers != nil { // Safeguard
		for _, nm := range sortedKeys(response.Headers) {
			h := response.Headers[nm]
			// reset explored schemas to get depth-first recursive-proof exploration
			ex.resetVisited()

			if h.Example != nil {
				red := newHeaderValidator(nm, &h, s.KnownFormats, ex.schemaOptions).Validate(h.Example) //#nosec
				red.relocate(responseHeaderPath(path, method, responseCodeAsStr, nm).child(swaggerExample))
				if red.HasErrorsOrWarnings() {
					res.addWarningsAt(responseHeaderPath(path, method, responseCodeAsStr, nm), exampleValueHeaderDoesNotValidateMsg(operationID, nm, responseName))
					res.MergeAsWarnings(red)
				} else if red.wantsRedeemOnMerge {
					redeemResult(red)
				}
			}

			// Headers have inline definition, like params
			if h.Items != nil {
				red := ex.validateExampleValueItemsAgainstSchema(responseHeaderPath(path, method, responseCodeAsStr, nm), "header", &h, h.Items) //#nosec
				if red.HasErrorsOrWarnings() {
					res.addWarningsAt(responseHeaderPath(path, method, responseCodeAsStr, nm), exampleValueHeaderItemsDoesNotValidateMsg(operationID, nm, responseName))
					res.MergeAsWarnings(red)
				} else if red.wantsRedeemOnMerge {
					redeemResult(red)
				}
			}

			if _, err := compileRegexp(h.Pattern); err != nil {
				res.addErrorsAt(responseHeaderPath(path, method, responseCodeAsStr, nm), invalidPatternInHeaderMsg(operationID, nm, responseName, h.Pattern, err))
			}

			// Headers don't have schema
		}
	}
	if response.Schema != nil {
		// reset explored schemas to get depth-first recursive-proof exploration
		ex.resetVisited()

		red := ex.validateExampleValueSchemaAgainstSchema(
			responsePath(path, method, responseCodeAsStr).structuralChild(jsonSchema), "response", response.Schema)
		if red.HasErrorsOrWarnings() {
			// Additional message to make sure the context of the error is not lost
			res.addWarningsAt(responsePath(path, method, responseCodeAsStr), exampleValueInDoesNotValidateMsg(operationID, responseName))
			res.Merge(red)
		} else if red.wantsRedeemOnMerge {
			redeemResult(red)
		}
	}

	if response.Examples != nil {
		if response.Schema != nil {
			if example, ok := response.Examples[jsonMimeApplicationJSON]; ok {
				exampleAt := responsePath(path, method, responseCodeAsStr).
					child(swaggerExamples).
					structuralChild(jsonMimeApplicationJSON)
				res.MergeAsWarnings(
					newSchemaValidator(response.Schema, s.spec.Spec(),
						exampleAt, s.KnownFormats, s.schemaOptions).Validate(example),
				)
			} else {
				// Proposal for enhancement: validate other media types too
				res.addWarningsAt(responsePath(path, method, responseCodeAsStr).child(swaggerExamples), examplesMimeNotSupportedMsg(operationID, responseName))
			}
		} else {
			res.addWarningsAt(responsePath(path, method, responseCodeAsStr).child(swaggerExamples), examplesWithoutSchemaMsg(operationID, responseName))
		}
	}
	return res
}

func (ex *exampleValidator) validateExampleValueSchemaAgainstSchema(path pathSegments, in string, schema *spec.Schema) *Result {
	if schema == nil || ex.isVisited(path) {
		// Avoids recursing if we are already done with that check
		return nil
	}
	ex.beingVisited(path)
	s := ex.SpecValidator
	res := validatorPools.results.Borrow()

	if schema.Example != nil {
		res.MergeAsWarnings(
			newSchemaValidator(schema, s.spec.Spec(), path.child(swaggerExample), s.KnownFormats, ex.schemaOptions).Validate(schema.Example),
		)
	}
	if schema.Items != nil {
		if schema.Items.Schema != nil {
			res.Merge(ex.validateExampleValueSchemaAgainstSchema(path.child(jsonItems), in, schema.Items.Schema))
		}
		// Multiple schemas in items
		if schema.Items.Schemas != nil { // Safeguard
			for i, sch := range schema.Items.Schemas {
				res.Merge(ex.validateExampleValueSchemaAgainstSchema(path.child(jsonItems).item(i), in, &sch)) //#nosec
			}
		}
	}
	if _, err := compileRegexp(schema.Pattern); err != nil {
		res.addErrorsAt(path, invalidPatternInMsg(path.dotted(), in, schema.Pattern))
	}
	if schema.AdditionalItems != nil && schema.AdditionalItems.Schema != nil {
		// NOTE: we keep validating values, even though additionalItems is unsupported in Swagger 2.0 (and 3.0 as well)
		res.Merge(ex.validateExampleValueSchemaAgainstSchema(path.child(jsonAdditionalItems), in, schema.AdditionalItems.Schema))
	}
	for _, propName := range sortedKeys(schema.Properties) {
		prop := schema.Properties[propName]
		res.Merge(ex.validateExampleValueSchemaAgainstSchema(path.structuralChild(jsonProperties).child(propName), in, &prop))
	}
	for _, propName := range sortedKeys(schema.PatternProperties) {
		prop := schema.PatternProperties[propName]
		res.Merge(ex.validateExampleValueSchemaAgainstSchema(path.structuralChild(jsonPatternProperties).child(propName), in, &prop))
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		res.Merge(ex.validateExampleValueSchemaAgainstSchema(path.child(jsonAdditionalProperties), in, schema.AdditionalProperties.Schema))
	}
	if schema.AllOf != nil {
		for i, aoSch := range schema.AllOf {
			res.Merge(ex.validateExampleValueSchemaAgainstSchema(path.child(jsonAllOf).item(i), in, &aoSch)) //#nosec
		}
	}
	return res
}

// NOTE: Temporary duplicated code. Need to refactor with examples
//

func (ex *exampleValidator) validateExampleValueItemsAgainstSchema(path pathSegments, in string, root any, items *spec.Items) *Result {
	res := validatorPools.results.Borrow()
	s := ex.SpecValidator
	if items != nil {
		if items.Example != nil {
			res.MergeAsWarnings(
				newItemsValidator(path, in, items, root, s.KnownFormats, ex.schemaOptions).Validate(0, items.Example),
			)
		}
		if items.Items != nil {
			res.Merge(ex.validateExampleValueItemsAgainstSchema(path.item(0), in, root, items.Items))
		}
		if _, err := compileRegexp(items.Pattern); err != nil {
			res.addErrorsAt(path, invalidPatternInMsg(path.dotted(), in, items.Pattern))
		}
	}

	return res
}
