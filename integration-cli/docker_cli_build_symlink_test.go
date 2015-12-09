package main

import (
	"archive/tar"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildSymlinkBreakout(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildsymlinkbreakout"
	tmpdir, err := ioutil.TempDir("", name)
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpdir)
	ctx := filepath.Join(tmpdir, "context")
	if err := os.MkdirAll(ctx, 0755); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(ctx, "Dockerfile"), []byte(`
	from busybox
	add symlink.tar /
	add inject /symlink/
	`), 0644); err != nil {
		c.Fatal(err)
	}
	inject := filepath.Join(ctx, "inject")
	if err := ioutil.WriteFile(inject, nil, 0644); err != nil {
		c.Fatal(err)
	}
	f, err := os.Create(filepath.Join(ctx, "symlink.tar"))
	if err != nil {
		c.Fatal(err)
	}
	w := tar.NewWriter(f)
	w.WriteHeader(&tar.Header{
		Name:     "symlink2",
		Typeflag: tar.TypeSymlink,
		Linkname: "/../../../../../../../../../../../../../../",
		Uid:      os.Getuid(),
		Gid:      os.Getgid(),
	})
	w.WriteHeader(&tar.Header{
		Name:     "symlink",
		Typeflag: tar.TypeSymlink,
		Linkname: filepath.Join("symlink2", tmpdir),
		Uid:      os.Getuid(),
		Gid:      os.Getgid(),
	})
	w.Close()
	f.Close()
	if _, err := buildImageFromContext(name, fakeContextFromDir(ctx), false); err != nil {
		c.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(tmpdir, "inject")); err == nil {
		c.Fatal("symlink breakout - inject")
	} else if !os.IsNotExist(err) {
		c.Fatalf("unexpected error: %v", err)
	}
}

// #17290
func (s *DockerSuite) TestBuildCacheBrokenSymlink(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildbrokensymlink"
	ctx, err := fakeContext(`
	FROM busybox
	COPY . ./`,
		map[string]string{
			"foo": "bar",
		})
	c.Assert(err, checker.IsNil)
	defer ctx.Close()

	err = os.Symlink(filepath.Join(ctx.Dir, "nosuchfile"), filepath.Join(ctx.Dir, "asymlink"))
	c.Assert(err, checker.IsNil)

	// warm up cache
	_, err = buildImageFromContext(name, ctx, true)
	c.Assert(err, checker.IsNil)

	// add new file to context, should invalidate cache
	err = ioutil.WriteFile(filepath.Join(ctx.Dir, "newfile"), []byte("foo"), 0644)
	c.Assert(err, checker.IsNil)

	_, out, err := buildImageFromContextWithOut(name, ctx, true)
	c.Assert(err, checker.IsNil)

	c.Assert(out, checker.Not(checker.Contains), "Using cache")

}

func (s *DockerSuite) TestBuildFollowSymlinkToFile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildbrokensymlink"
	ctx, err := fakeContext(`
	FROM busybox
	COPY asymlink target`,
		map[string]string{
			"foo": "bar",
		})
	c.Assert(err, checker.IsNil)
	defer ctx.Close()

	err = os.Symlink("foo", filepath.Join(ctx.Dir, "asymlink"))
	c.Assert(err, checker.IsNil)

	id, err := buildImageFromContext(name, ctx, true)
	c.Assert(err, checker.IsNil)

	out, _ := dockerCmd(c, "run", "--rm", id, "cat", "target")
	c.Assert(out, checker.Matches, "bar")

	// change target file should invalidate cache
	err = ioutil.WriteFile(filepath.Join(ctx.Dir, "foo"), []byte("baz"), 0644)
	c.Assert(err, checker.IsNil)

	id, out, err = buildImageFromContextWithOut(name, ctx, true)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), "Using cache")

	out, _ = dockerCmd(c, "run", "--rm", id, "cat", "target")
	c.Assert(out, checker.Matches, "baz")
}

func (s *DockerSuite) TestBuildFollowSymlinkToDir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildbrokensymlink"
	ctx, err := fakeContext(`
	FROM busybox
	COPY asymlink /`,
		map[string]string{
			"foo/abc": "bar",
			"foo/def": "baz",
		})
	c.Assert(err, checker.IsNil)
	defer ctx.Close()

	err = os.Symlink("foo", filepath.Join(ctx.Dir, "asymlink"))
	c.Assert(err, checker.IsNil)

	id, err := buildImageFromContext(name, ctx, true)
	c.Assert(err, checker.IsNil)

	out, _ := dockerCmd(c, "run", "--rm", id, "cat", "abc", "def")
	c.Assert(out, checker.Matches, "barbaz")

	// change target file should invalidate cache
	err = ioutil.WriteFile(filepath.Join(ctx.Dir, "foo/def"), []byte("bax"), 0644)
	c.Assert(err, checker.IsNil)

	id, out, err = buildImageFromContextWithOut(name, ctx, true)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), "Using cache")

	out, _ = dockerCmd(c, "run", "--rm", id, "cat", "abc", "def")
	c.Assert(out, checker.Matches, "barbax")

}

// TestBuildSymlinkBasename tests that target file gets basename from symlink,
// not from the target file.
func (s *DockerSuite) TestBuildSymlinkBasename(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildbrokensymlink"
	ctx, err := fakeContext(`
	FROM busybox
	COPY asymlink /`,
		map[string]string{
			"foo": "bar",
		})
	c.Assert(err, checker.IsNil)
	defer ctx.Close()

	err = os.Symlink("foo", filepath.Join(ctx.Dir, "asymlink"))
	c.Assert(err, checker.IsNil)

	id, err := buildImageFromContext(name, ctx, true)
	c.Assert(err, checker.IsNil)

	out, _ := dockerCmd(c, "run", "--rm", id, "cat", "asymlink")
	c.Assert(out, checker.Matches, "bar")

}
