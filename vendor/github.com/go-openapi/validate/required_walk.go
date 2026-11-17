// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/spec"
)

// schemaIdentity is how a message refers to the schema a required entry
// belongs to.
//
// A definition is named on its own, the way it always has been. A schema the
// definition holds is named by the way down to it, relative to the definition:
// "A.inner" rather than "A", which would send a reader to the wrong place.
type schemaIdentity struct {
	name   string
	nested bool
}

// identify names the schema a location leads to.
func identify(at pathSegments) schemaIdentity {
	const definitionDepth = 2 // "definitions", then the name of one

	return schemaIdentity{
		name:   strings.TrimPrefix(at.dotted(), swaggerDefinitions+"."),
		nested: len(at) > definitionDepth,
	}
}

func (i schemaIdentity) requiredButNotDefined(property string) errors.Error {
	if i.nested {
		return requiredButNotDefinedInSchemaMsg(property, i.name)
	}

	return requiredButNotDefinedMsg(property, i.name)
}

// maxCompositionHops bounds how far the search for a declared property follows
// allOf members and the local $ref they may be written as.
const maxCompositionHops = 20

// walkRequired checks the required entries of a schema, then those of every
// schema it holds inline.
//
// A definition is not the only place a document says an object must hold a
// property: so does the schema of a property, of an array item, of an
// additionalProperties. Each of those is a self-contained object definition,
// and a required entry naming something it never declares is the same slip
// wherever it sits.
//
// A schema written as a $ref is left alone: it is checked where it is defined,
// and following it here would report the same slip twice and, for a recursive
// definition, would not terminate.
//
// It reports whether the walk should carry on, which is how the caller stops on
// the first fault unless it was asked for everything.
func (s *SpecValidator) walkRequired(at pathSegments, v *spec.Schema, res *Result) bool {
	if v == nil || v.Ref.String() != "" {
		return true
	}

	for i, pn := range v.Required {
		// the offending entry of the required array, not the schema holding
		// it: that is what a reader has to go and amend
		red := s.validateRequiredProperties(pn, identify(at), at, at.child(jsonRequired).item(i), v)
		// NOTE: capture validity before merging: Merge may redeem `red` to the
		// pool (wantsRedeemOnMerge), after which reading it races with a
		// concurrent BorrowResult().cleared() in another goroutine.
		isValid := red.IsValid()
		res.Merge(red)
		if !isValid && !s.Options.ContinueOnErrors {
			return false
		}
	}

	return s.walkInlineSchemas(at, v, res)
}

// walkInlineSchemas descends into every schema a schema holds, without checking
// the required entries of a composition member.
//
// Inside allOf, anyOf, oneOf or not, a member is a fragment of a constraint
// rather than a complete definition: its required entries speak of the instance
// the whole composition describes, and are legitimately met by a sibling member
// or by no declaration at all. Those are honoured when data is validated, and
// saying anything about them here would be wrong. Their own members are still
// walked, because a property schema nested in one of them is a definition like
// any other.
func (s *SpecValidator) walkInlineSchemas(at pathSegments, v *spec.Schema, res *Result) bool {
	for _, name := range sortedKeys(v.Properties) {
		held := v.Properties[name]
		if !s.walkRequired(at.structuralChild(jsonProperties).child(name), &held, res) {
			return false
		}
	}

	for _, pattern := range sortedKeys(v.PatternProperties) {
		held := v.PatternProperties[pattern]
		if !s.walkRequired(at.structuralChild(jsonPatternProperties).child(pattern), &held, res) {
			return false
		}
	}

	if v.Items != nil {
		if v.Items.Schema != nil && !s.walkRequired(at.child(jsonItems), v.Items.Schema, res) {
			return false
		}
		for i := range v.Items.Schemas {
			if !s.walkRequired(at.child(jsonItems).item(i), &v.Items.Schemas[i], res) {
				return false
			}
		}
	}

	if v.AdditionalProperties != nil && v.AdditionalProperties.Schema != nil &&
		!s.walkRequired(at.child(jsonAdditionalProperties), v.AdditionalProperties.Schema, res) {
		return false
	}

	// a composition member is walked for the schemas it holds, never for its
	// own required entries
	for _, composition := range []struct {
		keyword string
		members []spec.Schema
	}{
		{jsonAllOf, v.AllOf},
		{jsonAnyOf, v.AnyOf},
		{jsonOneOf, v.OneOf},
	} {
		for i := range composition.members {
			if !s.walkInlineSchemas(at.child(composition.keyword).item(i), &composition.members[i], res) {
				return false
			}
		}
	}

	if v.Not != nil {
		return s.walkInlineSchemas(at.child(jsonNot), v.Not, res)
	}

	return true
}

// declaresProperty reports whether a schema, or any schema composed into it by
// allOf, declares the named property, and whether that declaration is readOnly.
//
// An allOf member may be written as a $ref, which is followed here: a property
// contributed by a base definition is declared just as plainly as one written
// in place.
func (s *SpecValidator) declaresProperty(v *spec.Schema, name string, hops int) (readOnly, declared bool) {
	if v == nil || hops <= 0 {
		return false, false
	}

	if held, ok := v.Properties[name]; ok {
		return held.ReadOnly, true
	}

	for i := range v.AllOf {
		member := &v.AllOf[i]
		if member.Ref.String() != "" {
			resolved, err := s.resolveRef(&member.Ref)
			if err != nil {
				continue
			}
			member = resolved
		}

		if readOnly, ok := s.declaresProperty(member, name, hops-1); ok {
			return readOnly, true
		}
	}

	return false, false
}
