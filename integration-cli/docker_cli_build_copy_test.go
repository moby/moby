package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildCopyAddMultipleFiles(c *check.C) {
	testRequires(c, DaemonIsLinux)
	server, err := fakeStorage(map[string]string{
		"robots.txt": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	name := "testcopymultiplefilestofile"
	ctx, err := fakeContext(fmt.Sprintf(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN mkdir /exists
RUN touch /exists/exists_file
RUN chown -R dockerio.dockerio /exists
COPY test_file1 test_file2 /exists/
ADD test_file3 test_file4 %s/robots.txt /exists/
RUN [ $(ls -l / | grep exists | awk '{print $3":"$4}') = 'dockerio:dockerio' ]
RUN [ $(ls -l /exists/test_file1 | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /exists/test_file2 | awk '{print $3":"$4}') = 'root:root' ]

RUN [ $(ls -l /exists/test_file3 | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /exists/test_file4 | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /exists/robots.txt | awk '{print $3":"$4}') = 'root:root' ]

RUN [ $(ls -l /exists/exists_file | awk '{print $3":"$4}') = 'dockerio:dockerio' ]
`, server.URL()),
		map[string]string{
			"test_file1": "test1",
			"test_file2": "test2",
			"test_file3": "test3",
			"test_file4": "test4",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCopyMultipleFilesToFile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopymultiplefilestofile"
	ctx, err := fakeContext(`FROM scratch
	COPY file1.txt file2.txt test
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	expected := "When using COPY with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

}

func (s *DockerSuite) TestBuildJSONCopyMultipleFilesToFile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testjsoncopymultiplefilestofile"
	ctx, err := fakeContext(`FROM scratch
	COPY ["file1.txt", "file2.txt", "test"]
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	expected := "When using COPY with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

}

func (s *DockerSuite) TestBuildCopyFileWithWhitespace(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopyfilewithwhitespace"
	ctx, err := fakeContext(`FROM busybox
RUN mkdir "/test dir"
RUN mkdir "/test_dir"
COPY [ "test file1", "/test_file1" ]
COPY [ "test_file2", "/test file2" ]
COPY [ "test file3", "/test file3" ]
COPY [ "test dir/test_file4", "/test_dir/test_file4" ]
COPY [ "test_dir/test_file5", "/test dir/test_file5" ]
COPY [ "test dir/test_file6", "/test dir/test_file6" ]
RUN [ $(cat "/test_file1") = 'test1' ]
RUN [ $(cat "/test file2") = 'test2' ]
RUN [ $(cat "/test file3") = 'test3' ]
RUN [ $(cat "/test_dir/test_file4") = 'test4' ]
RUN [ $(cat "/test dir/test_file5") = 'test5' ]
RUN [ $(cat "/test dir/test_file6") = 'test6' ]`,
		map[string]string{
			"test file1":          "test1",
			"test_file2":          "test2",
			"test file3":          "test3",
			"test dir/test_file4": "test4",
			"test_dir/test_file5": "test5",
			"test dir/test_file6": "test6",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCopyMultipleFilesToFileWithWhitespace(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopymultiplefilestofilewithwhitespace"
	ctx, err := fakeContext(`FROM busybox
	COPY [ "test file1", "test file2", "test" ]
        `,
		map[string]string{
			"test file1": "test1",
			"test file2": "test2",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	expected := "When using COPY with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

}

func (s *DockerSuite) TestBuildCopyWildcard(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopywildcard"
	server, err := fakeStorage(map[string]string{
		"robots.txt": "hello",
		"index.html": "world",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(fmt.Sprintf(`FROM busybox
	COPY file*.txt /tmp/
	RUN ls /tmp/file1.txt /tmp/file2.txt
	RUN mkdir /tmp1
	COPY dir* /tmp1/
	RUN ls /tmp1/dirt /tmp1/nested_file /tmp1/nested_dir/nest_nest_file
	RUN mkdir /tmp2
        ADD dir/*dir %s/robots.txt /tmp2/
	RUN ls /tmp2/nest_nest_file /tmp2/robots.txt
	`, server.URL()),
		map[string]string{
			"file1.txt":                     "test1",
			"file2.txt":                     "test2",
			"dir/nested_file":               "nested file",
			"dir/nested_dir/nest_nest_file": "2 times nested",
			"dirt": "dirty",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}

	// Now make sure we use a cache the 2nd time
	id2, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}

	if id1 != id2 {
		c.Fatal("didn't use the cache")
	}

}

func (s *DockerSuite) TestBuildCopyWildcardNoFind(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopywildcardnofind"
	ctx, err := fakeContext(`FROM busybox
	COPY file*.txt /tmp/
	`, nil)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	_, err = buildImageFromContext(name, ctx, true)
	if err == nil {
		c.Fatal("should have failed to find a file")
	}
	if !strings.Contains(err.Error(), "No source files were specified") {
		c.Fatalf("Wrong error %v, must be about no source files", err)
	}

}

func (s *DockerSuite) TestBuildCopyWildcardInName(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopywildcardinname"
	ctx, err := fakeContext(`FROM busybox
	COPY *.txt /tmp/
	RUN [ "$(cat /tmp/\*.txt)" = 'hi there' ]
	`, map[string]string{"*.txt": "hi there"})

	if err != nil {
		// Normally we would do c.Fatal(err) here but given that
		// the odds of this failing are so rare, it must be because
		// the OS we're running the client on doesn't support * in
		// filenames (like windows).  So, instead of failing the test
		// just let it pass. Then we don't need to explicitly
		// say which OSs this works on or not.
		return
	}
	defer ctx.Close()

	_, err = buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatalf("should have built: %q", err)
	}
}

func (s *DockerSuite) TestBuildCopyWildcardCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopywildcardcache"
	ctx, err := fakeContext(`FROM busybox
	COPY file1.txt /tmp/`,
		map[string]string{
			"file1.txt": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}

	// Now make sure we use a cache the 2nd time even with wild cards.
	// Use the same context so the file is the same and the checksum will match
	ctx.Add("Dockerfile", `FROM busybox
	COPY file*.txt /tmp/`)

	id2, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}

	if id1 != id2 {
		c.Fatal("didn't use the cache")
	}

}

func (s *DockerSuite) TestBuildCopySingleFileToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopysinglefiletoroot"
	ctx, err := fakeContext(fmt.Sprintf(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN touch /exists
RUN chown dockerio.dockerio /exists
COPY test_file /
RUN [ $(ls -l /test_file | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /test_file | awk '{print $1}') = '%s' ]
RUN [ $(ls -l /exists | awk '{print $3":"$4}') = 'dockerio:dockerio' ]`, expectedFileChmod),
		map[string]string{
			"test_file": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

// Issue #3960: "ADD src ." hangs - adapted for COPY
func (s *DockerSuite) TestBuildCopySingleFileToWorkdir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopysinglefiletoworkdir"
	ctx, err := fakeContext(`FROM busybox
COPY test_file .`,
		map[string]string{
			"test_file": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	errChan := make(chan error)
	go func() {
		_, err := buildImageFromContext(name, ctx, true)
		errChan <- err
		close(errChan)
	}()
	select {
	case <-time.After(15 * time.Second):
		c.Fatal("Build with adding to workdir timed out")
	case err := <-errChan:
		c.Assert(err, check.IsNil)
	}
}

func (s *DockerSuite) TestBuildCopySingleFileToExistDir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopysinglefiletoexistdir"
	ctx, err := fakeContext(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN mkdir /exists
RUN touch /exists/exists_file
RUN chown -R dockerio.dockerio /exists
COPY test_file /exists/
RUN [ $(ls -l / | grep exists | awk '{print $3":"$4}') = 'dockerio:dockerio' ]
RUN [ $(ls -l /exists/test_file | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /exists/exists_file | awk '{print $3":"$4}') = 'dockerio:dockerio' ]`,
		map[string]string{
			"test_file": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCopySingleFileToNonExistDir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopysinglefiletononexistdir"
	ctx, err := fakeContext(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN touch /exists
RUN chown dockerio.dockerio /exists
COPY test_file /test_dir/
RUN [ $(ls -l / | grep test_dir | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /test_dir/test_file | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /exists | awk '{print $3":"$4}') = 'dockerio:dockerio' ]`,
		map[string]string{
			"test_file": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCopyDirContentToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopydircontenttoroot"
	ctx, err := fakeContext(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN touch /exists
RUN chown dockerio.dockerio exists
COPY test_dir /
RUN [ $(ls -l /test_file | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /exists | awk '{print $3":"$4}') = 'dockerio:dockerio' ]`,
		map[string]string{
			"test_dir/test_file": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCopyDirContentToExistDir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopydircontenttoexistdir"
	ctx, err := fakeContext(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN mkdir /exists
RUN touch /exists/exists_file
RUN chown -R dockerio.dockerio /exists
COPY test_dir/ /exists/
RUN [ $(ls -l / | grep exists | awk '{print $3":"$4}') = 'dockerio:dockerio' ]
RUN [ $(ls -l /exists/exists_file | awk '{print $3":"$4}') = 'dockerio:dockerio' ]
RUN [ $(ls -l /exists/test_file | awk '{print $3":"$4}') = 'root:root' ]`,
		map[string]string{
			"test_dir/test_file": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCopyWholeDirToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopywholedirtoroot"
	ctx, err := fakeContext(fmt.Sprintf(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN touch /exists
RUN chown dockerio.dockerio exists
COPY test_dir /test_dir
RUN [ $(ls -l / | grep test_dir | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l / | grep test_dir | awk '{print $1}') = 'drwxr-xr-x' ]
RUN [ $(ls -l /test_dir/test_file | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /test_dir/test_file | awk '{print $1}') = '%s' ]
RUN [ $(ls -l /exists | awk '{print $3":"$4}') = 'dockerio:dockerio' ]`, expectedFileChmod),
		map[string]string{
			"test_dir/test_file": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCopyEtcToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopyetctoroot"
	ctx, err := fakeContext(`FROM scratch
COPY . /`,
		map[string]string{
			"etc/test_file": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCopyDisallowRemote(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testcopydisallowremote"
	_, out, err := buildImageWithOut(name, `FROM scratch
COPY https://index.docker.io/robots.txt /`,
		true)
	if err == nil || !strings.Contains(out, "Source can't be a URL for COPY") {
		c.Fatalf("Error should be about disallowed remote source, got err: %s, out: %q", err, out)
	}
}

func (s *DockerSuite) TestBuildCopyDirButNotFile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildcopydirbutnotfile"
	name2 := "testbuildcopydirbutnotfile2"
	dockerfile := `
        FROM scratch
        COPY dir /tmp/`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"dir/foo": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	// Check that adding file with similar name doesn't mess with cache
	if err := ctx.Add("dir_file", "hello2"); err != nil {
		c.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 != id2 {
		c.Fatal("The cache should have been used but wasn't")
	}
}

func (s *DockerSuite) TestBuildRelativeCopy(c *check.C) {
	// cat /test1/test2/foo gets permission denied for the user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	name := "testbuildrelativecopy"
	dockerfile := `
		FROM busybox
			WORKDIR /test1
			WORKDIR test2
			RUN [ "$PWD" = '/test1/test2' ]
			COPY foo ./
			RUN [ "$(cat /test1/test2/foo)" = 'hello' ]
			ADD foo ./bar/baz
			RUN [ "$(cat /test1/test2/bar/baz)" = 'hello' ]
			COPY foo ./bar/baz2
			RUN [ "$(cat /test1/test2/bar/baz2)" = 'hello' ]
			WORKDIR ..
			COPY foo ./
			RUN [ "$(cat /test1/foo)" = 'hello' ]
			COPY foo /test3/
			RUN [ "$(cat /test3/foo)" = 'hello' ]
			WORKDIR /test4
			COPY . .
			RUN [ "$(cat /test4/foo)" = 'hello' ]
			WORKDIR /test5/test6
			COPY foo ../
			RUN [ "$(cat /test5/foo)" = 'hello' ]
			`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	_, err = buildImageFromContext(name, ctx, false)
	if err != nil {
		c.Fatal(err)
	}
}
