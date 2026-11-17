// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/go-openapi/spec"
)

// defaultValidator validates default values in a spec.
// According to Swagger spec, default values MUST validate their schema.
type defaultValidator struct {
	SpecValidator  *SpecValidator
	visitedSchemas map[string]struct{}
	schemaOptions  *SchemaValidatorOptions
}

// Validate validates the default values declared in the swagger spec.
func (d *defaultValidator) Validate() *Result {
	errs := validatorPools.results.Borrow() // will redeem when merged

	if d == nil || d.SpecValidator == nil {
		return errs
	}
	d.resetVisited()
	errs.Merge(d.validateDefaultValueValidAgainstSchema()) // error -
	return errs
}

// resetVisited resets the internal state of visited schemas.
func (d *defaultValidator) resetVisited() {
	if d.visitedSchemas == nil {
		d.visitedSchemas = make(map[string]struct{})

		return
	}

	// NOTE(go1.21): clear(ex.visitedSchemas)
	for k := range d.visitedSchemas {
		delete(d.visitedSchemas, k)
	}
}

func isVisited(path pathSegments, visitedSchemas map[string]struct{}) bool {
	_, found := visitedSchemas[path.pointer()]
	if found {
		return true
	}

	// search for overlapping paths: a trailing run of tokens that already
	// appears at the end of what leads to it means we are going in circles.
	for i := 1; i < len(path); i++ {
		if path[:i].hasSuffix(path[i:]) {
			return true
		}
	}

	return false
}

// beingVisited asserts a schema is being visited.
func (d *defaultValidator) beingVisited(path pathSegments) {
	d.visitedSchemas[path.pointer()] = struct{}{}
}

// isVisited tells if a path has already been visited.
func (d *defaultValidator) isVisited(path pathSegments) bool {
	return isVisited(path, d.visitedSchemas)
}

//nolint:gocognit // refactor in a forthcoming PR
func (d *defaultValidator) validateDefaultValueValidAgainstSchema() *Result {
	// every default value that is specified must validate against the schema for that property
	// headers, items, parameters, schema

	res := validatorPools.results.Borrow() // will redeem when merged
	s := d.SpecValidator

	operations := s.expandedAnalyzer().Operations()
	for _, method := range sortedKeys(operations) {
		pathItem := operations[method]
		for _, path := range sortedKeys(pathItem) {
			op := pathItem[path]
			// parameters
			for _, param := range paramHelp.safeExpandedParamsFor(path, method, op.ID, res, s) {
				if param.Default != nil && param.Required {
					res.addWarningsAt(s.parameterPath(path, method, param.In, param.Name), requiredHasDefaultMsg(param.Name, param.In))
				}

				// reset explored schemas to get depth-first recursive-proof exploration
				d.resetVisited()

				// Check simple parameters first
				// default values provided must validate against their inline definition (no explicit schema)
				if param.Default != nil && param.Schema == nil {
					// check param default value is valid
					red := newParamValidator(&param, s.KnownFormats, d.schemaOptions).Validate(param.Default) //#nosec
					red.relocate(s.parameterPath(path, method, param.In, param.Name).child(jsonDefault))
					if red.HasErrorsOrWarnings() {
						res.addErrorsAt(s.parameterPath(path, method, param.In, param.Name), defaultValueDoesNotValidateMsg(param.Name, param.In))
						res.Merge(red)
					} else if red.wantsRedeemOnMerge {
						redeemResult(red)
					}
				}

				// Recursively follows Items and Schemas
				if param.Items != nil {
					red := d.validateDefaultValueItemsAgainstSchema(s.parameterPath(path, method, param.In, param.Name), param.In, &param, param.Items) //#nosec
					if red.HasErrorsOrWarnings() {
						res.addErrorsAt(s.parameterPath(path, method, param.In, param.Name), defaultValueItemsDoesNotValidateMsg(param.Name, param.In))
						res.Merge(red)
					} else if red.wantsRedeemOnMerge {
						redeemResult(red)
					}
				}

				if param.Schema != nil {
					// Validate default value against schema
					red := d.validateDefaultValueSchemaAgainstSchema(s.parameterPath(path, method, param.In, param.Name).structuralChild(jsonSchema), param.In, param.Schema)
					if red.HasErrorsOrWarnings() {
						res.addErrorsAt(s.parameterPath(path, method, param.In, param.Name), defaultValueDoesNotValidateMsg(param.Name, param.In))
						res.Merge(red)
					} else if red.wantsRedeemOnMerge {
						redeemResult(red)
					}
				}
			}

			if op.Responses != nil {
				if op.Responses.Default != nil {
					// Same constraint on default Response
					res.Merge(d.validateDefaultInResponse(op.Responses.Default, jsonDefault, path, method, 0, op.ID))
				}
				// Same constraint on regular Responses
				if op.Responses.StatusCodeResponses != nil { // Safeguard
					for _, code := range sortedKeys(op.Responses.StatusCodeResponses) {
						r := op.Responses.StatusCodeResponses[code]
						res.Merge(d.validateDefaultInResponse(&r, "response", path, method, code, op.ID))
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
		d.resetVisited()
		definitions := s.spec.Spec().Definitions
		for _, nm := range sortedKeys(definitions) {
			sch := definitions[nm]
			res.Merge(d.validateDefaultValueSchemaAgainstSchema(newPathSegments(swaggerDefinitions, nm), "body", &sch))
		}
	}
	return res
}

func (d *defaultValidator) validateDefaultInResponse(
	resp *spec.Response, responseType, path, method string, responseCode int, operationID string,
) *Result {
	s := d.SpecValidator

	responseName, responseCodeAsStr := responseHelp.responseMsgVariants(responseType, responseCode)
	response, res := responseHelp.expandResponseRef(resp, path, responsePath(path, method, responseCodeAsStr), s)
	if !res.IsValid() {
		return res
	}

	if response.Headers != nil { // Safeguard
		for _, nm := range sortedKeys(response.Headers) {
			h := response.Headers[nm]
			// reset explored schemas to get depth-first recursive-proof exploration
			d.resetVisited()

			if h.Default != nil {
				red := newHeaderValidator(nm, &h, s.KnownFormats, d.schemaOptions).Validate(h.Default) //#nosec
				red.relocate(responseHeaderPath(path, method, responseCodeAsStr, nm).child(jsonDefault))
				if red.HasErrorsOrWarnings() {
					res.addErrorsAt(responseHeaderPath(path, method, responseCodeAsStr, nm), defaultValueHeaderDoesNotValidateMsg(operationID, nm, responseName))
					res.Merge(red)
				} else if red.wantsRedeemOnMerge {
					redeemResult(red)
				}
			}

			// Headers have inline definition, like params
			if h.Items != nil {
				red := d.validateDefaultValueItemsAgainstSchema(responseHeaderPath(path, method, responseCodeAsStr, nm), "header", &h, h.Items) //#nosec
				if red.HasErrorsOrWarnings() {
					res.addErrorsAt(responseHeaderPath(path, method, responseCodeAsStr, nm), defaultValueHeaderItemsDoesNotValidateMsg(operationID, nm, responseName))
					res.Merge(red)
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
		d.resetVisited()

		red := d.validateDefaultValueSchemaAgainstSchema(
			responsePath(path, method, responseCodeAsStr).structuralChild(jsonSchema), "response", response.Schema)
		if red.HasErrorsOrWarnings() {
			// Additional message to make sure the context of the error is not lost
			res.addErrorsAt(responsePath(path, method, responseCodeAsStr), defaultValueInDoesNotValidateMsg(operationID, responseName))
			res.Merge(red)
		} else if red.wantsRedeemOnMerge {
			redeemResult(red)
		}
	}
	return res
}

func (d *defaultValidator) validateDefaultValueSchemaAgainstSchema(path pathSegments, in string, schema *spec.Schema) *Result {
	if schema == nil || d.isVisited(path) {
		// Avoids recursing if we are already done with that check
		return nil
	}
	d.beingVisited(path)
	res := validatorPools.results.Borrow()
	s := d.SpecValidator

	if schema.Default != nil {
		res.Merge(
			newSchemaValidator(schema, s.spec.Spec(), path.child(jsonDefault), s.KnownFormats, d.schemaOptions).Validate(schema.Default),
		)
	}
	if schema.Items != nil {
		if schema.Items.Schema != nil {
			res.Merge(d.validateDefaultValueSchemaAgainstSchema(path.child(jsonItems), in, schema.Items.Schema))
		}
		// Multiple schemas in items
		if schema.Items.Schemas != nil { // Safeguard
			for i, sch := range schema.Items.Schemas {
				res.Merge(d.validateDefaultValueSchemaAgainstSchema(path.child(jsonItems).item(i), in, &sch)) //#nosec
			}
		}
	}
	if _, err := compileRegexp(schema.Pattern); err != nil {
		res.addErrorsAt(path, invalidPatternInMsg(path.dotted(), in, schema.Pattern))
	}
	if schema.AdditionalItems != nil && schema.AdditionalItems.Schema != nil {
		// NOTE: we keep validating values, even though additionalItems is not supported by Swagger 2.0 (and 3.0 as well)
		res.Merge(d.validateDefaultValueSchemaAgainstSchema(path.child(jsonAdditionalItems), in, schema.AdditionalItems.Schema))
	}
	for _, propName := range sortedKeys(schema.Properties) {
		prop := schema.Properties[propName]
		res.Merge(d.validateDefaultValueSchemaAgainstSchema(path.structuralChild(jsonProperties).child(propName), in, &prop))
	}
	for _, propName := range sortedKeys(schema.PatternProperties) {
		prop := schema.PatternProperties[propName]
		res.Merge(d.validateDefaultValueSchemaAgainstSchema(path.structuralChild(jsonPatternProperties).child(propName), in, &prop))
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		res.Merge(d.validateDefaultValueSchemaAgainstSchema(path.child(jsonAdditionalProperties), in, schema.AdditionalProperties.Schema))
	}
	if schema.AllOf != nil {
		for i, aoSch := range schema.AllOf {
			res.Merge(d.validateDefaultValueSchemaAgainstSchema(path.child(jsonAllOf).item(i), in, &aoSch)) //#nosec
		}
	}
	return res
}

// NOTE: Temporary duplicated code. Need to refactor with examples

func (d *defaultValidator) validateDefaultValueItemsAgainstSchema(path pathSegments, in string, root any, items *spec.Items) *Result {
	res := validatorPools.results.Borrow()
	s := d.SpecValidator
	if items != nil {
		if items.Default != nil {
			res.Merge(
				newItemsValidator(path, in, items, root, s.KnownFormats, d.schemaOptions).Validate(0, items.Default),
			)
		}
		if items.Items != nil {
			res.Merge(d.validateDefaultValueItemsAgainstSchema(path.item(0), in, root, items.Items))
		}
		if _, err := compileRegexp(items.Pattern); err != nil {
			res.addErrorsAt(path, invalidPatternInMsg(path.dotted(), in, items.Pattern))
		}
	}
	return res
}
