package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/builder/dockerfile/parser"
)

func main() {
	var f *os.File
	var err error

	if len(os.Args) < 2 {
		fmt.Println("please supply filename(s)")
		os.Exit(1)
	}

	for _, fn := range os.Args[1:] {
		f, err = os.Open(fn)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		d := parser.Directive{LookingForDirectives: true}
		parser.SetEscapeToken(parser.DefaultEscapeToken, &d)

		ast, err := parser.Parse(f, &d)
		if err != nil {
			panic(err)
		} else {
			fmt.Println(ast.Dump())
		}
	}
}
