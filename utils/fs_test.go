package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func abs(p string) string {
	o, err := filepath.Abs(p)
	if err != nil {
		panic(err)
	}
	return o
}

func TestFollowSymLinkNormal(t *testing.T) {
	link := "testdata/fs/a/d/c/data"

	rewrite, err := FollowSymlink(link, "test")
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs("test/b/c/data"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}
}

func TestFollowSymLinkRandomString(t *testing.T) {
	rewrite, err := FollowSymlink("toto", "test")
	if err != nil {
		t.Fatal(err)
	}

	if rewrite != "toto" {
		t.Fatalf("Expected toto got %s", rewrite)
	}
}

func TestFollowSymLinkLastLink(t *testing.T) {
	link := "testdata/fs/a/d"

	rewrite, err := FollowSymlink(link, "test")
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs("test/b"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}
}

func TestFollowSymLinkRelativeLink(t *testing.T) {
	link := "testdata/fs/a/e/c/data"

	rewrite, err := FollowSymlink(link, "test")
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs("testdata/fs/a/e/c/data"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}
}

func TestFollowSymLinkRelativeLinkScope(t *testing.T) {
	link := "testdata/fs/a/f"
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(pwd, "testdata")

	rewrite, err := FollowSymlink(link, root)
	if err != nil {
		t.Fatal(err)
	}

	if expected := abs("testdata/test"); expected != rewrite {
		t.Fatalf("Expected %s got %s", expected, rewrite)
	}
}
