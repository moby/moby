package parser

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const testDir = "testfiles"

func TestTestData(t *testing.T) {
	f, err := os.Open(testDir)
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	dirs, err := f.Readdir(0)
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range dirs {
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

		if ast.Dump() != string(content) {
			t.Fatalf("%s: AST dump of dockerfile does not match result", dir.Name())
		}

		df.Close()
		rf.Close()
	}
}
