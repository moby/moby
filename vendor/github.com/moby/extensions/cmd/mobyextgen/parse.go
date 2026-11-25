// SPDX-FileCopyrightText: Copyright The Moby Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

// parsing

const streamImportPath = "github.com/moby/extensions/stream"

func parsePoint(dir string) (point, error) {
	return parseContract(dir, "")
}

func parseService(dir, identity string) (point, error) {
	return parseContract(dir, identity)
}

func parseContract(dir, serviceIdentity string) (point, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return point{}, err
	}

	var files []*ast.File
	foundPackage := false
	for name, pkg := range pkgs {
		if strings.HasSuffix(name, "_test") {
			continue
		}
		foundPackage = true
		for _, f := range pkg.Files {
			files = append(files, f)
		}
	}
	if !foundPackage {
		return point{}, fmt.Errorf("no package found in %s", dir)
	}

	var pt point

	if serviceIdentity != "" {
		pkg, service, ok := splitServiceIdentity(serviceIdentity)
		if !ok {
			return point{}, fmt.Errorf("service name %q must be fully qualified", serviceIdentity)
		}
		if _, _, _, pointErr := findDefinePoint(files); pointErr == nil {
			return point{}, errors.New("the -service option cannot be used with an ordinary Point contract")
		}
		pt.iface, pt.id, pt.service = service, pkg, service
	} else {
		iface, id, single, err := findDefinePoint(files)
		if err != nil {
			return point{}, err
		}
		pt.iface, pt.id, pt.isPoint, pt.isSingle = iface, id, true, single
		pt.service = iface
	}

	msgNames := messageNames(files)
	ifaceType := findInterface(files, pt.iface)
	if ifaceType == nil {
		return point{}, fmt.Errorf("interface %q not found", pt.iface)
	}
	pt.doc = interfaceComment(files, pt.iface)
	// Inherit the contract's copyright and license rather than imposing the
	// generator's notices on output produced for other projects.
	for _, f := range files {
		if ifaceType.Pos() < f.Pos() || ifaceType.End() > f.End() {
			continue
		}
		for _, group := range f.Comments {
			if group.Pos() > f.Package {
				break
			}
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "// SPDX-FileCopyrightText:") ||
					strings.HasPrefix(comment.Text, "// SPDX-License-Identifier:") {
					pt.licenseHeader += comment.Text + "\n"
				}
			}
		}
	}
	if pt.licenseHeader != "" {
		pt.licenseHeader += "\n"
	}
	pt.methods, err = parseMethods(files, ifaceType)
	if err != nil {
		return point{}, err
	}
	// Methods may reference empty messages that have no pb-tagged fields.
	for _, m := range pt.methods {
		msgNames[m.request] = true
		if !m.bareError {
			msgNames[m.response] = true
		}
	}
	pt.messages, err = parseMessages(files, msgNames)
	if err != nil {
		return point{}, err
	}
	return pt, nil
}

// findDefinePoint returns the provider interface, point id, and cardinality
// declared by DefinePoint or DefineSinglePoint.
func findDefinePoint(files []*ast.File) (iface, id string, single bool, err error) {
	for _, f := range files {
		for _, decl := range f.Decls {
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				idx, ok := call.Fun.(*ast.IndexExpr)
				if !ok {
					return true
				}
				sel, ok := idx.X.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "DefinePoint" && sel.Sel.Name != "DefineSinglePoint") {
					return true
				}
				single = sel.Sel.Name == "DefineSinglePoint"
				if t, ok := idx.Index.(*ast.Ident); ok {
					iface = t.Name
				}
				if len(call.Args) == 1 {
					if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if v, e := strconv.Unquote(lit.Value); e == nil {
							id = v
						}
					}
				}
				return false
			})
		}
	}
	if iface == "" || id == "" {
		return "", "", false, errors.New("no extensions.DefinePoint[T](\"id\") call found")
	}
	return iface, id, single, nil
}

func splitServiceIdentity(identity string) (pkg, service string, ok bool) {
	dot := strings.LastIndexByte(identity, '.')
	if dot <= 0 || dot == len(identity)-1 {
		return "", "", false
	}
	return identity[:dot], identity[dot+1:], true
}

func findInterface(files []*ast.File, name string) *ast.InterfaceType {
	for _, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				if ts.Name.Name != name {
					continue
				}
				if it, ok := ts.Type.(*ast.InterfaceType); ok {
					return it
				}
			}
		}
	}
	return nil
}

// messageNames returns struct types containing pb-tagged fields.
func messageNames(files []*ast.File) map[string]bool {
	names := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if structHasPBTag(st) {
					names[ts.Name.Name] = true
				}
			}
		}
	}
	return names
}

func structHasPBTag(st *ast.StructType) bool {
	for _, f := range st.Fields.List {
		// Keep malformed tags classified as message fields so parseMessage reports
		// the tag instead of silently ignoring the struct.
		reservation := len(f.Names) == 1 && f.Names[0].Name == "_"
		if _, present, _ := pbNumber(f, reservation); present {
			return true
		}
	}
	return false
}

func interfaceComment(files []*ast.File, name string) string {
	for _, file := range files {
		for _, decl := range file.Decls {
			group, ok := decl.(*ast.GenDecl)
			if !ok || group.Tok != token.TYPE {
				continue
			}
			for _, spec := range group.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				if typeSpec.Name.Name != name {
					continue
				}
				doc := typeSpec.Doc
				// Like go/doc, a group comment documents only a single-type group.
				if doc == nil && len(group.Specs) == 1 {
					doc = group.Doc
				}
				if doc != nil {
					return strings.TrimSpace(doc.Text())
				}
				return ""
			}
		}
	}
	return ""
}

func parseMethods(files []*ast.File, iface *ast.InterfaceType) ([]method, error) {
	var methods []method
	for _, m := range iface.Methods.List {
		ft, ok := m.Type.(*ast.FuncType)
		if !ok || len(m.Names) == 0 {
			continue
		}
		name := m.Names[0].Name
		paramFields := fieldsPerName(ft.Params)
		params := make([]methodParameter, len(paramFields))
		for i, param := range paramFields {
			var err error
			params[i], err = classifyMethodParameter(files, param.Type, i)
			if err != nil {
				return nil, fmt.Errorf("method %q parameter %d: %w", name, i+1, err)
			}
		}
		res := results(ft)
		for _, result := range res {
			if _, isStream, _ := parseStreamParameter(files, result); isStream {
				return nil, methodShapeError(name)
			}
		}

		doc := fieldComment(m)
		switch {
		case len(params) == 2 && params[0].kind == methodContextParameter && params[1].kind == methodMessageParameter:
			var (
				response  string
				bareError bool
			)
			switch {
			case len(res) == 1 && isIdent(res[0], "error"):
				response, bareError = name+"Response", true
			case len(res) == 2 && isIdent(res[1], "error"):
				var err error
				response, err = pointerIdent(res[0])
				if err != nil {
					return nil, methodShapeError(name)
				}
			default:
				return nil, methodShapeError(name)
			}
			methods = append(methods, method{
				name: name, request: params[1].message, response: response,
				bareError: bareError, doc: doc,
			})
		case len(params) == 3 && params[0].kind == methodContextParameter &&
			params[1].kind == methodMessageParameter && params[2].kind == methodSenderParameter:
			if len(res) != 1 || !isIdent(res[0], "error") {
				return nil, methodShapeError(name)
			}
			methods = append(methods, method{
				kind: methodServerStreaming, name: name,
				request: params[1].message, response: params[2].message, doc: doc,
			})
		case len(params) == 2 && params[0].kind == methodContextParameter && params[1].kind == methodReceiverParameter:
			if len(res) != 2 || !isIdent(res[1], "error") {
				return nil, methodShapeError(name)
			}
			response, err := pointerIdent(res[0])
			if err != nil {
				return nil, methodShapeError(name)
			}
			methods = append(methods, method{
				kind: methodClientStreaming, name: name,
				request: params[1].message, response: response, doc: doc,
			})
		default:
			return nil, methodShapeError(name)
		}
	}
	return methods, nil
}

type methodParameterKind int

const (
	methodContextParameter methodParameterKind = iota + 1
	methodMessageParameter
	methodSenderParameter
	methodReceiverParameter
)

type methodParameter struct {
	kind    methodParameterKind
	message string
}

const acceptedMethodShapes = "accepted shapes are M(context.Context, *Request) (*Reply, error), " +
	"M(context.Context, *Request) error, " +
	"M(context.Context, *Request, stream.Sender[Chunk]) error, and " +
	"M(context.Context, stream.Receiver[Chunk]) (*Reply, error)"

func methodShapeError(name string) error {
	return fmt.Errorf("method %q: unsupported signature; %s", name, acceptedMethodShapes)
}

func classifyMethodParameter(files []*ast.File, expr ast.Expr, index int) (methodParameter, error) {
	param, isStream, err := parseStreamParameter(files, expr)
	switch {
	case err != nil:
		return methodParameter{}, err
	case isStream && index == 0:
		return methodParameter{}, errors.New("stream parameters cannot replace context.Context")
	case isStream:
		return param, nil
	case index == 0 && !isContextType(files, expr):
		return methodParameter{}, errors.New("must be context.Context")
	case index == 0:
		return methodParameter{kind: methodContextParameter}, nil
	}
	message, err := pointerIdent(expr)
	if err != nil {
		return methodParameter{}, err
	}
	return methodParameter{kind: methodMessageParameter, message: message}, nil
}

// parseStreamParameter reports whether expr names stream.Sender or
// stream.Receiver and, if so, validates its import and message argument.
func parseStreamParameter(files []*ast.File, expr ast.Expr) (methodParameter, bool, error) {
	var (
		selector *ast.SelectorExpr
		args     []ast.Expr
	)
	switch expr := expr.(type) {
	case *ast.SelectorExpr:
		selector = expr
	case *ast.IndexExpr:
		selector, _ = expr.X.(*ast.SelectorExpr)
		args = []ast.Expr{expr.Index}
	case *ast.IndexListExpr:
		selector, _ = expr.X.(*ast.SelectorExpr)
		args = expr.Indices
	default:
		return methodParameter{}, false, nil
	}
	if selector == nil || (selector.Sel.Name != "Sender" && selector.Sel.Name != "Receiver") {
		return methodParameter{}, false, nil
	}
	use := "stream." + selector.Sel.Name
	file := fileForExpr(files, expr)
	if file == nil {
		return methodParameter{}, true, fmt.Errorf("%s must be imported from %q", use, streamImportPath)
	}
	if err := resolveStreamImport(file, selector.X, use); err != nil {
		return methodParameter{}, true, err
	}
	if len(args) != 1 {
		return methodParameter{}, true, fmt.Errorf("%s must have exactly one local message type argument", use)
	}
	message, err := streamMessageArgument(args[0])
	if err != nil {
		return methodParameter{}, true, fmt.Errorf("%s: %w", use, err)
	}
	if err := validateStreamMessage(files, message); err != nil {
		return methodParameter{}, true, fmt.Errorf("%s message %q: %w", use, message, err)
	}
	kind := methodSenderParameter
	if selector.Sel.Name == "Receiver" {
		kind = methodReceiverParameter
	}
	return methodParameter{kind: kind, message: message}, true, nil
}

func fileForExpr(files []*ast.File, expr ast.Expr) *ast.File {
	for _, file := range files {
		if file.Pos() <= expr.Pos() && expr.End() <= file.End() {
			return file
		}
	}
	return nil
}

func resolveStreamImport(file *ast.File, packageExpr ast.Expr, use string) error {
	ident, ok := packageExpr.(*ast.Ident)
	if !ok {
		return fmt.Errorf("%s must use a package imported from %q", use, streamImportPath)
	}
	importPath, ok := importedPath(file, ident.Name)
	if !ok {
		return fmt.Errorf("%s package name %q is not imported from %q", use, ident.Name, streamImportPath)
	}
	if importPath != streamImportPath {
		return fmt.Errorf("%s must be imported from %q", use, streamImportPath)
	}
	return nil
}

// importedPath returns the path of the package that file imports as name.
func importedPath(file *ast.File, name string) (string, bool) {
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		local := path.Base(importPath)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		if local == name {
			return importPath, true
		}
	}
	return "", false
}

// isContextType reports whether expr names context.Context under any import
// name.
func isContextType(files []*ast.File, expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Context" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	file := fileForExpr(files, expr)
	if file == nil {
		return false
	}
	importPath, ok := importedPath(file, ident.Name)
	return ok && importPath == "context"
}

func streamMessageArgument(expr ast.Expr) (string, error) {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name, nil
	}
	switch expr.(type) {
	case *ast.StarExpr:
		return "", errors.New("message type argument must not be a pointer")
	case *ast.SelectorExpr:
		return "", errors.New("imported message types are unsupported; use a local struct message")
	case *ast.IndexExpr, *ast.IndexListExpr:
		return "", errors.New("message type argument must be a non-generic local named type")
	default:
		return "", errors.New("message type argument must be a local named type")
	}
}

func validateStreamMessage(files []*ast.File, name string) error {
	var found *ast.TypeSpec
	for _, file := range files {
		for _, decl := range file.Decls {
			group, ok := decl.(*ast.GenDecl)
			if !ok || group.Tok != token.TYPE {
				continue
			}
			for _, spec := range group.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				if typeSpec.Name.Name != name {
					continue
				}
				if found != nil {
					return errors.New("is ambiguous; declare one local struct message")
				}
				found = typeSpec
			}
		}
	}
	if found == nil {
		return errors.New("is unresolved")
	}
	if found.Assign.IsValid() {
		return errors.New("is an alias")
	}
	if found.TypeParams != nil && len(found.TypeParams.List) != 0 {
		return errors.New("must not be generic")
	}
	if _, ok := found.Type.(*ast.StructType); !ok {
		return errors.New("must be a local struct declaration")
	}
	return nil
}

func fieldComment(field *ast.Field) string {
	group := field.Doc
	if group == nil {
		group = field.Comment
	}
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}

func parseMessages(files []*ast.File, msgNames map[string]bool) ([]message, error) {
	var messages []message
	for _, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				st, ok := ts.Type.(*ast.StructType)
				if !ok || !msgNames[ts.Name.Name] {
					continue
				}
				msg, err := parseMessage(ts.Name.Name, st, msgNames)
				if err != nil {
					return nil, err
				}
				messages = append(messages, msg)
			}
		}
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].name < messages[j].name })
	return messages, nil
}

func parseMessage(name string, st *ast.StructType, msgNames map[string]bool) (message, error) {
	msg := message{name: name}
	byNumber := map[int]string{} // field number -> Go field name, to catch reuse
	for _, f := range st.Fields.List {
		reserved := len(f.Names) == 1 && f.Names[0].Name == "_"
		num, present, err := pbNumber(f, reserved)
		if err != nil {
			field := "field"
			if len(f.Names) > 0 {
				field = f.Names[0].Name
			}
			return message{}, fmt.Errorf("%s.%s: %w", name, field, err)
		}
		if !present || len(f.Names) == 0 {
			continue
		}
		if len(f.Names) > 1 {
			names := make([]string, len(f.Names))
			for i, n := range f.Names {
				names[i] = n.Name
			}
			return message{}, fmt.Errorf("%s: fields %s share a single pb tag (field number %d); declare each field separately with its own number", name, strings.Join(names, ", "), num)
		}
		goName := f.Names[0].Name
		if prev, dup := byNumber[num]; dup {
			return message{}, fmt.Errorf("%s: pb field number %d is used by both %s and %s", name, num, prev, goName)
		}
		byNumber[num] = goName
		if reserved {
			empty, ok := f.Type.(*ast.StructType)
			if !ok || len(empty.Fields.List) != 0 {
				return message{}, fmt.Errorf("%s: reserved field number %d must use `_ struct{}`", name, num)
			}
			msg.reserved = append(msg.reserved, num)
			continue
		}
		protoName := camelToSnake(goName)
		fl := field{goName: goName, protoName: protoName, protoGoName: goCamelCase(protoName), number: num}
		if err := classify(f.Type, msgNames, &fl); err != nil {
			return message{}, fmt.Errorf("%s.%s: %w", name, goName, err)
		}
		msg.fields = append(msg.fields, fl)
	}
	sort.Slice(msg.fields, func(i, j int) bool { return msg.fields[i].number < msg.fields[j].number })
	sort.Ints(msg.reserved)
	return msg, nil
}

// classify fills in fl.kind, fl.protoType and fl.mapKey from a Go field type.
func classify(expr ast.Expr, msgNames map[string]bool, fl *field) error {
	switch t := expr.(type) {
	case *ast.Ident:
		if s, ok := scalarProtoType(t.Name); ok {
			fl.kind, fl.protoType = scalarSingle, s
			return nil
		}
		if err := rejectAmbiguousInt(t.Name); err != nil {
			return err
		}
		if msgNames[t.Name] {
			return fmt.Errorf("embed a message by pointer (*%s), not by value", t.Name)
		}
		return fmt.Errorf("unsupported field type %q", t.Name)
	case *ast.StarExpr:
		// Message fields use pointers to preserve proto3 absence semantics.
		id, ok := t.X.(*ast.Ident)
		if !ok || !msgNames[id.Name] {
			return errors.New("unsupported pointer field type (only *Message is allowed)")
		}
		fl.kind, fl.protoType = messageSingle, id.Name
		return nil
	case *ast.ArrayType:
		if id, ok := t.Elt.(*ast.Ident); ok && id.Name == "byte" {
			fl.kind, fl.protoType = scalarSingle, "bytes"
			return nil
		}
		elt, ok := t.Elt.(*ast.Ident)
		if !ok {
			return errors.New("unsupported slice element type")
		}
		if s, ok := scalarProtoType(elt.Name); ok {
			fl.kind, fl.protoType = scalarRepeated, s
			return nil
		}
		if err := rejectAmbiguousInt(elt.Name); err != nil {
			return err
		}
		if msgNames[elt.Name] {
			fl.kind, fl.protoType = messageRepeated, elt.Name
			return nil
		}
		return fmt.Errorf("unsupported slice element type %q", elt.Name)
	case *ast.MapType:
		// The contract supports string keys and scalar values only.
		key, ok := t.Key.(*ast.Ident)
		if !ok || key.Name != "string" {
			return errors.New("map keys must be strings")
		}
		val, ok := t.Value.(*ast.Ident)
		if !ok {
			return errors.New("unsupported map value type")
		}
		vs, ok := scalarProtoType(val.Name)
		if !ok {
			if err := rejectAmbiguousInt(val.Name); err != nil {
				return err
			}
			return fmt.Errorf("only scalar map values are supported (value type %q)", val.Name)
		}
		fl.kind, fl.protoType, fl.mapKey = scalarMap, vs, "string"
		return nil
	default:
		return errors.New("unsupported field type")
	}
}

// pbNumber returns a field's number and whether a pb tag is present. A malformed
// tag is present with an error, rather than being treated as absent. Reservations
// may name protobuf's implementation-reserved range because they emit no field.
func pbNumber(f *ast.Field, reservation bool) (n int, present bool, err error) {
	if f.Tag == nil {
		return 0, false, nil
	}
	tag, uerr := strconv.Unquote(f.Tag.Value)
	if uerr != nil {
		return 0, false, nil
	}
	v, ok := reflect.StructTag(tag).Lookup("pb")
	if !ok {
		return 0, false, nil
	}
	parsed, aerr := strconv.ParseInt(v, 10, 64)
	if aerr != nil {
		return 0, true, fmt.Errorf("pb tag %q is not a field number", v)
	}
	if parsed < int64(protowire.MinValidNumber) {
		return 0, true, fmt.Errorf("pb field number must be >= 1, got %d", parsed)
	}
	if parsed > int64(protowire.MaxValidNumber) {
		return 0, true, fmt.Errorf("pb field number must be <= %d, got %d", protowire.MaxValidNumber, parsed)
	}
	if !reservation && parsed >= int64(protowire.FirstReservedNumber) && parsed <= int64(protowire.LastReservedNumber) {
		return 0, true, fmt.Errorf("pb field number %d is reserved by the protobuf implementation", parsed)
	}
	return int(parsed), true, nil
}

func scalarProtoType(goType string) (string, bool) {
	switch goType {
	case "string":
		return "string", true
	case "bool":
		return "bool", true
	case "int32":
		return "int32", true
	case "int64":
		return "int64", true
	case "uint32":
		return "uint32", true
	case "uint64":
		return "uint64", true
	case "float32":
		return "float", true
	case "float64":
		return "double", true
	}
	return "", false
}

// rejectAmbiguousInt rejects int and uint because their wire width is implicit.
func rejectAmbiguousInt(goType string) error {
	if goType == "int" || goType == "uint" {
		return fmt.Errorf("%s has no fixed width on the wire; use a sized integer such as int32, int64, uint32, or uint64", goType)
	}
	return nil
}

func pointerIdent(expr ast.Expr) (string, error) {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return "", errors.New("expected a pointer type")
	}
	id, ok := star.X.(*ast.Ident)
	if !ok {
		return "", errors.New("expected a named type")
	}
	return id.Name, nil
}

func isIdent(expr ast.Expr, name string) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == name
}

func results(ft *ast.FuncType) []ast.Expr {
	var out []ast.Expr
	for _, f := range fieldsPerName(ft.Results) {
		out = append(out, f.Type)
	}
	return out
}

// fieldsPerName repeats each field once per declared name, so a grouped
// declaration such as `a, b *T` counts as two entries.
func fieldsPerName(list *ast.FieldList) []*ast.Field {
	if list == nil {
		return nil
	}
	var out []*ast.Field
	for _, f := range list.List {
		for range max(1, len(f.Names)) {
			out = append(out, f)
		}
	}
	return out
}
