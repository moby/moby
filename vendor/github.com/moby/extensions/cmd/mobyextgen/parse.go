package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

// parsing

func parsePoint(dir string) (point, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return point{}, err
	}

	var files []*ast.File
	var pkgName string
	for name, pkg := range pkgs {
		if strings.HasSuffix(name, "_test") {
			continue
		}
		pkgName = name
		for _, f := range pkg.Files {
			files = append(files, f)
		}
	}
	if pkgName == "" {
		return point{}, fmt.Errorf("no package found in %s", dir)
	}

	pt := point{pkgName: pkgName}

	svc, err := findServicePragma(files)
	if err != nil {
		return point{}, err
	}
	pt.service = svc.service
	if svc.iface != "" {
		pt.iface, pt.id = svc.iface, svc.pkg
	} else {
		iface, id, single, err := findDefinePoint(files)
		if err != nil {
			return point{}, err
		}
		pt.iface, pt.id, pt.isPoint, pt.isSingle = iface, id, true, single
	}

	msgNames := messageNames(files)
	ifaceType := findInterface(files, pt.iface)
	if ifaceType == nil {
		return point{}, fmt.Errorf("interface %q not found", pt.iface)
	}
	pt.methods, err = parseMethods(ifaceType)
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

// servicePragma declares a contract's gRPC service name, and reservedPragma
// preserves a removed field number.
//
//	//mobyextgen:service=CreateSpecHook
//	var Point = extensions.DefinePoint[Hook]("...create_spec.v0")
//
//	//mobyextgen:reserved=2
//	type PointDeclaration struct{ ... }
const (
	servicePragma  = "//mobyextgen:service="
	reservedPragma = "//mobyextgen:reserved="
)

// serviceDecl is the service pragma result. Point mode supplies only service;
// service mode supplies the interface and fully-qualified proto package too.
type serviceDecl struct {
	service string
	iface   string
	pkg     string
}

// findServicePragma returns the declared gRPC service. The name is required in
// source because it may differ from the Go interface name and is part of the
// wire contract. An interface pragma is fully qualified; a point pragma is not.
func findServicePragma(files []*ast.File) (serviceDecl, error) {
	var found serviceDecl
	var seen string
	for _, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				values := declPragmas(gd, spec, servicePragma)
				if len(values) == 0 {
					continue
				}
				for _, value := range values {
					if seen != "" && seen != value {
						return serviceDecl{}, fmt.Errorf("conflicting %s pragmas: %q and %q", servicePragma, seen, value)
					}
					seen = value
				}
				value := values[0]

				ts, isType := spec.(*ast.TypeSpec)
				if !isType {
					found = serviceDecl{service: value}
					continue
				}
				if _, isIface := ts.Type.(*ast.InterfaceType); !isIface {
					return serviceDecl{}, fmt.Errorf("%s on %s: the pragma may only document an interface or the point", servicePragma, ts.Name.Name)
				}
				pkg, service, ok := strings.Cut(reverse(value), ".")
				if !ok {
					return serviceDecl{}, fmt.Errorf("%s on interface %s: want a fully-qualified name like my.proto.package.%s", servicePragma, ts.Name.Name, ts.Name.Name)
				}
				found = serviceDecl{service: reverse(pkg), iface: ts.Name.Name, pkg: reverse(service)}
			}
		}
	}
	if seen == "" {
		return serviceDecl{}, fmt.Errorf("no service pragma found; declare the contract's gRPC service name as `%s<Name>` next to its DefinePoint call, or fully qualified on its service interface", servicePragma)
	}
	return found, nil
}

// reverse returns s with its bytes reversed.
func reverse(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

// declPragmas returns all values of pragma on spec and its enclosing declaration.
// Returning all values lets callers reject conflicting declarations.
func declPragmas(gd *ast.GenDecl, spec ast.Spec, pragma string) []string {
	docs := []*ast.CommentGroup{gd.Doc}
	switch sp := spec.(type) {
	case *ast.TypeSpec:
		docs = append(docs, sp.Doc)
	case *ast.ValueSpec:
		docs = append(docs, sp.Doc)
	}
	var values []string
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		for _, c := range doc.List {
			if value, ok := strings.CutPrefix(c.Text, pragma); ok {
				values = append(values, strings.TrimSpace(value))
			}
		}
	}
	return values
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
		if _, present, _ := pbNumber(f); present {
			return true
		}
	}
	return false
}

func parseMethods(iface *ast.InterfaceType) ([]method, error) {
	var methods []method
	for _, m := range iface.Methods.List {
		ft, ok := m.Type.(*ast.FuncType)
		if !ok || len(m.Names) == 0 {
			continue
		}
		name := m.Names[0].Name
		if ft.Params == nil || len(ft.Params.List) == 0 {
			return nil, fmt.Errorf("method %q: expected a request parameter", name)
		}
		reqType := ft.Params.List[len(ft.Params.List)-1].Type
		req, err := pointerIdent(reqType)
		if err != nil {
			return nil, fmt.Errorf("method %q request: %w", name, err)
		}
		res := results(ft)
		switch {
		case len(res) == 1 && isIdent(res[0], "error"):
			methods = append(methods, method{name: name, request: req, response: name + "Response", bareError: true})
		case len(res) == 2 && isIdent(res[1], "error"):
			resp, err := pointerIdent(res[0])
			if err != nil {
				return nil, fmt.Errorf("method %q response: %w", name, err)
			}
			methods = append(methods, method{name: name, request: req, response: resp})
		default:
			return nil, fmt.Errorf("method %q: result must be `error` or `(*Resp, error)`", name)
		}
	}
	return methods, nil
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
				if msg.reserved, err = parseReserved(gd, spec); err != nil {
					return nil, fmt.Errorf("%s: %w", ts.Name.Name, err)
				}
				messages = append(messages, msg)
			}
		}
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].name < messages[j].name })
	return messages, nil
}

// parseReserved returns field numbers burned by removed fields. Proto forbids
// reusing them because old peers may decode new bytes as the old field.
//
//	//mobyextgen:reserved=2 // was 'exclusive'
//	type PointDeclaration struct{ ... }
func parseReserved(gd *ast.GenDecl, spec ast.Spec) ([]int, error) {
	var numbers []int
	for _, value := range declPragmas(gd, spec, reservedPragma) {
		list, _, _ := strings.Cut(value, " ")
		for f := range strings.SplitSeq(list, ",") {
			parsed, err := strconv.ParseInt(strings.TrimSpace(f), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%s%s: want a comma-separated list of field numbers", reservedPragma, value)
			}
			if parsed < int64(protowire.MinValidNumber) {
				return nil, fmt.Errorf("reserved field number must be >= 1, got %d", parsed)
			}
			if parsed > int64(protowire.MaxValidNumber) {
				return nil, fmt.Errorf("reserved field number must be <= %d, got %d", protowire.MaxValidNumber, parsed)
			}
			n := int(parsed)
			numbers = append(numbers, n)
		}
	}
	sort.Ints(numbers)
	return slices.Compact(numbers), nil
}

func parseMessage(name string, st *ast.StructType, msgNames map[string]bool) (message, error) {
	msg := message{name: name}
	byNumber := map[int]string{} // field number -> Go field name, to catch reuse
	for _, f := range st.Fields.List {
		num, present, err := pbNumber(f)
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
		protoName := camelToSnake(goName)
		fl := field{goName: goName, protoName: protoName, protoGoName: goCamelCase(protoName), number: num}
		if err := classify(f.Type, msgNames, &fl); err != nil {
			return message{}, fmt.Errorf("%s.%s: %w", name, goName, err)
		}
		msg.fields = append(msg.fields, fl)
	}
	sort.Slice(msg.fields, func(i, j int) bool { return msg.fields[i].number < msg.fields[j].number })
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
// tag is present with an error, rather than being treated as absent.
func pbNumber(f *ast.Field) (n int, present bool, err error) {
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
	if parsed >= int64(protowire.FirstReservedNumber) && parsed <= int64(protowire.LastReservedNumber) {
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
	if ft.Results == nil {
		return nil
	}
	var out []ast.Expr
	for _, f := range ft.Results.List {
		if len(f.Names) == 0 {
			out = append(out, f.Type)
			continue
		}
		for range f.Names {
			out = append(out, f.Type)
		}
	}
	return out
}
