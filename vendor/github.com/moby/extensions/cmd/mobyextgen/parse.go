package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

// parsing

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
