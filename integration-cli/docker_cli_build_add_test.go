package main

import (
	"archive/tar"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildAddSingleFileToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddimg"
	ctx, err := fakeContext(fmt.Sprintf(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN touch /exists
RUN chown dockerio.dockerio /exists
ADD test_file /
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

// Issue #3960: "ADD src ." hangs
func (s *DockerSuite) TestBuildAddSingleFileToWorkdir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddsinglefiletoworkdir"
	ctx, err := fakeContext(`FROM busybox
ADD test_file .`,
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

func (s *DockerSuite) TestBuildAddSingleFileToExistDir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddsinglefiletoexistdir"
	ctx, err := fakeContext(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN mkdir /exists
RUN touch /exists/exists_file
RUN chown -R dockerio.dockerio /exists
ADD test_file /exists/
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

func (s *DockerSuite) TestBuildAddMultipleFilesToFile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddmultiplefilestofile"
	ctx, err := fakeContext(`FROM scratch
	ADD file1.txt file2.txt test
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

}

func (s *DockerSuite) TestBuildJSONAddMultipleFilesToFile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testjsonaddmultiplefilestofile"
	ctx, err := fakeContext(`FROM scratch
	ADD ["file1.txt", "file2.txt", "test"]
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

}

func (s *DockerSuite) TestBuildAddMultipleFilesToFileWild(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddmultiplefilestofilewild"
	ctx, err := fakeContext(`FROM scratch
	ADD file*.txt test
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

}

func (s *DockerSuite) TestBuildJSONAddMultipleFilesToFileWild(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testjsonaddmultiplefilestofilewild"
	ctx, err := fakeContext(`FROM scratch
	ADD ["file*.txt", "test"]
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

}

func (s *DockerSuite) TestBuildAddFileWithWhitespace(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddfilewithwhitespace"
	ctx, err := fakeContext(`FROM busybox
RUN mkdir "/test dir"
RUN mkdir "/test_dir"
ADD [ "test file1", "/test_file1" ]
ADD [ "test_file2", "/test file2" ]
ADD [ "test file3", "/test file3" ]
ADD [ "test dir/test_file4", "/test_dir/test_file4" ]
ADD [ "test_dir/test_file5", "/test dir/test_file5" ]
ADD [ "test dir/test_file6", "/test dir/test_file6" ]
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

func (s *DockerSuite) TestBuildAddMultipleFilesToFileWithWhitespace(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddmultiplefilestofilewithwhitespace"
	ctx, err := fakeContext(`FROM busybox
	ADD [ "test file1", "test file2", "test" ]
    `,
		map[string]string{
			"test file1": "test1",
			"test file2": "test2",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

}

func (s *DockerSuite) TestBuildAddSingleFileToNonExistingDir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddsinglefiletononexistingdir"
	ctx, err := fakeContext(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN touch /exists
RUN chown dockerio.dockerio /exists
ADD test_file /test_dir/
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

func (s *DockerSuite) TestBuildAddDirContentToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testadddircontenttoroot"
	ctx, err := fakeContext(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN touch /exists
RUN chown dockerio.dockerio exists
ADD test_dir /
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

func (s *DockerSuite) TestBuildAddDirContentToExistingDir(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testadddircontenttoexistingdir"
	ctx, err := fakeContext(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN mkdir /exists
RUN touch /exists/exists_file
RUN chown -R dockerio.dockerio /exists
ADD test_dir/ /exists/
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

func (s *DockerSuite) TestBuildAddWholeDirToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddwholedirtoroot"
	ctx, err := fakeContext(fmt.Sprintf(`FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN touch /exists
RUN chown dockerio.dockerio exists
ADD test_dir /test_dir
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

// Testing #5941
func (s *DockerSuite) TestBuildAddEtcToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddetctoroot"
	ctx, err := fakeContext(`FROM scratch
ADD . /`,
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

// Testing #9401
func (s *DockerSuite) TestBuildAddPreservesFilesSpecialBits(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testaddpreservesfilesspecialbits"
	ctx, err := fakeContext(`FROM busybox
ADD suidbin /usr/bin/suidbin
RUN chmod 4755 /usr/bin/suidbin
RUN [ $(ls -l /usr/bin/suidbin | awk '{print $1}') = '-rwsr-xr-x' ]
ADD ./data/ /
RUN [ $(ls -l /usr/bin/suidbin | awk '{print $1}') = '-rwsr-xr-x' ]`,
		map[string]string{
			"suidbin":             "suidbin",
			"/data/usr/test_file": "test1",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildAddBadLinks(c *check.C) {
	testRequires(c, DaemonIsLinux)
	const (
		dockerfile = `
			FROM scratch
			ADD links.tar /
			ADD foo.txt /symlink/
			`
		targetFile = "foo.txt"
	)
	var (
		name = "test-link-absolute"
	)
	ctx, err := fakeContext(dockerfile, nil)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	tempDir, err := ioutil.TempDir("", "test-link-absolute-temp-")
	if err != nil {
		c.Fatalf("failed to create temporary directory: %s", tempDir)
	}
	defer os.RemoveAll(tempDir)

	var symlinkTarget string
	if runtime.GOOS == "windows" {
		var driveLetter string
		if abs, err := filepath.Abs(tempDir); err != nil {
			c.Fatal(err)
		} else {
			driveLetter = abs[:1]
		}
		tempDirWithoutDrive := tempDir[2:]
		symlinkTarget = fmt.Sprintf(`%s:\..\..\..\..\..\..\..\..\..\..\..\..%s`, driveLetter, tempDirWithoutDrive)
	} else {
		symlinkTarget = fmt.Sprintf("/../../../../../../../../../../../..%s", tempDir)
	}

	tarPath := filepath.Join(ctx.Dir, "links.tar")
	nonExistingFile := filepath.Join(tempDir, targetFile)
	fooPath := filepath.Join(ctx.Dir, targetFile)

	tarOut, err := os.Create(tarPath)
	if err != nil {
		c.Fatal(err)
	}

	tarWriter := tar.NewWriter(tarOut)

	header := &tar.Header{
		Name:     "symlink",
		Typeflag: tar.TypeSymlink,
		Linkname: symlinkTarget,
		Mode:     0755,
		Uid:      0,
		Gid:      0,
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		c.Fatal(err)
	}

	tarWriter.Close()
	tarOut.Close()

	foo, err := os.Create(fooPath)
	if err != nil {
		c.Fatal(err)
	}
	defer foo.Close()

	if _, err := foo.WriteString("test"); err != nil {
		c.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

	if _, err := os.Stat(nonExistingFile); err == nil || err != nil && !os.IsNotExist(err) {
		c.Fatalf("%s shouldn't have been written and it shouldn't exist", nonExistingFile)
	}

}

func (s *DockerSuite) TestBuildAddBadLinksVolume(c *check.C) {
	testRequires(c, DaemonIsLinux)
	const (
		dockerfileTemplate = `
		FROM busybox
		RUN ln -s /../../../../../../../../%s /x
		VOLUME /x
		ADD foo.txt /x/`
		targetFile = "foo.txt"
	)
	var (
		name       = "test-link-absolute-volume"
		dockerfile = ""
	)

	tempDir, err := ioutil.TempDir("", "test-link-absolute-volume-temp-")
	if err != nil {
		c.Fatalf("failed to create temporary directory: %s", tempDir)
	}
	defer os.RemoveAll(tempDir)

	dockerfile = fmt.Sprintf(dockerfileTemplate, tempDir)
	nonExistingFile := filepath.Join(tempDir, targetFile)

	ctx, err := fakeContext(dockerfile, nil)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	fooPath := filepath.Join(ctx.Dir, targetFile)

	foo, err := os.Create(fooPath)
	if err != nil {
		c.Fatal(err)
	}
	defer foo.Close()

	if _, err := foo.WriteString("test"); err != nil {
		c.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

	if _, err := os.Stat(nonExistingFile); err == nil || err != nil && !os.IsNotExist(err) {
		c.Fatalf("%s shouldn't have been written and it shouldn't exist", nonExistingFile)
	}

}

func (s *DockerSuite) TestBuildAddLocalFileWithCache(c *check.C) {
	// local files are not owned by the correct user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddlocalfilewithcache"
	name2 := "testbuildaddlocalfilewithcache2"
	dockerfile := `
		FROM busybox
        MAINTAINER dockerio
        ADD foo /usr/lib/bla/bar
		RUN [ "$(cat /usr/lib/bla/bar)" = "hello" ]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 != id2 {
		c.Fatal("The cache should have been used but hasn't.")
	}
}

func (s *DockerSuite) TestBuildAddMultipleLocalFileWithCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddmultiplelocalfilewithcache"
	name2 := "testbuildaddmultiplelocalfilewithcache2"
	dockerfile := `
		FROM busybox
        MAINTAINER dockerio
        ADD foo Dockerfile /usr/lib/bla/
		RUN [ "$(cat /usr/lib/bla/foo)" = "hello" ]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 != id2 {
		c.Fatal("The cache should have been used but hasn't.")
	}
}

func (s *DockerSuite) TestBuildAddLocalFileWithoutCache(c *check.C) {
	// local files are not owned by the correct user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddlocalfilewithoutcache"
	name2 := "testbuildaddlocalfilewithoutcache2"
	dockerfile := `
		FROM busybox
        MAINTAINER dockerio
        ADD foo /usr/lib/bla/bar
		RUN [ "$(cat /usr/lib/bla/bar)" = "hello" ]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, false)
	if err != nil {
		c.Fatal(err)
	}
	if id1 == id2 {
		c.Fatal("The cache should have been invalided but hasn't.")
	}
}

func (s *DockerSuite) TestBuildAddCurrentDirWithCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddcurrentdirwithcache"
	name2 := name + "2"
	name3 := name + "3"
	name4 := name + "4"
	dockerfile := `
        FROM scratch
        MAINTAINER dockerio
        ADD . /usr/lib/bla`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	// Check that adding file invalidate cache of "ADD ."
	if err := ctx.Add("bar", "hello2"); err != nil {
		c.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 == id2 {
		c.Fatal("The cache should have been invalided but hasn't.")
	}
	// Check that changing file invalidate cache of "ADD ."
	if err := ctx.Add("foo", "hello1"); err != nil {
		c.Fatal(err)
	}
	id3, err := buildImageFromContext(name3, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	if id2 == id3 {
		c.Fatal("The cache should have been invalided but hasn't.")
	}
	// Check that changing file to same content with different mtime does not
	// invalidate cache of "ADD ."
	time.Sleep(1 * time.Second) // wait second because of mtime precision
	if err := ctx.Add("foo", "hello1"); err != nil {
		c.Fatal(err)
	}
	id4, err := buildImageFromContext(name4, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	if id3 != id4 {
		c.Fatal("The cache should have been used but hasn't.")
	}
}

func (s *DockerSuite) TestBuildAddCurrentDirWithoutCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddcurrentdirwithoutcache"
	name2 := "testbuildaddcurrentdirwithoutcache2"
	dockerfile := `
        FROM scratch
        MAINTAINER dockerio
        ADD . /usr/lib/bla`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, false)
	if err != nil {
		c.Fatal(err)
	}
	if id1 == id2 {
		c.Fatal("The cache should have been invalided but hasn't.")
	}
}

func (s *DockerSuite) TestBuildAddRemoteFileWithCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddremotefilewithcache"
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	id1, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 != id2 {
		c.Fatal("The cache should have been used but hasn't.")
	}
}

func (s *DockerSuite) TestBuildAddRemoteFileWithoutCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddremotefilewithoutcache"
	name2 := "testbuildaddremotefilewithoutcache2"
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	id1, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImage(name2,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		false)
	if err != nil {
		c.Fatal(err)
	}
	if id1 == id2 {
		c.Fatal("The cache should have been invalided but hasn't.")
	}
}

func (s *DockerSuite) TestBuildAddRemoteFileMTime(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddremotefilemtime"
	name2 := name + "2"
	name3 := name + "3"

	files := map[string]string{"baz": "hello"}
	server, err := fakeStorage(files)
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()), nil)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}

	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 != id2 {
		c.Fatal("The cache should have been used but wasn't - #1")
	}

	// Now create a different server with same contents (causes different mtime)
	// The cache should still be used

	// allow some time for clock to pass as mtime precision is only 1s
	time.Sleep(2 * time.Second)

	server2, err := fakeStorage(files)
	if err != nil {
		c.Fatal(err)
	}
	defer server2.Close()

	ctx2, err := fakeContext(fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server2.URL()), nil)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx2.Close()
	id3, err := buildImageFromContext(name3, ctx2, true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 != id3 {
		c.Fatal("The cache should have been used but wasn't")
	}
}

func (s *DockerSuite) TestBuildAddLocalAndRemoteFilesWithCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddlocalandremotefilewithcache"
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD foo /usr/lib/bla/bar
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		map[string]string{
			"foo": "hello world",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 != id2 {
		c.Fatal("The cache should have been used but hasn't.")
	}
}

// TODO: TestCaching
func (s *DockerSuite) TestBuildAddLocalAndRemoteFilesWithoutCache(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddlocalandremotefilewithoutcache"
	name2 := "testbuildaddlocalandremotefilewithoutcache2"
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD foo /usr/lib/bla/bar
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		map[string]string{
			"foo": "hello world",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, false)
	if err != nil {
		c.Fatal(err)
	}
	if id1 == id2 {
		c.Fatal("The cache should have been invalided but hasn't.")
	}
}

func (s *DockerSuite) TestBuildAddFileNotFound(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddnotfound"
	ctx, err := fakeContext(`FROM scratch
        ADD foo /usr/local/bar`,
		map[string]string{"bar": "hello"})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		if !strings.Contains(err.Error(), "foo: no such file or directory") {
			c.Fatalf("Wrong error %v, must be about missing foo file or directory", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}
}

// gh #2446
func (s *DockerSuite) TestBuildAddToSymlinkDest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddtosymlinkdest"
	ctx, err := fakeContext(`FROM busybox
        RUN mkdir /foo
        RUN ln -s /foo /bar
        ADD foo /bar/
        RUN [ -f /bar/foo ]
        RUN [ -f /foo/foo ]`,
		map[string]string{
			"foo": "hello",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildAddScript(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddscript"
	dockerfile := `
FROM busybox
ADD test /test
RUN ["chmod","+x","/test"]
RUN ["/test"]
RUN [ "$(cat /testfile)" = 'test!' ]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"test": "#!/bin/sh\necho 'test!' > /testfile",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	_, err = buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildAddTar(c *check.C) {
	// /test/foo is not owned by the correct user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddtar"

	ctx := func() *FakeContext {
		dockerfile := `
FROM busybox
ADD test.tar /
RUN cat /test/foo | grep Hi
ADD test.tar /test.tar
RUN cat /test.tar/test/foo | grep Hi
ADD test.tar /unlikely-to-exist
RUN cat /unlikely-to-exist/test/foo | grep Hi
ADD test.tar /unlikely-to-exist-trailing-slash/
RUN cat /unlikely-to-exist-trailing-slash/test/foo | grep Hi
RUN mkdir /existing-directory
ADD test.tar /existing-directory
RUN cat /existing-directory/test/foo | grep Hi
ADD test.tar /existing-directory-trailing-slash/
RUN cat /existing-directory-trailing-slash/test/foo | grep Hi`
		tmpDir, err := ioutil.TempDir("", "fake-context")
		c.Assert(err, check.IsNil)
		testTar, err := os.Create(filepath.Join(tmpDir, "test.tar"))
		if err != nil {
			c.Fatalf("failed to create test.tar archive: %v", err)
		}
		defer testTar.Close()

		tw := tar.NewWriter(testTar)

		if err := tw.WriteHeader(&tar.Header{
			Name: "test/foo",
			Size: 2,
		}); err != nil {
			c.Fatalf("failed to write tar file header: %v", err)
		}
		if _, err := tw.Write([]byte("Hi")); err != nil {
			c.Fatalf("failed to write tar file content: %v", err)
		}
		if err := tw.Close(); err != nil {
			c.Fatalf("failed to close tar archive: %v", err)
		}

		if err := ioutil.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
			c.Fatalf("failed to open destination dockerfile: %v", err)
		}
		return fakeContextFromDir(tmpDir)
	}()
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("build failed to complete for TestBuildAddTar: %v", err)
	}

}

func (s *DockerSuite) TestBuildAddBrokenTar(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddbrokentar"

	ctx := func() *FakeContext {
		dockerfile := `
FROM busybox
ADD test.tar /`
		tmpDir, err := ioutil.TempDir("", "fake-context")
		c.Assert(err, check.IsNil)
		testTar, err := os.Create(filepath.Join(tmpDir, "test.tar"))
		if err != nil {
			c.Fatalf("failed to create test.tar archive: %v", err)
		}
		defer testTar.Close()

		tw := tar.NewWriter(testTar)

		if err := tw.WriteHeader(&tar.Header{
			Name: "test/foo",
			Size: 2,
		}); err != nil {
			c.Fatalf("failed to write tar file header: %v", err)
		}
		if _, err := tw.Write([]byte("Hi")); err != nil {
			c.Fatalf("failed to write tar file content: %v", err)
		}
		if err := tw.Close(); err != nil {
			c.Fatalf("failed to close tar archive: %v", err)
		}

		// Corrupt the tar by removing one byte off the end
		stat, err := testTar.Stat()
		if err != nil {
			c.Fatalf("failed to stat tar archive: %v", err)
		}
		if err := testTar.Truncate(stat.Size() - 1); err != nil {
			c.Fatalf("failed to truncate tar archive: %v", err)
		}

		if err := ioutil.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
			c.Fatalf("failed to open destination dockerfile: %v", err)
		}
		return fakeContextFromDir(tmpDir)
	}()
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err == nil {
		c.Fatalf("build should have failed for TestBuildAddBrokenTar")
	}
}

func (s *DockerSuite) TestBuildAddNonTar(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddnontar"

	// Should not try to extract test.tar
	ctx, err := fakeContext(`
		FROM busybox
		ADD test.tar /
		RUN test -f /test.tar`,
		map[string]string{"test.tar": "not_a_tar_file"})

	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("build failed for TestBuildAddNonTar")
	}
}

func (s *DockerSuite) TestBuildAddTarXz(c *check.C) {
	// /test/foo is not owned by the correct user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddtarxz"

	ctx := func() *FakeContext {
		dockerfile := `
			FROM busybox
			ADD test.tar.xz /
			RUN cat /test/foo | grep Hi`
		tmpDir, err := ioutil.TempDir("", "fake-context")
		c.Assert(err, check.IsNil)
		testTar, err := os.Create(filepath.Join(tmpDir, "test.tar"))
		if err != nil {
			c.Fatalf("failed to create test.tar archive: %v", err)
		}
		defer testTar.Close()

		tw := tar.NewWriter(testTar)

		if err := tw.WriteHeader(&tar.Header{
			Name: "test/foo",
			Size: 2,
		}); err != nil {
			c.Fatalf("failed to write tar file header: %v", err)
		}
		if _, err := tw.Write([]byte("Hi")); err != nil {
			c.Fatalf("failed to write tar file content: %v", err)
		}
		if err := tw.Close(); err != nil {
			c.Fatalf("failed to close tar archive: %v", err)
		}

		xzCompressCmd := exec.Command("xz", "-k", "test.tar")
		xzCompressCmd.Dir = tmpDir
		out, _, err := runCommandWithOutput(xzCompressCmd)
		if err != nil {
			c.Fatal(err, out)
		}

		if err := ioutil.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
			c.Fatalf("failed to open destination dockerfile: %v", err)
		}
		return fakeContextFromDir(tmpDir)
	}()

	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("build failed to complete for TestBuildAddTarXz: %v", err)
	}

}

func (s *DockerSuite) TestBuildAddTarXzGz(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildaddtarxzgz"

	ctx := func() *FakeContext {
		dockerfile := `
			FROM busybox
			ADD test.tar.xz.gz /
			RUN ls /test.tar.xz.gz`
		tmpDir, err := ioutil.TempDir("", "fake-context")
		c.Assert(err, check.IsNil)
		testTar, err := os.Create(filepath.Join(tmpDir, "test.tar"))
		if err != nil {
			c.Fatalf("failed to create test.tar archive: %v", err)
		}
		defer testTar.Close()

		tw := tar.NewWriter(testTar)

		if err := tw.WriteHeader(&tar.Header{
			Name: "test/foo",
			Size: 2,
		}); err != nil {
			c.Fatalf("failed to write tar file header: %v", err)
		}
		if _, err := tw.Write([]byte("Hi")); err != nil {
			c.Fatalf("failed to write tar file content: %v", err)
		}
		if err := tw.Close(); err != nil {
			c.Fatalf("failed to close tar archive: %v", err)
		}

		xzCompressCmd := exec.Command("xz", "-k", "test.tar")
		xzCompressCmd.Dir = tmpDir
		out, _, err := runCommandWithOutput(xzCompressCmd)
		if err != nil {
			c.Fatal(err, out)
		}

		gzipCompressCmd := exec.Command("gzip", "test.tar.xz")
		gzipCompressCmd.Dir = tmpDir
		out, _, err = runCommandWithOutput(gzipCompressCmd)
		if err != nil {
			c.Fatal(err, out)
		}

		if err := ioutil.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
			c.Fatalf("failed to open destination dockerfile: %v", err)
		}
		return fakeContextFromDir(tmpDir)
	}()

	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatalf("build failed to complete for TestBuildAddTarXz: %v", err)
	}

}

func (s *DockerSuite) TestBuildCacheAdd(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildtwoimageswithadd"
	server, err := fakeStorage(map[string]string{
		"robots.txt": "hello",
		"index.html": "world",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	if _, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
		ADD %s/robots.txt /`, server.URL()),
		true); err != nil {
		c.Fatal(err)
	}
	if err != nil {
		c.Fatal(err)
	}
	deleteImages(name)
	_, out, err := buildImageWithOut(name,
		fmt.Sprintf(`FROM scratch
		ADD %s/index.html /`, server.URL()),
		true)
	if err != nil {
		c.Fatal(err)
	}
	if strings.Contains(out, "Using cache") {
		c.Fatal("2nd build used cache on ADD, it shouldn't")
	}

}
