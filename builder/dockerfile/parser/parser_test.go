package parser

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-check/check"
)

const testDir = "testfiles"
const negativeTestDir = "testfiles-negative"
const testFileLineInfo = "testfile-line/Dockerfile"

func getDirs(c *check.C, dir string) []string {
	f, err := os.Open(dir)
	if err != nil {
		c.Fatal(err)
	}

	defer f.Close()

	dirs, err := f.Readdirnames(0)
	if err != nil {
		c.Fatal(err)
	}

	return dirs
}

func (s *DockerSuite) TestTestNegative(c *check.C) {
	for _, dir := range getDirs(c, negativeTestDir) {
		dockerfile := filepath.Join(negativeTestDir, dir, "Dockerfile")

		df, err := os.Open(dockerfile)
		if err != nil {
			c.Fatalf("Dockerfile missing for %s: %v", dir, err)
		}
		defer df.Close()

		d := Directive{LookingForDirectives: true}
		SetEscapeToken(DefaultEscapeToken, &d)
		_, err = Parse(df, &d)
		if err == nil {
			c.Fatalf("No error parsing broken dockerfile for %s", dir)
		}
	}
}

func (s *DockerSuite) TestTestData(c *check.C) {
	for _, dir := range getDirs(c, testDir) {
		dockerfile := filepath.Join(testDir, dir, "Dockerfile")
		resultfile := filepath.Join(testDir, dir, "result")

		df, err := os.Open(dockerfile)
		if err != nil {
			c.Fatalf("Dockerfile missing for %s: %v", dir, err)
		}
		defer df.Close()

		d := Directive{LookingForDirectives: true}
		SetEscapeToken(DefaultEscapeToken, &d)
		ast, err := Parse(df, &d)
		if err != nil {
			c.Fatalf("Error parsing %s's dockerfile: %v", dir, err)
		}

		content, err := ioutil.ReadFile(resultfile)
		if err != nil {
			c.Fatalf("Error reading %s's result file: %v", dir, err)
		}

		if runtime.GOOS == "windows" {
			// CRLF --> CR to match Unix behavior
			content = bytes.Replace(content, []byte{'\x0d', '\x0a'}, []byte{'\x0a'}, -1)
		}

		if ast.Dump()+"\n" != string(content) {
			fmt.Fprintln(os.Stderr, "Result:\n"+ast.Dump())
			fmt.Fprintln(os.Stderr, "Expected:\n"+string(content))
			c.Fatalf("%s: AST dump of dockerfile does not match result", dir)
		}
	}
}

func (s *DockerSuite) TestParseWords(c *check.C) {
	tests := []map[string][]string{
		{
			"input":  {"foo"},
			"expect": {"foo"},
		},
		{
			"input":  {"foo bar"},
			"expect": {"foo", "bar"},
		},
		{
			"input":  {"foo\\ bar"},
			"expect": {"foo\\ bar"},
		},
		{
			"input":  {"foo=bar"},
			"expect": {"foo=bar"},
		},
		{
			"input":  {"foo bar 'abc xyz'"},
			"expect": {"foo", "bar", "'abc xyz'"},
		},
		{
			"input":  {`foo bar "abc xyz"`},
			"expect": {"foo", "bar", `"abc xyz"`},
		},
		{
			"input":  {"àöû"},
			"expect": {"àöû"},
		},
		{
			"input":  {`föo bàr "âbc xÿz"`},
			"expect": {"föo", "bàr", `"âbc xÿz"`},
		},
	}

	for _, test := range tests {
		d := Directive{LookingForDirectives: true}
		SetEscapeToken(DefaultEscapeToken, &d)
		words := parseWords(test["input"][0], &d)
		if len(words) != len(test["expect"]) {
			c.Fatalf("length check failed. input: %v, expect: %q, output: %q", test["input"][0], test["expect"], words)
		}
		for i, word := range words {
			if word != test["expect"][i] {
				c.Fatalf("word check failed for word: %q. input: %q, expect: %q, output: %q", word, test["input"][0], test["expect"], words)
			}
		}
	}
}

func (s *DockerSuite) TestLineInformation(c *check.C) {
	df, err := os.Open(testFileLineInfo)
	if err != nil {
		c.Fatalf("Dockerfile missing for %s: %v", testFileLineInfo, err)
	}
	defer df.Close()

	d := Directive{LookingForDirectives: true}
	SetEscapeToken(DefaultEscapeToken, &d)
	ast, err := Parse(df, &d)
	if err != nil {
		c.Fatalf("Error parsing dockerfile %s: %v", testFileLineInfo, err)
	}

	if ast.StartLine != 5 || ast.EndLine != 31 {
		fmt.Fprintf(os.Stderr, "Wrong root line information: expected(%d-%d), actual(%d-%d)\n", 5, 31, ast.StartLine, ast.EndLine)
		c.Fatalf("Root line information doesn't match result.")
	}
	if len(ast.Children) != 3 {
		fmt.Fprintf(os.Stderr, "Wrong number of child: expected(%d), actual(%d)\n", 3, len(ast.Children))
		c.Fatalf("Root line information doesn't match result for %s", testFileLineInfo)
	}
	expected := [][]int{
		{5, 5},
		{11, 12},
		{17, 31},
	}
	for i, child := range ast.Children {
		if child.StartLine != expected[i][0] || child.EndLine != expected[i][1] {
			c.Logf("Wrong line information for child %d: expected(%d-%d), actual(%d-%d)\n",
				i, expected[i][0], expected[i][1], child.StartLine, child.EndLine)
			c.Fatalf("Root line information doesn't match result.")
		}
	}
}
