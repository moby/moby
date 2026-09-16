// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/go-openapi/spec"
)

// Mixin merges one or more Swagger 2.0 documents into a primary document.
//
// # Argument order and precedence
//
// The first argument is the primary spec, which Mixin modifies in place.
// Subsequent arguments are mixins, listed in decreasing order of priority.
// On any collision, the primary always wins; among mixins, the earliest one
// wins.
//
// Example: given a primary spec with host "a.example.com" and a mixin with
// host "b.example.com", the merged result keeps "a.example.com" (primary
// wins, the mixin value is dropped). Given a primary without a host and a
// mixin with host "b.example.com", the merged result uses "b.example.com"
// (the mixin fills in the empty field on the primary).
//
// # What gets merged
//
// Top-level scalar fields on the primary are filled from the first mixin
// that provides them, but only if the primary's value is the zero value:
//
//   - Info (including the nested Contact and License)
//   - BasePath
//   - Host
//   - ExternalDocs
//
// Map and slice fields are merged entry by entry. This covers:
//
//   - paths, definitions, parameters, responses
//   - securityDefinitions, security, tags
//   - top-level and Info extensions
//
// Duplicate keys (or equal security requirements, or equal tag names) are
// skipped with a warning; warnings are returned as a slice and intended to
// be inspected by the caller (e.g. compared to an expected collision count
// in build scripts).
//
// Schemes, consumes and produces are merged as the union of distinct
// values. Duplicates there are silently dropped, no warning is emitted.
//
// Operation id collisions are auto-resolved by appending "Mixin<N>" to the
// mixin operation id (N is the mixin index), so the merged spec keeps
// unique operation ids.
//
// # Notes and limitations
//
// Consider calling [FixEmptyResponseDescriptions] on the modified primary
// if you read responses from storage and they are valid to start with.
//
// No key normalization takes place. Ensure paths, type names, etc. are
// canonical if your downstream tools rely on normalized forms.
//
// YAML anchors (& / *) are resolved by the YAML parser before Mixin sees
// the document, so they are not preserved in the merged output, and they
// cannot be shared across input files. Use $ref for cross-file reuse. See
// https://goswagger.io/go-swagger/faq/faq_swagger/#does-swagger-mixin-preserve-yaml-anchors
//
// The order of paths and definitions in the merged output is alphabetical:
// the underlying spec model stores them as Go maps, which serialize with
// sorted keys. Source-file order is not preserved. See
// https://goswagger.io/go-swagger/faq/faq_swagger/#can-i-control-the-path-or-operation-order-in-swagger-mixin-output
func Mixin(primary *spec.Swagger, mixins ...*spec.Swagger) []string {
	skipped := make([]string, 0, len(mixins))
	opIDs := getOpIDs(primary)
	initPrimary(primary)

	for i, m := range mixins {
		skipped = append(skipped, mergeSwaggerProps(primary, m)...)

		skipped = append(skipped, mergeConsumes(primary, m)...)

		skipped = append(skipped, mergeProduces(primary, m)...)

		skipped = append(skipped, mergeTags(primary, m)...)

		skipped = append(skipped, mergeSchemes(primary, m)...)

		skipped = append(skipped, mergeSecurityDefinitions(primary, m)...)

		skipped = append(skipped, mergeSecurityRequirements(primary, m)...)

		skipped = append(skipped, mergeDefinitions(primary, m)...)

		// merging paths requires a map of operationIDs to work with
		skipped = append(skipped, mergePaths(primary, m, opIDs, i)...)

		skipped = append(skipped, mergeParameters(primary, m)...)

		skipped = append(skipped, mergeResponses(primary, m)...)
	}

	return skipped
}

// getOpIDs extracts all the paths.<path>.operationIds from the given
// spec and returns them as the keys in a map with 'true' values.
func getOpIDs(s *spec.Swagger) map[string]bool {
	rv := make(map[string]bool)
	if s.Paths == nil {
		return rv
	}

	for _, v := range s.Paths.Paths {
		piops := pathItemOps(v)

		for _, op := range piops {
			rv[op.ID] = true
		}
	}

	return rv
}

func pathItemOps(p spec.PathItem) []*spec.Operation {
	var rv []*spec.Operation
	rv = appendOp(rv, p.Get)
	rv = appendOp(rv, p.Put)
	rv = appendOp(rv, p.Post)
	rv = appendOp(rv, p.Delete)
	rv = appendOp(rv, p.Head)
	rv = appendOp(rv, p.Options)
	rv = appendOp(rv, p.Patch)

	return rv
}

func appendOp(ops []*spec.Operation, op *spec.Operation) []*spec.Operation {
	if op == nil {
		return ops
	}

	return append(ops, op)
}

func mergeSecurityDefinitions(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	for k, v := range m.SecurityDefinitions {
		if _, exists := primary.SecurityDefinitions[k]; exists {
			warn := fmt.Sprintf(
				"SecurityDefinitions entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
			skipped = append(skipped, warn)

			continue
		}

		primary.SecurityDefinitions[k] = v
	}

	return
}

func mergeSecurityRequirements(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	for _, v := range m.Security {
		found := false
		for _, vv := range primary.Security {
			if reflect.DeepEqual(v, vv) {
				found = true

				break
			}
		}

		if found {
			warn := fmt.Sprintf(
				"Security requirement: '%v' already exists in primary or higher priority mixin, skipping\n", v)
			skipped = append(skipped, warn)

			continue
		}
		primary.Security = append(primary.Security, v)
	}

	return
}

func mergeDefinitions(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	for k, v := range m.Definitions {
		// assume name collisions represent IDENTICAL type. careful.
		if _, exists := primary.Definitions[k]; exists {
			warn := fmt.Sprintf(
				"definitions entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
			skipped = append(skipped, warn)

			continue
		}
		primary.Definitions[k] = v
	}

	return
}

func mergePaths(primary *spec.Swagger, m *spec.Swagger, opIDs map[string]bool, mixIndex int) (skipped []string) {
	if m.Paths != nil {
		for k, v := range m.Paths.Paths {
			if _, exists := primary.Paths.Paths[k]; exists {
				warn := fmt.Sprintf(
					"paths entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
				skipped = append(skipped, warn)

				continue
			}

			// Swagger requires that operationIds be
			// unique within a spec. If we find a
			// collision we append "Mixin0" to the
			// operationId we are adding, where 0 is mixin
			// index.  We assume that operationIds with
			// all the provided specs are already unique.
			piops := pathItemOps(v)
			for _, piop := range piops {
				if opIDs[piop.ID] {
					piop.ID = fmt.Sprintf("%v%v%v", piop.ID, "Mixin", mixIndex)
				}
				opIDs[piop.ID] = true
			}
			primary.Paths.Paths[k] = v
		}
	}

	return
}

func mergeParameters(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	for k, v := range m.Parameters {
		// could try to rename on conflict but would
		// have to fix $refs in the mixin. Complain
		// for now
		if _, exists := primary.Parameters[k]; exists {
			warn := fmt.Sprintf(
				"top level parameters entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
			skipped = append(skipped, warn)

			continue
		}
		primary.Parameters[k] = v
	}

	return
}

func mergeResponses(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	for k, v := range m.Responses {
		// could try to rename on conflict but would
		// have to fix $refs in the mixin. Complain
		// for now
		if _, exists := primary.Responses[k]; exists {
			warn := fmt.Sprintf(
				"top level responses entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
			skipped = append(skipped, warn)

			continue
		}
		primary.Responses[k] = v
	}

	return skipped
}

func mergeConsumes(primary *spec.Swagger, m *spec.Swagger) []string {
	for _, v := range m.Consumes {
		found := slices.Contains(primary.Consumes, v)

		if found {
			// no warning here: we just skip it
			continue
		}
		primary.Consumes = append(primary.Consumes, v)
	}

	return []string{}
}

func mergeProduces(primary *spec.Swagger, m *spec.Swagger) []string {
	for _, v := range m.Produces {
		found := slices.Contains(primary.Produces, v)

		if found {
			// no warning here: we just skip it
			continue
		}
		primary.Produces = append(primary.Produces, v)
	}

	return []string{}
}

func mergeTags(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	for _, v := range m.Tags {
		found := false
		for _, vv := range primary.Tags {
			if v.Name == vv.Name {
				found = true

				break
			}
		}

		if found {
			warn := fmt.Sprintf(
				"top level tags entry with name '%v' already exists in primary or higher priority mixin, skipping\n",
				v.Name,
			)
			skipped = append(skipped, warn)

			continue
		}

		primary.Tags = append(primary.Tags, v)
	}

	return
}

func mergeSchemes(primary *spec.Swagger, m *spec.Swagger) []string {
	for _, v := range m.Schemes {
		found := slices.Contains(primary.Schemes, v)

		if found {
			// no warning here: we just skip it
			continue
		}
		primary.Schemes = append(primary.Schemes, v)
	}

	return []string{}
}

func mergeSwaggerProps(primary *spec.Swagger, m *spec.Swagger) []string {
	var skipped, skippedInfo, skippedDocs []string

	primary.Extensions, skipped = mergeExtensions(primary.Extensions, m.Extensions)

	// merging details in swagger top properties
	if primary.Host == "" {
		primary.Host = m.Host
	}

	if primary.BasePath == "" {
		primary.BasePath = m.BasePath
	}

	if primary.Info == nil {
		primary.Info = m.Info
	} else if m.Info != nil {
		skippedInfo = mergeInfo(primary.Info, m.Info)
		skipped = append(skipped, skippedInfo...)
	}

	if primary.ExternalDocs == nil {
		primary.ExternalDocs = m.ExternalDocs
	} else if m.ExternalDocs != nil {
		skippedDocs = mergeExternalDocs(primary.ExternalDocs, m.ExternalDocs)
		skipped = append(skipped, skippedDocs...)
	}

	return skipped
}

//nolint:unparam
func mergeExternalDocs(primary *spec.ExternalDocumentation, m *spec.ExternalDocumentation) []string {
	if primary.Description == "" {
		primary.Description = m.Description
	}

	if primary.URL == "" {
		primary.URL = m.URL
	}

	return nil
}

func mergeInfo(primary *spec.Info, m *spec.Info) []string {
	var sk, skipped []string

	primary.Extensions, sk = mergeExtensions(primary.Extensions, m.Extensions)
	skipped = append(skipped, sk...)

	if primary.Description == "" {
		primary.Description = m.Description
	}

	if primary.Title == "" {
		primary.Title = m.Title
	}

	if primary.TermsOfService == "" {
		primary.TermsOfService = m.TermsOfService
	}

	if primary.Version == "" {
		primary.Version = m.Version
	}

	if primary.Contact == nil {
		primary.Contact = m.Contact
	} else if m.Contact != nil {
		var csk []string
		primary.Contact.Extensions, csk = mergeExtensions(primary.Contact.Extensions, m.Contact.Extensions)
		skipped = append(skipped, csk...)

		if primary.Contact.Name == "" {
			primary.Contact.Name = m.Contact.Name
		}

		if primary.Contact.URL == "" {
			primary.Contact.URL = m.Contact.URL
		}

		if primary.Contact.Email == "" {
			primary.Contact.Email = m.Contact.Email
		}
	}

	if primary.License == nil {
		primary.License = m.License
	} else if m.License != nil {
		var lsk []string
		primary.License.Extensions, lsk = mergeExtensions(primary.License.Extensions, m.License.Extensions)
		skipped = append(skipped, lsk...)

		if primary.License.Name == "" {
			primary.License.Name = m.License.Name
		}

		if primary.License.URL == "" {
			primary.License.URL = m.License.URL
		}
	}

	return skipped
}

func mergeExtensions(primary spec.Extensions, m spec.Extensions) (result spec.Extensions, skipped []string) {
	if primary == nil {
		result = m

		return
	}

	if m == nil {
		result = primary

		return
	}

	result = primary
	for k, v := range m {
		if _, found := primary[k]; found {
			skipped = append(skipped, k)

			continue
		}

		primary[k] = v
	}

	return
}

func initPrimary(primary *spec.Swagger) {
	if primary.SecurityDefinitions == nil {
		primary.SecurityDefinitions = make(map[string]*spec.SecurityScheme)
	}

	if primary.Security == nil {
		primary.Security = make([]map[string][]string, 0, allocSmallMap)
	}

	if primary.Produces == nil {
		primary.Produces = make([]string, 0, allocSmallMap)
	}

	if primary.Consumes == nil {
		primary.Consumes = make([]string, 0, allocSmallMap)
	}

	if primary.Tags == nil {
		primary.Tags = make([]spec.Tag, 0, allocSmallMap)
	}

	if primary.Schemes == nil {
		primary.Schemes = make([]string, 0, allocSmallMap)
	}

	if primary.Paths == nil {
		primary.Paths = &spec.Paths{Paths: make(map[string]spec.PathItem)}
	}

	if primary.Paths.Paths == nil {
		primary.Paths.Paths = make(map[string]spec.PathItem)
	}

	if primary.Definitions == nil {
		primary.Definitions = make(spec.Definitions)
	}

	if primary.Parameters == nil {
		primary.Parameters = make(map[string]spec.Parameter)
	}

	if primary.Responses == nil {
		primary.Responses = make(map[string]spec.Response)
	}
}
