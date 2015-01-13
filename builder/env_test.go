package builder

import (
	"encoding/json"
	"fmt"
	"github.com/docker/docker/builder/parser"
	"github.com/docker/docker/runconfig"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const testDir = "envTestFiles"

var testingEnv = func(b *Builder, args []string, attributes map[string]bool, original string) error {
	return nil
}

type stringArray []string

func init() {
	evaluateTable = map[string]func(*Builder, []string, map[string]bool, string) error{
		"env": testingEnv,
	}
}

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

func TestEnv(t *testing.T) {
	b := &Builder{OutStream: ioutil.Discard}
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

		ast, err := parser.Parse(df)
		if err != nil {
			t.Fatalf("Error parsing dockerfile for %s", dir.Name())
		}

		b.Config = &runconfig.Config{}
		for i, n := range ast.Children {
			err = b.dispatch(i, n)
			if err != nil {
				t.Fatalf("Evaluating dockerfile for %s failed with: %s", dir.Name(), err)
			}
		}

		env, err := json.Marshal(b.Config.Env)
		if err != nil {
			t.Fatal("Cannot serialise Env array")
		}

		content, err := ioutil.ReadAll(rf)
		if err != nil {
			t.Fatalf("Error reading %s's result file: %s", dir.Name(), err.Error())
		}

		if string(content) != string(env)+"\n" {
			fmt.Fprintf(os.Stderr, "Result: %s\n", env)
			fmt.Fprintf(os.Stderr, "Expected: %s\n", content)
			t.Fatalf("%s: Evaluated Env of Dockerfile does not match result", dir.Name())
		}

		df.Close()
	}
}
