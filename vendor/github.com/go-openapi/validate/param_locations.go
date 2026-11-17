// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strconv"

	"github.com/go-openapi/spec"
)

// paramLocations tells where an operation declares a parameter.
//
// Parameters are held in an array, so a name is not how a document addresses
// one: only its index is. The index is not something a validator can work out
// from an expanded parameter either, because expansion merges the parameters
// an operation declares with those its path item declares, and resolves the
// ones written as a $ref. So the unexpanded document is indexed once, and
// looked up by what a validator does know: the operation, and the name and
// location of the parameter it is reporting on.
type paramLocations map[paramKey]pathSegments

// paramKey identifies a parameter the way the swagger specification does:
// a name is unique only within an "in".
type paramKey struct {
	path   string
	method string
	in     string
	name   string
}

// newParamLocations indexes the parameters declared by an unexpanded document.
func newParamLocations(sp *spec.Swagger) paramLocations {
	locations := make(paramLocations)
	if sp == nil || sp.Paths == nil {
		return locations
	}

	for path, pathItem := range sp.Paths.Paths {
		at := newPathSegments(swaggerPaths, path)

		// parameters declared by the path item are shared by all its
		// operations: recorded once, without a method
		locations.collect(sp, paramKey{path: path}, at, pathItem.Parameters)

		for method, op := range operationsOf(&pathItem) { //#nosec
			if op == nil {
				continue
			}

			locations.collect(sp, paramKey{path: path, method: method}, at.child(method), op.Parameters)
		}
	}

	return locations
}

// at returns where an operation declares a parameter.
//
// A parameter the operation does not declare itself may come from its path
// item. When neither knows it, which happens for a parameter too broken to be
// identified, the pointer stops on the array holding it and the name is kept
// for the message alone: an index no one could work out would be a guess.
func (l paramLocations) at(path, method, in, name string) pathSegments {
	if found, isDeclared := l[paramKey{path: path, method: methodToken(method), in: in, name: name}]; isDeclared {
		return found
	}

	if found, isDeclared := l[paramKey{path: path, in: in, name: name}]; isDeclared {
		return found
	}

	return operationPath(path, method).child(swaggerParameters).cosmeticChild(name)
}

func (l paramLocations) collect(sp *spec.Swagger, key paramKey, at pathSegments, params []spec.Parameter) {
	for i := range params {
		name, in, ok := parameterIdentity(sp, &params[i])
		if !ok {
			continue
		}

		key.name = name
		key.in = in
		l[key] = at.child(swaggerParameters).childAs(strconv.Itoa(i), name)
	}
}

// parameterIdentity names a parameter as declared, resolving the one indirection
// a document may put in the way: an entry written as a local $ref.
func parameterIdentity(sp *spec.Swagger, param *spec.Parameter) (name, in string, ok bool) {
	if param.Ref.String() == "" {
		return param.Name, param.In, param.Name != ""
	}

	shared := localRefPath(param.Ref.String())
	const sharedParameterDepth = 2
	if len(shared) != sharedParameterDepth || shared.beforeLast() != swaggerParameters {
		return "", "", false
	}

	declared, isDeclared := sp.Parameters[shared.last()]
	if !isDeclared {
		return "", "", false
	}

	return declared.Name, declared.In, declared.Name != ""
}

// operationsOf yields the operations of a path item, keyed as the document
// spells them.
func operationsOf(pathItem *spec.PathItem) map[string]*spec.Operation {
	return map[string]*spec.Operation{
		"get":     pathItem.Get,
		"put":     pathItem.Put,
		"post":    pathItem.Post,
		"delete":  pathItem.Delete,
		"options": pathItem.Options,
		"head":    pathItem.Head,
		"patch":   pathItem.Patch,
	}
}
