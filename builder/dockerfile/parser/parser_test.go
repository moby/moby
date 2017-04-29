package parser

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDir = "testfiles"
const negativeTestDir = "testfiles-negative"
const testFileLineInfo = "testfile-line/Dockerfile"

func getDirs(t *testing.T, dir string) []string {
	f, err := os.Open(dir)
	require.NoError(t, err)
	defer f.Close()

	dirs, err := f.Readdirnames(0)
	require.NoError(t, err)
	return dirs
}

func TestTestNegative(t *testing.T) {
	for _, dir := range getDirs(t, negativeTestDir) {
		dockerfile := filepath.Join(negativeTestDir, dir, "Dockerfile")

		df, err := os.Open(dockerfile)
		require.NoError(t, err)
		defer df.Close()

		_, err = Parse(df)
		assert.Error(t, err)
	}
}

func TestTestData(t *testing.T) {
	for _, dir := range getDirs(t, testDir) {
		dockerfile := filepath.Join(testDir, dir, "Dockerfile")
		resultfile := filepath.Join(testDir, dir, "result")

		df, err := os.Open(dockerfile)
		require.NoError(t, err)
		defer df.Close()

		result, err := Parse(df)
		require.NoError(t, err)

		content, err := ioutil.ReadFile(resultfile)
		require.NoError(t, err)

		if runtime.GOOS == "windows" {
			// CRLF --> CR to match Unix behavior
			content = bytes.Replace(content, []byte{'\x0d', '\x0a'}, []byte{'\x0a'}, -1)
		}

		assert.Contains(t, result.AST.Dump()+"\n", string(content), "In "+dockerfile)
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
		assert.Equal(t, test["expect"], words)
	}
}

func TestLineInformation(t *testing.T) {
	df, err := os.Open(testFileLineInfo)
	require.NoError(t, err)
	defer df.Close()

	result, err := Parse(df)
	require.NoError(t, err)

	ast := result.AST
	if ast.StartLine != 5 || ast.endLine != 31 {
		fmt.Fprintf(os.Stderr, "Wrong root line information: expected(%d-%d), actual(%d-%d)\n", 5, 31, ast.StartLine, ast.endLine)
		t.Fatal("Root line information doesn't match result.")
	}
	assert.Len(t, ast.Children, 3)
	expected := [][]int{
		{5, 5},
		{11, 12},
		{17, 31},
	}
	for i, child := range ast.Children {
		if child.StartLine != expected[i][0] || child.endLine != expected[i][1] {
			t.Logf("Wrong line information for child %d: expected(%d-%d), actual(%d-%d)\n",
				i, expected[i][0], expected[i][1], child.StartLine, child.endLine)
			t.Fatal("Root line information doesn't match result.")
		}
	}
}
