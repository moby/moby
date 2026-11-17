// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"

	"github.com/go-openapi/spec"
)

// validateDiscriminators checks the discriminator of every definition, and of every schema a
// definition holds inline.
//
// A discriminator names the property that tells subtypes apart:
//
//	Pet:
//	  discriminator: petType
//	  required: [ petType ]
//	  properties:
//	    petType: { type: string }
//
// Swagger 2.0 asks two things of that property: the schema must define it, and must list it as
// required. Both matter for the same reason — an instance carries its subtype in that property,
// so a subtype cannot be resolved from an instance that has nowhere to put the value, or that is
// free to leave it out.
//
// The JSON meta-schema types discriminator as a plain string and never compares it against
// properties or required, so it can express neither check.
//
// A property contributed by an allOf member counts as defined, and one that member requires counts
// as required: [SpecValidator.declaresProperty] already reads a composed definition that way for
// the required rule, and a discriminator resolves against the instance the whole composition
// describes.
//
// The third clause of the rule — the value must name this schema or one that inherits it —
// constrains the data, not the document, so it belongs to schema validation rather than here.
func (s *SpecValidator) validateDiscriminators() *Result {
	res := validatorPools.results.Borrow()
	definitions := s.spec.Spec().Definitions

	for _, name := range sortedKeys(definitions) {
		schema := definitions[name]
		s.walkDiscriminators(newPathSegments(swaggerDefinitions, name), &schema, res)
	}

	return res
}

// walkDiscriminators checks the discriminator of a schema, then of every schema it holds inline.
//
// A schema written as a $ref is left alone: it is checked where it is defined, and following it
// here would report the same fault twice and, for a recursive definition, would not terminate.
// This mirrors [SpecValidator.walkRequired].
func (s *SpecValidator) walkDiscriminators(at pathSegments, v *spec.Schema, res *Result) {
	if v == nil || v.Ref.String() != "" {
		return
	}

	s.checkDiscriminator(at, v, res)

	for _, name := range sortedKeys(v.Properties) {
		held := v.Properties[name]
		s.walkDiscriminators(at.structuralChild(jsonProperties).child(name), &held, res)
	}

	for _, pattern := range sortedKeys(v.PatternProperties) {
		held := v.PatternProperties[pattern]
		s.walkDiscriminators(at.structuralChild(jsonPatternProperties).child(pattern), &held, res)
	}

	if v.Items != nil {
		if v.Items.Schema != nil {
			s.walkDiscriminators(at.child(jsonItems), v.Items.Schema, res)
		}
		for i := range v.Items.Schemas {
			s.walkDiscriminators(at.child(jsonItems).item(i), &v.Items.Schemas[i], res)
		}
	}

	if v.AdditionalProperties != nil && v.AdditionalProperties.Schema != nil {
		s.walkDiscriminators(at.child(jsonAdditionalProperties), v.AdditionalProperties.Schema, res)
	}

	for _, composition := range []struct {
		keyword string
		members []spec.Schema
	}{
		{jsonAllOf, v.AllOf},
		{jsonAnyOf, v.AnyOf},
		{jsonOneOf, v.OneOf},
	} {
		for i := range composition.members {
			s.walkDiscriminators(at.child(composition.keyword).item(i), &composition.members[i], res)
		}
	}

	if v.Not != nil {
		s.walkDiscriminators(at.child(jsonNot), v.Not, res)
	}
}

// checkDiscriminator checks the discriminator a single schema declares. A schema without one has
// nothing to answer for.
//
// Both findings are reported against the discriminator itself, which is the entry a reader has to
// go and amend, and both are reported when they apply: a discriminator naming a property that is
// neither defined nor required is two separate slips to fix.
func (s *SpecValidator) checkDiscriminator(at pathSegments, v *spec.Schema, res *Result) {
	if v.Discriminator == "" {
		return
	}

	of := identify(at).name
	discriminatorAt := at.child(jsonDiscriminator)

	if _, declared := s.declaresProperty(v, v.Discriminator, maxCompositionHops); !declared {
		res.addErrorsAt(discriminatorAt, discriminatorNotDefinedMsg(v.Discriminator, of))
	}

	if !s.requiresProperty(v, v.Discriminator, maxCompositionHops) {
		res.addErrorsAt(discriminatorAt, discriminatorNotRequiredMsg(v.Discriminator, of))
	}
}

// requiresProperty reports whether a schema, or any schema composed into it by allOf, lists the
// named property as required.
//
// It is the required-list counterpart of [SpecValidator.declaresProperty], and follows allOf the
// same way, including the local $ref an allOf member may be written as.
func (s *SpecValidator) requiresProperty(v *spec.Schema, name string, hops int) bool {
	if v == nil || hops <= 0 {
		return false
	}

	if slices.Contains(v.Required, name) {
		return true
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

		if s.requiresProperty(member, name, hops-1) {
			return true
		}
	}

	return false
}
