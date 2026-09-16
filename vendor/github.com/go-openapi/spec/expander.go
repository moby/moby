// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"encoding/json"
	"fmt"

	"github.com/go-openapi/swag/loading"
)

const smallPrealloc = 10

// DefaultMaxExpansionNodes is the default upper bound on the number of schema nodes
// expanded during a single ExpandSpec / ExpandSchema* call.
//
// It guards against maliciously crafted specifications whose $ref graph expands to an
// exponential number of nodes from a few kilobytes of input. For reference, expanding the
// full Kubernetes API specification (the largest real-world spec we test against) visits
// roughly 47,000 nodes, so this default leaves ample headroom for legitimate documents.
//
// See ExpandOptions.MaxExpansionNodes to tune or disable this budget.
const DefaultMaxExpansionNodes = 500_000

// ExpandOptions provides options for the spec expander.
//
// RelativeBase is the path to the root document. This can be a remote URL or a path to a local file.
//
// If left empty, the root document is assumed to be located in the current working directory:
// all relative $ref's will be resolved from there.
//
// PathLoader injects a document loading method. By default, this resolves to the function provided by the PathLoader package variable.
//
// PathLoaderWithOptions is an alternative document loader that accepts [loading.Option] values, matching the
// signature used by the go-openapi/swag/loading and go-openapi/loads loaders. When set, it takes precedence over
// PathLoader. This lets a caller inject an options-aware (e.g. path-confined) loader without an adapter closure.
//
// Security: the default loader is not sandboxed. When expanding an untrusted specification, inject a confined
// loader (for example one built with loading.WithRoot) — see the package "Security" section.
type ExpandOptions struct {
	RelativeBase        string                                // the path to the root document to expand. This is a file, not a directory
	SkipSchemas         bool                                  // do not expand schemas, just paths, parameters and responses
	ContinueOnError     bool                                  // continue expanding even after and error is found
	PathLoader          func(string) (json.RawMessage, error) `json:"-"` // the document loading method that takes a path as input and yields a json document
	AbsoluteCircularRef bool                                  // circular $ref remaining after expansion remain absolute URLs

	// PathLoaderWithOptions injects a document loading method that accepts loading options.
	//
	// It has the same role as PathLoader but matches the option-aware loader signature exposed by
	// github.com/go-openapi/swag/loading (and github.com/go-openapi/loads), so such a loader can be
	// injected directly, without wrapping it in an adapter closure.
	//
	// When set, PathLoaderWithOptions takes precedence over PathLoader. The provided loader is expected
	// to carry its own loading options (for example a path confinement built with loading.WithRoot);
	// the expander itself invokes it without adding options.
	PathLoaderWithOptions func(string, ...loading.Option) (json.RawMessage, error) `json:"-"`

	// MaxExpansionNodes caps the number of schema nodes expanded during a single expansion call,
	// as a safeguard against $ref amplification attacks (see ErrExpandTooManyNodes).
	//
	// The value is interpreted as follows:
	//
	//   0  (the zero value): use DefaultMaxExpansionNodes. Every caller is protected by default.
	//   <0: no limit (unbounded expansion). Use only with fully trusted specifications.
	//   >0: cap the expansion at this number of nodes.
	//
	// When the budget is exceeded, expansion stops and ErrExpandTooManyNodes is returned.
	// Because this is a resource-exhaustion safeguard, the error is always returned, even when
	// ContinueOnError is set.
	MaxExpansionNodes int
}

// maxExpansionNodes resolves the tri-state MaxExpansionNodes option into an effective budget.
//
// A returned value of 0 means "unbounded".
func (o *ExpandOptions) maxExpansionNodes() int {
	switch {
	case o.MaxExpansionNodes == 0:
		return DefaultMaxExpansionNodes
	case o.MaxExpansionNodes < 0:
		return 0 // unbounded
	default:
		return o.MaxExpansionNodes
	}
}

func optionsOrDefault(opts *ExpandOptions) *ExpandOptions {
	if opts != nil {
		clone := *opts // shallow clone to avoid internal changes to be propagated to the caller
		if clone.RelativeBase != "" {
			clone.RelativeBase = normalizeBase(clone.RelativeBase)
		}
		// if the relative base is empty, let the schema loader choose a pseudo root document
		return &clone
	}
	return &ExpandOptions{}
}

// ExpandSpec expands the references in a swagger spec.
//
// Security: with default options the document loader is not sandboxed, so a "$ref" in an
// untrusted spec can read local files or reach internal addresses. See the package "Security"
// section before expanding untrusted input.
func ExpandSpec(spec *Swagger, options *ExpandOptions) error {
	options = optionsOrDefault(options)
	resolver := defaultSchemaLoader(spec, options, nil, nil)

	specBasePath := options.RelativeBase

	if !options.SkipSchemas {
		for key, definition := range spec.Definitions {
			parentRefs := make([]string, 0, smallPrealloc)
			parentRefs = append(parentRefs, "#/definitions/"+key)

			def, err := expandSchema(definition, parentRefs, resolver, specBasePath)
			if resolver.shouldStopOnError(err) {
				return err
			}
			if def != nil {
				spec.Definitions[key] = *def
			}
		}
	}

	for key := range spec.Parameters {
		parameter := spec.Parameters[key]
		if err := expandParameterOrResponse(&parameter, resolver, specBasePath); resolver.shouldStopOnError(err) {
			return err
		}
		spec.Parameters[key] = parameter
	}

	for key := range spec.Responses {
		response := spec.Responses[key]
		if err := expandParameterOrResponse(&response, resolver, specBasePath); resolver.shouldStopOnError(err) {
			return err
		}
		spec.Responses[key] = response
	}

	if spec.Paths != nil {
		for key := range spec.Paths.Paths {
			pth := spec.Paths.Paths[key]
			if err := expandPathItem(&pth, resolver, specBasePath); resolver.shouldStopOnError(err) {
				return err
			}
			spec.Paths.Paths[key] = pth
		}
	}

	return nil
}

const rootBase = ".root"

// baseForRoot loads in the cache the root document and produces a fake ".root" base path entry
// for further $ref resolution.
func baseForRoot(root any, cache ResolutionCache) string {
	// cache the root document to resolve $ref's
	normalizedBase := normalizeBase(rootBase)

	if root == nil {
		// ensure that we never leave a nil root: always cache the root base pseudo-document
		cachedRoot, found := cache.Get(normalizedBase)
		if found && cachedRoot != nil {
			// the cache is already preloaded with a root
			return normalizedBase
		}

		root = map[string]any{}
	}

	cache.Set(normalizedBase, root)

	return normalizedBase
}

// ExpandSchema expands the refs in the schema object with reference to the root object.
//
// go-openapi/validate uses this function.
//
// Notice that it is impossible to reference a json schema in a different document other than root
// (use ExpandSchemaWithBasePath to resolve external references).
//
// Setting the cache is optional and this parameter may safely be left to nil.
//
// ExpandSchema uses the package default document loader, which is not sandboxed. To expand a
// schema whose $ref may derive from untrusted input, use [ExpandSchemaWithOptions] with a confined
// loader — see the package "Security" section.
func ExpandSchema(schema *Schema, root any, cache ResolutionCache) error {
	return ExpandSchemaWithOptions(schema, root, cache, nil)
}

// ExpandSchemaWithOptions expands the refs in the schema object with reference to the root object,
// honoring the provided expand options. It is the option-aware form of [ExpandSchema].
//
// In particular, set opts.PathLoaderWithOptions (or opts.PathLoader) to inject a confined document
// loader when expanding a schema whose $ref may derive from an untrusted source (see the package
// "Security" section). opts.ContinueOnError, opts.AbsoluteCircularRef and opts.MaxExpansionNodes
// are honored as well.
//
// The base path is always derived from root (as with [ExpandSchema]), so opts.RelativeBase and
// opts.SkipSchemas are ignored. Passing nil opts is equivalent to [ExpandSchema].
//
// Setting the cache is optional and this parameter may safely be left to nil.
func ExpandSchemaWithOptions(schema *Schema, root any, cache ResolutionCache, opts *ExpandOptions) error {
	cache = cacheOrDefault(cache)
	if root == nil {
		root = schema
	}

	effective := ExpandOptions{}
	if opts != nil {
		effective = *opts // preserve caller options (loader, ContinueOnError, budget, ...)
	}
	// when a root is specified, cache the root as an in-memory document for $ref retrieval
	effective.RelativeBase = baseForRoot(root, cache)
	effective.SkipSchemas = false

	return ExpandSchemaWithBasePath(schema, cache, &effective)
}

// ExpandSchemaWithBasePath expands the refs in the schema object, base path configured through expand options.
//
// Setting the cache is optional and this parameter may safely be left to nil.
//
// Security: with default options the document loader is not sandboxed, so a "$ref" in an
// untrusted schema can read local files or reach internal addresses. See the package "Security"
// section before expanding untrusted input.
func ExpandSchemaWithBasePath(schema *Schema, cache ResolutionCache, opts *ExpandOptions) error {
	if schema == nil {
		return nil
	}

	cache = cacheOrDefault(cache)

	opts = optionsOrDefault(opts)

	resolver := defaultSchemaLoader(nil, opts, cache, nil)

	parentRefs := make([]string, 0, smallPrealloc)
	s, err := expandSchema(*schema, parentRefs, resolver, opts.RelativeBase)
	if err != nil {
		return err
	}
	if s != nil {
		// guard for when continuing on error
		*schema = *s
	}

	return nil
}

func expandItems(target Schema, parentRefs []string, resolver *schemaLoader, basePath string) (*Schema, error) {
	if target.Items == nil {
		return &target, nil
	}

	// array
	if target.Items.Schema != nil {
		t, err := expandSchema(*target.Items.Schema, parentRefs, resolver, basePath)
		if err != nil {
			return nil, err
		}
		*target.Items.Schema = *t
	}

	// tuple
	for i := range target.Items.Schemas {
		t, err := expandSchema(target.Items.Schemas[i], parentRefs, resolver, basePath)
		if err != nil {
			return nil, err
		}
		target.Items.Schemas[i] = *t
	}

	return &target, nil
}

//nolint:gocognit,gocyclo,cyclop // complex but well-tested $ref expansion logic; refactoring deferred to dedicated PR
func expandSchema(target Schema, parentRefs []string, resolver *schemaLoader, basePath string) (*Schema, error) {
	if err := resolver.context.countNode(); err != nil {
		return &target, err
	}

	if target.Ref.String() == "" && target.Ref.IsRoot() {
		newRef := normalizeRef(&target.Ref, basePath)
		target.Ref = *newRef
		return &target, nil
	}

	// change the base path of resolution when an ID is encountered
	// otherwise the basePath should inherit the parent's
	if target.ID != "" {
		basePath, _ = resolver.setSchemaID(target, target.ID, basePath)
	}

	if target.Ref.String() != "" {
		if !resolver.options.SkipSchemas {
			return expandSchemaRef(target, parentRefs, resolver, basePath)
		}

		// when "expand" with SkipSchema, we just rebase the existing $ref without replacing
		// the full schema.
		rebasedRef, err := NewRef(normalizeURI(target.Ref.String(), basePath))
		if err != nil {
			return nil, err
		}
		target.Ref = denormalizeRef(&rebasedRef, resolver.context.basePath, resolver.context.rootID)

		return &target, nil
	}

	for k := range target.Definitions {
		tt, err := expandSchema(target.Definitions[k], parentRefs, resolver, basePath)
		if resolver.shouldStopOnError(err) {
			return &target, err
		}
		if tt != nil {
			target.Definitions[k] = *tt
		}
	}

	t, err := expandItems(target, parentRefs, resolver, basePath)
	if resolver.shouldStopOnError(err) {
		return &target, err
	}
	if t != nil {
		target = *t
	}

	for i := range target.AllOf {
		t, err := expandSchema(target.AllOf[i], parentRefs, resolver, basePath)
		if resolver.shouldStopOnError(err) {
			return &target, err
		}
		if t != nil {
			target.AllOf[i] = *t
		}
	}

	for i := range target.AnyOf {
		t, err := expandSchema(target.AnyOf[i], parentRefs, resolver, basePath)
		if resolver.shouldStopOnError(err) {
			return &target, err
		}
		if t != nil {
			target.AnyOf[i] = *t
		}
	}

	for i := range target.OneOf {
		t, err := expandSchema(target.OneOf[i], parentRefs, resolver, basePath)
		if resolver.shouldStopOnError(err) {
			return &target, err
		}
		if t != nil {
			target.OneOf[i] = *t
		}
	}

	if target.Not != nil {
		t, err := expandSchema(*target.Not, parentRefs, resolver, basePath)
		if resolver.shouldStopOnError(err) {
			return &target, err
		}
		if t != nil {
			*target.Not = *t
		}
	}

	for k := range target.Properties {
		t, err := expandSchema(target.Properties[k], parentRefs, resolver, basePath)
		if resolver.shouldStopOnError(err) {
			return &target, err
		}
		if t != nil {
			target.Properties[k] = *t
		}
	}

	if target.AdditionalProperties != nil && target.AdditionalProperties.Schema != nil {
		t, err := expandSchema(*target.AdditionalProperties.Schema, parentRefs, resolver, basePath)
		if resolver.shouldStopOnError(err) {
			return &target, err
		}
		if t != nil {
			*target.AdditionalProperties.Schema = *t
		}
	}

	for k := range target.PatternProperties {
		t, err := expandSchema(target.PatternProperties[k], parentRefs, resolver, basePath)
		if resolver.shouldStopOnError(err) {
			return &target, err
		}
		if t != nil {
			target.PatternProperties[k] = *t
		}
	}

	for k := range target.Dependencies {
		if target.Dependencies[k].Schema != nil {
			t, err := expandSchema(*target.Dependencies[k].Schema, parentRefs, resolver, basePath)
			if resolver.shouldStopOnError(err) {
				return &target, err
			}
			if t != nil {
				*target.Dependencies[k].Schema = *t
			}
		}
	}

	if target.AdditionalItems != nil && target.AdditionalItems.Schema != nil {
		t, err := expandSchema(*target.AdditionalItems.Schema, parentRefs, resolver, basePath)
		if resolver.shouldStopOnError(err) {
			return &target, err
		}
		if t != nil {
			*target.AdditionalItems.Schema = *t
		}
	}
	return &target, nil
}

func expandSchemaRef(target Schema, parentRefs []string, resolver *schemaLoader, basePath string) (*Schema, error) {
	// if a Ref is found, all sibling fields are skipped
	// Ref also changes the resolution scope of children expandSchema

	// here the resolution scope is changed because a $ref was encountered
	normalizedRef := normalizeRef(&target.Ref, basePath)
	normalizedBasePath := normalizedRef.RemoteURI()

	if resolver.isCircular(normalizedRef, basePath, parentRefs...) {
		// this means there is a cycle in the recursion tree: return the Ref
		// - circular refs cannot be expanded. We leave them as ref.
		// - denormalization means that a new local file ref is set relative to the original basePath
		debugLog("short circuit circular ref: basePath: %s, normalizedPath: %s, normalized ref: %s",
			basePath, normalizedBasePath, normalizedRef.String())
		if !resolver.options.AbsoluteCircularRef {
			target.Ref = denormalizeRef(normalizedRef, resolver.context.basePath, resolver.context.rootID)
		} else {
			target.Ref = *normalizedRef
		}
		return &target, nil
	}

	var t *Schema
	err := resolver.Resolve(&target.Ref, &t, basePath)
	if resolver.shouldStopOnError(err) {
		return nil, err
	}

	if t == nil {
		// guard for when continuing on error
		return &target, nil
	}

	parentRefs = append(parentRefs, normalizedRef.String())
	transitiveResolver := resolver.transitiveResolver(basePath, target.Ref)

	basePath = resolver.updateBasePath(transitiveResolver, normalizedBasePath)

	return expandSchema(*t, parentRefs, transitiveResolver, basePath)
}

func expandPathItem(pathItem *PathItem, resolver *schemaLoader, basePath string) error {
	if pathItem == nil {
		return nil
	}

	parentRefs := make([]string, 0, smallPrealloc)
	if err := resolver.deref(pathItem, parentRefs, basePath); resolver.shouldStopOnError(err) {
		return err
	}

	if pathItem.Ref.String() != "" {
		transitiveResolver := resolver.transitiveResolver(basePath, pathItem.Ref)
		basePath = transitiveResolver.updateBasePath(resolver, basePath)
		resolver = transitiveResolver
	}

	pathItem.Ref = Ref{}
	for i := range pathItem.Parameters {
		if err := expandParameterOrResponse(&(pathItem.Parameters[i]), resolver, basePath); resolver.shouldStopOnError(err) {
			return err
		}
	}

	ops := []*Operation{
		pathItem.Get,
		pathItem.Head,
		pathItem.Options,
		pathItem.Put,
		pathItem.Post,
		pathItem.Patch,
		pathItem.Delete,
	}
	for _, op := range ops {
		if err := expandOperation(op, resolver, basePath); resolver.shouldStopOnError(err) {
			return err
		}
	}

	return nil
}

func expandOperation(op *Operation, resolver *schemaLoader, basePath string) error {
	if op == nil {
		return nil
	}

	for i := range op.Parameters {
		param := op.Parameters[i]
		if err := expandParameterOrResponse(&param, resolver, basePath); resolver.shouldStopOnError(err) {
			return err
		}
		op.Parameters[i] = param
	}

	if op.Responses == nil {
		return nil
	}

	responses := op.Responses
	if err := expandParameterOrResponse(responses.Default, resolver, basePath); resolver.shouldStopOnError(err) {
		return err
	}

	for code := range responses.StatusCodeResponses {
		response := responses.StatusCodeResponses[code]
		if err := expandParameterOrResponse(&response, resolver, basePath); resolver.shouldStopOnError(err) {
			return err
		}
		responses.StatusCodeResponses[code] = response
	}

	return nil
}

// ExpandResponseWithRoot expands a response based on a root document, not a fetchable document
//
// Notice that it is impossible to reference a json schema in a different document other than root
// (use ExpandResponse to resolve external references).
//
// Setting the cache is optional and this parameter may safely be left to nil.
func ExpandResponseWithRoot(response *Response, root any, cache ResolutionCache) error {
	return ExpandResponseWithOptions(response, root, cache, nil)
}

// ExpandResponse expands a response based on a basepath
//
// All refs inside response will be resolved relative to basePath.
func ExpandResponse(response *Response, basePath string) error {
	return ExpandResponseWithOptions(response, nil, nil, &ExpandOptions{RelativeBase: basePath})
}

// ExpandResponseWithOptions expands a response, honoring the provided expand options.
//
// It is the option-aware form of [ExpandResponse] and [ExpandResponseWithRoot]. When root is
// non-nil, refs resolve against the in-memory root document; otherwise they resolve relative to
// opts.RelativeBase.
//
// Set opts.PathLoaderWithOptions (or opts.PathLoader) to inject a confined document loader when
// the response's $ref may derive from an untrusted source — see the package "Security" section.
//
// Setting the cache is optional and this parameter may safely be left to nil.
func ExpandResponseWithOptions(response *Response, root any, cache ResolutionCache, opts *ExpandOptions) error {
	return expandRefableWithOptions(response, root, cache, opts)
}

// ExpandParameterWithRoot expands a parameter based on a root document, not a fetchable document.
//
// Notice that it is impossible to reference a json schema in a different document other than root
// (use ExpandParameter to resolve external references).
func ExpandParameterWithRoot(parameter *Parameter, root any, cache ResolutionCache) error {
	return ExpandParameterWithOptions(parameter, root, cache, nil)
}

// ExpandParameter expands a parameter based on a basepath.
// This is the exported version of expandParameter
// all refs inside parameter will be resolved relative to basePath.
func ExpandParameter(parameter *Parameter, basePath string) error {
	return ExpandParameterWithOptions(parameter, nil, nil, &ExpandOptions{RelativeBase: basePath})
}

// ExpandParameterWithOptions expands a parameter, honoring the provided expand options.
//
// It is the option-aware form of [ExpandParameter] and [ExpandParameterWithRoot]. When root is
// non-nil, refs resolve against the in-memory root document; otherwise they resolve relative to
// opts.RelativeBase.
//
// Set opts.PathLoaderWithOptions (or opts.PathLoader) to inject a confined document loader when
// the parameter's $ref may derive from an untrusted source — see the package "Security" section.
//
// Setting the cache is optional and this parameter may safely be left to nil.
func ExpandParameterWithOptions(parameter *Parameter, root any, cache ResolutionCache, opts *ExpandOptions) error {
	return expandRefableWithOptions(parameter, root, cache, opts)
}

// expandRefableWithOptions is the shared implementation for the option-aware parameter/response
// expanders. When root is non-nil, refs resolve against the in-memory root (base derived from
// root); otherwise they resolve relative to opts.RelativeBase. opts carries the loader and other
// expand options.
func expandRefableWithOptions(input any, root any, cache ResolutionCache, opts *ExpandOptions) error {
	cache = cacheOrDefault(cache)
	effective := optionsOrDefault(opts) // clones and normalizes RelativeBase; preserves the loader
	if root != nil {
		effective.RelativeBase = baseForRoot(root, cache)
	}
	resolver := defaultSchemaLoader(root, effective, cache, nil)

	return expandParameterOrResponse(input, resolver, effective.RelativeBase)
}

func getRefAndSchema(input any) (*Ref, *Schema, error) {
	var (
		ref *Ref
		sch *Schema
	)

	switch refable := input.(type) {
	case *Parameter:
		if refable == nil {
			return nil, nil, nil
		}
		ref = &refable.Ref
		sch = refable.Schema
	case *Response:
		if refable == nil {
			return nil, nil, nil
		}
		ref = &refable.Ref
		sch = refable.Schema
	default:
		return nil, nil, fmt.Errorf("unsupported type: %T: %w", input, ErrExpandUnsupportedType)
	}

	return ref, sch, nil
}

func expandParameterOrResponse(input any, resolver *schemaLoader, basePath string) error {
	ref, sch, err := getRefAndSchema(input)
	if err != nil {
		return err
	}

	if ref == nil && sch == nil { // nothing to do
		return nil
	}

	parentRefs := make([]string, 0, smallPrealloc)
	if ref != nil {
		// dereference this $ref
		if err = resolver.deref(input, parentRefs, basePath); resolver.shouldStopOnError(err) {
			return err
		}

		ref, sch, _ = getRefAndSchema(input)
		if ref == nil {
			ref = &Ref{} // empty ref
		}
	}

	if ref.String() != "" {
		transitiveResolver := resolver.transitiveResolver(basePath, *ref)
		basePath = resolver.updateBasePath(transitiveResolver, basePath)
		resolver = transitiveResolver
	}

	if sch == nil {
		// nothing to be expanded
		if ref != nil {
			*ref = Ref{}
		}

		return nil
	}

	if sch.Ref.String() != "" { //nolint:nestif // intertwined ref rebasing and circularity check
		rebasedRef, ern := NewRef(normalizeURI(sch.Ref.String(), basePath))
		if ern != nil {
			return ern
		}

		if resolver.isCircular(&rebasedRef, basePath, parentRefs...) {
			// this is a circular $ref: stop expansion
			if !resolver.options.AbsoluteCircularRef {
				sch.Ref = denormalizeRef(&rebasedRef, resolver.context.basePath, resolver.context.rootID)
			} else {
				sch.Ref = rebasedRef
			}
		}
	}

	// $ref expansion or rebasing is performed by expandSchema below
	if ref != nil {
		*ref = Ref{}
	}

	// expand schema
	// yes, we do it even if options.SkipSchema is true: we have to go down that rabbit hole and rebase nested $ref)
	s, err := expandSchema(*sch, parentRefs, resolver, basePath)
	if resolver.shouldStopOnError(err) {
		return err
	}

	if s != nil { // guard for when continuing on error
		*sch = *s
	}

	return nil
}
