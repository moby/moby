// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/jsonpointer"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag/jsonutils"
)

// Spec validates an OpenAPI 2.0 specification document.
//
// Returns an error flattening in a single standard error, all validation messages.
//
// Options are forwarded to the underlying [SpecValidator]; in particular [WithPathLoader] injects a
// confined document loader for validating a specification from an untrusted source.
//
//   - Proposal for enhancement: $ref should not have siblings
//   - Proposal for enhancement: make sure documentation reflects all checks and warnings
//   - Proposal for enhancement: explicit message on unsupported keywords (better than "forbidden property"...)
//   - Proposal for enhancement: full list of unresolved refs
//   - Proposal for enhancement: validate numeric constraints (issue#581): this should be handled like defaults and examples
//   - Proposal for enhancement: option to determine if we validate for go-swagger or in a more general context
//   - Proposal for enhancement: check on required properties to support anyOf, allOf, oneOf
//
// NOTE: SecurityScopes are maps: no need to check uniqueness.
func Spec(doc *loads.Document, formats strfmt.Registry, options ...Option) error {
	errs, _ /*warns*/ := NewSpecValidator(doc.Schema(), formats, options...).Validate(doc)
	if errs.HasErrors() {
		return errors.CompositeValidationError(errs.Errors...)
	}
	return nil
}

// SpecValidator validates a swagger 2.0 spec.
type SpecValidator struct {
	schema         *spec.Schema // swagger 2.0 schema
	spec           *loads.Document
	analyzer       *analysis.Spec
	expanded       *loads.Document
	refLocations   refLocations
	refRedirects   refRedirects
	paramLocations paramLocations
	document       any // the document as decoded, to tell what it holds
	KnownFormats   strfmt.Registry
	Options        Opts // validation options
	schemaOptions  *SchemaValidatorOptions
}

// NewSpecValidator creates a new swagger spec validator instance.
//
// Options apply to the schema validators used internally. In particular, [WithPathLoader] injects
// the document loader used to resolve $ref while validating the specification — set a confined
// loader when validating a specification from an untrusted source (see the package "Security"
// notes on [WithPathLoader]).
func NewSpecValidator(schema *spec.Schema, formats strfmt.Registry, options ...Option) *SpecValidator {
	// schema options that apply to all called validators: built-in defaults first, then
	// caller-supplied options (which may add a loader or override a default).
	schemaOptions := new(SchemaValidatorOptions)
	for _, o := range append([]Option{
		SwaggerSchema(true),
		WithRecycleValidators(true),
		// withRecycleResults(true),
	}, options...) {
		o(schemaOptions)
	}

	return &SpecValidator{
		schema:        schema,
		KnownFormats:  formats,
		Options:       defaultOpts,
		schemaOptions: schemaOptions,
	}
}

// Validate validates the swagger spec.
func (s *SpecValidator) Validate(data any) (*Result, *Result) {
	s.schemaOptions.skipSchemataResult = s.Options.SkipSchemataResult
	var sd *loads.Document
	errs, warnings := new(Result), new(Result)

	if v, ok := data.(*loads.Document); ok {
		sd = v
	}
	if sd == nil {
		errs.AddErrors(invalidDocumentMsg())
		return errs, warnings // no point in continuing
	}

	// Validation expands what it walks - schemata, but also parameters, path items and responses -
	// and expansion rewrites what it is given. Take one copy of the whole document here and work
	// on that, so the caller gets back the document it handed over. Raw() still reads the bytes as
	// they were authored, so the checks below that go through them are unaffected.
	//
	// Cloning here rather than per schema is what keeps the cost flat: a copy taken inside
	// newSchemaValidator is paid again at every level of a recursive document.
	raw := sd.Raw()
	sd = sd.Pristine()
	s.schemaOptions.ownSchemata = true
	s.spec = sd
	s.analyzer = analysis.New(sd.Spec())
	// where each $ref sits, as authored: refs are reported against the
	// unexpanded document, before expansion flattens them away
	s.refLocations = newRefLocations(s.analyzer)
	// where each operation declares its parameters: the document addresses
	// them by index, and expansion loses that
	s.paramLocations = newParamLocations(sd.Spec())
	// where each $ref leads: checks walk the expanded document, and a finding
	// below a $ref has to be brought back to a node the document contains
	s.refRedirects = newRefRedirects(s.analyzer)

	// Raw spec unmarshalling errors
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		// NOTE: under normal conditions, the *load.Document has been already unmarshalled
		// So this one is just a paranoid check on the behavior of the spec package
		panic(InvalidDocumentError)
	}
	s.document = obj

	defer func() {
		// bring findings reached through a $ref back onto the document, then
		// hold every location to what the document actually addresses
		errs.redirect(s.refRedirects.through)
		errs.redirect(s.resolvable)
		// errs holds all errors and warnings,
		// warnings only warnings
		errs.MergeAsWarnings(warnings)
		// reported as errors of the warnings-only result, but keeping the
		// location each was recorded with
		warnings.carryErrors(errs.Warnings, errs.warningLocations)
	}()

	// Swagger schema validator
	schv := newSchemaValidator(s.schema, nil, rootPath(), s.KnownFormats, s.schemaOptions)
	errs.Merge(schv.Validate(obj)) // error -
	// There may be a point in continuing to try and determine more accurate errors
	if !s.Options.ContinueOnErrors && errs.HasErrors() {
		return errs, warnings // no point in continuing
	}

	errs.Merge(s.validateReferencesValid()) // error -
	// There may be a point in continuing to try and determine more accurate errors
	if !s.Options.ContinueOnErrors && errs.HasErrors() {
		return errs, warnings // no point in continuing
	}

	errs.Merge(s.validateDuplicateOperationIDs())
	errs.Merge(s.validateDuplicatePropertyNames()) // error -
	errs.Merge(s.validateParameters())             // error -
	errs.Merge(s.validateItems())                  // error -
	errs.Merge(s.validateSecurityRequirements())   // error and warning
	errs.Merge(s.validateDiscriminators())         // error -
	errs.Merge(s.validateCollectionFormats())      // warning only

	// Properties in required definition MUST validate their schema
	// Properties SHOULD NOT be declared as both required and readOnly (warning)
	errs.Merge(s.validateRequiredDefinitions()) // error and warning

	// There may be a point in continuing to try and determine more accurate errors
	if !s.Options.ContinueOnErrors && errs.HasErrors() {
		return errs, warnings // no point in continuing
	}

	// Values provided as default MUST validate their schema
	df := &defaultValidator{SpecValidator: s, schemaOptions: s.schemaOptions}
	errs.Merge(df.Validate())

	// Values provided as examples MUST validate their schema
	// Value provided as examples in a response without schema generate a warning
	// Known limitations: examples in responses for mime type not application/json are ignored (warning)
	ex := &exampleValidator{SpecValidator: s, schemaOptions: s.schemaOptions}
	errs.Merge(ex.Validate())

	errs.Merge(s.validateNonEmptyPathParamNames())

	// errs.Merge(s.validateRefNoSibling()) // warning only
	errs.Merge(s.validateReferenced())  // warning only
	errs.Merge(s.validateDubiousRefs()) // warning only

	return errs, warnings
}

// SetContinueOnErrors sets the ContinueOnErrors option for this validator.
func (s *SpecValidator) SetContinueOnErrors(c bool) {
	s.Options.ContinueOnErrors = c
}

func (s *SpecValidator) validateNonEmptyPathParamNames() *Result {
	res := validatorPools.results.Borrow()
	if s.spec.Spec().Paths == nil {
		// There is no Paths object: the document itself lacks it, so
		// there is no node below it to point at
		res.addErrorsAt(rootPath(), noValidPathMsg())

		return res
	}

	if s.spec.Spec().Paths.Paths == nil {
		// Paths may be empty: warning
		res.addWarningsAt(newPathSegments(swaggerPaths), noValidPathMsg())

		return res
	}

	for _, k := range sortedKeys(s.spec.Spec().Paths.Paths) {
		if strings.Contains(k, "{}") {
			res.addErrorsAt(newPathSegments(swaggerPaths, k), emptyPathParameterMsg(k))
		}
	}

	return res
}

func (s *SpecValidator) validateDuplicateOperationIDs() *Result {
	// OperationID, if specified, must be unique across the board
	var analyzer *analysis.Spec
	if s.expanded != nil {
		// $ref are valid: we can analyze operations on an expanded spec
		analyzer = analysis.New(s.expanded.Spec())
	} else {
		// fallback on possible incomplete picture because of previous errors
		analyzer = s.analyzer
	}
	res := validatorPools.results.Borrow()

	// the message says how many times an identifier is used, so the count is
	// what it needs; a reader needs somewhere to go, so the first operation to
	// declare the identifier is remembered along with it
	known := make(map[string]int)
	declaredAt := make(map[string]pathSegments)
	operations := analyzer.Operations()
	for _, method := range sortedKeys(operations) {
		byPath := operations[method]
		for _, path := range sortedKeys(byPath) {
			op := byPath[path]
			id := operationIdentity(method, path, op)
			known[id]++
			if _, isKnown := declaredAt[id]; !isKnown {
				declaredAt[id] = operationIDPath(path, method, op)
			}
		}
	}

	for _, k := range sortedKeys(known) {
		if v := known[k]; v > 1 {
			res.addErrorsAt(declaredAt[k], nonUniqueOperationIDMsg(k, v))
		}
	}
	return res
}

// operationIdentity names an operation the way the analyzer does: by its
// operationId, or by method and path when it declares none.
func operationIdentity(method, path string, op *spec.Operation) string {
	if op == nil || op.ID == "" {
		return strings.ToUpper(method) + " " + path
	}

	return op.ID
}

// operationIDPath locates the operationId of an operation, or the operation
// itself when it declares none.
func operationIDPath(path, method string, op *spec.Operation) pathSegments {
	at := operationPath(path, method)
	if op == nil || op.ID == "" {
		return at
	}

	return at.child(swaggerOperationID)
}

type dupProp struct {
	Name       string
	Definition string
}

func (s *SpecValidator) validateDuplicatePropertyNames() *Result {
	// definition can't declare a property that's already defined by one of its ancestors
	res := validatorPools.results.Borrow()
	definitions := s.spec.Spec().Definitions
	for _, k := range sortedKeys(definitions) {
		sch := definitions[k]
		if len(sch.AllOf) == 0 {
			continue
		}

		knownanc := map[string]struct{}{
			"#/definitions/" + k: {},
		}

		ancs, rec := s.validateCircularAncestry(k, sch, knownanc)
		if rec != nil && (rec.HasErrors() || !rec.HasWarnings()) {
			res.Merge(rec)
		}
		if len(ancs) > 0 {
			res.addErrorsAt(newPathSegments(swaggerDefinitions, k), circularAncestryDefinitionMsg(k, ancs))
			if !s.Options.ContinueOnErrors {
				return res
			}

			// the ancestry loops back on itself: searching it for duplicate
			// property names would not terminate, so this definition stops here
			// and the next one is examined.
			continue
		}

		knowns := make(map[string]struct{})
		dups, rep := s.validateSchemaPropertyNames(k, sch, knowns)
		if rep != nil && (rep.HasErrors() || rep.HasWarnings()) {
			res.Merge(rep)
		}
		if len(dups) > 0 {
			var pns []string
			for _, v := range dups {
				pns = append(pns, v.Definition+"."+v.Name)
			}
			res.addErrorsAt(newPathSegments(swaggerDefinitions, k), duplicatePropertiesMsg(k, pns))
		}

	}
	return res
}

func (s *SpecValidator) resolveRef(ref *spec.Ref) (*spec.Schema, error) {
	if s.spec.SpecFilePath() != "" {
		return spec.ResolveRefWithBase(s.spec.Spec(), ref, s.schemaOptions.expandOptions(s.spec.SpecFilePath()))
	}
	// NOTE: it looks like with the new spec resolver, this code is now unrecheable
	return spec.ResolveRef(s.spec.Spec(), ref)
}

func (s *SpecValidator) validateSchemaPropertyNames(nm string, sch spec.Schema, knowns map[string]struct{}) ([]dupProp, *Result) {
	var dups []dupProp

	schn := nm
	schc := &sch
	res := validatorPools.results.Borrow()

	for schc.Ref.String() != "" {
		// gather property names
		reso, err := s.resolveRef(&schc.Ref)
		if err != nil {
			errorHelp.addPointerError(res, err, schc.Ref.String(), nm)
			return dups, res
		}
		schc = reso
		schn = sch.Ref.String()
	}

	if len(schc.AllOf) > 0 {
		for _, chld := range schc.AllOf {
			dup, rep := s.validateSchemaPropertyNames(schn, chld, knowns)
			if rep != nil && (rep.HasErrors() || rep.HasWarnings()) {
				res.Merge(rep)
			}
			dups = append(dups, dup...)
		}
		return dups, res
	}

	for _, k := range sortedKeys(schc.Properties) {
		_, ok := knowns[k]
		if ok {
			dups = append(dups, dupProp{Name: k, Definition: schn})
		} else {
			knowns[k] = struct{}{}
		}
	}

	return dups, res
}

func (s *SpecValidator) validateCircularAncestry(nm string, sch spec.Schema, knowns map[string]struct{}) ([]string, *Result) {
	res := validatorPools.results.Borrow()

	if sch.Ref.String() == "" && len(sch.AllOf) == 0 { // Safeguard. We should not be able to actually get there
		return nil, res
	}
	var ancs []string

	schn := nm
	schc := &sch

	for schc.Ref.String() != "" {
		reso, err := s.resolveRef(&schc.Ref)
		if err != nil {
			errorHelp.addPointerError(res, err, schc.Ref.String(), nm)
			return ancs, res
		}
		schc = reso
		schn = sch.Ref.String()
	}

	if schn != nm && schn != "" {
		if _, ok := knowns[schn]; ok {
			ancs = append(ancs, schn)
		}
		knowns[schn] = struct{}{}

		if len(ancs) > 0 {
			return ancs, res
		}
	}

	if len(schc.AllOf) > 0 {
		for _, chld := range schc.AllOf {
			if chld.Ref.String() != "" || len(chld.AllOf) > 0 {
				anc, rec := s.validateCircularAncestry(schn, chld, knowns)
				if rec != nil && (rec.HasErrors() || !rec.HasWarnings()) {
					res.Merge(rec)
				}
				ancs = append(ancs, anc...)
				if len(ancs) > 0 {
					return ancs, res
				}
			}
		}
	}
	return ancs, res
}

//nolint:gocognit // refactor in a forthcoming PR
func (s *SpecValidator) validateItems() *Result {
	// validate parameter, items, schema and response objects for presence of item if type is array
	res := validatorPools.results.Borrow()

	operations := s.analyzer.Operations()
	for _, method := range sortedKeys(operations) {
		pi := operations[method]
		for _, path := range sortedKeys(pi) {
			op := pi[path]
			for _, param := range paramHelp.safeExpandedParamsFor(path, method, op.ID, res, s) {

				if param.TypeName() == arrayType && param.ItemsTypeName() == "" {
					res.addErrorsAt(s.parameterPath(path, method, param.In, param.Name), arrayInParamRequiresItemsMsg(param.Name, op.ID))
					continue
				}
				if param.In != swaggerBody {
					if param.Items != nil {
						items := param.Items
						for items.TypeName() == arrayType {
							if items.ItemsTypeName() == "" {
								res.addErrorsAt(s.parameterPath(path, method, param.In, param.Name), arrayInParamRequiresItemsMsg(param.Name, op.ID))
								break
							}
							items = items.Items
						}
					}
				} else {
					// In: body
					if param.Schema != nil {
						res.Merge(s.validateSchemaItems(*param.Schema, s.parameterPath(path, method, param.In, param.Name).child(jsonSchema),
							fmt.Sprintf("body param %q", param.Name), op.ID))
					}
				}
			}

			for _, resp := range responsesOf(op) {
				at := responsePath(path, method, resp.code)
				// Response headers with array
				for _, hn := range sortedKeys(resp.resp.Headers) {
					if hv := resp.resp.Headers[hn]; hv.TypeName() == arrayType && hv.ItemsTypeName() == "" {
						res.addErrorsAt(at.children(swaggerHeaders, hn), arrayInHeaderRequiresItemsMsg(hn, op.ID))
					}
				}
				if resp.resp.Schema != nil {
					res.Merge(s.validateSchemaItems(*resp.resp.Schema, at.child(jsonSchema), "response body", op.ID))
				}
			}
		}
	}
	return res
}

// Verifies constraints on array type.
func (s *SpecValidator) validateSchemaItems(schema spec.Schema, at pathSegments, prefix, opID string) *Result {
	res := validatorPools.results.Borrow()
	if !schema.Type.Contains(arrayType) {
		return res
	}

	if schema.Items == nil || schema.Items.Len() == 0 {
		res.addErrorsAt(at, arrayRequiresItemsMsg(prefix, opID))
		return res
	}

	if schema.Items.Schema != nil {
		schema = *schema.Items.Schema
		if _, err := compileRegexp(schema.Pattern); err != nil {
			res.addErrorsAt(at, invalidItemsPatternMsg(prefix, opID, schema.Pattern))
		}

		res.Merge(s.validateSchemaItems(schema, at.child(jsonItems), prefix, opID))
	}
	return res
}

func (s *SpecValidator) validatePathParamPresence(path string, fromPath, fromOperation []string) *Result {
	// Each defined operation path parameters must correspond to a named element in the API's path pattern.
	// (For example, you cannot have a path parameter named id for the following path /pets/{petId} but you must have a path parameter named petId.)
	res := validatorPools.results.Borrow()
	for _, l := range fromPath {
		var matched bool
		for _, r := range fromOperation {
			if l == "{"+r+"}" {
				matched = true
				break
			}
		}
		if !matched {
			res.addErrorsAt(newPathSegments(swaggerPaths, path), noParameterInPathMsg(l))
		}
	}

	for _, p := range fromOperation {
		var matched bool
		if slices.Contains(fromPath, "{"+p+"}") {
			matched = true
		}
		if !matched {
			res.addErrorsAt(newPathSegments(swaggerPaths, path), pathParamNotInPathMsg(path, p))
		}
	}

	return res
}

func (s *SpecValidator) validateReferenced() *Result {
	var res Result
	res.MergeAsWarnings(s.validateReferencedParameters())
	res.MergeAsWarnings(s.validateReferencedResponses())
	res.MergeAsWarnings(s.validateReferencedDefinitions())
	return &res
}

func (s *SpecValidator) validateReferencedParameters() *Result {
	// Each referenceable definition should have references.
	params := s.spec.Spec().Parameters
	if len(params) == 0 {
		return nil
	}

	expected := make(map[string]struct{})
	for k := range params {
		expected["#/parameters/"+jsonpointer.Escape(k)] = struct{}{}
	}
	for _, k := range s.analyzer.AllParameterReferences() {
		delete(expected, k)
	}

	if len(expected) == 0 {
		return nil
	}
	result := validatorPools.results.Borrow()
	for _, k := range sortedKeys(expected) {
		result.addWarningsAt(localRefPath(k), unusedParamMsg(k))
	}
	return result
}

func (s *SpecValidator) validateReferencedResponses() *Result {
	// Each referenceable definition should have references.
	responses := s.spec.Spec().Responses
	if len(responses) == 0 {
		return nil
	}

	expected := make(map[string]struct{})
	for k := range responses {
		expected["#/responses/"+jsonpointer.Escape(k)] = struct{}{}
	}
	for _, k := range s.analyzer.AllResponseReferences() {
		delete(expected, k)
	}

	if len(expected) == 0 {
		return nil
	}

	result := validatorPools.results.Borrow()
	for _, k := range sortedKeys(expected) {
		result.addWarningsAt(localRefPath(k), unusedResponseMsg(k))
	}

	return result
}

func (s *SpecValidator) validateReferencedDefinitions() *Result {
	// Each referenceable definition must have references.
	defs := s.spec.Spec().Definitions
	if len(defs) == 0 {
		return nil
	}

	expected := make(map[string]struct{})
	for k := range defs {
		expected["#/definitions/"+jsonpointer.Escape(k)] = struct{}{}
	}
	for _, k := range s.analyzer.AllDefinitionReferences() {
		delete(expected, k)
	}

	if len(expected) == 0 {
		return nil
	}

	result := new(Result)
	for _, k := range sortedKeys(expected) {
		result.addWarningsAt(localRefPath(k), unusedDefinitionMsg(k))
	}
	return result
}

func (s *SpecValidator) validateRequiredDefinitions() *Result {
	// Each property listed in the required array must be defined in the properties of the model
	res := validatorPools.results.Borrow()

	definitions := s.spec.Spec().Definitions

DEFINITIONS:
	for _, d := range sortedKeys(definitions) {
		schema := definitions[d]
		red := validatorPools.results.Borrow()
		keepGoing := s.walkRequired(newPathSegments(swaggerDefinitions, d), &schema, red) //#nosec
		res.Merge(red)
		if !keepGoing {
			break DEFINITIONS // there is an error, let's stop that bleeding
		}
	}
	return res
}

// validateRequiredProperties checks one entry of a required array.
//
// schemaAt locates the schema being searched for the property, which moves as
// the search descends into additionalProperties. requiredAt locates the entry
// of the required array that started it, and stays put.
func (s *SpecValidator) validateRequiredProperties(
	path string, of schemaIdentity, schemaAt, requiredAt pathSegments, v *spec.Schema,
) *Result {
	in := of.name
	// Takes care of recursive property definitions, which may be nested in additionalProperties schemas
	res := validatorPools.results.Borrow()
	propertyMatch := false
	patternMatch := false
	additionalPropertiesMatch := false
	isReadOnly := false

	// Regular properties, including those a base definition contributes
	if readOnly, declared := s.declaresProperty(v, path, maxCompositionHops); declared {
		propertyMatch = true
		isReadOnly = readOnly
	}

	// NOTE: patternProperties are not supported in swagger. Even though, we continue validation here
	// We check all defined patterns: if one regexp is invalid, croaks an error
	for _, pp := range sortedKeys(v.PatternProperties) {
		re, err := compileRegexp(pp)
		if err != nil {
			res.addErrorsAt(schemaAt, invalidPatternMsg(pp, in))
		} else if re.MatchString(path) {
			patternMatch = true
			if !propertyMatch {
				isReadOnly = v.PatternProperties[pp].ReadOnly
			}
		}
	}

	if !propertyMatch && !patternMatch {
		if v.AdditionalProperties != nil {
			if v.AdditionalProperties.Allows && v.AdditionalProperties.Schema == nil {
				additionalPropertiesMatch = true
			} else if v.AdditionalProperties.Schema != nil {
				// additionalProperties as schema are upported in swagger
				// recursively validates additionalProperties schema
				// Proposal for enhancement: anyOf, allOf, oneOf like in schemaPropsValidator
				red := s.validateRequiredProperties(path, of, schemaAt.child(jsonAdditionalProperties), requiredAt, v.AdditionalProperties.Schema)
				if red.IsValid() {
					additionalPropertiesMatch = true
					if !propertyMatch && !patternMatch {
						isReadOnly = v.AdditionalProperties.Schema.ReadOnly
					}
				}
				res.Merge(red)
			}
		}
	}

	if !propertyMatch && !patternMatch && !additionalPropertiesMatch {
		res.addErrorsAt(requiredAt, of.requiredButNotDefined(path))
	}

	if isReadOnly {
		res.addWarningsAt(requiredAt, readOnlyAndRequiredMsg(in, path))
	}
	return res
}

//nolint:gocognit // refactor in a forthcoming PR
func (s *SpecValidator) validateParameters() *Result {
	// - for each method, path is unique, regardless of path parameters
	//   e.g. GET:/petstore/{id}, GET:/petstore/{pet}, GET:/petstore are
	//   considered duplicate paths, if StrictPathParamUniqueness is enabled.
	// - each parameter should have a unique `name` and `type` combination
	// - each operation should have only 1 parameter of type body
	// - there must be at most 1 parameter in body
	// - parameters with pattern property must specify valid patterns
	// - $ref in parameters must resolve
	// - path param must be required
	res := validatorPools.results.Borrow()
	rexGarbledPathSegment := mustCompileRegexp(`.*[{}\s]+.*`)
	operations := s.expandedAnalyzer().Operations()
	for _, method := range sortedKeys(operations) {
		pi := operations[method]
		methodPaths := make(map[string]map[string]string)
		for _, path := range sortedKeys(pi) {
			op := pi[path]
			if s.Options.StrictPathParamUniqueness {
				pathToAdd := pathHelp.stripParametersInPath(path)

				// Warn on garbled path afer param stripping
				if rexGarbledPathSegment.MatchString(pathToAdd) {
					res.addWarningsAt(newPathSegments(swaggerPaths, path), pathStrippedParamGarbledMsg(pathToAdd))
				}

				// Check uniqueness of stripped paths
				if _, found := methodPaths[method][pathToAdd]; found {
					// Sort names for stable, testable output
					if strings.Compare(path, methodPaths[method][pathToAdd]) < 0 {
						res.addErrorsAt(newPathSegments(swaggerPaths, path), pathOverlapMsg(path, methodPaths[method][pathToAdd]))
					} else {
						res.addErrorsAt(newPathSegments(swaggerPaths, path), pathOverlapMsg(methodPaths[method][pathToAdd], path))
					}
				} else {
					if _, found := methodPaths[method]; !found {
						methodPaths[method] = map[string]string{}
					}
					methodPaths[method][pathToAdd] = path // Original non stripped path

				}
			}

			var bodyParams []string
			var paramNames []string
			var hasForm, hasBody bool

			// Check parameters names uniqueness for operation
			// NOTE: should be done after param expansion
			res.Merge(s.checkUniqueParams(path, method, op))

			// pick the root schema from the swagger specification which describes a parameter
			origSchema, ok := s.schema.Definitions["parameter"]
			if !ok {
				panic("unexpected swagger schema: missing #/definitions/parameter")
			}
			// clone it once to avoid expanding a global schema (e.g. swagger spec)
			paramSchema, err := deepCloneSchema(origSchema)
			if err != nil {
				panic(fmt.Errorf("can't clone schema: %w", err))
			}

			for _, pr := range paramHelp.safeExpandedParamsFor(path, method, op.ID, res, s) {
				// An expanded parameter must validate the Parameter schema (an unexpanded $ref always passes high-level schema validation)
				schv := newSchemaValidator(&paramSchema, s.schema, s.parameterPath(path, method, pr.In, pr.Name), s.KnownFormats, s.schemaOptions)
				var obj any
				if err := jsonutils.FromDynamicJSON(pr, &obj); err != nil {
					res.addErrorsAt(s.parameterPath(path, method, pr.In, pr.Name), err)

					return res
				}

				res.Merge(schv.Validate(obj))

				// Validate pattern regexp for parameters with a Pattern property
				if _, err := compileRegexp(pr.Pattern); err != nil {
					res.addErrorsAt(s.parameterPath(path, method, pr.In, pr.Name), invalidPatternInParamMsg(op.ID, pr.Name, pr.Pattern))
				}

				// There must be at most one parameter in body: list them all
				if pr.In == swaggerBody {
					bodyParams = append(bodyParams, fmt.Sprintf("%q", pr.Name))
					hasBody = true
				}

				if pr.In == "path" {
					paramNames = append(paramNames, pr.Name)
					// Path declared in path must have the required: true property
					if !pr.Required {
						res.addErrorsAt(s.parameterPath(path, method, pr.In, pr.Name), pathParamRequiredMsg(op.ID, pr.Name))
					}
				}

				if pr.In == "formData" {
					hasForm = true
				}

				if pr.Type != numberType && pr.Type != integerType &&
					(pr.Maximum != nil || pr.Minimum != nil || pr.MultipleOf != nil) {
					// A non-numeric parameter has validation keywords for numeric instances (number and integer)
					res.addWarningsAt(s.parameterPath(path, method, pr.In, pr.Name), parameterValidationTypeMismatchMsg(pr.Name, path, pr.Type))
				}

				if pr.Type != stringType &&
					// A non-string parameter has validation keywords for strings
					(pr.MaxLength != nil || pr.MinLength != nil || pr.Pattern != "") {
					res.addWarningsAt(s.parameterPath(path, method, pr.In, pr.Name), parameterValidationTypeMismatchMsg(pr.Name, path, pr.Type))
				}

				if pr.Type != arrayType &&
					// A non-array parameter has validation keywords for arrays
					(pr.MaxItems != nil || pr.MinItems != nil || pr.UniqueItems) {
					res.addWarningsAt(s.parameterPath(path, method, pr.In, pr.Name), parameterValidationTypeMismatchMsg(pr.Name, path, pr.Type))
				}
			}

			// In:formData and In:body are mutually exclusive
			if hasBody && hasForm {
				res.addErrorsAt(operationPath(path, method), bothFormDataAndBodyMsg(op.ID))
			}
			// There must be at most one body param
			// Accurately report situations when more than 1 body param is declared (possibly unnamed)
			if len(bodyParams) > 1 {
				sort.Strings(bodyParams)
				res.addErrorsAt(operationPath(path, method), multipleBodyParamMsg(op.ID, bodyParams))
			}

			// Check uniqueness of parameters in path
			paramsInPath := pathHelp.extractPathParams(path)
			for i, p := range paramsInPath {
				for j, q := range paramsInPath {
					if p == q && i > j {
						res.addErrorsAt(newPathSegments(swaggerPaths, path), pathParamNotUniqueMsg(path, p, q))
						break
					}
				}
			}

			// Warns about possible malformed params in path
			rexGarbledParam := mustCompileRegexp(`{.*[{}\s]+.*}`)
			for _, p := range paramsInPath {
				if rexGarbledParam.MatchString(p) {
					res.addWarningsAt(newPathSegments(swaggerPaths, path), pathParamGarbledMsg(path, p))
				}
			}

			// Match params from path vs params from params section
			res.Merge(s.validatePathParamPresence(path, paramsInPath, paramNames))
		}
	}
	return res
}

func (s *SpecValidator) validateReferencesValid() *Result {
	// each reference must point to a valid object
	res := validatorPools.results.Borrow()
	for _, r := range sortedRefs(s.analyzer.AllRefs()) {
		if !r.IsValidURI(s.spec.SpecFilePath()) { // Safeguard - spec should always yield a valid URI
			res.addErrorsAt(s.refLocations.at(r.String()), invalidRefMsg(r.String()))
		}
	}
	if !res.HasErrors() {
		// NOTE: with default settings, loads.Document.Expanded()
		// stops on first error. Anyhow, the expand option to continue
		// on errors fails to report errors at all.
		//
		// Pass the injected loader (if any) so whole-spec expansion is confined too. When no loader
		// is set, this is a no-op: loads falls back to the document's own loader.
		exp, err := s.spec.Expanded(s.schemaOptions.expandOptions(""))
		if err != nil {
			res.addErrorsAt(s.firstUnresolvableRef(), unresolvedReferencesMsg(err))
		}
		s.expanded = exp
	}
	return res
}

// firstUnresolvableRef locates the declaration of the first local $ref, in
// document order, that points at a node the document does not hold.
//
// Expansion reports the whole document in a single message, naming only the
// reference it happened to trip on, so the finding has no location of its own.
// A document usually has one broken reference; when it has several, this is the
// first one a reader would meet.
func (s *SpecValidator) firstUnresolvableRef() pathSegments {
	first := rootPath()
	found := false

	for _, r := range s.analyzer.AllRefs() {
		value := r.String()
		if !strings.HasPrefix(value, "#/") {
			// a remote reference cannot be checked against the document alone
			continue
		}

		pointer, err := jsonpointer.New(strings.TrimPrefix(value, "#"))
		if err != nil {
			continue
		}
		if _, _, err := pointer.Get(s.document); err == nil {
			continue
		}

		at := s.refLocations.at(value)
		if !found || at.pointer() < first.pointer() {
			first, found = at, true
		}
	}

	return first
}

func (s *SpecValidator) checkUniqueParams(path, method string, op *spec.Operation) *Result {
	// Check for duplicate parameters declaration in param section.
	// Each parameter should have a unique `name` and `type` combination
	// NOTE: this could be factorized in analysis (when constructing the params map)
	// However, there are some issues with such a factorization:
	// - analysis does not seem to fully expand params
	// - param keys may be altered by x-go-name
	res := validatorPools.results.Borrow()
	pnames := make(map[string]struct{})

	if op.Parameters != nil { // Safeguard
		for _, ppr := range op.Parameters {
			var ok bool
			pr, red := paramHelp.resolveParam(path, method, op.ID, &ppr, s) //#nosec
			res.Merge(red)

			if pr != nil && pr.Name != "" { // params with empty name does no participate the check
				key := fmt.Sprintf("%s#%s", pr.In, pr.Name)

				if _, ok = pnames[key]; ok {
					res.addErrorsAt(s.parameterPath(path, method, pr.In, pr.Name), duplicateParamNameMsg(pr.In, pr.Name, op.ID))
				}
				pnames[key] = struct{}{}
			}
		}
	}
	return res
}

// expandedAnalyzer returns expanded.Analyzer when it is available.
// otherwise just analyzer.
func (s *SpecValidator) expandedAnalyzer() *analysis.Spec {
	if s.expanded != nil && s.expanded.Analyzer != nil {
		return s.expanded.Analyzer
	}
	return s.analyzer
}

// deepCloneSchema returns a copy of src that shares nothing with it.
//
// The copy goes through JSON, which is the form [spec.Schema] is defined by. gob drops any field
// holding its zero value and flattens a pointer to what it points at, so a *float64 pointing at
// 0 - "minimum": 0, which the JSON Schema meta-schema spells for every positiveInteger - came
// back nil and the bound was lost. JSON is also the faster of the two on this model.
func deepCloneSchema(src spec.Schema) (spec.Schema, error) {
	var dst spec.Schema
	if err := jsonutils.FromDynamicJSON(src, &dst); err != nil {
		return spec.Schema{}, err
	}

	return dst, nil
}
