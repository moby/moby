// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.18
// +build go1.18

package typeparams

import (
	"go/ast"
	"go/types"
)

// IndexListExpr is an alias for ast.IndexListExpr.
type IndexListExpr = ast.IndexListExpr

// ForTypeSpec returns n.TypeParams.
func ForTypeSpec(n *ast.TypeSpec) *ast.FieldList {
	if n == nil {
		return nil
	}
	return n.TypeParams
}

// ForFuncType returns n.TypeParams.
func ForFuncType(n *ast.FuncType) *ast.FieldList {
	if n == nil {
		return nil
	}
	return n.TypeParams
}

// TypeParam is an alias for types.TypeParam
type TypeParam = types.TypeParam

// TypeParamList is an alias for types.TypeParamList
type TypeParamList = types.TypeParamList

// TypeList is an alias for types.TypeList
type TypeList = types.TypeList

// NewTypeParam calls types.NewTypeParam.
func NewTypeParam(name *types.TypeName, constraint types.Type) *TypeParam {
	return types.NewTypeParam(name, constraint)
}

// SetTypeParamConstraint calls tparam.SetConstraint(constraint).
func SetTypeParamConstraint(tparam *TypeParam, constraint types.Type) {
	tparam.SetConstraint(constraint)
}

// NewSignatureType calls types.NewSignatureType.
func NewSignatureType(recv *types.Var, recvTypeParams, typeParams []*TypeParam, params, results *types.Tuple, variadic bool) *types.Signature {
	return types.NewSignatureType(recv, recvTypeParams, typeParams, params, results, variadic)
}

// ForSignature returns sig.TypeParams()
func ForSignature(sig *types.Signature) *TypeParamList {
	return sig.TypeParams()
}

// RecvTypeParams returns sig.RecvTypeParams().
func RecvTypeParams(sig *types.Signature) *TypeParamList {
	return sig.RecvTypeParams()
}

// IsComparable calls iface.IsComparable().
func IsComparable(iface *types.Interface) bool {
	return iface.IsComparable()
}

// IsMethodSet calls iface.IsMethodSet().
func IsMethodSet(iface *types.Interface) bool {
	return iface.IsMethodSet()
}

// IsImplicit calls iface.IsImplicit().
func IsImplicit(iface *types.Interface) bool {
	return iface.IsImplicit()
}

// MarkImplicit calls iface.MarkImplicit().
func MarkImplicit(iface *types.Interface) {
	iface.MarkImplicit()
}

// ForNamed extracts the (possibly empty) type parameter object list from
// named.
func ForNamed(named *types.Named) *TypeParamList {
	return named.TypeParams()
}

// SetForNamed sets the type params tparams on n. Each tparam must be of
// dynamic type *types.TypeParam.
func SetForNamed(n *types.Named, tparams []*TypeParam) {
	n.SetTypeParams(tparams)
}

// NamedTypeArgs returns named.TypeArgs().
func NamedTypeArgs(named *types.Named) *TypeList {
	return named.TypeArgs()
}

// NamedTypeOrigin returns named.Orig().
func NamedTypeOrigin(named *types.Named) types.Type {
	return named.Origin()
}

// Term is an alias for types.Term.
type Term = types.Term

// NewTerm calls types.NewTerm.
func NewTerm(tilde bool, typ types.Type) *Term {
	return types.NewTerm(tilde, typ)
}

// Union is an alias for types.Union
type Union = types.Union

// NewUnion calls types.NewUnion.
func NewUnion(terms []*Term) *Union {
	return types.NewUnion(terms)
}

// InitInstanceInfo initializes info to record information about type and
// function instances.
func InitInstanceInfo(info *types.Info) {
	info.Instances = make(map[*ast.Ident]types.Instance)
}

// Instance is an alias for types.Instance.
type Instance = types.Instance

// GetInstances returns info.Instances.
func GetInstances(info *types.Info) map[*ast.Ident]Instance {
	return info.Instances
}

// Context is an alias for types.Context.
type Context = types.Context

// NewContext calls types.NewContext.
func NewContext() *Context {
	return types.NewContext()
}

// Instantiate calls types.Instantiate.
func Instantiate(ctxt *Context, typ types.Type, targs []types.Type, validate bool) (types.Type, error) {
	return types.Instantiate(ctxt, typ, targs, validate)
}
