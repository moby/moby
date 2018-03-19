package parser // import "github.com/docker/docker/builder/dockerfile/parser"

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

const testDir = "testfiles"
const negativeTestDir = "testfiles-negative"
const testFileLineInfo = "testfile-line/Dockerfile"

func getDirs(t *testing.T, dir string) []string {
	f, err := os.Open(dir)
	assert.NilError(t, err)
	defer f.Close()

	dirs, err := f.Readdirnames(0)
	assert.NilError(t, err)
	return dirs
}

func TestParseErrorCases(t *testing.T) {
	for _, dir := range getDirs(t, negativeTestDir) {
		dockerfile := filepath.Join(negativeTestDir, dir, "Dockerfile")

		df, err := os.Open(dockerfile)
		assert.NilError(t, err, dockerfile)
		defer df.Close()

		_, err = Parse(df)
		assert.Check(t, is.ErrorContains(err, ""), dockerfile)
	}
}

func TestParseCases(t *testing.T) {
	for _, dir := range getDirs(t, testDir) {
		dockerfile := filepath.Join(testDir, dir, "Dockerfile")
		resultfile := filepath.Join(testDir, dir, "result")

		df, err := os.Open(dockerfile)
		assert.NilError(t, err, dockerfile)
		defer df.Close()

		result, err := Parse(df)
		assert.NilError(t, err, dockerfile)

		content, err := ioutil.ReadFile(resultfile)
		assert.NilError(t, err, resultfile)

		if runtime.GOOS == "windows" {
			// CRLF --> CR to match Unix behavior
			content = bytes.Replace(content, []byte{'\x0d', '\x0a'}, []byte{'\x0a'}, -1)
		}
		assert.Check(t, is.Equal(result.AST.Dump()+"\n", string(content)), "In "+dockerfile)
	}
}

func TestParseWords(t *testing.T) {
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
		words := parseWords(test["input"][0], NewDefaultDirective())
		assert.Check(t, is.DeepEqual(test["expect"], words))
	}
}

func TestParseIncludesLineNumbers(t *testing.T) {
	df, err := os.Open(testFileLineInfo)
	assert.NilError(t, err)
	defer df.Close()

	result, err := Parse(df)
	assert.NilError(t, err)

	ast := result.AST
	assert.Check(t, is.Equal(5, ast.StartLine))
	assert.Check(t, is.Equal(31, ast.endLine))
	assert.Check(t, is.Len(ast.Children, 3))
	expected := [][]int{
		{5, 5},
		{11, 12},
		{17, 31},
	}
	for i, child := range ast.Children {
		msg := fmt.Sprintf("Child %d", i)
		assert.Check(t, is.DeepEqual(expected[i], []int{child.StartLine, child.endLine}), msg)
	}
}

func TestParseWarnsOnEmptyContinutationLine(t *testing.T) {
	dockerfile := bytes.NewBufferString(`
FROM alpine:3.6

RUN something \

    following \

    more

RUN another \

    thing
RUN non-indented \
# this is a comment
   after-comment

RUN indented \
    # this is an indented comment
    comment
	`)

	result, err := Parse(dockerfile)
	assert.NilError(t, err)
	warnings := result.Warnings
	assert.Check(t, is.Len(warnings, 3))
	assert.Check(t, is.Contains(warnings[0], "Empty continuation line found in"))
	assert.Check(t, is.Contains(warnings[0], "RUN something     following     more"))
	assert.Check(t, is.Contains(warnings[1], "RUN another     thing"))
	assert.Check(t, is.Contains(warnings[2], "will become errors in a future release"))
}

func TestParseReturnsScannerErrors(t *testing.T) {
	label := strings.Repeat("a", bufio.MaxScanTokenSize)

	dockerfile := strings.NewReader(fmt.Sprintf(`
		FROM image
		LABEL test=%s
`, label))
	_, err := Parse(dockerfile)
	assert.Check(t, is.Error(err, "dockerfile line greater than max allowed size of 65535"))
}
