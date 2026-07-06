// Command mobyextgen generates an extension point's proto contract and its
// transport wiring from a Go-first contract: a Go interface (the point's
// provider interface, named in its extensions.DefinePoint call) plus message
// structs whose fields carry pb:"N" tags giving their proto field numbers.
//
// For a package it emits two files:
//
//   - <proto>:       the .proto, derived from the Go interface and structs.
//   - wire.gen.go:   Provide, ClientProvider, the gRPC adapters, and the
//     Go<->proto conversions, on top of the protogen package
//     that protoc produces from the emitted .proto.
//
// The Go interface and structs are the source of truth; the .proto and wiring
// are generated. It supports the narrow shapes points use -- scalars, strings,
// bytes, repeated scalars, string-keyed maps, and repeated messages -- and
// errors on anything else rather than emitting something subtly wrong.
package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func main() {
	dir := flag.String("dir", ".", "package directory to read the contract from and write generated files to")
	importPath := flag.String("import", "", "import path of the package (protogen is its /protogen subpackage)")
	protoName := flag.String("proto", "", "file name of the .proto to emit")
	flag.Parse()

	if *importPath == "" || *protoName == "" {
		fmt.Fprintln(os.Stderr, "mobyextgen: -import and -proto are required")
		os.Exit(2)
	}
	if err := run(*dir, *importPath, *protoName); err != nil {
		fmt.Fprintln(os.Stderr, "mobyextgen:", err)
		os.Exit(1)
	}
}

func run(dir, importPath, protoName string) error {
	pt, err := parsePoint(dir)
	if err != nil {
		return err
	}
	pt.importPath = importPath
	pt.service = snakeToCamel(strings.TrimSuffix(protoName, ".proto"))

	proto, err := emitProto(pt)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, protoName), proto, 0o644); err != nil {
		return err
	}

	wire, err := emitWire(pt)
	if err != nil {
		return err
	}
	// The wiring lives in the protogen package, beside the proto/gRPC code, so
	// the contract package stays free of any protobuf/gRPC imports.
	protogenDir := filepath.Join(dir, "protogen")
	if err := os.MkdirAll(protogenDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(protogenDir, "wire.gen.go"), wire, 0o644)
}

// model

type point struct {
	pkgName    string // Go package name
	importPath string // package import path
	id         string // proto package == point id
	service    string // proto service name (derived from the .proto file name)
	iface      string // Go provider interface name (from DefinePoint)
	methods    []method
	messages   []message
}

func (p point) protogenImport() string { return p.importPath + "/protogen" }

type method struct {
	name      string // method/rpc name
	request   string // request message name
	response  string // response message name
	bareError bool   // true for `M(ctx, *Req) error`; response is a generated empty message
}

type message struct {
	name   string
	fields []field
}

type fieldKind int

const (
	scalarSingle fieldKind = iota
	scalarRepeated
	scalarMap
	messageSingle
	messageRepeated
)

type field struct {
	goName      string // Go field name on the contract struct (e.g. ContainerID)
	protoName   string // proto3 field name (e.g. container_id)
	protoGoName string // Go field name protoc-gen-go emits for protoName (e.g. ContainerId)
	number      int
	protoType   string // proto type token, or the message name for messageSingle/messageRepeated
	mapKey      string // proto key type for scalarMap
	kind        fieldKind
}

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

	// The DefinePoint call names the provider interface and the point id.
	iface, id, err := findDefinePoint(files)
	if err != nil {
		return point{}, err
	}
	pt.iface, pt.id = iface, id

	// Collect message structs (those whose fields carry pb tags) and the
	// provider interface methods.
	msgNames := messageNames(files)
	ifaceType := findInterface(files, iface)
	if ifaceType == nil {
		return point{}, fmt.Errorf("interface %q not found", iface)
	}
	pt.methods, err = parseMethods(ifaceType)
	if err != nil {
		return point{}, err
	}
	// A method's request -- and a non-bare response -- must be emitted even when
	// it has no fields (e.g. an empty marker query), so add them to the set of
	// messages to emit alongside the pb-tagged structs.
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

// findDefinePoint returns the type argument (the provider interface) and the
// string argument (point id) of the extensions.DefinePoint[T]("id") call.
func findDefinePoint(files []*ast.File) (iface, id string, err error) {
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
				if !ok || sel.Sel.Name != "DefinePoint" {
					return true
				}
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
		return "", "", errors.New("no extensions.DefinePoint[T](\"id\") call found")
	}
	return iface, id, nil
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

// messageNames returns the set of struct types that have at least one pb-tagged
// field -- the message types of the contract.
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
		if _, ok := pbNumber(f); ok {
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
		// The request is the last parameter (after ctx).
		reqType := ft.Params.List[len(ft.Params.List)-1].Type
		req, err := pointerIdent(reqType)
		if err != nil {
			return nil, fmt.Errorf("method %q request: %w", name, err)
		}
		// Supported result shapes: `error` (bare; an empty response message is
		// generated) or `(*Resp, error)` (typed; Resp is a message).
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
		num, ok := pbNumber(f)
		if !ok || len(f.Names) == 0 {
			continue
		}
		// A grouped declaration (A, B string `pb:"1"`) would give every name the
		// same field number, which is invalid. Reject it rather than silently
		// keeping only the first name and dropping the rest from the wire.
		if len(f.Names) > 1 {
			names := make([]string, len(f.Names))
			for i, n := range f.Names {
				names[i] = n.Name
			}
			return message{}, fmt.Errorf("%s: fields %s share a single pb tag (field number %d); declare each field separately with its own number", name, strings.Join(names, ", "), num)
		}
		goName := f.Names[0].Name
		// Proto field numbers start at 1, and each must be unique within the
		// message -- a duplicate or a zero would produce a broken wire contract.
		if num < 1 {
			return message{}, fmt.Errorf("%s.%s: pb field number must be >= 1, got %d", name, goName, num)
		}
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
		// A single embedded message: *SomeMessage. Proto message fields are
		// pointers in generated Go, so the contract uses a pointer too, which
		// also lets the field be nil / absent.
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
		// Proto3 forbids float, bytes, and message map keys; the framework
		// narrows this to string keys, the only kind points use and the shape
		// the docs promise. Rejecting other keys here keeps the generator from
		// emitting a .proto that protoc would reject (or, for floats, silently
		// mis-generate).
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

// emit: proto

func emitProto(pt point) ([]byte, error) {
	var b strings.Builder
	fmt.Fprintln(&b, "// Code generated by mobyextgen. DO NOT EDIT.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, `syntax = "proto3";`)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "package %s;\n\n", pt.id)
	fmt.Fprintf(&b, "option go_package = %q;\n\n", pt.protogenImport())

	fmt.Fprintf(&b, "service %s {\n", pt.service)
	for _, m := range pt.methods {
		fmt.Fprintf(&b, "  rpc %s(%s) returns (%s);\n", m.name, m.request, m.response)
	}
	fmt.Fprintln(&b, "}")

	for _, msg := range pt.messages {
		fmt.Fprintf(&b, "\nmessage %s {\n", msg.name)
		for _, f := range msg.fields {
			switch f.kind {
			case scalarSingle, messageSingle:
				fmt.Fprintf(&b, "  %s %s = %d;\n", f.protoType, f.protoName, f.number)
			case scalarRepeated, messageRepeated:
				fmt.Fprintf(&b, "  repeated %s %s = %d;\n", f.protoType, f.protoName, f.number)
			case scalarMap:
				fmt.Fprintf(&b, "  map<%s, %s> %s = %d;\n", f.mapKey, f.protoType, f.protoName, f.number)
			}
		}
		fmt.Fprintln(&b, "}")
	}

	// Empty response messages for the bare-`error` methods.
	for _, m := range pt.methods {
		if m.bareError {
			fmt.Fprintf(&b, "\nmessage %s {}\n", m.response)
		}
	}
	return []byte(b.String()), nil
}

// emit: wire.gen.go

func emitWire(pt point) ([]byte, error) {
	var b strings.Builder
	cpkg := pt.pkgName
	svc, iface := pt.service, pt.iface
	fmt.Fprintln(&b, "// Code generated by mobyextgen. DO NOT EDIT.")
	fmt.Fprintf(&b, "package %s\n\n", path.Base(pt.protogenImport()))
	fmt.Fprintln(&b, "import (")
	fmt.Fprintln(&b, `	context "context"`)
	fmt.Fprintln(&b, `	extensions "github.com/moby/moby/v2/internal/extensions"`)
	fmt.Fprintln(&b, `	clientpoint "github.com/moby/moby/v2/internal/extensions/clientpoint"`)
	fmt.Fprintln(&b, `	serverpoint "github.com/moby/moby/v2/internal/extensions/serverpoint"`)
	fmt.Fprintf(&b, "	%s %q\n", cpkg, pt.importPath)
	fmt.Fprintln(&b, `	grpc "google.golang.org/grpc"`)
	fmt.Fprintln(&b, ")")

	// The proto/gRPC types are local to this (protogen) package; the contract
	// types are reached through the contract package, cpkg.
	fmt.Fprintf(&b, `
// ServerPoint serves the %[1]s point: it registers the point's gRPC service for
// a provider with an SDK server. A binary passes it to (*sdk.Server).Register.
var ServerPoint = serverpoint.Registration{
	Point: %[2]s.Point.ID(),
	Register: func(r grpc.ServiceRegistrar, impl any) {
		Register%[1]sServer(r, &grpcServer{impl: impl.(%[2]s.%[3]s)})
	},
}

// ClientProvider builds a broker provider for the %[1]s point from an
// out-of-process gRPC connection.
func ClientProvider(conn grpc.ClientConnInterface) extensions.Provider {
	return %[2]s.Point.Provide(&grpcClient{client: New%[1]sClient(conn)})
}

// ClientPoint registers ClientProvider for the %[1]s point with a host.
var ClientPoint = clientpoint.Registration{Point: %[2]s.Point.ID(), Provider: ClientProvider}

type grpcServer struct {
	Unimplemented%[1]sServer
	impl %[2]s.%[3]s
}
`, svc, cpkg, iface)

	for _, m := range pt.methods {
		if m.bareError {
			fmt.Fprintf(&b, `
func (s *grpcServer) %[1]s(ctx context.Context, req *%[2]s) (*%[3]s, error) {
	if err := s.impl.%[1]s(ctx, %[4]sFromProto(req)); err != nil {
		return nil, err
	}
	return &%[3]s{}, nil
}
`, m.name, m.request, m.response, lowerFirst(m.request))
		} else {
			fmt.Fprintf(&b, `
func (s *grpcServer) %[1]s(ctx context.Context, req *%[2]s) (*%[3]s, error) {
	resp, err := s.impl.%[1]s(ctx, %[4]sFromProto(req))
	if err != nil {
		return nil, err
	}
	return %[5]sToProto(resp), nil
}
`, m.name, m.request, m.response, lowerFirst(m.request), lowerFirst(m.response))
		}
	}

	fmt.Fprintf(&b, `
type grpcClient struct {
	client %sClient
}
`, svc)

	for _, m := range pt.methods {
		if m.bareError {
			fmt.Fprintf(&b, `
func (c *grpcClient) %[1]s(ctx context.Context, req *%[2]s.%[3]s) error {
	_, err := c.client.%[1]s(ctx, %[4]sToProto(req))
	return err
}
`, m.name, cpkg, m.request, lowerFirst(m.request))
		} else {
			fmt.Fprintf(&b, `
func (c *grpcClient) %[1]s(ctx context.Context, req *%[2]s.%[3]s) (*%[2]s.%[5]s, error) {
	resp, err := c.client.%[1]s(ctx, %[4]sToProto(req))
	if err != nil {
		return nil, err
	}
	return %[6]sFromProto(resp), nil
}
`, m.name, cpkg, m.request, lowerFirst(m.request), m.response, lowerFirst(m.response))
		}
	}

	for _, msg := range pt.messages {
		emitConversions(&b, cpkg, msg)
	}

	return format.Source([]byte(b.String()))
}

func emitConversions(b *strings.Builder, cpkg string, msg message) {
	conv := lowerFirst(msg.name)

	fmt.Fprintf(b, "\nfunc %sToProto(in *%s.%s) *%s {\n", conv, cpkg, msg.name, msg.name)
	fmt.Fprintln(b, "\tif in == nil {\n\t\treturn nil\n\t}")
	fmt.Fprintf(b, "\tout := &%s{}\n", msg.name)
	for _, f := range msg.fields {
		switch f.kind {
		case scalarSingle, scalarRepeated, scalarMap:
			fmt.Fprintf(b, "\tout.%s = in.%s\n", f.protoGoName, f.goName)
		case messageSingle:
			fmt.Fprintf(b, "\tout.%s = %sToProto(in.%s)\n", f.protoGoName, lowerFirst(f.protoType), f.goName)
		case messageRepeated:
			fmt.Fprintf(b, "\tfor i := range in.%s {\n\t\tout.%s = append(out.%s, %sToProto(&in.%s[i]))\n\t}\n",
				f.goName, f.protoGoName, f.protoGoName, lowerFirst(f.protoType), f.goName)
		}
	}
	fmt.Fprintln(b, "\treturn out\n}")

	fmt.Fprintf(b, "\nfunc %sFromProto(in *%s) *%s.%s {\n", conv, msg.name, cpkg, msg.name)
	fmt.Fprintln(b, "\tif in == nil {\n\t\treturn nil\n\t}")
	fmt.Fprintf(b, "\tout := &%s.%s{}\n", cpkg, msg.name)
	for _, f := range msg.fields {
		switch f.kind {
		case scalarSingle, scalarRepeated, scalarMap:
			fmt.Fprintf(b, "\tout.%s = in.Get%s()\n", f.goName, f.protoGoName)
		case messageSingle:
			fmt.Fprintf(b, "\tout.%s = %sFromProto(in.Get%s())\n", f.goName, lowerFirst(f.protoType), f.protoGoName)
		case messageRepeated:
			fmt.Fprintf(b, "\tfor _, e := range in.Get%s() {\n\t\tout.%s = append(out.%s, *%sFromProto(e))\n\t}\n",
				f.protoGoName, f.goName, f.goName, lowerFirst(f.protoType))
		}
	}
	fmt.Fprintln(b, "\treturn out\n}")
}

// helpers

func pbNumber(f *ast.Field) (int, bool) {
	if f.Tag == nil {
		return 0, false
	}
	tag, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return 0, false
	}
	v, ok := reflect.StructTag(tag).Lookup("pb")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
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

// rejectAmbiguousInt reports an error for the width-ambiguous Go integer types
// int and uint. Proto has no such type: protoc-gen-go emits int64/uint64 for
// them, so the wire conversions -- which assign the contract field to the
// generated field directly -- would not even compile (int is not int64). Rather
// than pick a width silently, the contract must name an explicit one.
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

// results flattens a function's result list into one expr per result.
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

// camelToSnake converts a Go field name to a proto3 snake_case field name,
// treating an initialism run as a single word: ContainerID -> container_id,
// HTTPServer -> http_server, APIKey -> api_key. A word boundary (underscore) is
// inserted before an uppercase letter that either follows a lowercase or digit,
// or begins a new word after an acronym (i.e. it is itself followed by a
// lowercase). The old rule -- an underscore before every uppercase -- produced
// container_i_d; it round-tripped back to ContainerID through protoc's casing,
// but leaked broken field names into the wire contract every non-Go author reads.
//
// A lone trailing lowercase "s" is treated as a plural suffix on the acronym,
// not the start of a new word, so ContainerIDs -> container_ids and CPUs ->
// cpus rather than container_i_ds / cp_us.
func camelToSnake(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i, c := range r {
		if i > 0 && c >= 'A' && c <= 'Z' {
			prev := r[i-1]
			prevIsLowerOrDigit := (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9')
			nextIsLower := i+1 < len(r) && r[i+1] >= 'a' && r[i+1] <= 'z'
			// An uppercase that ends an acronym and is followed only by a plural
			// "s" (end of name, or the "s" then another word) is not a new word:
			// the "s" pluralizes the acronym (IDs, CPUs), so no boundary here.
			pluralS := nextIsLower && r[i+1] == 's' && (i+2 == len(r) || (r[i+2] >= 'A' && r[i+2] <= 'Z'))
			if prevIsLowerOrDigit || (prev >= 'A' && prev <= 'Z' && nextIsLower && !pluralS) {
				b.WriteByte('_')
			}
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteRune(c)
	}
	return b.String()
}

// goCamelCase converts a proto field name to the Go identifier protoc-gen-go
// generates for it, so the wire conversions can address the generated struct and
// its getters by their real names. It mirrors the algorithm in
// google.golang.org/protobuf/internal/strs.GoCamelCase: protoc-gen-go is not
// initialism-aware (container_id -> ContainerId, url -> Url), so reproducing it
// exactly is what lets a clean snake_case proto name coexist with a Go-idiomatic
// contract name (ContainerID) -- the conversions bridge the two.
func goCamelCase(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.' && i+1 < len(s) && isASCIILower(s[i+1]):
			// Skip over '.' in ".{{lowercase}}".
		case c == '.':
			b = append(b, '_') // convert '.' to '_'
		case c == '_' && (i == 0 || s[i-1] == '.'):
			// Convert initial '_' to ensure we start with a capital letter.
			b = append(b, 'X')
		case c == '_' && i+1 < len(s) && isASCIILower(s[i+1]):
			// Skip over '_' in "_{{lowercase}}".
		case isASCIIDigit(c):
			b = append(b, c)
		default:
			// The next word is a sequence starting with an upper-case letter,
			// followed by any lower-case letters (an acronym stays upper-case).
			if isASCIILower(c) {
				c -= 'a' - 'A'
			}
			b = append(b, c)
			for ; i+1 < len(s) && isASCIILower(s[i+1]); i++ {
				b = append(b, s[i+1])
			}
		}
	}
	return string(b)
}

func isASCIILower(c byte) bool { return 'a' <= c && c <= 'z' }
func isASCIIDigit(c byte) bool { return '0' <= c && c <= '9' }

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func snakeToCamel(s string) string {
	var b strings.Builder
	for _, part := range strings.Split(s, "_") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}
