// Command apidiff determines whether two versions of a package are compatible
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/token"
	"go/types"
	"os"

	"golang.org/x/exp/apidiff"
	"golang.org/x/tools/go/gcexportdata"
	"golang.org/x/tools/go/packages"
)

var (
	exportDataOutfile = flag.String("w", "", "file for export data")
	incompatibleOnly  = flag.Bool("incompatible", false, "display only incompatible changes")
)

func main() {
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "usage:\n")
		fmt.Fprintf(w, "apidiff OLD NEW\n")
		fmt.Fprintf(w, "   compares OLD and NEW package APIs\n")
		fmt.Fprintf(w, "   where OLD and NEW are either import paths or files of export data\n")
		fmt.Fprintf(w, "apidiff -w FILE IMPORT_PATH\n")
		fmt.Fprintf(w, "   writes export data of the package at IMPORT_PATH to FILE\n")
		fmt.Fprintf(w, "   NOTE: In a GOPATH-less environment, this option consults the\n")
		fmt.Fprintf(w, "   module cache by default, unless used in the directory that\n")
		fmt.Fprintf(w, "   contains the go.mod module definition that IMPORT_PATH belongs\n")
		fmt.Fprintf(w, "   to. In most cases users want the latter behavior, so be sure\n")
		fmt.Fprintf(w, "   to cd to the exact directory which contains the module\n")
		fmt.Fprintf(w, "   definition of IMPORT_PATH.\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	if *exportDataOutfile != "" {
		if len(flag.Args()) != 1 {
			flag.Usage()
			os.Exit(2)
		}
		pkg := mustLoadPackage(flag.Arg(0))
		if err := writeExportData(pkg, *exportDataOutfile); err != nil {
			die("writing export data: %v", err)
		}
	} else {
		if len(flag.Args()) != 2 {
			flag.Usage()
			os.Exit(2)
		}
		oldpkg := mustLoadOrRead(flag.Arg(0))
		newpkg := mustLoadOrRead(flag.Arg(1))

		report := apidiff.Changes(oldpkg, newpkg)
		var err error
		if *incompatibleOnly {
			err = report.TextIncompatible(os.Stdout, false)
		} else {
			err = report.Text(os.Stdout)
		}
		if err != nil {
			die("writing report: %v", err)
		}
	}
}

func mustLoadOrRead(importPathOrFile string) *types.Package {
	fileInfo, err := os.Stat(importPathOrFile)
	if err == nil && fileInfo.Mode().IsRegular() {
		pkg, err := readExportData(importPathOrFile)
		if err != nil {
			die("reading export data from %s: %v", importPathOrFile, err)
		}
		return pkg
	} else {
		return mustLoadPackage(importPathOrFile).Types
	}
}

func mustLoadPackage(importPath string) *packages.Package {
	pkg, err := loadPackage(importPath)
	if err != nil {
		die("loading %s: %v", importPath, err)
	}
	return pkg
}

func loadPackage(importPath string) (*packages.Package, error) {
	cfg := &packages.Config{Mode: packages.LoadTypes}
	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("found no packages for import %s", importPath)
	}
	if len(pkgs[0].Errors) > 0 {
		return nil, pkgs[0].Errors[0]
	}
	return pkgs[0], nil
}

func readExportData(filename string) (*types.Package, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	m := map[string]*types.Package{}
	pkgPath, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	pkgPath = pkgPath[:len(pkgPath)-1] // remove delimiter
	return gcexportdata.Read(r, token.NewFileSet(), m, pkgPath)
}

func writeExportData(pkg *packages.Package, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	// Include the package path in the file. The exportdata format does
	// not record the path of the package being written.
	fmt.Fprintln(f, pkg.PkgPath)
	err1 := gcexportdata.Write(f, pkg.Fset, pkg.Types)
	err2 := f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
