// Command mobyextgen generates an extension point's wire contract and transport
// code from a Go interface and pb-tagged message structs.
//
// Run with no arguments from the point's package, typically through `go generate`.
//
//   - <service>.proto:       the reviewable wire contract.
//   - protogen/<service>.pb.go: the protobuf message code.
//   - protogen/wire.gen.go:  the gRPC service, adapters, and conversions.
//
// The Go interface and structs are the source of truth. Unsupported field
// shapes are rejected rather than emitted incorrectly.
//
// Generation needs only the Go toolchain; see descriptor.go for protobuf output.
package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

func main() {
	dir := "."
	if args := os.Args[1:]; len(args) == 1 {
		dir = args[0]
	} else if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "usage: mobyextgen [dir]")
		os.Exit(2)
	}
	if err := run(dir); err != nil {
		fmt.Fprintln(os.Stderr, "mobyextgen:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	pt, err := parsePoint(dir)
	if err != nil {
		return err
	}
	importPath, relDir, err := locate(dir)
	if err != nil {
		return err
	}
	pt.importPath = importPath
	protoName := camelToSnake(pt.service) + ".proto"
	pt.protoPath = path.Join(relDir, protoName)

	protoFile, err := emitProto(pt)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, protoName), protoFile, 0o644); err != nil {
		return err
	}

	messages, err := emitMessages(pt)
	if err != nil {
		return err
	}
	wire, err := emitWire(pt)
	if err != nil {
		return err
	}
	protogenDir := filepath.Join(dir, "protogen")
	if err := os.MkdirAll(protogenDir, 0o755); err != nil {
		return err
	}
	pbName := strings.TrimSuffix(protoName, ".proto") + ".pb.go"
	if err := os.WriteFile(filepath.Join(protogenDir, pbName), messages, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(protogenDir, "wire.gen.go"), wire, 0o644)
}

// locate returns the package import path and module-relative directory for dir.
func locate(dir string) (importPath, relDir string, _ error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	for root := abs; ; {
		gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err == nil {
			module, err := modulePath(gomod)
			if err != nil {
				return "", "", err
			}
			rel, err := filepath.Rel(root, abs)
			if err != nil {
				return "", "", err
			}
			relDir = filepath.ToSlash(rel)
			return path.Join(module, relDir), relDir, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", "", fmt.Errorf("no go.mod found above %s", abs)
		}
		root = parent
	}
}

// modulePath returns the module path declared by a go.mod file.
func modulePath(gomod []byte) (string, error) {
	for line := range strings.Lines(string(gomod)) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", errors.New("no module directive in go.mod")
}

// model

type point struct {
	pkgName    string // Go package name
	importPath string // package import path
	protoPath  string // module-relative path of the .proto
	id         string // proto package: the point id, or the pragma's package in service mode
	service    string // gRPC service name (from the mobyextgen:service pragma)
	iface      string // Go service interface name
	isPoint    bool   // whether the contract declares an extensions.Point
	isSingle   bool   // whether the point was declared with DefineSinglePoint
	methods    []method
	messages   []message
}

// grpcService returns the fully-qualified gRPC service name.
func (p point) grpcService() string { return p.id + "." + p.service }

func (p point) protogenImport() string { return p.importPath + "/protogen" }

type method struct {
	name      string // method/rpc name
	request   string // request message name
	response  string // response message name
	bareError bool   // true for `M(ctx, *Req) error`; response is a generated empty message
}

type message struct {
	name     string
	fields   []field
	reserved []int // field numbers burned by a removed field
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
			if slices.Contains(msg.reserved, f.number) {
				return nil, fmt.Errorf("%s.%s: field number %d is reserved", msg.name, f.protoName, f.number)
			}
			switch f.kind {
			case scalarSingle, messageSingle:
				fmt.Fprintf(&b, "  %s %s = %d;\n", f.protoType, f.protoName, f.number)
			case scalarRepeated, messageRepeated:
				fmt.Fprintf(&b, "  repeated %s %s = %d;\n", f.protoType, f.protoName, f.number)
			case scalarMap:
				fmt.Fprintf(&b, "  map<%s, %s> %s = %d;\n", f.mapKey, f.protoType, f.protoName, f.number)
			}
		}
		for _, n := range msg.reserved {
			fmt.Fprintf(&b, "  reserved %d;\n", n)
		}
		fmt.Fprintln(&b, "}")
	}

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
	if pt.isPoint {
		fmt.Fprintln(&b, `	extensions "github.com/moby/moby/v2/internal/extensions"`)
		fmt.Fprintln(&b, `	clientpoint "github.com/moby/moby/v2/internal/extensions/clientpoint"`)
		fmt.Fprintln(&b, `	serverpoint "github.com/moby/moby/v2/internal/extensions/serverpoint"`)
	}
	fmt.Fprintf(&b, "	%s %q\n", cpkg, pt.importPath)
	fmt.Fprintln(&b, `	grpc "google.golang.org/grpc"`)
	fmt.Fprintln(&b, ")")

	fmt.Fprintf(&b, `
// serviceName is the point's fully-qualified gRPC service name.
const serviceName = %q
`, pt.grpcService())

	fmt.Fprintln(&b, "\nconst (")
	for _, m := range pt.methods {
		fmt.Fprintf(&b, "\t%s = \"/\" + serviceName + %q\n", methodConst(m), "/"+m.name)
	}
	fmt.Fprintln(&b, ")")

	fmt.Fprintf(&b, `
// %[1]sServer is the server side of the point's gRPC service. It is the
// proto-level shape of the point, not the point's Go interface: a contract
// method returning a bare error returns an empty response message here.
type %[1]sServer interface {
`, svc)
	for _, m := range pt.methods {
		fmt.Fprintf(&b, "\t%s(context.Context, *%s) (*%s, error)\n", m.name, m.request, m.response)
	}
	fmt.Fprintln(&b, "}")

	fmt.Fprintf(&b, `
// serviceDesc describes the point's gRPC service to a server. HandlerType is
// what a registrar type-checks an implementation against, so registering the
// wrong provider for this point is caught at registration.
var serviceDesc = grpc.ServiceDesc{
	ServiceName: serviceName,
	HandlerType: (*%[1]sServer)(nil),
	Metadata:    %[2]q,
	Methods: []grpc.MethodDesc{
`, svc, pt.protoPath)
	for _, m := range pt.methods {
		fmt.Fprintf(&b, "\t\t{MethodName: %q, Handler: %s},\n", m.name, handlerName(m))
	}
	fmt.Fprintln(&b, "\t},\n}")

	for _, m := range pt.methods {
		fmt.Fprintf(&b, `
func %[1]s(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(%[2]s)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(%[3]sServer).%[4]s(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: %[5]s}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(%[3]sServer).%[4]s(ctx, req.(*%[2]s))
	})
}
`, handlerName(m), m.request, svc, m.name, methodConst(m))
	}

	fmt.Fprintf(&b, `
// %[1]sClient calls the point's gRPC service. It is exported so a client
// outside the framework - one calling a service an extension publishes on the
// API socket - can reach it with a plain gRPC client.
type %[1]sClient interface {
`, svc)
	for _, m := range pt.methods {
		fmt.Fprintf(&b, "\t%s(ctx context.Context, in *%s, opts ...grpc.CallOption) (*%s, error)\n", m.name, m.request, m.response)
	}
	fmt.Fprintln(&b, "}")

	fmt.Fprintf(&b, `
// New%[1]sClient returns a client for the point's gRPC service on cc.
func New%[1]sClient(cc grpc.ClientConnInterface) %[1]sClient { return &serviceClient{cc: cc} }

type serviceClient struct{ cc grpc.ClientConnInterface }
`, svc)

	for _, m := range pt.methods {
		fmt.Fprintf(&b, `
func (c *serviceClient) %[1]s(ctx context.Context, in *%[2]s, opts ...grpc.CallOption) (*%[3]s, error) {
	out := new(%[3]s)
	if err := c.cc.Invoke(ctx, %[4]s, in, out, append([]grpc.CallOption{grpc.StaticMethod()}, opts...)...); err != nil {
		return nil, err
	}
	return out, nil
}
`, m.name, m.request, m.response, methodConst(m))
	}

	if pt.isPoint {
		fmt.Fprintf(&b, `
// ServerPoint serves the %[1]s point: it registers the point's gRPC service for
// a provider with an SDK server. A binary passes it to (*sdk.Server).Register.
var ServerPoint = serverpoint.Registration{
	Point: %[2]s.Point.ID(),
	Register: func(r grpc.ServiceRegistrar, impl any) {
		r.RegisterService(&serviceDesc, &grpcServer{impl: impl.(%[2]s.%[3]s)})
	},
}

// ClientProvider builds a broker provider for the %[1]s point from an
// out-of-process gRPC connection.
func ClientProvider(conn grpc.ClientConnInterface) extensions.Provider {
	return %[2]s.Point.Provide(&grpcClient{client: New%[1]sClient(conn)})
}

// ClientPoint registers ClientProvider for the %[1]s point with a host.%[4]s
var ClientPoint = clientpoint.Registration{Point: %[2]s.Point.ID(), Provider: ClientProvider%[5]s}
`, svc, cpkg, iface, singleDoc(pt), singleField(pt))
	} else {
		fmt.Fprintf(&b, `
// RegisterServer serves impl as the %[1]s service on r.
func RegisterServer(r grpc.ServiceRegistrar, impl %[2]s.%[3]s) {
	r.RegisterService(&serviceDesc, &grpcServer{impl: impl})
}

// NewClient returns a %[2]s.%[3]s that calls the %[1]s service over conn.
func NewClient(conn grpc.ClientConnInterface) %[2]s.%[3]s {
	return &grpcClient{client: New%[1]sClient(conn)}
}
`, svc, cpkg, iface)
	}

	fmt.Fprintf(&b, `
// grpcServer serves an implementation of the contract's Go interface.
type grpcServer struct {
	impl %[1]s.%[2]s
}
`, cpkg, iface)

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

// camelToSnake converts Go field names to proto3 snake_case while keeping
// initialisms together (ContainerID -> container_id, HTTPServer -> http_server).
// A trailing s remains part of an initialism's plural form.
func camelToSnake(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i, c := range r {
		if i > 0 && c >= 'A' && c <= 'Z' {
			prev := r[i-1]
			prevIsLowerOrDigit := (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9')
			nextIsLower := i+1 < len(r) && r[i+1] >= 'a' && r[i+1] <= 'z'
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

// goCamelCase matches protoc-gen-go's field-name conversion so conversions use
// the generated struct and getter names.
func goCamelCase(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.' && i+1 < len(s) && isASCIILower(s[i+1]):
		case c == '.':
			b = append(b, '_')
		case c == '_' && (i == 0 || s[i-1] == '.'):
			b = append(b, 'X')
		case c == '_' && i+1 < len(s) && isASCIILower(s[i+1]):
		case isASCIIDigit(c):
			b = append(b, c)
		default:
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

// singleDoc and singleField carry DefineSinglePoint cardinality into generated
// registration.
func singleDoc(pt point) string {
	if !pt.isSingle {
		return ""
	}
	return "\n// Single carries the contract's cardinality: the point admits one provider."
}

func singleField(pt point) string {
	if !pt.isSingle {
		return ""
	}
	return ", Single: true"
}

func methodConst(m method) string { return "method" + m.name }
func handlerName(m method) string { return "handle" + m.name }
