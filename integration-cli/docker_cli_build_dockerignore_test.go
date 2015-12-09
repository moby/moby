package main

import (
	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildDockerignore(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilddockerignore"
	dockerfile := `
        FROM busybox
        ADD . /bla
		RUN [[ -f /bla/src/x.go ]]
		RUN [[ -f /bla/Makefile ]]
		RUN [[ ! -e /bla/src/_vendor ]]
		RUN [[ ! -e /bla/.gitignore ]]
		RUN [[ ! -e /bla/README.md ]]
		RUN [[ ! -e /bla/dir/foo ]]
		RUN [[ ! -e /bla/foo ]]
		RUN [[ ! -e /bla/.git ]]
		RUN [[ ! -e v.cc ]]
		RUN [[ ! -e src/v.cc ]]
		RUN [[ ! -e src/_vendor/v.cc ]]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Makefile":         "all:",
		".git/HEAD":        "ref: foo",
		"src/x.go":         "package main",
		"src/_vendor/v.go": "package main",
		"src/_vendor/v.cc": "package main",
		"src/v.cc":         "package main",
		"v.cc":             "package main",
		"dir/foo":          "",
		".gitignore":       "",
		"README.md":        "readme",
		".dockerignore": `
.git
pkg
.gitignore
src/_vendor
*.md
**/*.cc
dir`,
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildDockerignoreCleanPaths(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilddockerignorecleanpaths"
	dockerfile := `
        FROM busybox
        ADD . /tmp/
        RUN (! ls /tmp/foo) && (! ls /tmp/foo2) && (! ls /tmp/dir1/foo)`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo":           "foo",
		"foo2":          "foo2",
		"dir1/foo":      "foo in dir1",
		".dockerignore": "./foo\ndir1//foo\n./dir1/../foo2",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildDockerignoreExceptions(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilddockerignoreexceptions"
	dockerfile := `
        FROM busybox
        ADD . /bla
		RUN [[ -f /bla/src/x.go ]]
		RUN [[ -f /bla/Makefile ]]
		RUN [[ ! -e /bla/src/_vendor ]]
		RUN [[ ! -e /bla/.gitignore ]]
		RUN [[ ! -e /bla/README.md ]]
		RUN [[  -e /bla/dir/dir/foo ]]
		RUN [[ ! -e /bla/dir/foo1 ]]
		RUN [[ -f /bla/dir/e ]]
		RUN [[ -f /bla/dir/e-dir/foo ]]
		RUN [[ ! -e /bla/foo ]]
		RUN [[ ! -e /bla/.git ]]
		RUN [[ -e /bla/dir/a.cc ]]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Makefile":         "all:",
		".git/HEAD":        "ref: foo",
		"src/x.go":         "package main",
		"src/_vendor/v.go": "package main",
		"dir/foo":          "",
		"dir/foo1":         "",
		"dir/dir/f1":       "",
		"dir/dir/foo":      "",
		"dir/e":            "",
		"dir/e-dir/foo":    "",
		".gitignore":       "",
		"README.md":        "readme",
		"dir/a.cc":         "hello",
		".dockerignore": `
.git
pkg
.gitignore
src/_vendor
*.md
dir
!dir/e*
!dir/dir/foo
**/*.cc
!**/*.cc`,
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildDockerignoringDockerfile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilddockerignoredockerfile"
	dockerfile := `
        FROM busybox
		ADD . /tmp/
		RUN ! ls /tmp/Dockerfile
		RUN ls /tmp/.dockerignore`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    dockerfile,
		".dockerignore": "Dockerfile\n",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("Didn't ignore Dockerfile correctly:%s", err)
	}

	// now try it with ./Dockerfile
	ctx.Add(".dockerignore", "./Dockerfile\n")
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("Didn't ignore ./Dockerfile correctly:%s", err)
	}

}

func (s *DockerSuite) TestBuildDockerignoringRenamedDockerfile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilddockerignoredockerfile"
	dockerfile := `
        FROM busybox
		ADD . /tmp/
		RUN ls /tmp/Dockerfile
		RUN ! ls /tmp/MyDockerfile
		RUN ls /tmp/.dockerignore`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    "Should not use me",
		"MyDockerfile":  dockerfile,
		".dockerignore": "MyDockerfile\n",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("Didn't ignore MyDockerfile correctly:%s", err)
	}

	// now try it with ./MyDockerfile
	ctx.Add(".dockerignore", "./MyDockerfile\n")
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("Didn't ignore ./MyDockerfile correctly:%s", err)
	}

}

func (s *DockerSuite) TestBuildDockerignoringDockerignore(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilddockerignoredockerignore"
	dockerfile := `
        FROM busybox
		ADD . /tmp/
		RUN ! ls /tmp/.dockerignore
		RUN ls /tmp/Dockerfile`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    dockerfile,
		".dockerignore": ".dockerignore\n",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("Didn't ignore .dockerignore correctly:%s", err)
	}
}

func (s *DockerSuite) TestBuildDockerignoreTouchDockerfile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	var id1 string
	var id2 string

	name := "testbuilddockerignoretouchdockerfile"
	dockerfile := `
        FROM busybox
		ADD . /tmp/`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    dockerfile,
		".dockerignore": "Dockerfile\n",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if id1, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("Didn't build it correctly:%s", err)
	}

	if id2, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("Didn't build it correctly:%s", err)
	}
	if id1 != id2 {
		c.Fatalf("Didn't use the cache - 1")
	}

	// Now make sure touching Dockerfile doesn't invalidate the cache
	if err = ctx.Add("Dockerfile", dockerfile+"\n# hi"); err != nil {
		c.Fatalf("Didn't add Dockerfile: %s", err)
	}
	if id2, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("Didn't build it correctly:%s", err)
	}
	if id1 != id2 {
		c.Fatalf("Didn't use the cache - 2")
	}

	// One more time but just 'touch' it instead of changing the content
	if err = ctx.Add("Dockerfile", dockerfile+"\n# hi"); err != nil {
		c.Fatalf("Didn't add Dockerfile: %s", err)
	}
	if id2, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("Didn't build it correctly:%s", err)
	}
	if id1 != id2 {
		c.Fatalf("Didn't use the cache - 3")
	}

}

func (s *DockerSuite) TestBuildDockerignoringWholeDir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilddockerignorewholedir"
	dockerfile := `
        FROM busybox
		COPY . /
		RUN [[ ! -e /.gitignore ]]
		RUN [[ -f /Makefile ]]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    "FROM scratch",
		"Makefile":      "all:",
		".gitignore":    "",
		".dockerignore": ".*\n",
	})
	c.Assert(err, check.IsNil)
	defer ctx.Close()
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

	c.Assert(ctx.Add(".dockerfile", "*"), check.IsNil)
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

	c.Assert(ctx.Add(".dockerfile", "."), check.IsNil)
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

	c.Assert(ctx.Add(".dockerfile", "?"), check.IsNil)
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildDockerignoringBadExclusion(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilddockerignorebadexclusion"
	dockerfile := `
        FROM busybox
		COPY . /
		RUN [[ ! -e /.gitignore ]]
		RUN [[ -f /Makefile ]]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    "FROM scratch",
		"Makefile":      "all:",
		".gitignore":    "",
		".dockerignore": "!\n",
	})
	c.Assert(err, check.IsNil)
	defer ctx.Close()
	if _, err = buildImageFromContext(name, ctx, true); err == nil {
		c.Fatalf("Build was supposed to fail but didn't")
	}

	if err.Error() != "failed to build the image: Error checking context: 'Illegal exclusion pattern: !'.\n" {
		c.Fatalf("Incorrect output, got:%q", err.Error())
	}
}

func (s *DockerSuite) TestBuildDockerignoringWildTopDir(c *check.C) {
	testRequires(c, DaemonIsLinux)

	dockerfile := `
        FROM busybox
		COPY . /
		RUN [[ ! -e /.dockerignore ]]
		RUN [[ ! -e /Dockerfile ]]
		RUN [[ ! -e /file1 ]]
		RUN [[ ! -e /dir ]]`

	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile": "FROM scratch",
		"file1":      "",
		"dir/dfile1": "",
	})
	c.Assert(err, check.IsNil)
	defer ctx.Close()

	// All of these should result in ignoring all files
	for _, variant := range []string{"**", "**/", "**/**", "*"} {
		ctx.Add(".dockerignore", variant)
		_, err = buildImageFromContext("noname", ctx, true)
		c.Assert(err, check.IsNil, check.Commentf("variant: %s", variant))
	}
}

func (s *DockerSuite) TestBuildDockerignoringWildDirs(c *check.C) {
	testRequires(c, DaemonIsLinux)

	dockerfile := `
        FROM busybox
		COPY . /
		RUN [[ -e /.dockerignore ]]
		RUN [[ -e /Dockerfile ]]

		RUN [[ ! -e /file0 ]]
		RUN [[ ! -e /dir1/file0 ]]
		RUN [[ ! -e /dir2/file0 ]]

		RUN [[ ! -e /file1 ]]
		RUN [[ ! -e /dir1/file1 ]]
		RUN [[ ! -e /dir1/dir2/file1 ]]

		RUN [[ ! -e /dir1/file2 ]]
		RUN [[   -e /dir1/dir2/file2 ]]

		RUN [[ ! -e /dir1/dir2/file4 ]]
		RUN [[ ! -e /dir1/dir2/file5 ]]
		RUN [[ ! -e /dir1/dir2/file6 ]]
		RUN [[ ! -e /dir1/dir3/file7 ]]
		RUN [[ ! -e /dir1/dir3/file8 ]]
		RUN [[   -e /dir1/dir3 ]]
		RUN [[   -e /dir1/dir4 ]]

		RUN [[ ! -e 'dir1/dir5/fileAA' ]]
		RUN [[   -e 'dir1/dir5/fileAB' ]]
		RUN [[   -e 'dir1/dir5/fileB' ]]   # "." in pattern means nothing

		RUN echo all done!`

	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":      "FROM scratch",
		"file0":           "",
		"dir1/file0":      "",
		"dir1/dir2/file0": "",

		"file1":           "",
		"dir1/file1":      "",
		"dir1/dir2/file1": "",

		"dir1/file2":      "",
		"dir1/dir2/file2": "", // remains

		"dir1/dir2/file4": "",
		"dir1/dir2/file5": "",
		"dir1/dir2/file6": "",
		"dir1/dir3/file7": "",
		"dir1/dir3/file8": "",
		"dir1/dir4/file9": "",

		"dir1/dir5/fileAA": "",
		"dir1/dir5/fileAB": "",
		"dir1/dir5/fileB":  "",

		".dockerignore": `
**/file0
**/*file1
**/dir1/file2
dir1/**/file4
**/dir2/file5
**/dir1/dir2/file6
dir1/dir3/**
**/dir4/**
**/file?A
**/file\?B
**/dir5/file.
`,
	})
	c.Assert(err, check.IsNil)
	defer ctx.Close()

	_, err = buildImageFromContext("noname", ctx, true)
	c.Assert(err, check.IsNil)
}
