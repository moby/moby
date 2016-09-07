package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

const testFixture = "fixtures/foo.go"

func (s *DockerSuite) TestParseEmptyInterface(c *check.C) {
	pkg, err := Parse(testFixture, "Fooer")
	if err != nil {
		c.Fatal(err)
	}

	assertName(c, "foo", pkg.Name)
	assertNum(c, 0, len(pkg.Functions))
}

func (s *DockerSuite) TestParseNonInterfaceType(c *check.C) {
	_, err := Parse(testFixture, "wobble")
	if _, ok := err.(errUnexpectedType); !ok {
		c.Fatal("expected type error when parsing non-interface type")
	}
}

func (s *DockerSuite) TestParseWithOneFunction(c *check.C) {
	pkg, err := Parse(testFixture, "Fooer2")
	if err != nil {
		c.Fatal(err)
	}

	assertName(c, "foo", pkg.Name)
	assertNum(c, 1, len(pkg.Functions))
	assertName(c, "Foo", pkg.Functions[0].Name)
	assertNum(c, 0, len(pkg.Functions[0].Args))
	assertNum(c, 0, len(pkg.Functions[0].Returns))
}

func (s *DockerSuite) TestParseWithMultipleFuncs(c *check.C) {
	pkg, err := Parse(testFixture, "Fooer3")
	if err != nil {
		c.Fatal(err)
	}

	assertName(c, "foo", pkg.Name)
	assertNum(c, 7, len(pkg.Functions))

	f := pkg.Functions[0]
	assertName(c, "Foo", f.Name)
	assertNum(c, 0, len(f.Args))
	assertNum(c, 0, len(f.Returns))

	f = pkg.Functions[1]
	assertName(c, "Bar", f.Name)
	assertNum(c, 1, len(f.Args))
	assertNum(c, 0, len(f.Returns))
	arg := f.Args[0]
	assertName(c, "a", arg.Name)
	assertName(c, "string", arg.ArgType)

	f = pkg.Functions[2]
	assertName(c, "Baz", f.Name)
	assertNum(c, 1, len(f.Args))
	assertNum(c, 1, len(f.Returns))
	arg = f.Args[0]
	assertName(c, "a", arg.Name)
	assertName(c, "string", arg.ArgType)
	arg = f.Returns[0]
	assertName(c, "err", arg.Name)
	assertName(c, "error", arg.ArgType)

	f = pkg.Functions[3]
	assertName(c, "Qux", f.Name)
	assertNum(c, 2, len(f.Args))
	assertNum(c, 2, len(f.Returns))
	arg = f.Args[0]
	assertName(c, "a", f.Args[0].Name)
	assertName(c, "string", f.Args[0].ArgType)
	arg = f.Args[1]
	assertName(c, "b", arg.Name)
	assertName(c, "string", arg.ArgType)
	arg = f.Returns[0]
	assertName(c, "val", arg.Name)
	assertName(c, "string", arg.ArgType)
	arg = f.Returns[1]
	assertName(c, "err", arg.Name)
	assertName(c, "error", arg.ArgType)

	f = pkg.Functions[4]
	assertName(c, "Wobble", f.Name)
	assertNum(c, 0, len(f.Args))
	assertNum(c, 1, len(f.Returns))
	arg = f.Returns[0]
	assertName(c, "w", arg.Name)
	assertName(c, "*wobble", arg.ArgType)

	f = pkg.Functions[5]
	assertName(c, "Wiggle", f.Name)
	assertNum(c, 0, len(f.Args))
	assertNum(c, 1, len(f.Returns))
	arg = f.Returns[0]
	assertName(c, "w", arg.Name)
	assertName(c, "wobble", arg.ArgType)

	f = pkg.Functions[6]
	assertName(c, "WiggleWobble", f.Name)
	assertNum(c, 6, len(f.Args))
	assertNum(c, 6, len(f.Returns))
	expectedArgs := [][]string{
		{"a", "[]*wobble"},
		{"b", "[]wobble"},
		{"c", "map[string]*wobble"},
		{"d", "map[*wobble]wobble"},
		{"e", "map[string][]wobble"},
		{"f", "[]*otherfixture.Spaceship"},
	}
	for i, arg := range f.Args {
		assertName(c, expectedArgs[i][0], arg.Name)
		assertName(c, expectedArgs[i][1], arg.ArgType)
	}
	expectedReturns := [][]string{
		{"g", "map[*wobble]wobble"},
		{"h", "[][]*wobble"},
		{"i", "otherfixture.Spaceship"},
		{"j", "*otherfixture.Spaceship"},
		{"k", "map[*otherfixture.Spaceship]otherfixture.Spaceship"},
		{"l", "[]otherfixture.Spaceship"},
	}
	for i, ret := range f.Returns {
		assertName(c, expectedReturns[i][0], ret.Name)
		assertName(c, expectedReturns[i][1], ret.ArgType)
	}
}

func (s *DockerSuite) TestParseWithUnamedReturn(c *check.C) {
	_, err := Parse(testFixture, "Fooer4")
	if !strings.HasSuffix(err.Error(), errBadReturn.Error()) {
		c.Fatalf("expected ErrBadReturn, got %v", err)
	}
}

func (s *DockerSuite) TestEmbeddedInterface(c *check.C) {
	pkg, err := Parse(testFixture, "Fooer5")
	if err != nil {
		c.Fatal(err)
	}

	assertName(c, "foo", pkg.Name)
	assertNum(c, 2, len(pkg.Functions))

	f := pkg.Functions[0]
	assertName(c, "Foo", f.Name)
	assertNum(c, 0, len(f.Args))
	assertNum(c, 0, len(f.Returns))

	f = pkg.Functions[1]
	assertName(c, "Boo", f.Name)
	assertNum(c, 2, len(f.Args))
	assertNum(c, 2, len(f.Returns))

	arg := f.Args[0]
	assertName(c, "a", arg.Name)
	assertName(c, "string", arg.ArgType)

	arg = f.Args[1]
	assertName(c, "b", arg.Name)
	assertName(c, "string", arg.ArgType)

	arg = f.Returns[0]
	assertName(c, "s", arg.Name)
	assertName(c, "string", arg.ArgType)

	arg = f.Returns[1]
	assertName(c, "err", arg.Name)
	assertName(c, "error", arg.ArgType)
}

func (s *DockerSuite) TestParsedImports(c *check.C) {
	cases := []string{"Fooer6", "Fooer7", "Fooer8", "Fooer9", "Fooer10", "Fooer11"}
	for _, testCase := range cases {
		pkg, err := Parse(testFixture, testCase)
		if err != nil {
			c.Fatal(err)
		}

		assertNum(c, 1, len(pkg.Imports))
		importPath := strings.Split(pkg.Imports[0].Path, "/")
		assertName(c, "otherfixture\"", importPath[len(importPath)-1])
		assertName(c, "", pkg.Imports[0].Name)
	}
}

func (s *DockerSuite) TestAliasedImports(c *check.C) {
	pkg, err := Parse(testFixture, "Fooer12")
	if err != nil {
		c.Fatal(err)
	}

	assertNum(c, 1, len(pkg.Imports))
	assertName(c, "aliasedio", pkg.Imports[0].Name)
}

func assertName(c *check.C, expected, actual string) {
	if expected != actual {
		fatalOut(c, fmt.Sprintf("expected name to be `%s`, got: %s", expected, actual))
	}
}

func assertNum(c *check.C, expected, actual int) {
	if expected != actual {
		fatalOut(c, fmt.Sprintf("expected number to be %d, got: %d", expected, actual))
	}
}

func fatalOut(c *check.C, msg string) {
	_, file, ln, _ := runtime.Caller(2)
	c.Fatalf("%s:%d: %s", filepath.Base(file), ln, msg)
}
