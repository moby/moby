// staticcheck analyses Go code and makes it better.
package main // import "honnef.co/go/tools/cmd/staticcheck"

import (
	"os"

	"honnef.co/go/tools/lint"
	"honnef.co/go/tools/lint/lintutil"
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
	"honnef.co/go/tools/unused"
)

func main() {
	fs := lintutil.FlagSet("staticcheck")
	fs.Parse(os.Args[1:])

	checkers := []lint.Checker{
		simple.NewChecker(),
		staticcheck.NewChecker(),
		stylecheck.NewChecker(),
		&unused.Checker{},
	}

	lintutil.ProcessFlagSet(checkers, fs)
}
