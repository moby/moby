package dockerignore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// collectWalker implements Walk and collects all the paths seen.
type collectWalker struct{ Paths []string }

func (wc *collectWalker) Walk(filePath string, fi os.FileInfo, err error) error {
	wc.Paths = append(wc.Paths, filePath)
	return err
}

type fakeFile struct {
	filePath string
	// use a nil pointer for everything else, since they should be unused.
	os.FileInfo
}

func (f fakeFile) IsDir() bool {
	return strings.HasSuffix(f.filePath, "/")
}

// mockWalk is a convenience function for walking a fake filesystem.
// Paths ending in `/` are considered to be directories.
func mockWalk(walkFn filepath.WalkFunc) func(string) error {
	return func(filePath string) error {
		return walkFn(filePath, &fakeFile{filePath, nil}, nil)
	}
}

func TestExcluderBasicExclusion(t *testing.T) {
	// One exclusion rule, ordinary file.
	e, err := NewExcluder(".", []string{
		"ignored",
	})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	collector := collectWalker{}
	walkFn := e.Wrap(collector.Walk)
	walk := mockWalk(walkFn)

	if err := walk("not-ignored"); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(collector.Paths) != 1 {
		t.Fatalf("Expected 1 walked, got: %v", len(collector.Paths))
	}

	if collector.Paths[0] != "not-ignored" {
		t.Fatalf(`Expected "not-ignored", got %v`, collector.Paths[0])
	}

	collector.Paths = nil // Reset for the next check

	if err := walk("ignored"); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	switch {
	case len(collector.Paths) != 0:
		t.Fatalf("Expected 0 walked, got: %v", len(collector.Paths))
	}
}

func TestExcluderSubdirectoryExclusion(t *testing.T) {
	// One exclusion rule.
	// It should apply only at the root.
	e, err := NewExcluder(".", []string{
		"foo",
	})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	collector := collectWalker{}
	walkFn := e.Wrap(collector.Walk)
	walk := mockWalk(walkFn)

	if err := walk("foo"); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if err := walk("directory/foo"); err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(collector.Paths) != 1 {
		t.Fatalf("Expected 1 walked, got: %q", collector.Paths)
	}

	switch {
	case collector.Paths[0] != "directory/foo":
		t.Fatalf(`Expected "directory/foo", got %v`, collector.Paths[0])
	}
}

func TestExcluderDirectoryExclusion(t *testing.T) {
	// One exclusion rule, directory.
	e, err := NewExcluder(".", []string{
		"ignored-directory/",
	})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	collector := collectWalker{}
	walkFn := e.Wrap(collector.Walk)
	walk := mockWalk(walkFn)

	if err := walk("not-ignored"); err != nil {
		t.Fatal(err)
	}
	if err := walk("ignored-directory/foo"); err != nil {
		t.Fatal(err)
	}
	if err := walk("ignored-directory/bar"); err != nil {
		t.Fatal(err)
	}

	switch {
	case len(collector.Paths) != 1:
		t.Fatalf("Expected 1 walked, got: %v", len(collector.Paths))
	case collector.Paths[0] != "not-ignored":
		t.Fatalf(`Expected "not-ignored", got %v`, collector.Paths[0])
	}

}

func TestExcluderNested(t *testing.T) {
	// One exclusion rule,
	// ignore a file named "ignored" anywhere.
	e, err := NewExcluder(".", []string{
		"**/ignored",
	})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	collector := collectWalker{}
	walkFn := e.Wrap(collector.Walk)
	walk := mockWalk(walkFn)

	if err := walk("not-ignored"); err != nil {
		t.Fatal(err)
	}
	if err := walk("directory/not-ignored"); err != nil {
		t.Fatal(err)
	}
	if err := walk("directory/directory/not-ignored"); err != nil {
		t.Fatal(err)
	}
	if err := walk("directory/ignored"); err != nil {
		t.Fatal(err)
	}
	if err := walk("directory/directory/ignored"); err != nil {
		t.Fatal(err)
	}

	switch {
	case len(collector.Paths) != 3:
		t.Fatalf("Expected 3 walked, got: %v", len(collector.Paths))
	case collector.Paths[0] != "not-ignored":
		t.Fatalf(`Expected "not-ignored", got %v`, collector.Paths[0])
	case collector.Paths[1] != "directory/not-ignored":
		t.Fatalf(`Expected "directory/not-ignored", got %v`, collector.Paths[1])
	case collector.Paths[2] != "directory/directory/not-ignored":
		t.Fatalf(`Expected "directory/directory/not-ignored", got %v`, collector.Paths[2])
	}
}

func TestExcluderIgnoredDirectoryNegation(t *testing.T) {
	// Two rules. One directory excluded, one file within included.
	e, err := NewExcluder(".", []string{
		"ignored-directory/",
		"!ignored-directory/not-ignored",
	})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	collector := collectWalker{}
	walkFn := e.Wrap(collector.Walk)
	walk := mockWalk(walkFn)

	if err := walk("not-ignored"); err != nil {
		t.Fatal(err)
	}
	if err := walk("ignored-directory/"); err != nil {
		t.Fatal(err)
	}
	if err := walk("ignored-directory/ignored"); err != nil {
		t.Fatal(err)
	}
	if err := walk("ignored-directory/not-ignored"); err != nil {
		t.Fatal(err)
	}

	switch {
	case len(collector.Paths) != 3:
		t.Fatalf("Expected 3 walked, got: %v: %v", len(collector.Paths), collector.Paths)
	case collector.Paths[0] != "not-ignored":
		t.Fatalf(`Expected "not-ignored", got %v`, collector.Paths[0])
	case collector.Paths[1] != "ignored-directory/":
		t.Fatalf(`Expected "ignored-directory/", got %v`, collector.Paths[1])
	case collector.Paths[2] != "ignored-directory/not-ignored":
		t.Fatalf(`Expected "not-ignored", got %v`, collector.Paths[2])
	}
}

func TestExcluderExtension(t *testing.T) {
	// Exclude files named *.binary found anywhere.
	e, err := NewExcluder(".", []string{
		"**/*.binary",
	})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	collector := collectWalker{}
	walkFn := e.Wrap(collector.Walk)
	walk := mockWalk(walkFn)

	// Only this should be visible.
	if err := walk("not-ignored"); err != nil {
		t.Fatal(err)
	}
	if err := walk("test.binary"); err != nil {
		t.Fatal(err)
	}
	if err := walk("test.binary/test"); err != nil {
		t.Fatal(err)
	}
	if err := walk("directory/test.binary"); err != nil {
		t.Fatal(err)
	}
	if err := walk("directory/directory/test.binary"); err != nil {
		t.Fatal(err)
	}

	switch {
	case len(collector.Paths) != 1:
		t.Fatalf("Expected 1 walked, got: %v: %v", len(collector.Paths), collector.Paths)
	case collector.Paths[0] != "not-ignored":
		t.Fatalf(`Expected "not-ignored", got %v`, collector.Paths[0])
	}
}

// Taken from
// https://github.com/docker/docker/blob/46a61b72/docs/reference/builder.md#dockerignore-file
func TestExcluderREADMESecretmd(t *testing.T) {
	// Two rules. One directory excluded, one file within included.
	e, err := NewExcluder(".", []string{
		"*.md",
		"!README*.md",
	})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	collector := collectWalker{}
	walkFn := e.Wrap(collector.Walk)
	walk := mockWalk(walkFn)

	if err := walk("boring.md"); err != nil {
		t.Fatal(err)
	}
	if err := walk("README.md"); err != nil {
		t.Fatal(err)
	}
	if err := walk("README-interesting.md"); err != nil {
		t.Fatal(err)
	}

	switch {
	case len(collector.Paths) != 2:
		t.Fatalf("Expected 2 walked, got: %v: %v", len(collector.Paths), collector.Paths)
	case collector.Paths[0] != "README.md":
		t.Fatalf(`Expected "README.md", got %v`, collector.Paths[0])
	case collector.Paths[1] != "README-interesting.md":
		t.Fatalf(`Expected "README-interesting.md", got %v`, collector.Paths[1])
	}
}
