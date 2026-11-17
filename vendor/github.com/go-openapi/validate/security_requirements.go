// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"

	"github.com/go-openapi/spec"
)

// validateSecurityRequirements checks the security requirements declared by the document and by
// each of its operations against the security definitions.
//
// A requirement names a security scheme and lists the scopes an operation needs from it:
//
//	security:
//	  - petstore_auth: [ "write:pets" ]
//	  - api_key: []
//
// Three rules apply:
//
//   - the name must be declared in securityDefinitions (error)
//   - only an oauth2 requirement carries scopes; every other scheme type must list none (error)
//   - an oauth2 requirement should only name scopes its scheme declares (warning)
//
// The JSON meta-schema types a requirement as an object of string arrays and never reads
// securityDefinitions, so it can express none of the three.
//
// An empty requirement object ({}) names no scheme and passes. So does an empty security array,
// which an operation uses to drop the requirements the document sets for every operation.
func (s *SpecValidator) validateSecurityRequirements() *Result {
	res := validatorPools.results.Borrow()
	definitions := s.spec.Spec().SecurityDefinitions

	res.Merge(checkSecurityRequirements(
		newPathSegments(swaggerSecurity),
		s.spec.Spec().Security,
		definitions,
	))

	operations := s.expandedAnalyzer().Operations()
	for _, method := range sortedKeys(operations) {
		byPath := operations[method]
		for _, path := range sortedKeys(byPath) {
			op := byPath[path]
			if op == nil {
				continue
			}

			res.Merge(checkSecurityRequirements(
				operationPath(path, method).child(swaggerSecurity),
				op.Security,
				definitions,
			))
		}
	}

	return res
}

// checkSecurityRequirements checks the list of security requirements held at the given location.
//
// Requirements are checked in the order the document lists them, and the schemes one requirement
// names in sorted order: a requirement is a map, so the document's own order is lost (see
// [sortedKeys]).
func checkSecurityRequirements(at pathSegments, requirements []map[string][]string, definitions spec.SecurityDefinitions) *Result {
	res := validatorPools.results.Borrow()

	for i, requirement := range requirements {
		for _, name := range sortedKeys(requirement) {
			scopes := requirement[name]
			schemeAt := at.item(i).child(name)

			scheme, isDeclared := definitions[name]
			if !isDeclared || scheme == nil {
				res.addErrorsAt(schemeAt, securitySchemeNotDeclaredMsg(name))

				continue
			}

			if scheme.Type != securitySchemeOAuth2 {
				if len(scopes) > 0 {
					res.addErrorsAt(schemeAt, securityScopesNotEmptyMsg(name, strings.Join(scopes, ", "), scheme.Type))
				}

				continue
			}

			for _, scope := range scopes {
				if _, isKnown := scheme.Scopes[scope]; !isKnown {
					res.addWarningsAt(schemeAt, securityScopeNotDeclaredMsg(name, scope))
				}
			}
		}
	}

	return res
}
