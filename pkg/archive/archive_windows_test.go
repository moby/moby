//go:build windows
// +build windows

package archive // import "github.com/docker/docker/pkg/archive"

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileWithInvalidDest(t *testing.T) {
	// TODO Windows: This is currently failing. Not sure what has
	// recently changed in CopyWithTar as used to pass. Further investigation
	// is required.
	t.Skip("Currently fails")
	folder, err := os.MkdirTemp("", "docker-archive-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(folder)
	dest := "c:dest"
	srcFolder := filepath.Join(folder, "src")
	src := filepath.Join(folder, "src", "src")
	err = os.MkdirAll(srcFolder, 0740)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(src, []byte("content"), 0777)
	err = defaultCopyWithTar(src, dest)
	if err == nil {
		t.Fatalf("archiver.CopyWithTar should throw an error on invalid dest.")
	}
}

func TestCanonicalTarName(t *testing.T) {
	cases := []struct {
		in       string
		isDir    bool
		expected string
	}{
		{"foo", false, "foo"},
		{"foo", true, "foo/"},
		{`foo\bar`, false, "foo/bar"},
		{`foo\bar`, true, "foo/bar/"},
	}
	for _, v := range cases {
		if canonicalTarName(v.in, v.isDir) != v.expected {
			t.Fatalf("wrong canonical tar name. expected:%s got:%s", v.expected, canonicalTarName(v.in, v.isDir))
		}
	}
}

func TestChmodTarEntry(t *testing.T) {
	cases := []struct {
		in, expected os.FileMode
	}{
		{0000, 0111},
		{0777, 0755},
		{0644, 0755},
		{0755, 0755},
		{0444, 0555},
		{0755 | os.ModeDir, 0755 | os.ModeDir},
		{0755 | os.ModeSymlink, 0755 | os.ModeSymlink},
	}
	for _, v := range cases {
		if out := chmodTarEntry(v.in); out != v.expected {
			t.Fatalf("wrong chmod. expected:%v got:%v", v.expected, out)
		}
	}
}
