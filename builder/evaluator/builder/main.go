package main

import (
	"os"

	"github.com/erikh/buildfile/evaluator"
)

func main() {
	if len(os.Args) < 2 {
		os.Stderr.WriteString("Please supply filename(s) to evaluate")
		os.Exit(1)
	}

	for _, fn := range os.Args[1:] {
		f, err := os.Open(fn)
		if err != nil {
			panic(err)
		}

		opts := &evaluator.BuildOpts{}

		bf, err := opts.NewBuildFile(f)
		if err != nil {
			panic(err)
		}
		if err := bf.Run(); err != nil {
			panic(err)
		}
	}
}
