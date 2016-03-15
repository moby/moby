package precompiledregexp

import (
	"fmt"
	"github.com/docker/docker/pkg/fileutils"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type testInstance struct {
	pattern             []string
	negative            bool
	shouldErrorOnCreate bool
	shouldErrorOnMatch  bool
	shouldMatchStrings  map[string]bool
}

var testStrings = []string{
	"",
	".",
	"a",
	"b",
	"c",
	"a/b",
	"a/c",
	"a/b/c",
	"/a",
	"/b",
	"/c",
	"/a/b",
	"/a/c",
	"/a/b/c",
	"precompiledregexp.go",
	"precompiledregexp_test.go",
	"builder/builder.go",
	"docs/README.md",
	"docs/DOCUMENTATION.md",
}

func TestPrecompiledRegExp(t *testing.T) {

	tests := []testInstance{
		{
			pattern:             []string{""},
			shouldErrorOnCreate: true,
		},
		{
			pattern: []string{},
		},
		{
			pattern:            []string{"!"},
			shouldErrorOnMatch: true,
		},
		{
			pattern:             []string{"."},
			shouldErrorOnCreate: true,
		},
		{
			pattern:             []string{"["},
			shouldErrorOnCreate: true,
		},
		{
			pattern:            []string{"!."},
			shouldErrorOnMatch: true,
		},
		{
			pattern: []string{"a"},
			shouldMatchStrings: map[string]bool{
				"a":     true,
				"a/b":   true,
				"a/c":   true,
				"a/b/c": true,
			},
		},
		{
			pattern:            []string{"!a"},
			shouldMatchStrings: map[string]bool{},
		},
		{
			pattern: []string{"/a"},
			shouldMatchStrings: map[string]bool{
				"/a":     true,
				"/a/b":   true,
				"/a/c":   true,
				"/a/b/c": true,
			},
		},
		{
			pattern:            []string{"!/a"},
			shouldMatchStrings: map[string]bool{},
		},
		{
			pattern: []string{"./a"},
			shouldMatchStrings: map[string]bool{
				"a":     true,
				"a/b":   true,
				"a/c":   true,
				"a/b/c": true,
			},
		},
		{
			pattern:            []string{"!./a"},
			shouldMatchStrings: map[string]bool{},
		},
		{
			pattern:             []string{"../a"},
			shouldErrorOnCreate: true,
		},
		{
			pattern:             []string{"!../a"},
			shouldErrorOnCreate: true,
		},
		{
			pattern: []string{"/../a"},
			shouldMatchStrings: map[string]bool{
				"/a":     true,
				"/a/b":   true,
				"/a/c":   true,
				"/a/b/c": true,
			},
		},
		{
			pattern:            []string{"!/../a"},
			shouldMatchStrings: map[string]bool{},
		},
		{
			pattern: []string{"/a/b/c"},
			shouldMatchStrings: map[string]bool{
				"/a/b/c": true,
			},
		},
		{
			pattern:            []string{"!/a/b/c"},
			shouldMatchStrings: map[string]bool{},
		},
		{
			pattern: []string{"/a/c"},
			shouldMatchStrings: map[string]bool{
				"/a/c": true,
			},
		},
		{
			pattern:            []string{"!/a/c"},
			shouldMatchStrings: map[string]bool{},
		},
		{
			pattern: []string{"*"},
			shouldMatchStrings: map[string]bool{
				"":                          true,
				".":                         true,
				"a":                         true,
				"b":                         true,
				"c":                         true,
				"a/b":                       true,
				"a/c":                       true,
				"a/b/c":                     true,
				"/a":                        true,
				"/b":                        true,
				"/c":                        true,
				"/a/b":                      true,
				"/a/c":                      true,
				"/a/b/c":                    true,
				"precompiledregexp.go":      true,
				"precompiledregexp_test.go": true,
				"builder/builder.go":        true,
				"docs/README.md":            true,
				"docs/DOCUMENTATION.md":     true,
			},
		},
		{
			pattern: []string{"**/c"},
			shouldMatchStrings: map[string]bool{
				"c":      true,
				"a/c":    true,
				"a/b/c":  true,
				"/c":     true,
				"/a/c":   true,
				"/a/b/c": true,
			},
		},
		{
			pattern: []string{"*.go"},
			shouldMatchStrings: map[string]bool{
				"precompiledregexp.go":      true,
				"precompiledregexp_test.go": true,
			},
		},
		{
			pattern: []string{
				"!precompiledregexp.go",
				"**/*.go",
			},
			shouldMatchStrings: map[string]bool{
				"precompiledregexp.go":      true,
				"precompiledregexp_test.go": true,
				"builder/builder.go":        true,
			},
		},
		{
			pattern: []string{
				"**/*.go",
				"!precompiledregexp.go",
			},
			shouldMatchStrings: map[string]bool{
				"precompiledregexp_test.go": true,
				"builder/builder.go":        true,
			},
		},
		{
			pattern: []string{
				"docs/",
				"!docs/README.md",
			},
			shouldMatchStrings: map[string]bool{
				"docs/DOCUMENTATION.md": true,
			},
		},
	}

	for _, entry := range tests {
		runTestInstance(t, entry, entry.negative)
	}
}

func runTestInstance(t *testing.T, entry testInstance, isNegative bool) {
	precompiledRegExprs, err := FromStringExpressions(entry.pattern)

	if err != nil {
		if !entry.shouldErrorOnCreate {
			t.Fatalf("Did not expect matching to fail pattern: [%s]", entry.pattern)
		} else {
			return
		}
	} else if entry.shouldErrorOnCreate && err == nil {
		t.Fatalf("Expected creation to fail for pattern: [%s]", entry.pattern)
	}

	stringExprs := ToStringExpressions(precompiledRegExprs)
	if len(entry.pattern) != len(stringExprs) {
		t.Fatalf("Incorrect number of string expressions found: %d; expected %d for pattern [%s]",
			len(entry.pattern),
			len(ToStringExpressions(precompiledRegExprs)),
			entry.pattern)
	}

	_, err = Matches("", precompiledRegExprs)
	if err != nil {
		if !entry.shouldErrorOnMatch {
			t.Fatalf("Did not expect matching to fail pattern: [%s]", entry.pattern)
		} else {
			return
		}
	} else if entry.shouldErrorOnMatch && err == nil {
		t.Fatalf("Expected matching to fail for pattern: [%s]", entry.pattern)
	}

	// Test matches using slice of precompiled regular expressions
	for _, str := range testStrings {
		matches, err := Matches(str, precompiledRegExprs)
		if err != nil {
			t.Fatalf("Encountered error when matching string [%s] using pattern [%s]",
				str, entry.pattern)
		}
		if matches != entry.shouldMatchStrings[str] {
			fmt.Printf("[%t]", entry.shouldMatchStrings[str])
			t.Fatalf("Pattern [%s] did not match string [%s]",
				entry.pattern, str)
		}
	}

	// Test matches on each individual regular expression
	for _, str := range testStrings {
		matches := false
		for _, regExpr := range precompiledRegExprs {
			internalMatch, err := regExpr.internalMatches(str)
			if err != nil {
				t.Fatalf("Encountered error when matching string [%s] using pattern [%s]",
					str, entry.pattern)
			}
			match, err := regExpr.Matches(str)
			if err != nil {
				t.Fatalf("Encountered error when matching string [%s] using pattern [%s]",
					str, entry.pattern)
			}

			if internalMatch {
				matches = !regExpr.Negative()
				if !regExpr.Negative() != match {
					t.Fatalf("internalMatch and Match functions did not agree for pattern [%s]",
						entry.pattern)
				}
			}
		}
		if matches != entry.shouldMatchStrings[str] {
			fmt.Printf("[%t]", entry.shouldMatchStrings[str])
			t.Fatalf("Pattern [%s] did not match string [%s]",
				entry.pattern, str)
		}
	}
}

func TestWorksSameAsFileUtils(t *testing.T) {
	/*
		Test Directory Structure:
		> root
		>> v.cc
		>> bla
		>>> .gitignore
		>>> README.md
		>>> foo
		>>> Makefile
		>>> .git
		>>> src
		>>>> x.go
		>>>> _vendor
		>>> dir
		>>>> foo
		>> src
		>>> v.cc
		>>> _vendor
		>>>> v.cc


	*/
	//tests := []string{
	//	"v.cc",
	//	"/bla/.gitignore",
	//	"/bla/README.md",
	//	"/bla/foo",
	//	"/bla/Makefile",
	//	"/bla/.git",
	//	"/bla/src/x.go",
	//	"/bla/src/_vendor",
	//	"/bla/dir/foo.x",
	//	"src/v.cc",
	//	"src/_vendor/v.cc",
	//}

	patterns := []string{
		".git",
		"pkg",
		".gitignore",
		"src/_vendor",
		"*.md",
		"**/*.cc",
		"!src/_vendor/v.cc",
		"dir",
	}

	baseDir := createTmpDir(t, "", "precompiledregexp_test")
	createDir(t, filepath.Join(baseDir, "bla"))
	createDir(t, filepath.Join(baseDir, "bla/foo"))
	createDir(t, filepath.Join(baseDir, "bla/src/_vendor"))
	createDir(t, filepath.Join(baseDir, "/bla/dir/foo"))
	createDir(t, filepath.Join(baseDir, "bla/dir"))
	createDir(t, filepath.Join(baseDir, "src/_vendor"))
	createFile(t, filepath.Join(baseDir, "v.cc"))
	createFile(t, filepath.Join(baseDir, "/bla/.gitignore"))
	createFile(t, filepath.Join(baseDir, "/bla/README.md"))
	createFile(t, filepath.Join(baseDir, "/bla/Makefile"))
	createFile(t, filepath.Join(baseDir, "/bla/.git"))
	createFile(t, filepath.Join(baseDir, "/bla/src/x.go"))
	createFile(t, filepath.Join(baseDir, "/bla/src/_vendor"))
	createFile(t, filepath.Join(baseDir, "src/v.cc"))
	createFile(t, filepath.Join(baseDir, "src/_vendor/v.cc"))

	precompiledRegExprs := make([]PrecompiledRegExp, 0, len(patterns))
	for _, pattern := range patterns {
		precompiledRegExprs = appendNewRegExpr(t, pattern, precompiledRegExprs)
	}

	recursiveWalk(t, baseDir, baseDir, patterns, precompiledRegExprs)

}

func recursiveWalk(t *testing.T, baseDir, targetDir string, patterns []string, regexps []PrecompiledRegExp) {
	files, _ := ioutil.ReadDir(targetDir)
	for _, f := range files {
		// Calculate subDir path
		subDir := filepath.Join(targetDir, f.Name())
		// Calculate relative path
		relSubDir, err := filepath.Rel(baseDir, subDir)
		if err != nil {
			t.Fatal(err)
		}

		stringMatched, err := fileutils.Matches(relSubDir, patterns)
		if err != nil {
			t.Fatal(err)
		}
		regexMatched, err := Matches(relSubDir, regexps)
		if err != nil {
			t.Fatal(err)
		}

		if stringMatched != regexMatched {
			t.Fatalf("Failed on %s; string: %t regex: %t", relSubDir, stringMatched, regexMatched)
		}

		if f.IsDir() {
			recursiveWalk(t, baseDir, subDir, patterns, regexps)
		}
	}
}

func appendNewRegExpr(t *testing.T, pattern string, list []PrecompiledRegExp) []PrecompiledRegExp {

	isNeg := false
	if pattern[0] == '!' {
		pattern = pattern[1:]
		isNeg = true
	}
	new, err := NewPreCompiledRegExp(pattern, isNeg)
	if err != nil {
		t.Fatal(err)
	}
	return append(list, *new)
}

func createTmpDir(t *testing.T, dir string, prefix string) string {

	result, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func createDir(t *testing.T, dir string) {

	os.MkdirAll(dir, 0777)
}

func createFile(t *testing.T, filename string) {
	os.MkdirAll(filepath.Dir(filename), 0777)
	os.Create(filename)
}
