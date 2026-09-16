// Command mobyextgen generates an extension point or service's wire contract
// and transport code from a Go interface and pb-tagged message structs.
//
// Run with no arguments from a Point contract's package, typically through
// `go generate`. Use -service with a fully qualified gRPC service name for an
// internal transport contract that is not a Point.
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
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	service := flag.String("service", "", "fully qualified name of a non-Point gRPC service")
	flag.Usage = func() {
		_, _ = fmt.Fprintln(flag.CommandLine.Output(), "usage: mobyextgen [-service package.Service] [dir]")
		flag.PrintDefaults()
	}
	flag.Parse()

	dir := "."
	if args := flag.Args(); len(args) == 1 {
		dir = args[0]
	} else if len(args) > 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(dir, *service); err != nil {
		fmt.Fprintln(os.Stderr, "mobyextgen:", err)
		os.Exit(1)
	}
}

func run(dir, service string) error {
	var pt point
	var err error
	if service == "" {
		pt, err = parsePoint(dir)
	} else {
		pt, err = parseService(dir, service)
	}
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
