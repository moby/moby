package parser

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const testDir = "testfiles"
const negativeTestDir = "testfiles-negative"

func getDirs(t *testing.T, dir string) []os.FileInfo {
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	dirs, err := f.Readdir(0)
	if err != nil {
		t.Fatal(err)
	}

	return dirs
}

func TestTestNegative(t *testing.T) {
	for _, dir := range getDirs(t, negativeTestDir) {
		dockerfile := filepath.Join(negativeTestDir, dir.Name(), "Dockerfile")

		df, err := os.Open(dockerfile)
		if err != nil {
			t.Fatalf("Dockerfile missing for %s: %s", dir.Name(), err.Error())
		}

		_, err = Parse(df)
		if err == nil {
			t.Fatalf("No error parsing broken dockerfile for %s", dir.Name())
		}

		df.Close()
	}
}

func TestTestData(t *testing.T) {
	for _, dir := range getDirs(t, testDir) {
		dockerfile := filepath.Join(testDir, dir.Name(), "Dockerfile")
		resultfile := filepath.Join(testDir, dir.Name(), "result")

		df, err := os.Open(dockerfile)
		if err != nil {
			t.Fatalf("Dockerfile missing for %s: %s", dir.Name(), err.Error())
		}

		rf, err := os.Open(resultfile)
		if err != nil {
			t.Fatalf("Result file missing for %s: %s", dir.Name(), err.Error())
		}

		ast, err := Parse(df)
		if err != nil {
			t.Fatalf("Error parsing %s's dockerfile: %s", dir.Name(), err.Error())
		}

		content, err := ioutil.ReadAll(rf)
		if err != nil {
			t.Fatalf("Error reading %s's result file: %s", dir.Name(), err.Error())
		}

		if ast.Dump()+"\n" != string(content) {
			fmt.Fprintln(os.Stderr, "Result:\n"+ast.Dump())
			fmt.Fprintln(os.Stderr, "Expected:\n"+string(content))
			t.Fatalf("%s: AST dump of dockerfile does not match result", dir.Name())
		}

		df.Close()
		rf.Close()
	}
}
