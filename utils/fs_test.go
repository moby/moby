package utils

import (
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
	link := "testdata/fs/a/f"

	rewrite, err := FollowSymlinkInScope(link, "testdata")
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs(t, "testdata/test"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}

	link = "testdata/fs/b/h"

	rewrite, err = FollowSymlinkInScope(link, "testdata")
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs(t, "testdata/root"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}
}
