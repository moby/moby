package symlink

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func abs(t *testing.T, p string) string {
	o, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestFollowSymLinkNormal(t *testing.T) {
	link := "testdata/fs/a/d/c/data"

	rewrite, err := FollowSymlinkInScope(link, "testdata")
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs(t, "testdata/b/c/data"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}
}

func TestFollowSymLinkRelativePath(t *testing.T) {
	link := "testdata/fs/i"

	rewrite, err := FollowSymlinkInScope(link, "testdata")
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs(t, "testdata/fs/a"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}
}

func TestFollowSymLinkUnderLinkedDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "docker-fs-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	os.Mkdir(filepath.Join(dir, "realdir"), 0700)
	os.Symlink("realdir", filepath.Join(dir, "linkdir"))

	linkDir := filepath.Join(dir, "linkdir", "foo")
	dirUnderLinkDir := filepath.Join(dir, "linkdir", "foo", "bar")
	os.MkdirAll(dirUnderLinkDir, 0700)

	rewrite, err := FollowSymlinkInScope(dirUnderLinkDir, linkDir)
	if err != nil {
		t.Fatal(err)
	}

	if rewrite != dirUnderLinkDir {
		t.Fatalf("Expected %s got %s", dirUnderLinkDir, rewrite)
	}
}

func TestFollowSymLinkRandomString(t *testing.T) {
	if _, err := FollowSymlinkInScope("toto", "testdata"); err == nil {
		t.Fatal("Random string should fail but didn't")
	}
}

func TestFollowSymLinkLastLink(t *testing.T) {
	link := "testdata/fs/a/d"

	rewrite, err := FollowSymlinkInScope(link, "testdata")
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs(t, "testdata/b"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}
}

func TestFollowSymLinkRelativeLink(t *testing.T) {
	link := "testdata/fs/a/e/c/data"

	rewrite, err := FollowSymlinkInScope(link, "testdata")
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs(t, "testdata/fs/b/c/data"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}
}

func TestFollowSymLinkRelativeLinkScope(t *testing.T) {
	// avoid letting symlink f lead us out of the "testdata" scope
	// we don't normalize because symlink f is in scope and there is no
	// information leak
	{
		link := "testdata/fs/a/f"

		rewrite, err := FollowSymlinkInScope(link, "testdata")
		if err != nil {
			t.Fatal(err)
		}

		if expected := abs(t, "testdata/test"); expected != rewrite {
			t.Fatalf("Expected %s got %s", expected, rewrite)
		}
	}

	// avoid letting symlink f lead us out of the "testdata/fs" scope
	// we don't normalize because symlink f is in scope and there is no
	// information leak
	{
		link := "testdata/fs/a/f"

		rewrite, err := FollowSymlinkInScope(link, "testdata/fs")
		if err != nil {
			t.Fatal(err)
		}

		if expected := abs(t, "testdata/fs/test"); expected != rewrite {
			t.Fatalf("Expected %s got %s", expected, rewrite)
		}
	}

	// avoid letting symlink g (pointed at by symlink h) take out of scope
	// TODO: we should probably normalize to scope here because ../[....]/root
	// is out of scope and we leak information
	{
		link := "testdata/fs/b/h"

		rewrite, err := FollowSymlinkInScope(link, "testdata")
		if err != nil {
			t.Fatal(err)
		}

		if expected := abs(t, "testdata/root"); expected != rewrite {
			t.Fatalf("Expected %s got %s", expected, rewrite)
		}
	}

	// avoid letting allowing symlink e lead us to ../b
	// normalize to the "testdata/fs/a"
	{
		link := "testdata/fs/a/e"

		rewrite, err := FollowSymlinkInScope(link, "testdata/fs/a")
		if err != nil {
			t.Fatal(err)
		}

		if expected := abs(t, "testdata/fs/a"); expected != rewrite {
			t.Fatalf("Expected %s got %s", expected, rewrite)
		}
	}

	// avoid letting symlink -> ../directory/file escape from scope
	// normalize to "testdata/fs/j"
	{
		link := "testdata/fs/j/k"

		rewrite, err := FollowSymlinkInScope(link, "testdata/fs/j")
		if err != nil {
			t.Fatal(err)
		}

		if expected := abs(t, "testdata/fs/j"); expected != rewrite {
			t.Fatalf("Expected %s got %s", expected, rewrite)
		}
	}

	// make sure we don't allow escaping to /
	// normalize to dir
	{
		dir, err := ioutil.TempDir("", "docker-fs-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		linkFile := filepath.Join(dir, "foo")
		os.Mkdir(filepath.Join(dir, ""), 0700)
		os.Symlink("/", linkFile)

		rewrite, err := FollowSymlinkInScope(linkFile, dir)
		if err != nil {
			t.Fatal(err)
		}

		if rewrite != dir {
			t.Fatalf("Expected %s got %s", dir, rewrite)
		}
	}

	// make sure we don't allow escaping to /
	// normalize to dir
	{
		dir, err := ioutil.TempDir("", "docker-fs-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		linkFile := filepath.Join(dir, "foo")
		os.Mkdir(filepath.Join(dir, ""), 0700)
		os.Symlink("/../../", linkFile)

		rewrite, err := FollowSymlinkInScope(linkFile, dir)
		if err != nil {
			t.Fatal(err)
		}

		if rewrite != dir {
			t.Fatalf("Expected %s got %s", dir, rewrite)
		}
	}

	// make sure we stay in scope without leaking information
	// this also checks for escaping to /
	// normalize to dir
	{
		dir, err := ioutil.TempDir("", "docker-fs-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		linkFile := filepath.Join(dir, "foo")
		os.Mkdir(filepath.Join(dir, ""), 0700)
		os.Symlink("../../", linkFile)

		rewrite, err := FollowSymlinkInScope(linkFile, dir)
		if err != nil {
			t.Fatal(err)
		}

		if rewrite != dir {
			t.Fatalf("Expected %s got %s", dir, rewrite)
		}
	}
}
