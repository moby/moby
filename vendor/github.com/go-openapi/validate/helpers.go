// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

// Proposal for enhancement: define this as package validate/internal
// This must be done while keeping CI intact with all tests and test coverage

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/jsonpointer"
	"github.com/go-openapi/spec"
)

const (
	swaggerBody     = "body"
	swaggerExample  = "example"
	swaggerExamples = "examples"
)

const (
	objectType  = "object"
	arrayType   = "array"
	stringType  = "string"
	integerType = "integer"
	numberType  = "number"
	booleanType = "boolean"
	fileType    = "file"
	nullType    = "null"
)

const (
	jsonProperties        = "properties"
	jsonPatternProperties = "patternProperties"
	jsonItems             = "items"
	jsonType              = "type"
	jsonSchema            = "schema"
	jsonRequired          = "required"
	jsonRef               = "$ref"
	jsonDefault           = "default"
	jsonDiscriminator     = "discriminator"

	jsonAllOf                = "allOf"
	jsonAnyOf                = "anyOf"
	jsonOneOf                = "oneOf"
	jsonNot                  = "not"
	jsonAdditionalItems      = "additionalItems"
	jsonAdditionalProperties = "additionalProperties"

	swaggerPaths            = "paths"
	swaggerDefinitions      = "definitions"
	swaggerResponses        = "responses"
	swaggerParameters       = "parameters"
	swaggerHeaders          = "headers"
	swaggerOperationID      = "operationId"
	swaggerSecurity         = "security"
	swaggerCollectionFormat = "collectionFormat"

	// securitySchemeOAuth2 is the only security scheme type whose requirements carry scopes.
	securitySchemeOAuth2 = "oauth2"

	jsonMimeApplicationJSON = "application/json"
)

// operationPath locates an operation in the spec document.
func operationPath(path, method string) pathSegments {
	return newPathSegments(swaggerPaths, path, methodToken(method))
}

// parameterPath locates a parameter of an operation in the spec document.
func (s *SpecValidator) parameterPath(path, method, in, name string) pathSegments {
	return s.paramLocations.at(path, method, in, name)
}

// responsePath locates a response of an operation in the spec document.
func responsePath(path, method, responseCode string) pathSegments {
	return operationPath(path, method).children(swaggerResponses, responseCode)
}

// responseHeaderPath locates a header declared by a response.
func responseHeaderPath(path, method, responseCode, header string) pathSegments {
	return responsePath(path, method, responseCode).children(swaggerHeaders, header)
}

// methodToken normalizes an HTTP method into the key under which the operation
// is found in the document: the analyzer hands them over in upper case, but a
// path item spells them in lower case.
func methodToken(method string) string {
	return strings.ToLower(method)
}

// localRefPath turns a local JSON reference such as "#/definitions/Pet" into
// the location of what it points to.
//
// It yields the document root for anything that does not address a local
// fragment, a remote reference in particular.
func localRefPath(ref string) pathSegments {
	rest, isLocal := strings.CutPrefix(ref, "#/")
	if !isLocal {
		return rootPath()
	}

	tokens := strings.Split(rest, "/")
	for i, token := range tokens {
		tokens[i] = jsonpointer.Unescape(token)
	}

	return newPathSegments(tokens...)
}

const (
	stringFormatDate     = "date"
	stringFormatDateTime = "date-time"
	stringFormatPassword = "password"
	stringFormatByte     = "byte"
	// stringFormatBinary       = "binary".
	stringFormatCreditCard   = "creditcard"
	stringFormatDuration     = "duration"
	stringFormatEmail        = "email"
	stringFormatHexColor     = "hexcolor"
	stringFormatHostname     = "hostname"
	stringFormatIPv4         = "ipv4"
	stringFormatIPv6         = "ipv6"
	stringFormatISBN         = "isbn"
	stringFormatISBN10       = "isbn10"
	stringFormatISBN13       = "isbn13"
	stringFormatMAC          = "mac"
	stringFormatBSONObjectID = "bsonobjectid"
	stringFormatRGBColor     = "rgbcolor"
	stringFormatSSN          = "ssn"
	stringFormatURI          = "uri"
	stringFormatUUID         = "uuid"
	stringFormatUUID3        = "uuid3"
	stringFormatUUID4        = "uuid4"
	stringFormatUUID5        = "uuid5"

	integerFormatInt32  = "int32"
	integerFormatInt64  = "int64"
	integerFormatUInt32 = "uint32"
	integerFormatUInt64 = "uint64"

	numberFormatFloat32 = "float32"
	numberFormatFloat64 = "float64"
	numberFormatFloat   = "float"
	numberFormatDouble  = "double"
)

// Helpers available at the package level.
var (
	pathHelp     *pathHelper
	valueHelp    *valueHelper
	errorHelp    *errorHelper
	paramHelp    *paramHelper
	responseHelp *responseHelper
)

type errorHelper struct {
	// A collection of unexported helpers for error construction
}

func (h *errorHelper) sErr(err errors.Error, recycle bool) *Result {
	return h.sErrAt(nil, err, recycle)
}

// sErrAt builds a Result from a standard errors.Error reported at a known location.
func (h *errorHelper) sErrAt(at pathSegments, err errors.Error, recycle bool) *Result {
	var result *Result
	if recycle {
		result = validatorPools.results.Borrow()
	} else {
		result = new(Result)
	}
	result.addErrorsAt(at, err)

	return result
}

func (h *errorHelper) addPointerError(res *Result, err error, ref string, fromPath string) *Result {
	return h.addPointerErrorAt(res, nil, err, ref, fromPath)
}

// addPointerErrorAt provides more context on error messages reported by the
// jsonpointer package, by altering the passed Result.
func (h *errorHelper) addPointerErrorAt(res *Result, at pathSegments, err error, ref string, fromPath string) *Result {
	if err != nil {
		res.addErrorsAt(at, cannotResolveRefMsg(fromPath, ref, err))
	}

	return res
}

type pathHelper struct {
	// A collection of unexported helpers for path validation
}

func (h *pathHelper) stripParametersInPath(path string) string {
	// Returns a path stripped from all path parameters, with multiple or trailing slashes removed.
	//
	// Stripping is performed on a slash-separated basis, e.g '/a{/b}' remains a{/b} and not /a.
	//  - Trailing "/" make a difference, e.g. /a/ !~ /a (ex: canary/bitbucket.org/swagger.json)
	//  - presence or absence of a parameter makes a difference, e.g. /a/{log} !~ /a/ (ex: canary/kubernetes/swagger.json)

	// Regexp to extract parameters from path, with surrounding {}.
	// NOTE: important non-greedy modifier
	rexParsePathParam := mustCompileRegexp(`{[^{}]+?}`)
	segments := strings.Split(path, "/")
	strippedSegments := make([]string, len(segments))

	for i, segment := range segments {
		strippedSegments[i] = rexParsePathParam.ReplaceAllString(segment, "X")
	}
	return strings.Join(strippedSegments, "/")
}

func (h *pathHelper) extractPathParams(path string) (params []string) {
	// Extracts all params from a path, with surrounding "{}"
	rexParsePathParam := mustCompileRegexp(`{[^{}]+?}`)

	for segment := range strings.SplitSeq(path, "/") {
		for _, v := range rexParsePathParam.FindAllStringSubmatch(segment, -1) {
			params = append(params, v...)
		}
	}
	return
}

type valueHelper struct {
	// A collection of unexported helpers for value validation
}

func (h *valueHelper) asInt64(val any) int64 {
	// Number conversion function for int64, without error checking
	// (implements an implicit type upgrade).
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint()) //nolint:gosec
	case reflect.Float32, reflect.Float64:
		return int64(v.Float())
	default:
		// panic("Non numeric value in asInt64()")
		return 0
	}
}

func (h *valueHelper) asUint64(val any) uint64 {
	// Number conversion function for uint64, without error checking
	// (implements an implicit type upgrade).
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(v.Int()) //nolint:gosec
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return uint64(v.Float())
	default:
		// panic("Non numeric value in asUint64()")
		return 0
	}
}

// Same for unsigned floats.
func (h *valueHelper) asFloat64(val any) float64 {
	// Number conversion function for float64, without error checking
	// (implements an implicit type upgrade).
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	default:
		// panic("Non numeric value in asFloat64()")
		return 0
	}
}

type paramHelper struct {
	// A collection of unexported helpers for parameters resolution
}

func (h *paramHelper) safeExpandedParamsFor(path, method, operationID string, res *Result, s *SpecValidator) (params []spec.Parameter) {
	operation, ok := s.expandedAnalyzer().OperationFor(method, path)
	if ok {
		// expand parameters first if necessary
		resolvedParams := []spec.Parameter{}
		for _, ppr := range operation.Parameters {
			resolvedParam, red := h.resolveParam(path, method, operationID, &ppr, s) //#nosec
			res.Merge(red)
			if resolvedParam != nil {
				resolvedParams = append(resolvedParams, *resolvedParam)
			}
		}
		// remove params with invalid expansion from Slice
		operation.Parameters = resolvedParams

		// the analyzer keys parameters by name and location: walk those keys in
		// order, so that findings about an operation's parameters come out the
		// same way on every run
		safeParams := s.expandedAnalyzer().SafeParamsFor(method, path,
			func(_ spec.Parameter, err error) bool {
				// since params have already been expanded, there are few causes for error
				res.addErrorsAt(operationPath(path, method), someParametersBrokenMsg(path, method, operationID))
				// original error from analyzer
				res.addErrorsAt(operationPath(path, method), err)
				return true
			})
		for _, k := range sortedKeys(safeParams) {
			params = append(params, safeParams[k])
		}
	}
	return
}

func (h *paramHelper) resolveParam(path, method, operationID string, param *spec.Parameter, s *SpecValidator) (*spec.Parameter, *Result) {
	// Ensure parameter is expanded
	var err error
	res := new(Result)
	isRef := param.Ref.String() != ""
	if s.spec.SpecFilePath() == "" {
		err = spec.ExpandParameterWithOptions(param, s.spec.Spec(), nil, s.schemaOptions.expandOptions(""))
	} else {
		err = spec.ExpandParameterWithOptions(param, nil, nil, s.schemaOptions.expandOptions(s.spec.SpecFilePath()))
	}
	if err != nil { // Safeguard
		// NOTE: we may enter here when the whole parameter is an unresolved $ref
		refPath := strings.Join([]string{"\"" + path + "\"", method}, ".")
		errorHelp.addPointerErrorAt(res, s.parameterPath(path, method, param.In, param.Name), err, param.Ref.String(), refPath)
		return nil, res
	}
	res.Merge(h.checkExpandedParam(param, param.Name, param.In, operationID, s.parameterPath(path, method, param.In, param.Name), isRef))
	return param, res
}

func (h *paramHelper) checkExpandedParam(
	pr *spec.Parameter, path, in, operation string, at pathSegments, isRef bool,
) *Result {
	// Secure parameter structure after $ref resolution
	res := new(Result)
	simpleZero := spec.SimpleSchema{}
	// Try to explain why... best guess
	switch {
	case pr.In == swaggerBody && (pr.SimpleSchema != simpleZero && pr.Type != objectType):
		if isRef {
			// Most likely, a $ref with a sibling is an unwanted situation: in itself this is a warning...
			// but we detect it because of the following error:
			// schema took over Parameter for an unexplained reason
			res.addWarningsAt(at, refShouldNotHaveSiblingsMsg(path, operation))
		}
		res.addErrorsAt(at, invalidParameterDefinitionMsg(path, in, operation))
	case pr.In != swaggerBody && pr.Schema != nil:
		if isRef {
			res.addWarningsAt(at, refShouldNotHaveSiblingsMsg(path, operation))
		}
		res.addErrorsAt(at, invalidParameterDefinitionAsSchemaMsg(path, in, operation))
	case (pr.In == swaggerBody && pr.Schema == nil) || (pr.In != swaggerBody && pr.SimpleSchema == simpleZero):
		// Other unexpected mishaps
		res.addErrorsAt(at, invalidParameterDefinitionMsg(path, in, operation))
	}
	return res
}

type responseHelper struct {
	// A collection of unexported helpers for response resolution
}

func (r *responseHelper) expandResponseRef(
	response *spec.Response,
	path string, at pathSegments, s *SpecValidator,
) (*spec.Response, *Result) {
	// Ensure response is expanded
	var err error
	res := new(Result)
	if s.spec.SpecFilePath() == "" {
		// there is no physical document to resolve $ref in response
		err = spec.ExpandResponseWithOptions(response, s.spec.Spec(), nil, s.schemaOptions.expandOptions(""))
	} else {
		err = spec.ExpandResponseWithOptions(response, nil, nil, s.schemaOptions.expandOptions(s.spec.SpecFilePath()))
	}
	if err != nil { // Safeguard
		// NOTE: we may enter here when the whole response is an unresolved $ref.
		errorHelp.addPointerErrorAt(res, at, err, response.Ref.String(), path)
		return nil, res
	}

	return response, res
}

func (r *responseHelper) responseMsgVariants(
	responseType string,
	responseCode int,
) (responseName, responseCodeAsStr string) {
	// Path variants for messages
	if responseType == jsonDefault {
		responseCodeAsStr = jsonDefault
		responseName = "default response"
	} else {
		responseCodeAsStr = strconv.Itoa(responseCode)
		responseName = "response " + responseCodeAsStr
	}
	return
}
