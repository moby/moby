// +build !windows

package archive

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"syscall"
	"testing"
)

func TestCanonicalTarNameForPath(t *testing.T) {
	cases := []struct{ in, expected string }{
		{"foo", "foo"},
		{"foo/bar", "foo/bar"},
		{"foo/dir/", "foo/dir/"},
	}
	for _, v := range cases {
		if out, err := CanonicalTarNameForPath(v.in); err != nil {
			t.Fatalf("cannot get canonical name for path: %s: %v", v.in, err)
		} else if out != v.expected {
			t.Fatalf("wrong canonical tar name. expected:%s got:%s", v.expected, out)
		}
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
		{"foo/bar", false, "foo/bar"},
		{"foo/bar", true, "foo/bar/"},
	}
	for _, v := range cases {
		if out, err := canonicalTarName(v.in, v.isDir); err != nil {
			t.Fatalf("cannot get canonical name for path: %s: %v", v.in, err)
		} else if out != v.expected {
			t.Fatalf("wrong canonical tar name. expected:%s got:%s", v.expected, out)
		}
	}
}

func TestChmodTarEntry(t *testing.T) {
	cases := []struct {
		in, expected os.FileMode
	}{
		{0000, 0000},
		{0777, 0777},
		{0644, 0644},
		{0755, 0755},
		{0444, 0444},
	}
	for _, v := range cases {
		if out := chmodTarEntry(v.in); out != v.expected {
			t.Fatalf("wrong chmod. expected:%v got:%v", v.expected, out)
		}
	}
}

func TestTarWithHardLink(t *testing.T) {
	origin, err := ioutil.TempDir("", "docker-test-tar-hardlink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(origin)
	if err := ioutil.WriteFile(path.Join(origin, "1"), []byte("hello world"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path.Join(origin, "1"), path.Join(origin, "2")); err != nil {
		t.Fatal(err)
	}

	var i1, i2 uint64
	if i1, err = getNlink(path.Join(origin, "1")); err != nil {
		t.Fatal(err)
	}
	// sanity check that we can hardlink
	if i1 != 2 {
		t.Skipf("skipping since hardlinks don't work here; expected 2 links, got %d", i1)
	}

	dest, err := ioutil.TempDir("", "docker-test-tar-hardlink-dest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dest)

	// we'll do this in two steps to separate failure
	fh, err := Tar(origin, Uncompressed)
	if err != nil {
		t.Fatal(err)
	}

	// ensure we can read the whole thing with no error, before writing back out
	buf, err := ioutil.ReadAll(fh)
	if err != nil {
		t.Fatal(err)
	}

	bRdr := bytes.NewReader(buf)
	err = Untar(bRdr, dest, &TarOptions{Compression: Uncompressed})
	if err != nil {
		t.Fatal(err)
	}

	if i1, err = getInode(path.Join(dest, "1")); err != nil {
		t.Fatal(err)
	}
	if i2, err = getInode(path.Join(dest, "2")); err != nil {
		t.Fatal(err)
	}

	if i1 != i2 {
		t.Errorf("expected matching inodes, but got %d and %d", i1, i2)
	}
}

func getNlink(path string) (uint64, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	statT, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("expected type *syscall.Stat_t, got %t", stat.Sys())
	}
	// We need this conversion on ARM64
	return uint64(statT.Nlink), nil
}

func getInode(path string) (uint64, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	statT, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("expected type *syscall.Stat_t, got %t", stat.Sys())
	}
	return statT.Ino, nil
}
