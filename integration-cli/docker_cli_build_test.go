package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/integration/checker"
	icmd "github.com/docker/docker/pkg/integration/cmd"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildJSONEmptyRun(c *check.C) {
	name := "testbuildjsonemptyrun"

	_, err := buildImage(
		name,
		`
    FROM busybox
    RUN []
    `,
		true)

	if err != nil {
		c.Fatal("error when dealing with a RUN statement with empty JSON array")
	}

}

func (s *DockerSuite) TestBuildShCmdJSONEntrypoint(c *check.C) {
	name := "testbuildshcmdjsonentrypoint"

	_, err := buildImage(
		name,
		`
    FROM busybox
    ENTRYPOINT ["echo"]
    CMD echo test
    `,
		true)
	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--rm", name)

	if daemonPlatform == "windows" {
		if !strings.Contains(out, "cmd /S /C echo test") {
			c.Fatalf("CMD did not contain cmd /S /C echo test : %q", out)
		}
	} else {
		if strings.TrimSpace(out) != "/bin/sh -c echo test" {
			c.Fatalf("CMD did not contain /bin/sh -c : %q", out)
		}
	}

}

func (s *DockerSuite) TestBuildEnvironmentReplacementUser(c *check.C) {
	// Windows does not support FROM scratch or the USER command
	testRequires(c, DaemonIsLinux)
	name := "testbuildenvironmentreplacement"

	_, err := buildImage(name, `
  FROM scratch
  ENV user foo
  USER ${user}
  `, true)
	if err != nil {
		c.Fatal(err)
	}

	res := inspectFieldJSON(c, name, "Config.User")

	if res != `"foo"` {
		c.Fatal("User foo from environment not in Config.User on image")
	}

}

func (s *DockerSuite) TestBuildEnvironmentReplacementVolume(c *check.C) {
	name := "testbuildenvironmentreplacement"

	var volumePath string

	if daemonPlatform == "windows" {
		volumePath = "c:/quux"
	} else {
		volumePath = "/quux"
	}

	_, err := buildImage(name, `
  FROM `+minimalBaseImage()+`
  ENV volume `+volumePath+`
  VOLUME ${volume}
  `, true)
	if err != nil {
		c.Fatal(err)
	}

	res := inspectFieldJSON(c, name, "Config.Volumes")

	var volumes map[string]interface{}

	if err := json.Unmarshal([]byte(res), &volumes); err != nil {
		c.Fatal(err)
	}

	if _, ok := volumes[volumePath]; !ok {
		c.Fatal("Volume " + volumePath + " from environment not in Config.Volumes on image")
	}

}

func (s *DockerSuite) TestBuildEnvironmentReplacementExpose(c *check.C) {
	// Windows does not support FROM scratch or the EXPOSE command
	testRequires(c, DaemonIsLinux)
	name := "testbuildenvironmentreplacement"

	_, err := buildImage(name, `
  FROM scratch
  ENV port 80
  EXPOSE ${port}
  ENV ports "  99   100 "
  EXPOSE ${ports}
  `, true)
	if err != nil {
		c.Fatal(err)
	}

	res := inspectFieldJSON(c, name, "Config.ExposedPorts")

	var exposedPorts map[string]interface{}

	if err := json.Unmarshal([]byte(res), &exposedPorts); err != nil {
		c.Fatal(err)
	}

	exp := []int{80, 99, 100}

	for _, p := range exp {
		tmp := fmt.Sprintf("%d/tcp", p)
		if _, ok := exposedPorts[tmp]; !ok {
			c.Fatalf("Exposed port %d from environment not in Config.ExposedPorts on image", p)
		}
	}

}

func (s *DockerSuite) TestBuildEnvironmentReplacementWorkdir(c *check.C) {
	name := "testbuildenvironmentreplacement"

	_, err := buildImage(name, `
  FROM busybox
  ENV MYWORKDIR /work
  RUN mkdir ${MYWORKDIR}
  WORKDIR ${MYWORKDIR}
  `, true)

	if err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestBuildEnvironmentReplacementAddCopy(c *check.C) {
	name := "testbuildenvironmentreplacement"

	ctx, err := fakeContext(`
  FROM `+minimalBaseImage()+`
  ENV baz foo
  ENV quux bar
  ENV dot .
  ENV fee fff
  ENV gee ggg

  ADD ${baz} ${dot}
  COPY ${quux} ${dot}
  ADD ${zzz:-${fee}} ${dot}
  COPY ${zzz:-${gee}} ${dot}
  `,
		map[string]string{
			"foo": "test1",
			"bar": "test2",
			"fff": "test3",
			"ggg": "test4",
		})

	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestBuildEnvironmentReplacementEnv(c *check.C) {
	// ENV expansions work differently in Windows
	testRequires(c, DaemonIsLinux)
	name := "testbuildenvironmentreplacement"

	_, err := buildImage(name,
		`
  FROM busybox
  ENV foo zzz
  ENV bar ${foo}
  ENV abc1='$foo'
  ENV env1=$foo env2=${foo} env3="$foo" env4="${foo}"
  RUN [ "$abc1" = '$foo' ] && (echo "$abc1" | grep -q foo)
  ENV abc2="\$foo"
  RUN [ "$abc2" = '$foo' ] && (echo "$abc2" | grep -q foo)
  ENV abc3 '$foo'
  RUN [ "$abc3" = '$foo' ] && (echo "$abc3" | grep -q foo)
  ENV abc4 "\$foo"
  RUN [ "$abc4" = '$foo' ] && (echo "$abc4" | grep -q foo)
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	res := inspectFieldJSON(c, name, "Config.Env")

	envResult := []string{}

	if err = json.Unmarshal([]byte(res), &envResult); err != nil {
		c.Fatal(err)
	}

	found := false
	envCount := 0

	for _, env := range envResult {
		parts := strings.SplitN(env, "=", 2)
		if parts[0] == "bar" {
			found = true
			if parts[1] != "zzz" {
				c.Fatalf("Could not find replaced var for env `bar`: got %q instead of `zzz`", parts[1])
			}
		} else if strings.HasPrefix(parts[0], "env") {
			envCount++
			if parts[1] != "zzz" {
				c.Fatalf("%s should be 'foo' but instead its %q", parts[0], parts[1])
			}
		} else if strings.HasPrefix(parts[0], "env") {
			envCount++
			if parts[1] != "foo" {
				c.Fatalf("%s should be 'foo' but instead its %q", parts[0], parts[1])
			}
		}
	}

	if !found {
		c.Fatal("Never found the `bar` env variable")
	}

	if envCount != 4 {
		c.Fatalf("Didn't find all env vars - only saw %d\n%s", envCount, envResult)
	}

}

func (s *DockerSuite) TestBuildHandleEscapes(c *check.C) {
	// The volume paths used in this test are invalid on Windows
	testRequires(c, DaemonIsLinux)
	name := "testbuildhandleescapes"

	_, err := buildImage(name,
		`
  FROM scratch
  ENV FOO bar
  VOLUME ${FOO}
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	var result map[string]map[string]struct{}

	res := inspectFieldJSON(c, name, "Config.Volumes")

	if err = json.Unmarshal([]byte(res), &result); err != nil {
		c.Fatal(err)
	}

	if _, ok := result["bar"]; !ok {
		c.Fatalf("Could not find volume bar set from env foo in volumes table, got %q", result)
	}

	deleteImages(name)

	_, err = buildImage(name,
		`
  FROM scratch
  ENV FOO bar
  VOLUME \${FOO}
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	res = inspectFieldJSON(c, name, "Config.Volumes")

	if err = json.Unmarshal([]byte(res), &result); err != nil {
		c.Fatal(err)
	}

	if _, ok := result["${FOO}"]; !ok {
		c.Fatalf("Could not find volume ${FOO} set from env foo in volumes table, got %q", result)
	}

	deleteImages(name)

	// this test in particular provides *7* backslashes and expects 6 to come back.
	// Like above, the first escape is swallowed and the rest are treated as
	// literals, this one is just less obvious because of all the character noise.

	_, err = buildImage(name,
		`
  FROM scratch
  ENV FOO bar
  VOLUME \\\\\\\${FOO}
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	res = inspectFieldJSON(c, name, "Config.Volumes")

	if err = json.Unmarshal([]byte(res), &result); err != nil {
		c.Fatal(err)
	}

	if _, ok := result[`\\\${FOO}`]; !ok {
		c.Fatalf(`Could not find volume \\\${FOO} set from env foo in volumes table, got %q`, result)
	}

}

func (s *DockerSuite) TestBuildOnBuildLowercase(c *check.C) {
	name := "testbuildonbuildlowercase"
	name2 := "testbuildonbuildlowercase2"

	_, err := buildImage(name,
		`
  FROM busybox
  onbuild run echo quux
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	_, out, err := buildImageWithOut(name2, fmt.Sprintf(`
  FROM %s
  `, name), true)

	if err != nil {
		c.Fatal(err)
	}

	if !strings.Contains(out, "quux") {
		c.Fatalf("Did not receive the expected echo text, got %s", out)
	}

	if strings.Contains(out, "ONBUILD ONBUILD") {
		c.Fatalf("Got an ONBUILD ONBUILD error with no error: got %s", out)
	}

}

func (s *DockerSuite) TestBuildEnvEscapes(c *check.C) {
	// ENV expansions work differently in Windows
	testRequires(c, DaemonIsLinux)
	name := "testbuildenvescapes"
	_, err := buildImage(name,
		`
    FROM busybox
    ENV TEST foo
    CMD echo \$
    `,
		true)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "-t", name)

	if strings.TrimSpace(out) != "$" {
		c.Fatalf("Env TEST was not overwritten with bar when foo was supplied to dockerfile: was %q", strings.TrimSpace(out))
	}

}

func (s *DockerSuite) TestBuildEnvOverwrite(c *check.C) {
	// ENV expansions work differently in Windows
	testRequires(c, DaemonIsLinux)
	name := "testbuildenvoverwrite"

	_, err := buildImage(name,
		`
    FROM busybox
    ENV TEST foo
    CMD echo ${TEST}
    `,
		true)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "-e", "TEST=bar", "-t", name)

	if strings.TrimSpace(out) != "bar" {
		c.Fatalf("Env TEST was not overwritten with bar when foo was supplied to dockerfile: was %q", strings.TrimSpace(out))
	}

}

func (s *DockerSuite) TestBuildOnBuildCmdEntrypointJSON(c *check.C) {
	name1 := "onbuildcmd"
	name2 := "onbuildgenerated"

	_, err := buildImage(name1, `
FROM busybox
ONBUILD CMD ["hello world"]
ONBUILD ENTRYPOINT ["echo"]
ONBUILD RUN ["true"]`,
		false)

	if err != nil {
		c.Fatal(err)
	}

	_, err = buildImage(name2, fmt.Sprintf(`FROM %s`, name1), false)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", name2)

	if !regexp.MustCompile(`(?m)^hello world`).MatchString(out) {
		c.Fatalf("did not get echo output from onbuild. Got: %q", out)
	}

}

func (s *DockerSuite) TestBuildOnBuildEntrypointJSON(c *check.C) {
	name1 := "onbuildcmd"
	name2 := "onbuildgenerated"

	_, err := buildImage(name1, `
FROM busybox
ONBUILD ENTRYPOINT ["echo"]`,
		false)

	if err != nil {
		c.Fatal(err)
	}

	_, err = buildImage(name2, fmt.Sprintf("FROM %s\nCMD [\"hello world\"]\n", name1), false)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", name2)

	if !regexp.MustCompile(`(?m)^hello world`).MatchString(out) {
		c.Fatal("got malformed output from onbuild", out)
	}

}

func (s *DockerSuite) TestBuildCacheAdd(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows doesn't have httpserver image yet
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

func (s *DockerSuite) TestBuildLastModified(c *check.C) {
	name := "testbuildlastmodified"

	server, err := fakeStorage(map[string]string{
		"file": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	var out, out2 string

	dFmt := `FROM busybox
ADD %s/file /`

	dockerfile := fmt.Sprintf(dFmt, server.URL())

	if _, _, err = buildImageWithOut(name, dockerfile, false); err != nil {
		c.Fatal(err)
	}

	out, _ = dockerCmd(c, "run", name, "ls", "-le", "/file")

	// Build it again and make sure the mtime of the file didn't change.
	// Wait a few seconds to make sure the time changed enough to notice
	time.Sleep(2 * time.Second)

	if _, _, err = buildImageWithOut(name, dockerfile, false); err != nil {
		c.Fatal(err)
	}
	out2, _ = dockerCmd(c, "run", name, "ls", "-le", "/file")

	if out != out2 {
		c.Fatalf("MTime changed:\nOrigin:%s\nNew:%s", out, out2)
	}

	// Now 'touch' the file and make sure the timestamp DID change this time
	// Create a new fakeStorage instead of just using Add() to help windows
	server, err = fakeStorage(map[string]string{
		"file": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	dockerfile = fmt.Sprintf(dFmt, server.URL())

	if _, _, err = buildImageWithOut(name, dockerfile, false); err != nil {
		c.Fatal(err)
	}
	out2, _ = dockerCmd(c, "run", name, "ls", "-le", "/file")

	if out == out2 {
		c.Fatalf("MTime didn't change:\nOrigin:%s\nNew:%s", out, out2)
	}

}

func (s *DockerSuite) TestBuildAddSingleFileToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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

func (s *DockerSuite) TestBuildCopyAddMultipleFiles(c *check.C) {
	testRequires(c, DaemonIsLinux) // Linux specific test
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

// This test is mainly for user namespaces to verify that new directories
// are created as the remapped root uid/gid pair
func (s *DockerSuite) TestBuildAddToNewDestination(c *check.C) {
	testRequires(c, DaemonIsLinux) // Linux specific test
	name := "testaddtonewdest"
	ctx, err := fakeContext(`FROM busybox
ADD . /new_dir
RUN ls -l /
RUN [ $(ls -l / | grep new_dir | awk '{print $3":"$4}') = 'root:root' ]`,
		map[string]string{
			"test_dir/test_file": "test file",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

// This test is mainly for user namespaces to verify that new directories
// are created as the remapped root uid/gid pair
func (s *DockerSuite) TestBuildCopyToNewParentDirectory(c *check.C) {
	testRequires(c, DaemonIsLinux) // Linux specific test
	name := "testcopytonewdir"
	ctx, err := fakeContext(`FROM busybox
COPY test_dir /new_dir
RUN ls -l /new_dir
RUN [ $(ls -l / | grep new_dir | awk '{print $3":"$4}') = 'root:root' ]`,
		map[string]string{
			"test_dir/test_file": "test file",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

// This test is mainly for user namespaces to verify that new directories
// are created as the remapped root uid/gid pair
func (s *DockerSuite) TestBuildWorkdirIsContainerRoot(c *check.C) {
	testRequires(c, DaemonIsLinux) // Linux specific test
	name := "testworkdirownership"
	if _, err := buildImage(name, `FROM busybox
WORKDIR /new_dir
RUN ls -l /
RUN [ $(ls -l / | grep new_dir | awk '{print $3":"$4}') = 'root:root' ]`, true); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildAddFileWithWhitespace(c *check.C) {
	testRequires(c, DaemonIsLinux) // Not currently passing on Windows
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

func (s *DockerSuite) TestBuildCopyFileWithWhitespace(c *check.C) {
	dockerfile := `FROM busybox
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
RUN [ $(cat "/test dir/test_file6") = 'test6' ]`

	if daemonPlatform == "windows" {
		dockerfile = `FROM ` + WindowsBaseImage + `
RUN mkdir "C:/test dir"
RUN mkdir "C:/test_dir"
COPY [ "test file1", "/test_file1" ]
COPY [ "test_file2", "/test file2" ]
COPY [ "test file3", "/test file3" ]
COPY [ "test dir/test_file4", "/test_dir/test_file4" ]
COPY [ "test_dir/test_file5", "/test dir/test_file5" ]
COPY [ "test dir/test_file6", "/test dir/test_file6" ]
RUN find "test1" "C:/test_file1"
RUN find "test2" "C:/test file2"
RUN find "test3" "C:/test file3"
RUN find "test4" "C:/test_dir/test_file4"
RUN find "test5" "C:/test dir/test_file5"
RUN find "test6" "C:/test dir/test_file6"`
	}

	name := "testcopyfilewithwhitespace"
	ctx, err := fakeContext(dockerfile,
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

func (s *DockerSuite) TestBuildCopyWildcard(c *check.C) {
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
	RUN [ "mkdir",  "/tmp1" ]
	COPY dir* /tmp1/
	RUN ls /tmp1/dirt /tmp1/nested_file /tmp1/nested_dir/nest_nest_file
	RUN [ "mkdir",  "/tmp2" ]
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

func (s *DockerSuite) TestBuildCopyWildcardInName(c *check.C) {
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

func (s *DockerSuite) TestBuildAddSingleFileToNonExistingDir(c *check.C) {
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	name := "testaddetctoroot"

	ctx, err := fakeContext(`FROM `+minimalBaseImage()+`
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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

func (s *DockerSuite) TestBuildCopySingleFileToRoot(c *check.C) {
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	testRequires(c, DaemonIsLinux) // Linux specific test
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
	name := "testcopyetctoroot"

	ctx, err := fakeContext(`FROM `+minimalBaseImage()+`
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

func (s *DockerSuite) TestBuildAddBadLinks(c *check.C) {
	testRequires(c, DaemonIsLinux) // Not currently working on Windows

	dockerfile := `
		FROM scratch
		ADD links.tar /
		ADD foo.txt /symlink/
		`
	targetFile := "foo.txt"
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
	testRequires(c, DaemonIsLinux) // ln not implemented on Windows busybox
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

// Issue #5270 - ensure we throw a better error than "unexpected EOF"
// when we can't access files in the context.
func (s *DockerSuite) TestBuildWithInaccessibleFilesInContext(c *check.C) {
	testRequires(c, DaemonIsLinux, UnixCli) // test uses chown/chmod: not available on windows

	{
		name := "testbuildinaccessiblefiles"
		ctx, err := fakeContext("FROM scratch\nADD . /foo/", map[string]string{"fileWithoutReadAccess": "foo"})
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()
		// This is used to ensure we detect inaccessible files early during build in the cli client
		pathToFileWithoutReadAccess := filepath.Join(ctx.Dir, "fileWithoutReadAccess")

		if err = os.Chown(pathToFileWithoutReadAccess, 0, 0); err != nil {
			c.Fatalf("failed to chown file to root: %s", err)
		}
		if err = os.Chmod(pathToFileWithoutReadAccess, 0700); err != nil {
			c.Fatalf("failed to chmod file to 700: %s", err)
		}
		buildCmd := exec.Command("su", "unprivilegeduser", "-c", fmt.Sprintf("%s build -t %s .", dockerBinary, name))
		buildCmd.Dir = ctx.Dir
		out, _, err := runCommandWithOutput(buildCmd)
		if err == nil {
			c.Fatalf("build should have failed: %s %s", err, out)
		}

		// check if we've detected the failure before we started building
		if !strings.Contains(out, "no permission to read from ") {
			c.Fatalf("output should've contained the string: no permission to read from but contained: %s", out)
		}

		if !strings.Contains(out, "Error checking context") {
			c.Fatalf("output should've contained the string: Error checking context")
		}
	}
	{
		name := "testbuildinaccessibledirectory"
		ctx, err := fakeContext("FROM scratch\nADD . /foo/", map[string]string{"directoryWeCantStat/bar": "foo"})
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()
		// This is used to ensure we detect inaccessible directories early during build in the cli client
		pathToDirectoryWithoutReadAccess := filepath.Join(ctx.Dir, "directoryWeCantStat")
		pathToFileInDirectoryWithoutReadAccess := filepath.Join(pathToDirectoryWithoutReadAccess, "bar")

		if err = os.Chown(pathToDirectoryWithoutReadAccess, 0, 0); err != nil {
			c.Fatalf("failed to chown directory to root: %s", err)
		}
		if err = os.Chmod(pathToDirectoryWithoutReadAccess, 0444); err != nil {
			c.Fatalf("failed to chmod directory to 444: %s", err)
		}
		if err = os.Chmod(pathToFileInDirectoryWithoutReadAccess, 0700); err != nil {
			c.Fatalf("failed to chmod file to 700: %s", err)
		}

		buildCmd := exec.Command("su", "unprivilegeduser", "-c", fmt.Sprintf("%s build -t %s .", dockerBinary, name))
		buildCmd.Dir = ctx.Dir
		out, _, err := runCommandWithOutput(buildCmd)
		if err == nil {
			c.Fatalf("build should have failed: %s %s", err, out)
		}

		// check if we've detected the failure before we started building
		if !strings.Contains(out, "can't stat") {
			c.Fatalf("output should've contained the string: can't access %s", out)
		}

		if !strings.Contains(out, "Error checking context") {
			c.Fatalf("output should've contained the string: Error checking context\ngot:%s", out)
		}

	}
	{
		name := "testlinksok"
		ctx, err := fakeContext("FROM scratch\nADD . /foo/", nil)
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()

		target := "../../../../../../../../../../../../../../../../../../../azA"
		if err := os.Symlink(filepath.Join(ctx.Dir, "g"), target); err != nil {
			c.Fatal(err)
		}
		defer os.Remove(target)
		// This is used to ensure we don't follow links when checking if everything in the context is accessible
		// This test doesn't require that we run commands as an unprivileged user
		if _, err := buildImageFromContext(name, ctx, true); err != nil {
			c.Fatal(err)
		}
	}
	{
		name := "testbuildignoredinaccessible"
		ctx, err := fakeContext("FROM scratch\nADD . /foo/",
			map[string]string{
				"directoryWeCantStat/bar": "foo",
				".dockerignore":           "directoryWeCantStat",
			})
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()
		// This is used to ensure we don't try to add inaccessible files when they are ignored by a .dockerignore pattern
		pathToDirectoryWithoutReadAccess := filepath.Join(ctx.Dir, "directoryWeCantStat")
		pathToFileInDirectoryWithoutReadAccess := filepath.Join(pathToDirectoryWithoutReadAccess, "bar")
		if err = os.Chown(pathToDirectoryWithoutReadAccess, 0, 0); err != nil {
			c.Fatalf("failed to chown directory to root: %s", err)
		}
		if err = os.Chmod(pathToDirectoryWithoutReadAccess, 0444); err != nil {
			c.Fatalf("failed to chmod directory to 444: %s", err)
		}
		if err = os.Chmod(pathToFileInDirectoryWithoutReadAccess, 0700); err != nil {
			c.Fatalf("failed to chmod file to 700: %s", err)
		}

		result := icmd.RunCmd(icmd.Cmd{
			Dir: ctx.Dir,
			Command: []string{"su", "unprivilegeduser", "-c",
				fmt.Sprintf("%s build -t %s .", dockerBinary, name)},
		})
		result.Assert(c, icmd.Expected{})
	}
}

func (s *DockerSuite) TestBuildForceRm(c *check.C) {
	containerCountBefore, err := getContainerCount()
	if err != nil {
		c.Fatalf("failed to get the container count: %s", err)
	}
	name := "testbuildforcerm"

	ctx, err := fakeContext(`FROM `+minimalBaseImage()+`
	RUN true
	RUN thiswillfail`, nil)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	dockerCmdInDir(c, ctx.Dir, "build", "-t", name, "--force-rm", ".")

	containerCountAfter, err := getContainerCount()
	if err != nil {
		c.Fatalf("failed to get the container count: %s", err)
	}

	if containerCountBefore != containerCountAfter {
		c.Fatalf("--force-rm shouldn't have left containers behind")
	}

}

func (s *DockerSuite) TestBuildRm(c *check.C) {
	name := "testbuildrm"

	ctx, err := fakeContext(`FROM `+minimalBaseImage()+`
	ADD foo /
	ADD foo /`, map[string]string{"foo": "bar"})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "--rm", "-t", name, ".")

		if err != nil {
			c.Fatal("failed to build the image", out)
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore != containerCountAfter {
			c.Fatalf("-rm shouldn't have left containers behind")
		}
		deleteImages(name)
	}

	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", name, ".")

		if err != nil {
			c.Fatal("failed to build the image", out)
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore != containerCountAfter {
			c.Fatalf("--rm shouldn't have left containers behind")
		}
		deleteImages(name)
	}

	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "--rm=false", "-t", name, ".")

		if err != nil {
			c.Fatal("failed to build the image", out)
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			c.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore == containerCountAfter {
			c.Fatalf("--rm=false should have left containers behind")
		}
		deleteImages(name)

	}

}

func (s *DockerSuite) TestBuildWithVolumes(c *check.C) {
	testRequires(c, DaemonIsLinux) // Invalid volume paths on Windows
	var (
		result   map[string]map[string]struct{}
		name     = "testbuildvolumes"
		emptyMap = make(map[string]struct{})
		expected = map[string]map[string]struct{}{
			"/test1":  emptyMap,
			"/test2":  emptyMap,
			"/test3":  emptyMap,
			"/test4":  emptyMap,
			"/test5":  emptyMap,
			"/test6":  emptyMap,
			"[/test7": emptyMap,
			"/test8]": emptyMap,
		}
	)
	_, err := buildImage(name,
		`FROM scratch
		VOLUME /test1
		VOLUME /test2
    VOLUME /test3 /test4
    VOLUME ["/test5", "/test6"]
    VOLUME [/test7 /test8]
    `,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectFieldJSON(c, name, "Config.Volumes")

	err = json.Unmarshal([]byte(res), &result)
	if err != nil {
		c.Fatal(err)
	}

	equal := reflect.DeepEqual(&result, &expected)

	if !equal {
		c.Fatalf("Volumes %s, expected %s", result, expected)
	}

}

func (s *DockerSuite) TestBuildMaintainer(c *check.C) {
	name := "testbuildmaintainer"

	expected := "dockerio"
	_, err := buildImage(name,
		`FROM `+minimalBaseImage()+`
        MAINTAINER dockerio`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Author")
	if res != expected {
		c.Fatalf("Maintainer %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildUser(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuilduser"
	expected := "dockerio"
	_, err := buildImage(name,
		`FROM busybox
		RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
		USER dockerio
		RUN [ $(whoami) = 'dockerio' ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.User")
	if res != expected {
		c.Fatalf("User %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildRelativeWorkdir(c *check.C) {
	name := "testbuildrelativeworkdir"

	var (
		expected1     string
		expected2     string
		expected3     string
		expected4     string
		expectedFinal string
	)

	if daemonPlatform == "windows" {
		expected1 = `C:/`
		expected2 = `C:/test1`
		expected3 = `C:/test2`
		expected4 = `C:/test2/test3`
		expectedFinal = `C:\test2\test3` // Note inspect is going to return Windows paths, as it's not in busybox
	} else {
		expected1 = `/`
		expected2 = `/test1`
		expected3 = `/test2`
		expected4 = `/test2/test3`
		expectedFinal = `/test2/test3`
	}

	_, err := buildImage(name,
		`FROM busybox
		RUN sh -c "[ "$PWD" = "`+expected1+`" ]"
		WORKDIR test1
		RUN sh -c "[ "$PWD" = "`+expected2+`" ]"
		WORKDIR /test2
		RUN sh -c "[ "$PWD" = "`+expected3+`" ]"
		WORKDIR test3
		RUN sh -c "[ "$PWD" = "`+expected4+`" ]"`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.WorkingDir")
	if res != expectedFinal {
		c.Fatalf("Workdir %s, expected %s", res, expectedFinal)
	}
}

// #22181 Regression test. Single end-to-end test of using
// Windows semantics. Most path handling verifications are in unit tests
func (s *DockerSuite) TestBuildWindowsWorkdirProcessing(c *check.C) {
	testRequires(c, DaemonIsWindows)
	name := "testbuildwindowsworkdirprocessing"
	_, err := buildImage(name,
		`FROM busybox
		WORKDIR C:\\foo
		WORKDIR bar
		RUN sh -c "[ "$PWD" = "C:/foo/bar" ]"
		`,
		true)
	if err != nil {
		c.Fatal(err)
	}
}

// #22181 Regression test. Most paths handling verifications are in unit test.
// One functional test for end-to-end
func (s *DockerSuite) TestBuildWindowsAddCopyPathProcessing(c *check.C) {
	testRequires(c, DaemonIsWindows)
	name := "testbuildwindowsaddcopypathprocessing"
	// TODO Windows (@jhowardmsft). Needs a follow-up PR to 22181 to
	// support backslash such as .\\ being equivalent to ./ and c:\\ being
	// equivalent to c:/. This is not currently (nor ever has been) supported
	// by docker on the Windows platform.
	dockerfile := `
		FROM busybox
			# No trailing slash on COPY/ADD
			# Results in dir being changed to a file
			WORKDIR /wc1
			COPY wc1 c:/wc1
			WORKDIR /wc2
			ADD wc2 c:/wc2
			WORKDIR c:/
			RUN sh -c "[ $(cat c:/wc1) = 'hellowc1' ]"
			RUN sh -c "[ $(cat c:/wc2) = 'worldwc2' ]"

			# Trailing slash on COPY/ADD, Windows-style path.
			WORKDIR /wd1
			COPY wd1 c:/wd1/
			WORKDIR /wd2
			ADD wd2 c:/wd2/
			RUN sh -c "[ $(cat c:/wd1/wd1) = 'hellowd1' ]"
			RUN sh -c "[ $(cat c:/wd2/wd2) = 'worldwd2' ]"
			`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"wc1": "hellowc1",
		"wc2": "worldwc2",
		"wd1": "hellowd1",
		"wd2": "worldwd2",
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

func (s *DockerSuite) TestBuildWorkdirWithEnvVariables(c *check.C) {
	name := "testbuildworkdirwithenvvariables"

	var expected string
	if daemonPlatform == "windows" {
		expected = `C:\test1\test2`
	} else {
		expected = `/test1/test2`
	}

	_, err := buildImage(name,
		`FROM busybox
		ENV DIRPATH /test1
		ENV SUBDIRNAME test2
		WORKDIR $DIRPATH
		WORKDIR $SUBDIRNAME/$MISSING_VAR`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.WorkingDir")
	if res != expected {
		c.Fatalf("Workdir %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildRelativeCopy(c *check.C) {
	// cat /test1/test2/foo gets permission denied for the user
	testRequires(c, NotUserNamespace)

	var expected string
	if daemonPlatform == "windows" {
		expected = `C:/test1/test2`
	} else {
		expected = `/test1/test2`
	}

	name := "testbuildrelativecopy"
	dockerfile := `
		FROM busybox
			WORKDIR /test1
			WORKDIR test2
			RUN sh -c "[ "$PWD" = '` + expected + `' ]"
			COPY foo ./
			RUN sh -c "[ $(cat /test1/test2/foo) = 'hello' ]"
			ADD foo ./bar/baz
			RUN sh -c "[ $(cat /test1/test2/bar/baz) = 'hello' ]"
			COPY foo ./bar/baz2
			RUN sh -c "[ $(cat /test1/test2/bar/baz2) = 'hello' ]"
			WORKDIR ..
			COPY foo ./
			RUN sh -c "[ $(cat /test1/foo) = 'hello' ]"
			COPY foo /test3/
			RUN sh -c "[ $(cat /test3/foo) = 'hello' ]"
			WORKDIR /test4
			COPY . .
			RUN sh -c "[ $(cat /test4/foo) = 'hello' ]"
			WORKDIR /test5/test6
			COPY foo ../
			RUN sh -c "[ $(cat /test5/foo) = 'hello' ]"
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

func (s *DockerSuite) TestBuildBlankName(c *check.C) {
	name := "testbuildblankname"
	_, _, stderr, err := buildImageWithStdoutStderr(name,
		`FROM busybox
		ENV =`,
		true)
	if err == nil {
		c.Fatal("Build was supposed to fail but didn't")
	}
	if !strings.Contains(stderr, "ENV names can not be blank") {
		c.Fatalf("Missing error message, got: %s", stderr)
	}

	_, _, stderr, err = buildImageWithStdoutStderr(name,
		`FROM busybox
		LABEL =`,
		true)
	if err == nil {
		c.Fatal("Build was supposed to fail but didn't")
	}
	if !strings.Contains(stderr, "LABEL names can not be blank") {
		c.Fatalf("Missing error message, got: %s", stderr)
	}

	_, _, stderr, err = buildImageWithStdoutStderr(name,
		`FROM busybox
		ARG =foo`,
		true)
	if err == nil {
		c.Fatal("Build was supposed to fail but didn't")
	}
	if !strings.Contains(stderr, "ARG names can not be blank") {
		c.Fatalf("Missing error message, got: %s", stderr)
	}
}

func (s *DockerSuite) TestBuildEnv(c *check.C) {
	testRequires(c, DaemonIsLinux) // ENV expansion is different in Windows
	name := "testbuildenv"
	expected := "[PATH=/test:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin PORT=2375]"
	_, err := buildImage(name,
		`FROM busybox
		ENV PATH /test:$PATH
        ENV PORT 2375
		RUN [ $(env | grep PORT) = 'PORT=2375' ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.Env")
	if res != expected {
		c.Fatalf("Env %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildPATH(c *check.C) {
	testRequires(c, DaemonIsLinux) // ENV expansion is different in Windows

	defPath := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	fn := func(dockerfile string, exp string) {
		_, err := buildImage("testbldpath", dockerfile, true)
		c.Assert(err, check.IsNil)

		res := inspectField(c, "testbldpath", "Config.Env")

		if res != exp {
			c.Fatalf("Env %q, expected %q for dockerfile:%q", res, exp, dockerfile)
		}
	}

	tests := []struct{ dockerfile, exp string }{
		{"FROM scratch\nMAINTAINER me", "[PATH=" + defPath + "]"},
		{"FROM busybox\nMAINTAINER me", "[PATH=" + defPath + "]"},
		{"FROM scratch\nENV FOO=bar", "[PATH=" + defPath + " FOO=bar]"},
		{"FROM busybox\nENV FOO=bar", "[PATH=" + defPath + " FOO=bar]"},
		{"FROM scratch\nENV PATH=/test", "[PATH=/test]"},
		{"FROM busybox\nENV PATH=/test", "[PATH=/test]"},
		{"FROM scratch\nENV PATH=''", "[PATH=]"},
		{"FROM busybox\nENV PATH=''", "[PATH=]"},
	}

	for _, test := range tests {
		fn(test.dockerfile, test.exp)
	}
}

func (s *DockerSuite) TestBuildContextCleanup(c *check.C) {
	testRequires(c, SameHostDaemon)

	name := "testbuildcontextcleanup"
	entries, err := ioutil.ReadDir(filepath.Join(dockerBasePath, "tmp"))
	if err != nil {
		c.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	_, err = buildImage(name,
		`FROM `+minimalBaseImage()+`
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	entriesFinal, err := ioutil.ReadDir(filepath.Join(dockerBasePath, "tmp"))
	if err != nil {
		c.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	if err = compareDirectoryEntries(entries, entriesFinal); err != nil {
		c.Fatalf("context should have been deleted, but wasn't")
	}

}

func (s *DockerSuite) TestBuildContextCleanupFailedBuild(c *check.C) {
	testRequires(c, SameHostDaemon)

	name := "testbuildcontextcleanup"
	entries, err := ioutil.ReadDir(filepath.Join(dockerBasePath, "tmp"))
	if err != nil {
		c.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	_, err = buildImage(name,
		`FROM `+minimalBaseImage()+`
	RUN /non/existing/command`,
		true)
	if err == nil {
		c.Fatalf("expected build to fail, but it didn't")
	}
	entriesFinal, err := ioutil.ReadDir(filepath.Join(dockerBasePath, "tmp"))
	if err != nil {
		c.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	if err = compareDirectoryEntries(entries, entriesFinal); err != nil {
		c.Fatalf("context should have been deleted, but wasn't")
	}

}

func (s *DockerSuite) TestBuildCmd(c *check.C) {
	name := "testbuildcmd"

	expected := "[/bin/echo Hello World]"
	_, err := buildImage(name,
		`FROM `+minimalBaseImage()+`
        CMD ["/bin/echo", "Hello World"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.Cmd")
	if res != expected {
		c.Fatalf("Cmd %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildExpose(c *check.C) {
	testRequires(c, DaemonIsLinux) // Expose not implemented on Windows
	name := "testbuildexpose"
	expected := "map[2375/tcp:{}]"
	_, err := buildImage(name,
		`FROM scratch
        EXPOSE 2375`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.ExposedPorts")
	if res != expected {
		c.Fatalf("Exposed ports %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildExposeMorePorts(c *check.C) {
	testRequires(c, DaemonIsLinux) // Expose not implemented on Windows
	// start building docker file with a large number of ports
	portList := make([]string, 50)
	line := make([]string, 100)
	expectedPorts := make([]int, len(portList)*len(line))
	for i := 0; i < len(portList); i++ {
		for j := 0; j < len(line); j++ {
			p := i*len(line) + j + 1
			line[j] = strconv.Itoa(p)
			expectedPorts[p-1] = p
		}
		if i == len(portList)-1 {
			portList[i] = strings.Join(line, " ")
		} else {
			portList[i] = strings.Join(line, " ") + ` \`
		}
	}

	dockerfile := `FROM scratch
	EXPOSE {{range .}} {{.}}
	{{end}}`
	tmpl := template.Must(template.New("dockerfile").Parse(dockerfile))
	buf := bytes.NewBuffer(nil)
	tmpl.Execute(buf, portList)

	name := "testbuildexpose"
	_, err := buildImage(name, buf.String(), true)
	if err != nil {
		c.Fatal(err)
	}

	// check if all the ports are saved inside Config.ExposedPorts
	res := inspectFieldJSON(c, name, "Config.ExposedPorts")
	var exposedPorts map[string]interface{}
	if err := json.Unmarshal([]byte(res), &exposedPorts); err != nil {
		c.Fatal(err)
	}

	for _, p := range expectedPorts {
		ep := fmt.Sprintf("%d/tcp", p)
		if _, ok := exposedPorts[ep]; !ok {
			c.Errorf("Port(%s) is not exposed", ep)
		} else {
			delete(exposedPorts, ep)
		}
	}
	if len(exposedPorts) != 0 {
		c.Errorf("Unexpected extra exposed ports %v", exposedPorts)
	}
}

func (s *DockerSuite) TestBuildExposeOrder(c *check.C) {
	testRequires(c, DaemonIsLinux) // Expose not implemented on Windows
	buildID := func(name, exposed string) string {
		_, err := buildImage(name, fmt.Sprintf(`FROM scratch
		EXPOSE %s`, exposed), true)
		if err != nil {
			c.Fatal(err)
		}
		id := inspectField(c, name, "Id")
		return id
	}

	id1 := buildID("testbuildexpose1", "80 2375")
	id2 := buildID("testbuildexpose2", "2375 80")
	if id1 != id2 {
		c.Errorf("EXPOSE should invalidate the cache only when ports actually changed")
	}
}

func (s *DockerSuite) TestBuildExposeUpperCaseProto(c *check.C) {
	testRequires(c, DaemonIsLinux) // Expose not implemented on Windows
	name := "testbuildexposeuppercaseproto"
	expected := "map[5678/udp:{}]"
	_, err := buildImage(name,
		`FROM scratch
        EXPOSE 5678/UDP`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.ExposedPorts")
	if res != expected {
		c.Fatalf("Exposed ports %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildEmptyEntrypointInheritance(c *check.C) {
	name := "testbuildentrypointinheritance"
	name2 := "testbuildentrypointinheritance2"

	_, err := buildImage(name,
		`FROM busybox
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.Entrypoint")

	expected := "[/bin/echo]"
	if res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

	_, err = buildImage(name2,
		fmt.Sprintf(`FROM %s
        ENTRYPOINT []`, name),
		true)
	if err != nil {
		c.Fatal(err)
	}
	res = inspectField(c, name2, "Config.Entrypoint")

	expected = "[]"

	if res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

}

func (s *DockerSuite) TestBuildEmptyEntrypoint(c *check.C) {
	name := "testbuildentrypoint"
	expected := "[]"

	_, err := buildImage(name,
		`FROM busybox
        ENTRYPOINT []`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.Entrypoint")
	if res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

}

func (s *DockerSuite) TestBuildEntrypoint(c *check.C) {
	name := "testbuildentrypoint"

	expected := "[/bin/echo]"
	_, err := buildImage(name,
		`FROM `+minimalBaseImage()+`
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.Entrypoint")
	if res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

}

// #6445 ensure ONBUILD triggers aren't committed to grandchildren
func (s *DockerSuite) TestBuildOnBuildLimitedInheritence(c *check.C) {
	var (
		out2, out3 string
	)
	{
		name1 := "testonbuildtrigger1"
		dockerfile1 := `
		FROM busybox
		RUN echo "GRANDPARENT"
		ONBUILD RUN echo "ONBUILD PARENT"
		`
		ctx, err := fakeContext(dockerfile1, nil)
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()

		out1, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", name1, ".")
		if err != nil {
			c.Fatalf("build failed to complete: %s, %v", out1, err)
		}
	}
	{
		name2 := "testonbuildtrigger2"
		dockerfile2 := `
		FROM testonbuildtrigger1
		`
		ctx, err := fakeContext(dockerfile2, nil)
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()

		out2, _, err = dockerCmdInDir(c, ctx.Dir, "build", "-t", name2, ".")
		if err != nil {
			c.Fatalf("build failed to complete: %s, %v", out2, err)
		}
	}
	{
		name3 := "testonbuildtrigger3"
		dockerfile3 := `
		FROM testonbuildtrigger2
		`
		ctx, err := fakeContext(dockerfile3, nil)
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()

		out3, _, err = dockerCmdInDir(c, ctx.Dir, "build", "-t", name3, ".")
		if err != nil {
			c.Fatalf("build failed to complete: %s, %v", out3, err)
		}

	}

	// ONBUILD should be run in second build.
	if !strings.Contains(out2, "ONBUILD PARENT") {
		c.Fatalf("ONBUILD instruction did not run in child of ONBUILD parent")
	}

	// ONBUILD should *not* be run in third build.
	if strings.Contains(out3, "ONBUILD PARENT") {
		c.Fatalf("ONBUILD instruction ran in grandchild of ONBUILD parent")
	}

}

func (s *DockerSuite) TestBuildWithCache(c *check.C) {
	testRequires(c, DaemonIsLinux) // Expose not implemented on Windows
	name := "testbuildwithcache"
	id1, err := buildImage(name,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImage(name,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	if id1 != id2 {
		c.Fatal("The cache should have been used but hasn't.")
	}
}

func (s *DockerSuite) TestBuildWithoutCache(c *check.C) {
	testRequires(c, DaemonIsLinux) // Expose not implemented on Windows
	name := "testbuildwithoutcache"
	name2 := "testbuildwithoutcache2"
	id1, err := buildImage(name,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	id2, err := buildImage(name2,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		false)
	if err != nil {
		c.Fatal(err)
	}
	if id1 == id2 {
		c.Fatal("The cache should have been invalided but hasn't.")
	}
}

func (s *DockerSuite) TestBuildConditionalCache(c *check.C) {
	name := "testbuildconditionalcache"

	dockerfile := `
		FROM busybox
        ADD foo /tmp/`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatalf("Error building #1: %s", err)
	}

	if err := ctx.Add("foo", "bye"); err != nil {
		c.Fatalf("Error modifying foo: %s", err)
	}

	id2, err := buildImageFromContext(name, ctx, false)
	if err != nil {
		c.Fatalf("Error building #2: %s", err)
	}
	if id2 == id1 {
		c.Fatal("Should not have used the cache")
	}

	id3, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatalf("Error building #3: %s", err)
	}
	if id3 != id2 {
		c.Fatal("Should have used the cache")
	}
}

func (s *DockerSuite) TestBuildAddLocalFileWithCache(c *check.C) {
	// local files are not owned by the correct user
	testRequires(c, NotUserNamespace)
	name := "testbuildaddlocalfilewithcache"
	name2 := "testbuildaddlocalfilewithcache2"
	dockerfile := `
		FROM busybox
        MAINTAINER dockerio
        ADD foo /usr/lib/bla/bar
		RUN sh -c "[ $(cat /usr/lib/bla/bar) = "hello" ]"`
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
	name := "testbuildaddmultiplelocalfilewithcache"
	name2 := "testbuildaddmultiplelocalfilewithcache2"
	dockerfile := `
		FROM busybox
        MAINTAINER dockerio
        ADD foo Dockerfile /usr/lib/bla/
		RUN sh -c "[ $(cat /usr/lib/bla/foo) = "hello" ]"`
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
	name := "testbuildaddlocalfilewithoutcache"
	name2 := "testbuildaddlocalfilewithoutcache2"
	dockerfile := `
		FROM busybox
        MAINTAINER dockerio
        ADD foo /usr/lib/bla/bar
		RUN sh -c "[ $(cat /usr/lib/bla/bar) = "hello" ]"`
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

func (s *DockerSuite) TestBuildCopyDirButNotFile(c *check.C) {
	name := "testbuildcopydirbutnotfile"
	name2 := "testbuildcopydirbutnotfile2"

	dockerfile := `
        FROM ` + minimalBaseImage() + `
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

func (s *DockerSuite) TestBuildAddCurrentDirWithCache(c *check.C) {
	name := "testbuildaddcurrentdirwithcache"
	name2 := name + "2"
	name3 := name + "3"
	name4 := name + "4"
	dockerfile := `
        FROM ` + minimalBaseImage() + `
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
	name := "testbuildaddcurrentdirwithoutcache"
	name2 := "testbuildaddcurrentdirwithoutcache2"
	dockerfile := `
        FROM ` + minimalBaseImage() + `
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
	name := "testbuildaddremotefilewithcache"
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	id1, err := buildImage(name,
		fmt.Sprintf(`FROM `+minimalBaseImage()+`
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImage(name,
		fmt.Sprintf(`FROM `+minimalBaseImage()+`
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
		fmt.Sprintf(`FROM `+minimalBaseImage()+`
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		true)
	if err != nil {
		c.Fatal(err)
	}
	id2, err := buildImage(name2,
		fmt.Sprintf(`FROM `+minimalBaseImage()+`
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
	name := "testbuildaddremotefilemtime"
	name2 := name + "2"
	name3 := name + "3"

	files := map[string]string{"baz": "hello"}
	server, err := fakeStorage(files)
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(fmt.Sprintf(`FROM `+minimalBaseImage()+`
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

	ctx2, err := fakeContext(fmt.Sprintf(`FROM `+minimalBaseImage()+`
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
	name := "testbuildaddlocalandremotefilewithcache"
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(fmt.Sprintf(`FROM `+minimalBaseImage()+`
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

func testContextTar(c *check.C, compression archive.Compression) {
	ctx, err := fakeContext(
		`FROM busybox
ADD foo /foo
CMD ["cat", "/foo"]`,
		map[string]string{
			"foo": "bar",
		},
	)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	context, err := archive.Tar(ctx.Dir, compression)
	if err != nil {
		c.Fatalf("failed to build context tar: %v", err)
	}
	name := "contexttar"
	buildCmd := exec.Command(dockerBinary, "build", "-t", name, "-")
	buildCmd.Stdin = context

	if out, _, err := runCommandWithOutput(buildCmd); err != nil {
		c.Fatalf("build failed to complete: %v %v", out, err)
	}
}

func (s *DockerSuite) TestBuildContextTarGzip(c *check.C) {
	testContextTar(c, archive.Gzip)
}

func (s *DockerSuite) TestBuildContextTarNoCompression(c *check.C) {
	testContextTar(c, archive.Uncompressed)
}

func (s *DockerSuite) TestBuildNoContext(c *check.C) {
	buildCmd := exec.Command(dockerBinary, "build", "-t", "nocontext", "-")
	buildCmd.Stdin = strings.NewReader(
		`FROM busybox
		CMD ["echo", "ok"]`)

	if out, _, err := runCommandWithOutput(buildCmd); err != nil {
		c.Fatalf("build failed to complete: %v %v", out, err)
	}

	if out, _ := dockerCmd(c, "run", "--rm", "nocontext"); out != "ok\n" {
		c.Fatalf("run produced invalid output: %q, expected %q", out, "ok")
	}
}

// TODO: TestCaching
func (s *DockerSuite) TestBuildAddLocalAndRemoteFilesWithoutCache(c *check.C) {
	name := "testbuildaddlocalandremotefilewithoutcache"
	name2 := "testbuildaddlocalandremotefilewithoutcache2"
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(fmt.Sprintf(`FROM `+minimalBaseImage()+`
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
		c.Fatal("The cache should have been invalidated but hasn't.")
	}
}

func (s *DockerSuite) TestBuildWithVolumeOwnership(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildimg"

	_, err := buildImage(name,
		`FROM busybox:latest
        RUN mkdir /test && chown daemon:daemon /test && chmod 0600 /test
        VOLUME /test`,
		true)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--rm", "testbuildimg", "ls", "-la", "/test")

	if expected := "drw-------"; !strings.Contains(out, expected) {
		c.Fatalf("expected %s received %s", expected, out)
	}

	if expected := "daemon   daemon"; !strings.Contains(out, expected) {
		c.Fatalf("expected %s received %s", expected, out)
	}

}

// testing #1405 - config.Cmd does not get cleaned up if
// utilizing cache
func (s *DockerSuite) TestBuildEntrypointRunCleanup(c *check.C) {
	name := "testbuildcmdcleanup"
	if _, err := buildImage(name,
		`FROM busybox
        RUN echo "hello"`,
		true); err != nil {
		c.Fatal(err)
	}

	ctx, err := fakeContext(`FROM busybox
        RUN echo "hello"
        ADD foo /foo
        ENTRYPOINT ["/bin/echo"]`,
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
	res := inspectField(c, name, "Config.Cmd")
	// Cmd must be cleaned up
	if res != "[]" {
		c.Fatalf("Cmd %s, expected nil", res)
	}
}

func (s *DockerSuite) TestBuildAddFileNotFound(c *check.C) {
	name := "testbuildaddnotfound"
	expected := "foo: no such file or directory"

	if daemonPlatform == "windows" {
		expected = "foo: The system cannot find the file specified"
	}

	ctx, err := fakeContext(`FROM `+minimalBaseImage()+`
        ADD foo /usr/local/bar`,
		map[string]string{"bar": "hello"})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		if !strings.Contains(err.Error(), expected) {
			c.Fatalf("Wrong error %v, must be about missing foo file or directory", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}
}

func (s *DockerSuite) TestBuildInheritance(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildinheritance"

	_, err := buildImage(name,
		`FROM scratch
		EXPOSE 2375`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	ports1 := inspectField(c, name, "Config.ExposedPorts")

	_, err = buildImage(name,
		fmt.Sprintf(`FROM %s
		ENTRYPOINT ["/bin/echo"]`, name),
		true)
	if err != nil {
		c.Fatal(err)
	}

	res := inspectField(c, name, "Config.Entrypoint")
	if expected := "[/bin/echo]"; res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}
	ports2 := inspectField(c, name, "Config.ExposedPorts")
	if ports1 != ports2 {
		c.Fatalf("Ports must be same: %s != %s", ports1, ports2)
	}
}

func (s *DockerSuite) TestBuildFails(c *check.C) {
	name := "testbuildfails"
	_, err := buildImage(name,
		`FROM busybox
		RUN sh -c "exit 23"`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "returned a non-zero code: 23") {
			c.Fatalf("Wrong error %v, must be about non-zero code 23", err)
		}
	} else {
		c.Fatal("Error must not be nil")
	}
}

func (s *DockerSuite) TestBuildOnBuild(c *check.C) {
	name := "testbuildonbuild"
	_, err := buildImage(name,
		`FROM busybox
		ONBUILD RUN touch foobar`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	_, err = buildImage(name,
		fmt.Sprintf(`FROM %s
		RUN [ -f foobar ]`, name),
		true)
	if err != nil {
		c.Fatal(err)
	}
}

// gh #2446
func (s *DockerSuite) TestBuildAddToSymlinkDest(c *check.C) {
	makeLink := `ln -s /foo /bar`
	if daemonPlatform == "windows" {
		makeLink = `mklink /D C:\bar C:\foo`
	}
	name := "testbuildaddtosymlinkdest"
	ctx, err := fakeContext(`FROM busybox
        RUN sh -c "mkdir /foo"
        RUN `+makeLink+`
        ADD foo /bar/
        RUN sh -c "[ -f /bar/foo ]"
        RUN sh -c "[ -f /foo/foo ]"`,
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

func (s *DockerSuite) TestBuildEscapeWhitespace(c *check.C) {
	name := "testbuildescapewhitespace"

	_, err := buildImage(name, `
  # ESCAPE=\
  FROM busybox
  MAINTAINER "Docker \
IO <io@\
docker.com>"
  `, true)
	if err != nil {
		c.Fatal(err)
	}

	res := inspectField(c, name, "Author")

	if res != "\"Docker IO <io@docker.com>\"" {
		c.Fatalf("Parsed string did not match the escaped string. Got: %q", res)
	}

}

func (s *DockerSuite) TestBuildVerifyIntString(c *check.C) {
	// Verify that strings that look like ints are still passed as strings
	name := "testbuildstringing"

	_, err := buildImage(name, `
  FROM busybox
  MAINTAINER 123
  `, true)

	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "inspect", name)

	if !strings.Contains(out, "\"123\"") {
		c.Fatalf("Output does not contain the int as a string:\n%s", out)
	}

}

func (s *DockerSuite) TestBuildDockerignore(c *check.C) {
	name := "testbuilddockerignore"
	dockerfile := `
        FROM busybox
        ADD . /bla
		RUN sh -c "[[ -f /bla/src/x.go ]]"
		RUN sh -c "[[ -f /bla/Makefile ]]"
		RUN sh -c "[[ ! -e /bla/src/_vendor ]]"
		RUN sh -c "[[ ! -e /bla/.gitignore ]]"
		RUN sh -c "[[ ! -e /bla/README.md ]]"
		RUN sh -c "[[ ! -e /bla/dir/foo ]]"
		RUN sh -c "[[ ! -e /bla/foo ]]"
		RUN sh -c "[[ ! -e /bla/.git ]]"
		RUN sh -c "[[ ! -e v.cc ]]"
		RUN sh -c "[[ ! -e src/v.cc ]]"
		RUN sh -c "[[ ! -e src/_vendor/v.cc ]]"`
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
	name := "testbuilddockerignorecleanpaths"
	dockerfile := `
        FROM busybox
        ADD . /tmp/
        RUN sh -c "(! ls /tmp/foo) && (! ls /tmp/foo2) && (! ls /tmp/dir1/foo)"`
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
	name := "testbuilddockerignoreexceptions"
	dockerfile := `
        FROM busybox
        ADD . /bla
		RUN sh -c "[[ -f /bla/src/x.go ]]"
		RUN sh -c "[[ -f /bla/Makefile ]]"
		RUN sh -c "[[ ! -e /bla/src/_vendor ]]"
		RUN sh -c "[[ ! -e /bla/.gitignore ]]"
		RUN sh -c "[[ ! -e /bla/README.md ]]"
		RUN sh -c "[[  -e /bla/dir/dir/foo ]]"
		RUN sh -c "[[ ! -e /bla/dir/foo1 ]]"
		RUN sh -c "[[ -f /bla/dir/e ]]"
		RUN sh -c "[[ -f /bla/dir/e-dir/foo ]]"
		RUN sh -c "[[ ! -e /bla/foo ]]"
		RUN sh -c "[[ ! -e /bla/.git ]]"
		RUN sh -c "[[ -e /bla/dir/a.cc ]]"`
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
	name := "testbuilddockerignoredockerfile"
	dockerfile := `
        FROM busybox
		ADD . /tmp/
		RUN sh -c "! ls /tmp/Dockerfile"
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
	name := "testbuilddockerignoredockerfile"
	dockerfile := `
        FROM busybox
		ADD . /tmp/
		RUN ls /tmp/Dockerfile
		RUN sh -c "! ls /tmp/MyDockerfile"
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
	name := "testbuilddockerignoredockerignore"
	dockerfile := `
        FROM busybox
		ADD . /tmp/
		RUN sh -c "! ls /tmp/.dockerignore"
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
	name := "testbuilddockerignorewholedir"
	dockerfile := `
        FROM busybox
		COPY . /
		RUN sh -c "[[ ! -e /.gitignore ]]"
		RUN sh -c "[[ -f /Makefile ]]"`
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
	name := "testbuilddockerignorebadexclusion"
	dockerfile := `
        FROM busybox
		COPY . /
		RUN sh -c "[[ ! -e /.gitignore ]]"
		RUN sh -c "[[ -f /Makefile ]]"`
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
	dockerfile := `
        FROM busybox
		COPY . /
		RUN sh -c "[[ ! -e /.dockerignore ]]"
		RUN sh -c "[[ ! -e /Dockerfile ]]"
		RUN sh -c "[[ ! -e /file1 ]]"
		RUN sh -c "[[ ! -e /dir ]]"`

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
	dockerfile := `
        FROM busybox
		COPY . /
		#RUN sh -c "[[ -e /.dockerignore ]]"
		RUN sh -c "[[ -e /Dockerfile ]]           && \
		           [[ ! -e /file0 ]]              && \
		           [[ ! -e /dir1/file0 ]]         && \
		           [[ ! -e /dir2/file0 ]]         && \
		           [[ ! -e /file1 ]]              && \
		           [[ ! -e /dir1/file1 ]]         && \
		           [[ ! -e /dir1/dir2/file1 ]]    && \
		           [[ ! -e /dir1/file2 ]]         && \
		           [[   -e /dir1/dir2/file2 ]]    && \
		           [[ ! -e /dir1/dir2/file4 ]]    && \
		           [[ ! -e /dir1/dir2/file5 ]]    && \
		           [[ ! -e /dir1/dir2/file6 ]]    && \
		           [[ ! -e /dir1/dir3/file7 ]]    && \
		           [[ ! -e /dir1/dir3/file8 ]]    && \
		           [[   -e /dir1/dir3 ]]          && \
		           [[   -e /dir1/dir4 ]]          && \
		           [[ ! -e 'dir1/dir5/fileAA' ]]  && \
		           [[   -e 'dir1/dir5/fileAB' ]]  && \
		           [[   -e 'dir1/dir5/fileB' ]]"   # "." in pattern means nothing

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

func (s *DockerSuite) TestBuildLineBreak(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildlinebreak"
	_, err := buildImage(name,
		`FROM  busybox
RUN    sh -c 'echo root:testpass \
	> /tmp/passwd'
RUN    mkdir -p /var/run/sshd
RUN    sh -c "[ "$(cat /tmp/passwd)" = "root:testpass" ]"
RUN    sh -c "[ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]"`,
		true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildEOLInLine(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildeolinline"
	_, err := buildImage(name,
		`FROM   busybox
RUN    sh -c 'echo root:testpass > /tmp/passwd'
RUN    echo "foo \n bar"; echo "baz"
RUN    mkdir -p /var/run/sshd
RUN    sh -c "[ "$(cat /tmp/passwd)" = "root:testpass" ]"
RUN    sh -c "[ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]"`,
		true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildCommentsShebangs(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildcomments"
	_, err := buildImage(name,
		`FROM busybox
# This is an ordinary comment.
RUN { echo '#!/bin/sh'; echo 'echo hello world'; } > /hello.sh
RUN [ ! -x /hello.sh ]
# comment with line break \
RUN chmod +x /hello.sh
RUN [ -x /hello.sh ]
RUN [ "$(cat /hello.sh)" = $'#!/bin/sh\necho hello world' ]
RUN [ "$(/hello.sh)" = "hello world" ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildUsersAndGroups(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildusers"
	_, err := buildImage(name,
		`FROM busybox

# Make sure our defaults work
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)" = '0:0/root:root' ]

# TODO decide if "args.user = strconv.Itoa(syscall.Getuid())" is acceptable behavior for changeUser in sysvinit instead of "return nil" when "USER" isn't specified (so that we get the proper group list even if that is the empty list, even in the default case of not supplying an explicit USER to run as, which implies USER 0)
USER root
RUN [ "$(id -G):$(id -Gn)" = '0 10:root wheel' ]

# Setup dockerio user and group
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd && \
	echo 'dockerio:x:1001:' >> /etc/group

# Make sure we can switch to our user and all the information is exactly as we expect it to be
USER dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]

# Switch back to root and double check that worked exactly as we might expect it to
USER root
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '0:0/root:root/0 10:root wheel' ] && \
	# Add a "supplementary" group for our dockerio user \
	echo 'supplementary:x:1002:dockerio' >> /etc/group

# ... and then go verify that we get it like we expect
USER dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001 1002:dockerio supplementary' ]
USER 1001
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001 1002:dockerio supplementary' ]

# super test the new "user:group" syntax
USER dockerio:dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]
USER 1001:dockerio
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]
USER dockerio:1001
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]
USER 1001:1001
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]
USER dockerio:supplementary
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1002/dockerio:supplementary/1002:supplementary' ]
USER dockerio:1002
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1002/dockerio:supplementary/1002:supplementary' ]
USER 1001:supplementary
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1002/dockerio:supplementary/1002:supplementary' ]
USER 1001:1002
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1002/dockerio:supplementary/1002:supplementary' ]

# make sure unknown uid/gid still works properly
USER 1042:1043
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1042:1043/1042:1043/1043:1043' ]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildEnvUsage(c *check.C) {
	// /docker/world/hello is not owned by the correct user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	name := "testbuildenvusage"
	dockerfile := `FROM busybox
ENV    HOME /root
ENV    PATH $HOME/bin:$PATH
ENV    PATH /tmp:$PATH
RUN    [ "$PATH" = "/tmp:$HOME/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" ]
ENV    FOO /foo/baz
ENV    BAR /bar
ENV    BAZ $BAR
ENV    FOOPATH $PATH:$FOO
RUN    [ "$BAR" = "$BAZ" ]
RUN    [ "$FOOPATH" = "$PATH:/foo/baz" ]
ENV	   FROM hello/docker/world
ENV    TO /docker/world/hello
ADD    $FROM $TO
RUN    [ "$(cat $TO)" = "hello" ]
ENV    abc=def
ENV    ghi=$abc
RUN    [ "$ghi" = "def" ]
`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"hello/docker/world": "hello",
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

func (s *DockerSuite) TestBuildEnvUsage2(c *check.C) {
	// /docker/world/hello is not owned by the correct user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	name := "testbuildenvusage2"
	dockerfile := `FROM busybox
ENV    abc=def def="hello world"
RUN    [ "$abc,$def" = "def,hello world" ]
ENV    def=hello\ world v1=abc v2="hi there" v3='boogie nights' v4="with'quotes too"
RUN    [ "$def,$v1,$v2,$v3,$v4" = "hello world,abc,hi there,boogie nights,with'quotes too" ]
ENV    abc=zzz FROM=hello/docker/world
ENV    abc=zzz TO=/docker/world/hello
ADD    $FROM $TO
RUN    [ "$abc,$(cat $TO)" = "zzz,hello" ]
ENV    abc 'yyy'
RUN    [ $abc = 'yyy' ]
ENV    abc=
RUN    [ "$abc" = "" ]

# use grep to make sure if the builder substitutes \$foo by mistake
# we don't get a false positive
ENV    abc=\$foo
RUN    [ "$abc" = "\$foo" ] && (echo "$abc" | grep foo)
ENV    abc \$foo
RUN    [ "$abc" = "\$foo" ] && (echo "$abc" | grep foo)

ENV    abc=\'foo\' abc2=\"foo\"
RUN    [ "$abc,$abc2" = "'foo',\"foo\"" ]
ENV    abc "foo"
RUN    [ "$abc" = "foo" ]
ENV    abc 'foo'
RUN    [ "$abc" = 'foo' ]
ENV    abc \'foo\'
RUN    [ "$abc" = "'foo'" ]
ENV    abc \"foo\"
RUN    [ "$abc" = '"foo"' ]

ENV    abc=ABC
RUN    [ "$abc" = "ABC" ]
ENV    def1=${abc:-DEF} def2=${ccc:-DEF}
ENV    def3=${ccc:-${def2}xx} def4=${abc:+ALT} def5=${def2:+${abc}:} def6=${ccc:-\$abc:} def7=${ccc:-\${abc}:}
RUN    [ "$def1,$def2,$def3,$def4,$def5,$def6,$def7" = 'ABC,DEF,DEFxx,ALT,ABC:,$abc:,${abc:}' ]
ENV    mypath=${mypath:+$mypath:}/home
ENV    mypath=${mypath:+$mypath:}/away
RUN    [ "$mypath" = '/home:/away' ]

ENV    e1=bar
ENV    e2=$e1 e3=$e11 e4=\$e1 e5=\$e11
RUN    [ "$e0,$e1,$e2,$e3,$e4,$e5" = ',bar,bar,,$e1,$e11' ]

ENV    ee1 bar
ENV    ee2 $ee1
ENV    ee3 $ee11
ENV    ee4 \$ee1
ENV    ee5 \$ee11
RUN    [ "$ee1,$ee2,$ee3,$ee4,$ee5" = 'bar,bar,,$ee1,$ee11' ]

ENV    eee1="foo" eee2='foo'
ENV    eee3 "foo"
ENV    eee4 'foo'
RUN    [ "$eee1,$eee2,$eee3,$eee4" = 'foo,foo,foo,foo' ]

`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"hello/docker/world": "hello",
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
RUN sh -c "mkdir /existing-directory" #sh -c is needed on Windows to use the correct mkdir
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

func (s *DockerSuite) TestBuildFromGit(c *check.C) {
	name := "testbuildfromgit"
	git, err := newFakeGit("repo", map[string]string{
		"Dockerfile": `FROM busybox
					ADD first /first
					RUN [ -f /first ]
					MAINTAINER docker`,
		"first": "test git data",
	}, true)
	if err != nil {
		c.Fatal(err)
	}
	defer git.Close()

	_, err = buildImageFromPath(name, git.RepoURL, true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Author")
	if res != "docker" {
		c.Fatalf("Maintainer should be docker, got %s", res)
	}
}

func (s *DockerSuite) TestBuildFromGitWithContext(c *check.C) {
	name := "testbuildfromgit"
	git, err := newFakeGit("repo", map[string]string{
		"docker/Dockerfile": `FROM busybox
					ADD first /first
					RUN [ -f /first ]
					MAINTAINER docker`,
		"docker/first": "test git data",
	}, true)
	if err != nil {
		c.Fatal(err)
	}
	defer git.Close()

	u := fmt.Sprintf("%s#master:docker", git.RepoURL)
	_, err = buildImageFromPath(name, u, true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Author")
	if res != "docker" {
		c.Fatalf("Maintainer should be docker, got %s", res)
	}
}

func (s *DockerSuite) TestBuildFromGitwithF(c *check.C) {
	name := "testbuildfromgitwithf"
	git, err := newFakeGit("repo", map[string]string{
		"myApp/myDockerfile": `FROM busybox
					RUN echo hi from Dockerfile`,
	}, true)
	if err != nil {
		c.Fatal(err)
	}
	defer git.Close()

	out, _, err := dockerCmdWithError("build", "-t", name, "--no-cache", "-f", "myApp/myDockerfile", git.RepoURL)
	if err != nil {
		c.Fatalf("Error on build. Out: %s\nErr: %v", out, err)
	}

	if !strings.Contains(out, "hi from Dockerfile") {
		c.Fatalf("Missing expected output, got:\n%s", out)
	}
}

func (s *DockerSuite) TestBuildFromRemoteTarball(c *check.C) {
	name := "testbuildfromremotetarball"

	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte(`FROM busybox
					MAINTAINER docker`)
	if err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfile)),
	}); err != nil {
		c.Fatalf("failed to write tar file header: %v", err)
	}
	if _, err := tw.Write(dockerfile); err != nil {
		c.Fatalf("failed to write tar file content: %v", err)
	}
	if err := tw.Close(); err != nil {
		c.Fatalf("failed to close tar archive: %v", err)
	}

	server, err := fakeBinaryStorage(map[string]*bytes.Buffer{
		"testT.tar": buffer,
	})
	c.Assert(err, check.IsNil)

	defer server.Close()

	_, err = buildImageFromPath(name, server.URL()+"/testT.tar", true)
	c.Assert(err, check.IsNil)

	res := inspectField(c, name, "Author")

	if res != "docker" {
		c.Fatalf("Maintainer should be docker, got %s", res)
	}
}

func (s *DockerSuite) TestBuildCleanupCmdOnEntrypoint(c *check.C) {
	name := "testbuildcmdcleanuponentrypoint"
	if _, err := buildImage(name,
		`FROM `+minimalBaseImage()+`
        CMD ["test"]
		ENTRYPOINT ["echo"]`,
		true); err != nil {
		c.Fatal(err)
	}
	if _, err := buildImage(name,
		fmt.Sprintf(`FROM %s
		ENTRYPOINT ["cat"]`, name),
		true); err != nil {
		c.Fatal(err)
	}
	res := inspectField(c, name, "Config.Cmd")
	if res != "[]" {
		c.Fatalf("Cmd %s, expected nil", res)
	}

	res = inspectField(c, name, "Config.Entrypoint")
	if expected := "[cat]"; res != expected {
		c.Fatalf("Entrypoint %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildClearCmd(c *check.C) {
	name := "testbuildclearcmd"
	_, err := buildImage(name,
		`From `+minimalBaseImage()+`
   ENTRYPOINT ["/bin/bash"]
   CMD []`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectFieldJSON(c, name, "Config.Cmd")
	if res != "[]" {
		c.Fatalf("Cmd %s, expected %s", res, "[]")
	}
}

func (s *DockerSuite) TestBuildEmptyCmd(c *check.C) {
	// Skip on Windows. Base image on Windows has a CMD set in the image.
	testRequires(c, DaemonIsLinux)

	name := "testbuildemptycmd"
	if _, err := buildImage(name, "FROM "+minimalBaseImage()+"\nMAINTAINER quux\n", true); err != nil {
		c.Fatal(err)
	}
	res := inspectFieldJSON(c, name, "Config.Cmd")
	if res != "null" {
		c.Fatalf("Cmd %s, expected %s", res, "null")
	}
}

func (s *DockerSuite) TestBuildOnBuildOutput(c *check.C) {
	name := "testbuildonbuildparent"
	if _, err := buildImage(name, "FROM busybox\nONBUILD RUN echo foo\n", true); err != nil {
		c.Fatal(err)
	}

	_, out, err := buildImageWithOut(name, "FROM "+name+"\nMAINTAINER quux\n", true)
	if err != nil {
		c.Fatal(err)
	}

	if !strings.Contains(out, "# Executing 1 build trigger") {
		c.Fatal("failed to find the build trigger output", out)
	}
}

func (s *DockerSuite) TestBuildInvalidTag(c *check.C) {
	name := "abcd:" + stringutils.GenerateRandomAlphaOnlyString(200)
	_, out, err := buildImageWithOut(name, "FROM "+minimalBaseImage()+"\nMAINTAINER quux\n", true)
	// if the error doesn't check for illegal tag name, or the image is built
	// then this should fail
	if !strings.Contains(out, "Error parsing reference") || strings.Contains(out, "Sending build context to Docker daemon") {
		c.Fatalf("failed to stop before building. Error: %s, Output: %s", err, out)
	}
}

func (s *DockerSuite) TestBuildCmdShDashC(c *check.C) {
	name := "testbuildcmdshc"
	if _, err := buildImage(name, "FROM busybox\nCMD echo cmd\n", true); err != nil {
		c.Fatal(err)
	}

	res := inspectFieldJSON(c, name, "Config.Cmd")

	expected := `["/bin/sh","-c","echo cmd"]`
	if daemonPlatform == "windows" {
		expected = `["cmd","/S","/C","echo cmd"]`
	}

	if res != expected {
		c.Fatalf("Expected value %s not in Config.Cmd: %s", expected, res)
	}

}

func (s *DockerSuite) TestBuildCmdSpaces(c *check.C) {
	// Test to make sure that when we strcat arrays we take into account
	// the arg separator to make sure ["echo","hi"] and ["echo hi"] don't
	// look the same
	name := "testbuildcmdspaces"
	var id1 string
	var id2 string
	var err error

	if id1, err = buildImage(name, "FROM busybox\nCMD [\"echo hi\"]\n", true); err != nil {
		c.Fatal(err)
	}

	if id2, err = buildImage(name, "FROM busybox\nCMD [\"echo\", \"hi\"]\n", true); err != nil {
		c.Fatal(err)
	}

	if id1 == id2 {
		c.Fatal("Should not have resulted in the same CMD")
	}

	// Now do the same with ENTRYPOINT
	if id1, err = buildImage(name, "FROM busybox\nENTRYPOINT [\"echo hi\"]\n", true); err != nil {
		c.Fatal(err)
	}

	if id2, err = buildImage(name, "FROM busybox\nENTRYPOINT [\"echo\", \"hi\"]\n", true); err != nil {
		c.Fatal(err)
	}

	if id1 == id2 {
		c.Fatal("Should not have resulted in the same ENTRYPOINT")
	}

}

func (s *DockerSuite) TestBuildCmdJSONNoShDashC(c *check.C) {
	name := "testbuildcmdjson"
	if _, err := buildImage(name, "FROM busybox\nCMD [\"echo\", \"cmd\"]", true); err != nil {
		c.Fatal(err)
	}

	res := inspectFieldJSON(c, name, "Config.Cmd")

	expected := `["echo","cmd"]`

	if res != expected {
		c.Fatalf("Expected value %s not in Config.Cmd: %s", expected, res)
	}

}

func (s *DockerSuite) TestBuildEntrypointInheritance(c *check.C) {

	if _, err := buildImage("parent", `
    FROM busybox
    ENTRYPOINT exit 130
    `, true); err != nil {
		c.Fatal(err)
	}

	if _, status, _ := dockerCmdWithError("run", "parent"); status != 130 {
		c.Fatalf("expected exit code 130 but received %d", status)
	}

	if _, err := buildImage("child", `
    FROM parent
    ENTRYPOINT exit 5
    `, true); err != nil {
		c.Fatal(err)
	}

	if _, status, _ := dockerCmdWithError("run", "child"); status != 5 {
		c.Fatalf("expected exit code 5 but received %d", status)
	}

}

func (s *DockerSuite) TestBuildEntrypointInheritanceInspect(c *check.C) {
	var (
		name     = "testbuildepinherit"
		name2    = "testbuildepinherit2"
		expected = `["/bin/sh","-c","echo quux"]`
	)

	if daemonPlatform == "windows" {
		expected = `["cmd","/S","/C","echo quux"]`
	}

	if _, err := buildImage(name, "FROM busybox\nENTRYPOINT /foo/bar", true); err != nil {
		c.Fatal(err)
	}

	if _, err := buildImage(name2, fmt.Sprintf("FROM %s\nENTRYPOINT echo quux", name), true); err != nil {
		c.Fatal(err)
	}

	res := inspectFieldJSON(c, name2, "Config.Entrypoint")

	if res != expected {
		c.Fatalf("Expected value %s not in Config.Entrypoint: %s", expected, res)
	}

	out, _ := dockerCmd(c, "run", name2)

	expected = "quux"

	if strings.TrimSpace(out) != expected {
		c.Fatalf("Expected output is %s, got %s", expected, out)
	}

}

func (s *DockerSuite) TestBuildRunShEntrypoint(c *check.C) {
	name := "testbuildentrypoint"
	_, err := buildImage(name,
		`FROM busybox
                                ENTRYPOINT echo`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	dockerCmd(c, "run", "--rm", name)
}

func (s *DockerSuite) TestBuildExoticShellInterpolation(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildexoticshellinterpolation"

	_, err := buildImage(name, `
		FROM busybox

		ENV SOME_VAR a.b.c

		RUN [ "$SOME_VAR"       = 'a.b.c' ]
		RUN [ "${SOME_VAR}"     = 'a.b.c' ]
		RUN [ "${SOME_VAR%.*}"  = 'a.b'   ]
		RUN [ "${SOME_VAR%%.*}" = 'a'     ]
		RUN [ "${SOME_VAR#*.}"  = 'b.c'   ]
		RUN [ "${SOME_VAR##*.}" = 'c'     ]
		RUN [ "${SOME_VAR/c/d}" = 'a.b.d' ]
		RUN [ "${#SOME_VAR}"    = '5'     ]

		RUN [ "${SOME_UNSET_VAR:-$SOME_VAR}" = 'a.b.c' ]
		RUN [ "${SOME_VAR:+Version: ${SOME_VAR}}" = 'Version: a.b.c' ]
		RUN [ "${SOME_UNSET_VAR:+${SOME_VAR}}" = '' ]
		RUN [ "${SOME_UNSET_VAR:-${SOME_VAR:-d.e.f}}" = 'a.b.c' ]
	`, false)
	if err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestBuildVerifySingleQuoteFails(c *check.C) {
	// This testcase is supposed to generate an error because the
	// JSON array we're passing in on the CMD uses single quotes instead
	// of double quotes (per the JSON spec). This means we interpret it
	// as a "string" instead of "JSON array" and pass it on to "sh -c" and
	// it should barf on it.
	name := "testbuildsinglequotefails"

	if _, err := buildImage(name,
		`FROM busybox
		CMD [ '/bin/sh', '-c', 'echo hi' ]`,
		true); err != nil {
		c.Fatal(err)
	}

	if _, _, err := dockerCmdWithError("run", "--rm", name); err == nil {
		c.Fatal("The image was not supposed to be able to run")
	}

}

func (s *DockerSuite) TestBuildVerboseOut(c *check.C) {
	name := "testbuildverboseout"
	expected := "\n123\n"

	if daemonPlatform == "windows" {
		expected = "\n123\r\n"
	}

	_, out, err := buildImageWithOut(name,
		`FROM busybox
RUN echo 123`,
		false)

	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(out, expected) {
		c.Fatalf("Output should contain %q: %q", "123", out)
	}

}

func (s *DockerSuite) TestBuildWithTabs(c *check.C) {
	name := "testbuildwithtabs"
	_, err := buildImage(name,
		"FROM busybox\nRUN echo\tone\t\ttwo", true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectFieldJSON(c, name, "ContainerConfig.Cmd")
	expected1 := `["/bin/sh","-c","echo\tone\t\ttwo"]`
	expected2 := `["/bin/sh","-c","echo\u0009one\u0009\u0009two"]` // syntactically equivalent, and what Go 1.3 generates
	if daemonPlatform == "windows" {
		expected1 = `["cmd","/S","/C","echo\tone\t\ttwo"]`
		expected2 = `["cmd","/S","/C","echo\u0009one\u0009\u0009two"]` // syntactically equivalent, and what Go 1.3 generates
	}
	if res != expected1 && res != expected2 {
		c.Fatalf("Missing tabs.\nGot: %s\nExp: %s or %s", res, expected1, expected2)
	}
}

func (s *DockerSuite) TestBuildLabels(c *check.C) {
	name := "testbuildlabel"
	expected := `{"License":"GPL","Vendor":"Acme"}`
	_, err := buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme
                LABEL License GPL`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectFieldJSON(c, name, "Config.Labels")
	if res != expected {
		c.Fatalf("Labels %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildLabelsCache(c *check.C) {
	name := "testbuildlabelcache"

	id1, err := buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme`, false)
	if err != nil {
		c.Fatalf("Build 1 should have worked: %v", err)
	}

	id2, err := buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme`, true)
	if err != nil || id1 != id2 {
		c.Fatalf("Build 2 should have worked & used cache(%s,%s): %v", id1, id2, err)
	}

	id2, err = buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme1`, true)
	if err != nil || id1 == id2 {
		c.Fatalf("Build 3 should have worked & NOT used cache(%s,%s): %v", id1, id2, err)
	}

	id2, err = buildImage(name,
		`FROM busybox
		LABEL Vendor Acme`, true) // Note: " " and "=" should be same
	if err != nil || id1 != id2 {
		c.Fatalf("Build 4 should have worked & used cache(%s,%s): %v", id1, id2, err)
	}

	// Now make sure the cache isn't used by mistake
	id1, err = buildImage(name,
		`FROM busybox
       LABEL f1=b1 f2=b2`, false)
	if err != nil {
		c.Fatalf("Build 5 should have worked: %q", err)
	}

	id2, err = buildImage(name,
		`FROM busybox
       LABEL f1="b1 f2=b2"`, true)
	if err != nil || id1 == id2 {
		c.Fatalf("Build 6 should have worked & NOT used the cache(%s,%s): %q", id1, id2, err)
	}

}

func (s *DockerSuite) TestBuildNotVerboseSuccess(c *check.C) {
	// This test makes sure that -q works correctly when build is successful:
	// stdout has only the image ID (long image ID) and stderr is empty.
	var stdout, stderr string
	var err error
	outRegexp := regexp.MustCompile("^(sha256:|)[a-z0-9]{64}\\n$")

	tt := []struct {
		Name      string
		BuildFunc func(string)
	}{
		{
			Name: "quiet_build_stdin_success",
			BuildFunc: func(name string) {
				_, stdout, stderr, err = buildImageWithStdoutStderr(name, "FROM busybox", true, "-q", "--force-rm", "--rm")
			},
		},
		{
			Name: "quiet_build_ctx_success",
			BuildFunc: func(name string) {
				ctx, err := fakeContext("FROM busybox", map[string]string{
					"quiet_build_success_fctx": "test",
				})
				if err != nil {
					c.Fatalf("Failed to create context: %s", err.Error())
				}
				defer ctx.Close()
				_, stdout, stderr, err = buildImageFromContextWithStdoutStderr(name, ctx, true, "-q", "--force-rm", "--rm")
			},
		},
		{
			Name: "quiet_build_git_success",
			BuildFunc: func(name string) {
				git, err := newFakeGit("repo", map[string]string{
					"Dockerfile": "FROM busybox",
				}, true)
				if err != nil {
					c.Fatalf("Failed to create the git repo: %s", err.Error())
				}
				defer git.Close()
				_, stdout, stderr, err = buildImageFromGitWithStdoutStderr(name, git, true, "-q", "--force-rm", "--rm")

			},
		},
	}

	for _, te := range tt {
		te.BuildFunc(te.Name)
		if err != nil {
			c.Fatalf("Test %s shouldn't fail, but got the following error: %s", te.Name, err.Error())
		}
		if outRegexp.Find([]byte(stdout)) == nil {
			c.Fatalf("Test %s expected stdout to match the [%v] regexp, but it is [%v]", te.Name, outRegexp, stdout)
		}

		if stderr != "" {
			c.Fatalf("Test %s expected stderr to be empty, but it is [%#v]", te.Name, stderr)
		}
	}

}

func (s *DockerSuite) TestBuildNotVerboseFailureWithNonExistImage(c *check.C) {
	// This test makes sure that -q works correctly when build fails by
	// comparing between the stderr output in quiet mode and in stdout
	// and stderr output in verbose mode
	testRequires(c, Network)
	testName := "quiet_build_not_exists_image"
	buildCmd := "FROM busybox11"
	_, _, qstderr, qerr := buildImageWithStdoutStderr(testName, buildCmd, false, "-q", "--force-rm", "--rm")
	_, vstdout, vstderr, verr := buildImageWithStdoutStderr(testName, buildCmd, false, "--force-rm", "--rm")
	if verr == nil || qerr == nil {
		c.Fatal(fmt.Errorf("Test [%s] expected to fail but didn't", testName))
	}
	if qstderr != vstdout+vstderr {
		c.Fatal(fmt.Errorf("Test[%s] expected that quiet stderr and verbose stdout are equal; quiet [%v], verbose [%v]", testName, qstderr, vstdout+vstderr))
	}
}

func (s *DockerSuite) TestBuildNotVerboseFailure(c *check.C) {
	// This test makes sure that -q works correctly when build fails by
	// comparing between the stderr output in quiet mode and in stdout
	// and stderr output in verbose mode
	tt := []struct {
		TestName  string
		BuildCmds string
	}{
		{"quiet_build_no_from_at_the_beginning", "RUN whoami"},
		{"quiet_build_unknown_instr", "FROMD busybox"},
	}

	for _, te := range tt {
		_, _, qstderr, qerr := buildImageWithStdoutStderr(te.TestName, te.BuildCmds, false, "-q", "--force-rm", "--rm")
		_, vstdout, vstderr, verr := buildImageWithStdoutStderr(te.TestName, te.BuildCmds, false, "--force-rm", "--rm")
		if verr == nil || qerr == nil {
			c.Fatal(fmt.Errorf("Test [%s] expected to fail but didn't", te.TestName))
		}
		if qstderr != vstdout+vstderr {
			c.Fatal(fmt.Errorf("Test[%s] expected that quiet stderr and verbose stdout are equal; quiet [%v], verbose [%v]", te.TestName, qstderr, vstdout+vstderr))
		}
	}
}

func (s *DockerSuite) TestBuildNotVerboseFailureRemote(c *check.C) {
	// This test ensures that when given a wrong URL, stderr in quiet mode and
	// stderr in verbose mode are identical.
	// TODO(vdemeester) with cobra, stdout has a carriage return too much so this test should not check stdout
	URL := "http://something.invalid"
	Name := "quiet_build_wrong_remote"
	_, _, qstderr, qerr := buildImageWithStdoutStderr(Name, "", false, "-q", "--force-rm", "--rm", URL)
	_, _, vstderr, verr := buildImageWithStdoutStderr(Name, "", false, "--force-rm", "--rm", URL)
	if qerr == nil || verr == nil {
		c.Fatal(fmt.Errorf("Test [%s] expected to fail but didn't", Name))
	}
	if qstderr != vstderr {
		c.Fatal(fmt.Errorf("Test[%s] expected that quiet stderr and verbose stdout are equal; quiet [%v], verbose [%v]", Name, qstderr, vstderr))
	}
}

func (s *DockerSuite) TestBuildStderr(c *check.C) {
	// This test just makes sure that no non-error output goes
	// to stderr
	name := "testbuildstderr"
	_, _, stderr, err := buildImageWithStdoutStderr(name,
		"FROM busybox\nRUN echo one", true)
	if err != nil {
		c.Fatal(err)
	}

	if runtime.GOOS == "windows" &&
		daemonPlatform != "windows" {
		// Windows to non-Windows should have a security warning
		if !strings.Contains(stderr, "SECURITY WARNING:") {
			c.Fatalf("Stderr contains unexpected output: %q", stderr)
		}
	} else {
		// Other platform combinations should have no stderr written too
		if stderr != "" {
			c.Fatalf("Stderr should have been empty, instead it's: %q", stderr)
		}
	}
}

func (s *DockerSuite) TestBuildChownSingleFile(c *check.C) {
	testRequires(c, UnixCli) // test uses chown: not available on windows
	testRequires(c, DaemonIsLinux)

	name := "testbuildchownsinglefile"

	ctx, err := fakeContext(`
FROM busybox
COPY test /
RUN ls -l /test
RUN [ $(ls -l /test | awk '{print $3":"$4}') = 'root:root' ]
`, map[string]string{
		"test": "test",
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if err := os.Chown(filepath.Join(ctx.Dir, "test"), 4242, 4242); err != nil {
		c.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestBuildSymlinkBreakout(c *check.C) {
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

func (s *DockerSuite) TestBuildXZHost(c *check.C) {
	// /usr/local/sbin/xz gets permission denied for the user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	name := "testbuildxzhost"

	ctx, err := fakeContext(`
FROM busybox
ADD xz /usr/local/sbin/
RUN chmod 755 /usr/local/sbin/xz
ADD test.xz /
RUN [ ! -e /injected ]`,
		map[string]string{
			"test.xz": "\xfd\x37\x7a\x58\x5a\x00\x00\x04\xe6\xd6\xb4\x46\x02\x00" +
				"\x21\x01\x16\x00\x00\x00\x74\x2f\xe5\xa3\x01\x00\x3f\xfd" +
				"\x37\x7a\x58\x5a\x00\x00\x04\xe6\xd6\xb4\x46\x02\x00\x21",
			"xz": "#!/bin/sh\ntouch /injected",
		})

	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}

}

func (s *DockerSuite) TestBuildVolumesRetainContents(c *check.C) {
	// /foo/file gets permission denied for the user
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux) // TODO Windows: Issue #20127
	var (
		name     = "testbuildvolumescontent"
		expected = "some text"
		volName  = "/foo"
	)

	if daemonPlatform == "windows" {
		volName = "C:/foo"
	}

	ctx, err := fakeContext(`
FROM busybox
COPY content /foo/file
VOLUME `+volName+`
CMD cat /foo/file`,
		map[string]string{
			"content": expected,
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, false); err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--rm", name)
	if out != expected {
		c.Fatalf("expected file contents for /foo/file to be %q but received %q", expected, out)
	}

}

func (s *DockerSuite) TestBuildRenamedDockerfile(c *check.C) {

	ctx, err := fakeContext(`FROM busybox
	RUN echo from Dockerfile`,
		map[string]string{
			"Dockerfile":       "FROM busybox\nRUN echo from Dockerfile",
			"files/Dockerfile": "FROM busybox\nRUN echo from files/Dockerfile",
			"files/dFile":      "FROM busybox\nRUN echo from files/dFile",
			"dFile":            "FROM busybox\nRUN echo from dFile",
			"files/dFile2":     "FROM busybox\nRUN echo from files/dFile2",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", "test1", ".")
	if err != nil {
		c.Fatalf("Failed to build: %s\n%s", out, err)
	}
	if !strings.Contains(out, "from Dockerfile") {
		c.Fatalf("test1 should have used Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, ctx.Dir, "build", "-f", filepath.Join("files", "Dockerfile"), "-t", "test2", ".")
	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(out, "from files/Dockerfile") {
		c.Fatalf("test2 should have used files/Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, ctx.Dir, "build", fmt.Sprintf("--file=%s", filepath.Join("files", "dFile")), "-t", "test3", ".")
	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(out, "from files/dFile") {
		c.Fatalf("test3 should have used files/dFile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, ctx.Dir, "build", "--file=dFile", "-t", "test4", ".")
	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(out, "from dFile") {
		c.Fatalf("test4 should have used dFile, output:%s", out)
	}

	dirWithNoDockerfile, err := ioutil.TempDir(os.TempDir(), "test5")
	c.Assert(err, check.IsNil)
	nonDockerfileFile := filepath.Join(dirWithNoDockerfile, "notDockerfile")
	if _, err = os.Create(nonDockerfileFile); err != nil {
		c.Fatal(err)
	}
	out, _, err = dockerCmdInDir(c, ctx.Dir, "build", fmt.Sprintf("--file=%s", nonDockerfileFile), "-t", "test5", ".")

	if err == nil {
		c.Fatalf("test5 was supposed to fail to find passwd")
	}

	if expected := fmt.Sprintf("The Dockerfile (%s) must be within the build context (.)", nonDockerfileFile); !strings.Contains(out, expected) {
		c.Fatalf("wrong error message:%v\nexpected to contain=%v", out, expected)
	}

	out, _, err = dockerCmdInDir(c, filepath.Join(ctx.Dir, "files"), "build", "-f", filepath.Join("..", "Dockerfile"), "-t", "test6", "..")
	if err != nil {
		c.Fatalf("test6 failed: %s", err)
	}
	if !strings.Contains(out, "from Dockerfile") {
		c.Fatalf("test6 should have used root Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, filepath.Join(ctx.Dir, "files"), "build", "-f", filepath.Join(ctx.Dir, "files", "Dockerfile"), "-t", "test7", "..")
	if err != nil {
		c.Fatalf("test7 failed: %s", err)
	}
	if !strings.Contains(out, "from files/Dockerfile") {
		c.Fatalf("test7 should have used files Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, filepath.Join(ctx.Dir, "files"), "build", "-f", filepath.Join("..", "Dockerfile"), "-t", "test8", ".")
	if err == nil || !strings.Contains(out, "must be within the build context") {
		c.Fatalf("test8 should have failed with Dockerfile out of context: %s", err)
	}

	tmpDir := os.TempDir()
	out, _, err = dockerCmdInDir(c, tmpDir, "build", "-t", "test9", ctx.Dir)
	if err != nil {
		c.Fatalf("test9 - failed: %s", err)
	}
	if !strings.Contains(out, "from Dockerfile") {
		c.Fatalf("test9 should have used root Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(c, filepath.Join(ctx.Dir, "files"), "build", "-f", "dFile2", "-t", "test10", ".")
	if err != nil {
		c.Fatalf("test10 should have worked: %s", err)
	}
	if !strings.Contains(out, "from files/dFile2") {
		c.Fatalf("test10 should have used files/dFile2, output:%s", out)
	}

}

func (s *DockerSuite) TestBuildFromMixedcaseDockerfile(c *check.C) {
	testRequires(c, UnixCli) // Dockerfile overwrites dockerfile on windows
	testRequires(c, DaemonIsLinux)

	ctx, err := fakeContext(`FROM busybox
	RUN echo from dockerfile`,
		map[string]string{
			"dockerfile": "FROM busybox\nRUN echo from dockerfile",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", "test1", ".")
	if err != nil {
		c.Fatalf("Failed to build: %s\n%s", out, err)
	}

	if !strings.Contains(out, "from dockerfile") {
		c.Fatalf("Missing proper output: %s", out)
	}

}

func (s *DockerSuite) TestBuildWithTwoDockerfiles(c *check.C) {
	testRequires(c, UnixCli) // Dockerfile overwrites dockerfile on windows
	testRequires(c, DaemonIsLinux)

	ctx, err := fakeContext(`FROM busybox
RUN echo from Dockerfile`,
		map[string]string{
			"dockerfile": "FROM busybox\nRUN echo from dockerfile",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-t", "test1", ".")
	if err != nil {
		c.Fatalf("Failed to build: %s\n%s", out, err)
	}

	if !strings.Contains(out, "from Dockerfile") {
		c.Fatalf("Missing proper output: %s", out)
	}

}

func (s *DockerSuite) TestBuildFromURLWithF(c *check.C) {
	server, err := fakeStorage(map[string]string{"baz": `FROM busybox
RUN echo from baz
COPY * /tmp/
RUN find /tmp/`})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(`FROM busybox
RUN echo from Dockerfile`,
		map[string]string{})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	// Make sure that -f is ignored and that we don't use the Dockerfile
	// that's in the current dir
	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "-f", "baz", "-t", "test1", server.URL()+"/baz")
	if err != nil {
		c.Fatalf("Failed to build: %s\n%s", out, err)
	}

	if !strings.Contains(out, "from baz") ||
		strings.Contains(out, "/tmp/baz") ||
		!strings.Contains(out, "/tmp/Dockerfile") {
		c.Fatalf("Missing proper output: %s", out)
	}

}

func (s *DockerSuite) TestBuildFromStdinWithF(c *check.C) {
	testRequires(c, DaemonIsLinux) // TODO Windows: This test is flaky; no idea why
	ctx, err := fakeContext(`FROM busybox
RUN echo "from Dockerfile"`,
		map[string]string{})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	// Make sure that -f is ignored and that we don't use the Dockerfile
	// that's in the current dir
	dockerCommand := exec.Command(dockerBinary, "build", "-f", "baz", "-t", "test1", "-")
	dockerCommand.Dir = ctx.Dir
	dockerCommand.Stdin = strings.NewReader(`FROM busybox
RUN echo "from baz"
COPY * /tmp/
RUN sh -c "find /tmp/" # sh -c is needed on Windows to use the correct find`)
	out, status, err := runCommandWithOutput(dockerCommand)
	if err != nil || status != 0 {
		c.Fatalf("Error building: %s", err)
	}

	if !strings.Contains(out, "from baz") ||
		strings.Contains(out, "/tmp/baz") ||
		!strings.Contains(out, "/tmp/Dockerfile") {
		c.Fatalf("Missing proper output: %s", out)
	}

}

func (s *DockerSuite) TestBuildFromOfficialNames(c *check.C) {
	name := "testbuildfromofficial"
	fromNames := []string{
		"busybox",
		"docker.io/busybox",
		"index.docker.io/busybox",
		"library/busybox",
		"docker.io/library/busybox",
		"index.docker.io/library/busybox",
	}
	for idx, fromName := range fromNames {
		imgName := fmt.Sprintf("%s%d", name, idx)
		_, err := buildImage(imgName, "FROM "+fromName, true)
		if err != nil {
			c.Errorf("Build failed using FROM %s: %s", fromName, err)
		}
		deleteImages(imgName)
	}
}

func (s *DockerSuite) TestBuildDockerfileOutsideContext(c *check.C) {
	testRequires(c, UnixCli) // uses os.Symlink: not implemented in windows at the time of writing (go-1.4.2)
	testRequires(c, DaemonIsLinux)

	name := "testbuilddockerfileoutsidecontext"
	tmpdir, err := ioutil.TempDir("", name)
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpdir)
	ctx := filepath.Join(tmpdir, "context")
	if err := os.MkdirAll(ctx, 0755); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(ctx, "Dockerfile"), []byte("FROM scratch\nENV X Y"), 0644); err != nil {
		c.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		c.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(ctx); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(tmpdir, "outsideDockerfile"), []byte("FROM scratch\nENV x y"), 0644); err != nil {
		c.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "outsideDockerfile"), filepath.Join(ctx, "dockerfile1")); err != nil {
		c.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(tmpdir, "outsideDockerfile"), filepath.Join(ctx, "dockerfile2")); err != nil {
		c.Fatal(err)
	}

	for _, dockerfilePath := range []string{
		filepath.Join("..", "outsideDockerfile"),
		filepath.Join(ctx, "dockerfile1"),
		filepath.Join(ctx, "dockerfile2"),
	} {
		result := dockerCmdWithResult("build", "-t", name, "--no-cache", "-f", dockerfilePath, ".")
		c.Assert(result, icmd.Matches, icmd.Expected{
			Err:      "must be within the build context",
			ExitCode: 1,
		})
		deleteImages(name)
	}

	os.Chdir(tmpdir)

	// Path to Dockerfile should be resolved relative to working directory, not relative to context.
	// There is a Dockerfile in the context, but since there is no Dockerfile in the current directory, the following should fail
	out, _, err := dockerCmdWithError("build", "-t", name, "--no-cache", "-f", "Dockerfile", ctx)
	if err == nil {
		c.Fatalf("Expected error. Out: %s", out)
	}
}

func (s *DockerSuite) TestBuildSpaces(c *check.C) {
	// Test to make sure that leading/trailing spaces on a command
	// doesn't change the error msg we get
	var (
		err1 error
		err2 error
	)

	name := "testspaces"
	ctx, err := fakeContext("FROM busybox\nCOPY\n",
		map[string]string{
			"Dockerfile": "FROM busybox\nCOPY\n",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err1 = buildImageFromContext(name, ctx, false); err1 == nil {
		c.Fatal("Build 1 was supposed to fail, but didn't")
	}

	ctx.Add("Dockerfile", "FROM busybox\nCOPY    ")
	if _, err2 = buildImageFromContext(name, ctx, false); err2 == nil {
		c.Fatal("Build 2 was supposed to fail, but didn't")
	}

	removeLogTimestamps := func(s string) string {
		return regexp.MustCompile(`time="(.*?)"`).ReplaceAllString(s, `time=[TIMESTAMP]`)
	}

	// Skip over the times
	e1 := removeLogTimestamps(err1.Error())
	e2 := removeLogTimestamps(err2.Error())

	// Ignore whitespace since that's what were verifying doesn't change stuff
	if strings.Replace(e1, " ", "", -1) != strings.Replace(e2, " ", "", -1) {
		c.Fatalf("Build 2's error wasn't the same as build 1's\n1:%s\n2:%s", err1, err2)
	}

	ctx.Add("Dockerfile", "FROM busybox\n   COPY")
	if _, err2 = buildImageFromContext(name, ctx, false); err2 == nil {
		c.Fatal("Build 3 was supposed to fail, but didn't")
	}

	// Skip over the times
	e1 = removeLogTimestamps(err1.Error())
	e2 = removeLogTimestamps(err2.Error())

	// Ignore whitespace since that's what were verifying doesn't change stuff
	if strings.Replace(e1, " ", "", -1) != strings.Replace(e2, " ", "", -1) {
		c.Fatalf("Build 3's error wasn't the same as build 1's\n1:%s\n3:%s", err1, err2)
	}

	ctx.Add("Dockerfile", "FROM busybox\n   COPY    ")
	if _, err2 = buildImageFromContext(name, ctx, false); err2 == nil {
		c.Fatal("Build 4 was supposed to fail, but didn't")
	}

	// Skip over the times
	e1 = removeLogTimestamps(err1.Error())
	e2 = removeLogTimestamps(err2.Error())

	// Ignore whitespace since that's what were verifying doesn't change stuff
	if strings.Replace(e1, " ", "", -1) != strings.Replace(e2, " ", "", -1) {
		c.Fatalf("Build 4's error wasn't the same as build 1's\n1:%s\n4:%s", err1, err2)
	}

}

func (s *DockerSuite) TestBuildSpacesWithQuotes(c *check.C) {
	// Test to make sure that spaces in quotes aren't lost
	name := "testspacesquotes"

	dockerfile := `FROM busybox
RUN echo "  \
  foo  "`

	_, out, err := buildImageWithOut(name, dockerfile, false)
	if err != nil {
		c.Fatal("Build failed:", err)
	}

	expecting := "\n    foo  \n"
	// Windows uses the builtin echo, which preserves quotes
	if daemonPlatform == "windows" {
		expecting = "\"    foo  \""
	}
	if !strings.Contains(out, expecting) {
		c.Fatalf("Bad output: %q expecting to contain %q", out, expecting)
	}

}

// #4393
func (s *DockerSuite) TestBuildVolumeFileExistsinContainer(c *check.C) {
	testRequires(c, DaemonIsLinux) // TODO Windows: This should error out
	buildCmd := exec.Command(dockerBinary, "build", "-t", "docker-test-errcreatevolumewithfile", "-")
	buildCmd.Stdin = strings.NewReader(`
	FROM busybox
	RUN touch /foo
	VOLUME /foo
	`)

	out, _, err := runCommandWithOutput(buildCmd)
	if err == nil || !strings.Contains(out, "file exists") {
		c.Fatalf("expected build to fail when file exists in container at requested volume path")
	}

}

func (s *DockerSuite) TestBuildMissingArgs(c *check.C) {
	// Test to make sure that all Dockerfile commands (except the ones listed
	// in skipCmds) will generate an error if no args are provided.
	// Note: INSERT is deprecated so we exclude it because of that.
	skipCmds := map[string]struct{}{
		"CMD":        {},
		"RUN":        {},
		"ENTRYPOINT": {},
		"INSERT":     {},
	}

	if daemonPlatform == "windows" {
		skipCmds = map[string]struct{}{
			"CMD":        {},
			"RUN":        {},
			"ENTRYPOINT": {},
			"INSERT":     {},
			"STOPSIGNAL": {},
			"ARG":        {},
			"USER":       {},
			"EXPOSE":     {},
		}
	}

	for cmd := range command.Commands {
		cmd = strings.ToUpper(cmd)
		if _, ok := skipCmds[cmd]; ok {
			continue
		}

		var dockerfile string
		if cmd == "FROM" {
			dockerfile = cmd
		} else {
			// Add FROM to make sure we don't complain about it missing
			dockerfile = "FROM busybox\n" + cmd
		}

		ctx, err := fakeContext(dockerfile, map[string]string{})
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()
		var out string
		if out, err = buildImageFromContext("args", ctx, true); err == nil {
			c.Fatalf("%s was supposed to fail. Out:%s", cmd, out)
		}
		if !strings.Contains(err.Error(), cmd+" requires") {
			c.Fatalf("%s returned the wrong type of error:%s", cmd, err)
		}
	}

}

func (s *DockerSuite) TestBuildEmptyScratch(c *check.C) {
	testRequires(c, DaemonIsLinux)
	_, out, err := buildImageWithOut("sc", "FROM scratch", true)
	if err == nil {
		c.Fatalf("Build was supposed to fail")
	}
	if !strings.Contains(out, "No image was generated") {
		c.Fatalf("Wrong error message: %v", out)
	}
}

func (s *DockerSuite) TestBuildDotDotFile(c *check.C) {
	ctx, err := fakeContext("FROM busybox\n",
		map[string]string{
			"..gitme": "",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err = buildImageFromContext("sc", ctx, false); err != nil {
		c.Fatalf("Build was supposed to work: %s", err)
	}
}

func (s *DockerSuite) TestBuildRUNoneJSON(c *check.C) {
	testRequires(c, DaemonIsLinux) // No hello-world Windows image
	name := "testbuildrunonejson"

	ctx, err := fakeContext(`FROM hello-world:frozen
RUN [ "/hello" ]`, map[string]string{})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	out, _, err := dockerCmdInDir(c, ctx.Dir, "build", "--no-cache", "-t", name, ".")
	if err != nil {
		c.Fatalf("failed to build the image: %s, %v", out, err)
	}

	if !strings.Contains(out, "Hello from Docker") {
		c.Fatalf("bad output: %s", out)
	}

}

func (s *DockerSuite) TestBuildEmptyStringVolume(c *check.C) {
	name := "testbuildemptystringvolume"

	_, err := buildImage(name, `
  FROM busybox
  ENV foo=""
  VOLUME $foo
  `, false)
	if err == nil {
		c.Fatal("Should have failed to build")
	}

}

func (s *DockerSuite) TestBuildContainerWithCgroupParent(c *check.C) {
	testRequires(c, SameHostDaemon)
	testRequires(c, DaemonIsLinux)

	cgroupParent := "test"
	data, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		c.Fatalf("failed to read '/proc/self/cgroup - %v", err)
	}
	selfCgroupPaths := parseCgroupPaths(string(data))
	_, found := selfCgroupPaths["memory"]
	if !found {
		c.Fatalf("unable to find self memory cgroup path. CgroupsPath: %v", selfCgroupPaths)
	}
	cmd := exec.Command(dockerBinary, "build", "--cgroup-parent", cgroupParent, "-")
	cmd.Stdin = strings.NewReader(`
FROM busybox
RUN cat /proc/self/cgroup
`)

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("unexpected failure when running container with --cgroup-parent option - %s\n%v", string(out), err)
	}
	m, err := regexp.MatchString(fmt.Sprintf("memory:.*/%s/.*", cgroupParent), out)
	c.Assert(err, check.IsNil)
	if !m {
		c.Fatalf("There is no expected memory cgroup with parent /%s/: %s", cgroupParent, out)
	}
}

func (s *DockerSuite) TestBuildNoDupOutput(c *check.C) {
	// Check to make sure our build output prints the Dockerfile cmd
	// property - there was a bug that caused it to be duplicated on the
	// Step X  line
	name := "testbuildnodupoutput"

	_, out, err := buildImageWithOut(name, `
  FROM busybox
  RUN env`, false)
	if err != nil {
		c.Fatalf("Build should have worked: %q", err)
	}

	exp := "\nStep 2/2 : RUN env\n"
	if !strings.Contains(out, exp) {
		c.Fatalf("Bad output\nGot:%s\n\nExpected to contain:%s\n", out, exp)
	}
}

// GH15826
func (s *DockerSuite) TestBuildStartsFromOne(c *check.C) {
	// Explicit check to ensure that build starts from step 1 rather than 0
	name := "testbuildstartsfromone"

	_, out, err := buildImageWithOut(name, `
  FROM busybox`, false)
	if err != nil {
		c.Fatalf("Build should have worked: %q", err)
	}

	exp := "\nStep 1/1 : FROM busybox\n"
	if !strings.Contains(out, exp) {
		c.Fatalf("Bad output\nGot:%s\n\nExpected to contain:%s\n", out, exp)
	}
}

func (s *DockerSuite) TestBuildRUNErrMsg(c *check.C) {
	// Test to make sure the bad command is quoted with just "s and
	// not as a Go []string
	name := "testbuildbadrunerrmsg"
	_, out, err := buildImageWithOut(name, `
  FROM busybox
  RUN badEXE a1 \& a2	a3`, false) // tab between a2 and a3
	if err == nil {
		c.Fatal("Should have failed to build")
	}
	shell := "/bin/sh -c"
	exitCode := "127"
	if daemonPlatform == "windows" {
		shell = "cmd /S /C"
		// architectural - Windows has to start the container to determine the exe is bad, Linux does not
		exitCode = "1"
	}
	exp := `The command '` + shell + ` badEXE a1 \& a2	a3' returned a non-zero code: ` + exitCode
	if !strings.Contains(out, exp) {
		c.Fatalf("RUN doesn't have the correct output:\nGot:%s\nExpected:%s", out, exp)
	}
}

func (s *DockerTrustSuite) TestTrustedBuild(c *check.C) {
	repoName := s.setupTrustedImage(c, "trusted-build")
	dockerFile := fmt.Sprintf(`
  FROM %s
  RUN []
    `, repoName)

	name := "testtrustedbuild"

	buildCmd := buildImageCmd(name, dockerFile, true)
	s.trustedCmd(buildCmd)
	out, _, err := runCommandWithOutput(buildCmd)
	if err != nil {
		c.Fatalf("Error running trusted build: %s\n%s", err, out)
	}

	if !strings.Contains(out, fmt.Sprintf("FROM %s@sha", repoName[:len(repoName)-7])) {
		c.Fatalf("Unexpected output on trusted build:\n%s", out)
	}

	// We should also have a tag reference for the image.
	if out, exitCode := dockerCmd(c, "inspect", repoName); exitCode != 0 {
		c.Fatalf("unexpected exit code inspecting image %q: %d: %s", repoName, exitCode, out)
	}

	// We should now be able to remove the tag reference.
	if out, exitCode := dockerCmd(c, "rmi", repoName); exitCode != 0 {
		c.Fatalf("unexpected exit code inspecting image %q: %d: %s", repoName, exitCode, out)
	}
}

func (s *DockerTrustSuite) TestTrustedBuildUntrustedTag(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/build-untrusted-tag:latest", privateRegistryURL)
	dockerFile := fmt.Sprintf(`
  FROM %s
  RUN []
    `, repoName)

	name := "testtrustedbuilduntrustedtag"

	buildCmd := buildImageCmd(name, dockerFile, true)
	s.trustedCmd(buildCmd)
	out, _, err := runCommandWithOutput(buildCmd)
	if err == nil {
		c.Fatalf("Expected error on trusted build with untrusted tag: %s\n%s", err, out)
	}

	if !strings.Contains(out, "does not have trust data for") {
		c.Fatalf("Unexpected output on trusted build with untrusted tag:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestBuildContextDirIsSymlink(c *check.C) {
	testRequires(c, DaemonIsLinux)
	tempDir, err := ioutil.TempDir("", "test-build-dir-is-symlink-")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tempDir)

	// Make a real context directory in this temp directory with a simple
	// Dockerfile.
	realContextDirname := filepath.Join(tempDir, "context")
	if err := os.Mkdir(realContextDirname, os.FileMode(0755)); err != nil {
		c.Fatal(err)
	}

	if err = ioutil.WriteFile(
		filepath.Join(realContextDirname, "Dockerfile"),
		[]byte(`
			FROM busybox
			RUN echo hello world
		`),
		os.FileMode(0644),
	); err != nil {
		c.Fatal(err)
	}

	// Make a symlink to the real context directory.
	contextSymlinkName := filepath.Join(tempDir, "context_link")
	if err := os.Symlink(realContextDirname, contextSymlinkName); err != nil {
		c.Fatal(err)
	}

	// Executing the build with the symlink as the specified context should
	// *not* fail.
	if out, exitStatus := dockerCmd(c, "build", contextSymlinkName); exitStatus != 0 {
		c.Fatalf("build failed with exit status %d: %s", exitStatus, out)
	}
}

func (s *DockerTrustSuite) TestTrustedBuildTagFromReleasesRole(c *check.C) {
	testRequires(c, NotaryHosting)

	latestTag := s.setupTrustedImage(c, "trusted-build-releases-role")
	repoName := strings.TrimSuffix(latestTag, ":latest")

	// Now create the releases role
	s.notaryCreateDelegation(c, repoName, "targets/releases", s.not.keys[0].Public)
	s.notaryImportKey(c, repoName, "targets/releases", s.not.keys[0].Private)
	s.notaryPublish(c, repoName)

	// push a different tag to the releases role
	otherTag := fmt.Sprintf("%s:other", repoName)
	dockerCmd(c, "tag", "busybox", otherTag)

	pushCmd := exec.Command(dockerBinary, "push", otherTag)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("Trusted push failed: %s", out))
	s.assertTargetInRoles(c, repoName, "other", "targets/releases")
	s.assertTargetNotInRoles(c, repoName, "other", "targets")

	out, status := dockerCmd(c, "rmi", otherTag)
	c.Assert(status, check.Equals, 0, check.Commentf("docker rmi failed: %s", out))

	dockerFile := fmt.Sprintf(`
  FROM %s
  RUN []
    `, otherTag)

	name := "testtrustedbuildreleasesrole"

	buildCmd := buildImageCmd(name, dockerFile, true)
	s.trustedCmd(buildCmd)
	out, _, err = runCommandWithOutput(buildCmd)
	c.Assert(err, check.IsNil, check.Commentf("Trusted build failed: %s", out))
	c.Assert(out, checker.Contains, fmt.Sprintf("FROM %s@sha", repoName))
}

func (s *DockerTrustSuite) TestTrustedBuildTagIgnoresOtherDelegationRoles(c *check.C) {
	testRequires(c, NotaryHosting)

	latestTag := s.setupTrustedImage(c, "trusted-build-releases-role")
	repoName := strings.TrimSuffix(latestTag, ":latest")

	// Now create a non-releases delegation role
	s.notaryCreateDelegation(c, repoName, "targets/other", s.not.keys[0].Public)
	s.notaryImportKey(c, repoName, "targets/other", s.not.keys[0].Private)
	s.notaryPublish(c, repoName)

	// push a different tag to the other role
	otherTag := fmt.Sprintf("%s:other", repoName)
	dockerCmd(c, "tag", "busybox", otherTag)

	pushCmd := exec.Command(dockerBinary, "push", otherTag)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	c.Assert(err, check.IsNil, check.Commentf("Trusted push failed: %s", out))
	s.assertTargetInRoles(c, repoName, "other", "targets/other")
	s.assertTargetNotInRoles(c, repoName, "other", "targets")

	out, status := dockerCmd(c, "rmi", otherTag)
	c.Assert(status, check.Equals, 0, check.Commentf("docker rmi failed: %s", out))

	dockerFile := fmt.Sprintf(`
  FROM %s
  RUN []
    `, otherTag)

	name := "testtrustedbuildotherrole"

	buildCmd := buildImageCmd(name, dockerFile, true)
	s.trustedCmd(buildCmd)
	out, _, err = runCommandWithOutput(buildCmd)
	c.Assert(err, check.NotNil, check.Commentf("Trusted build expected to fail: %s", out))
}

// Issue #15634: COPY fails when path starts with "null"
func (s *DockerSuite) TestBuildNullStringInAddCopyVolume(c *check.C) {
	name := "testbuildnullstringinaddcopyvolume"

	volName := "nullvolume"

	if daemonPlatform == "windows" {
		volName = `C:\\nullvolume`
	}

	ctx, err := fakeContext(`
		FROM busybox

		ADD null /
		COPY nullfile /
		VOLUME `+volName+`
		`,
		map[string]string{
			"null":     "test1",
			"nullfile": "test2",
		},
	)
	c.Assert(err, check.IsNil)
	defer ctx.Close()

	_, err = buildImageFromContext(name, ctx, true)
	c.Assert(err, check.IsNil)
}

func (s *DockerSuite) TestBuildStopSignal(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support STOPSIGNAL yet
	imgName := "test_build_stop_signal"
	_, err := buildImage(imgName,
		`FROM busybox
		 STOPSIGNAL SIGKILL`,
		true)
	c.Assert(err, check.IsNil)
	res := inspectFieldJSON(c, imgName, "Config.StopSignal")
	if res != `"SIGKILL"` {
		c.Fatalf("Signal %s, expected SIGKILL", res)
	}

	containerName := "test-container-stop-signal"
	dockerCmd(c, "run", "-d", "--name", containerName, imgName, "top")

	res = inspectFieldJSON(c, containerName, "Config.StopSignal")
	if res != `"SIGKILL"` {
		c.Fatalf("Signal %s, expected SIGKILL", res)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArg(c *check.C) {
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	args := []string{"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal)}
	var dockerfile string
	if daemonPlatform == "windows" {
		// Bugs in Windows busybox port - use the default base image and native cmd stuff
		dockerfile = fmt.Sprintf(`FROM `+minimalBaseImage()+`
			ARG %s
			RUN echo %%%s%%
			CMD setlocal enableextensions && if defined %s (echo %%%s%%)`, envKey, envKey, envKey, envKey)
	} else {
		dockerfile = fmt.Sprintf(`FROM busybox
			ARG %s
			RUN echo $%s
			CMD echo $%s`, envKey, envKey, envKey)

	}

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || !strings.Contains(out, envVal) {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envVal)
	}

	containerName := "bldargCont"
	out, _ := dockerCmd(c, "run", "--name", containerName, imgName)
	out = strings.Trim(out, " \r\n'")
	if out != "" {
		c.Fatalf("run produced invalid output: %q, expected empty string", out)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgHistory(c *check.C) {
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	envDef := "bar1"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s=%s`, envKey, envDef)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || !strings.Contains(out, envVal) {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envVal)
	}

	out, _ := dockerCmd(c, "history", "--no-trunc", imgName)
	outputTabs := strings.Split(out, "\n")[1]
	if !strings.Contains(outputTabs, envDef) {
		c.Fatalf("failed to find arg default in image history output: %q expected: %q", outputTabs, envDef)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgCacheHit(c *check.C) {
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		RUN echo $%s`, envKey, envKey)

	origImgID := ""
	var err error
	if origImgID, err = buildImage(imgName, dockerfile, true, args...); err != nil {
		c.Fatal(err)
	}

	imgNameCache := "bldargtestcachehit"
	if newImgID, err := buildImage(imgNameCache, dockerfile, true, args...); err != nil || newImgID != origImgID {
		if err != nil {
			c.Fatal(err)
		}
		c.Fatalf("build didn't use cache! expected image id: %q built image id: %q", origImgID, newImgID)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgCacheMissExtraArg(c *check.C) {
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	extraEnvKey := "foo1"
	extraEnvVal := "bar1"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}

	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		ARG %s
		RUN echo $%s`, envKey, extraEnvKey, envKey)

	origImgID := ""
	var err error
	if origImgID, err = buildImage(imgName, dockerfile, true, args...); err != nil {
		c.Fatal(err)
	}

	imgNameCache := "bldargtestcachemiss"
	args = append(args, "--build-arg", fmt.Sprintf("%s=%s", extraEnvKey, extraEnvVal))
	if newImgID, err := buildImage(imgNameCache, dockerfile, true, args...); err != nil || newImgID == origImgID {
		if err != nil {
			c.Fatal(err)
		}
		c.Fatalf("build used cache, expected a miss!")
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgCacheMissSameArgDiffVal(c *check.C) {
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	newEnvVal := "bar1"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}

	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		RUN echo $%s`, envKey, envKey)

	origImgID := ""
	var err error
	if origImgID, err = buildImage(imgName, dockerfile, true, args...); err != nil {
		c.Fatal(err)
	}

	imgNameCache := "bldargtestcachemiss"
	args = []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, newEnvVal),
	}
	if newImgID, err := buildImage(imgNameCache, dockerfile, true, args...); err != nil || newImgID == origImgID {
		if err != nil {
			c.Fatal(err)
		}
		c.Fatalf("build used cache, expected a miss!")
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgOverrideArgDefinedBeforeEnv(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support ARG
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	envValOveride := "barOverride"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		ENV %s %s
		RUN echo $%s
		CMD echo $%s
        `, envKey, envKey, envValOveride, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envValOveride) != 2 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envValOveride)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); !strings.Contains(out, envValOveride) {
		c.Fatalf("run produced invalid output: %q, expected %q", out, envValOveride)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgOverrideEnvDefinedBeforeArg(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support ARG
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	envValOveride := "barOverride"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ENV %s %s
		ARG %s
		RUN echo $%s
		CMD echo $%s
        `, envKey, envValOveride, envKey, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envValOveride) != 2 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envValOveride)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); !strings.Contains(out, envValOveride) {
		c.Fatalf("run produced invalid output: %q, expected %q", out, envValOveride)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgExpansion(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support ARG
	imgName := "bldvarstest"

	wdVar := "WDIR"
	wdVal := "/tmp/"
	addVar := "AFILE"
	addVal := "addFile"
	copyVar := "CFILE"
	copyVal := "copyFile"
	envVar := "foo"
	envVal := "bar"
	exposeVar := "EPORT"
	exposeVal := "9999"
	userVar := "USER"
	userVal := "testUser"
	volVar := "VOL"
	volVal := "/testVol/"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", wdVar, wdVal),
		"--build-arg", fmt.Sprintf("%s=%s", addVar, addVal),
		"--build-arg", fmt.Sprintf("%s=%s", copyVar, copyVal),
		"--build-arg", fmt.Sprintf("%s=%s", envVar, envVal),
		"--build-arg", fmt.Sprintf("%s=%s", exposeVar, exposeVal),
		"--build-arg", fmt.Sprintf("%s=%s", userVar, userVal),
		"--build-arg", fmt.Sprintf("%s=%s", volVar, volVal),
	}
	ctx, err := fakeContext(fmt.Sprintf(`FROM busybox
		ARG %s
		WORKDIR ${%s}
		ARG %s
		ADD ${%s} testDir/
		ARG %s
		COPY $%s testDir/
		ARG %s
		ENV %s=${%s}
		ARG %s
		EXPOSE $%s
		ARG %s
		USER $%s
		ARG %s
		VOLUME ${%s}`,
		wdVar, wdVar, addVar, addVar, copyVar, copyVar, envVar, envVar,
		envVar, exposeVar, exposeVar, userVar, userVar, volVar, volVar),
		map[string]string{
			addVal:  "some stuff",
			copyVal: "some stuff",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(imgName, ctx, true, args...); err != nil {
		c.Fatal(err)
	}

	var resMap map[string]interface{}
	var resArr []string
	res := ""
	res = inspectField(c, imgName, "Config.WorkingDir")
	if res != filepath.ToSlash(filepath.Clean(wdVal)) {
		c.Fatalf("Config.WorkingDir value mismatch. Expected: %s, got: %s", filepath.ToSlash(filepath.Clean(wdVal)), res)
	}

	inspectFieldAndMarshall(c, imgName, "Config.Env", &resArr)

	found := false
	for _, v := range resArr {
		if fmt.Sprintf("%s=%s", envVar, envVal) == v {
			found = true
			break
		}
	}
	if !found {
		c.Fatalf("Config.Env value mismatch. Expected <key=value> to exist: %s=%s, got: %v",
			envVar, envVal, resArr)
	}

	inspectFieldAndMarshall(c, imgName, "Config.ExposedPorts", &resMap)
	if _, ok := resMap[fmt.Sprintf("%s/tcp", exposeVal)]; !ok {
		c.Fatalf("Config.ExposedPorts value mismatch. Expected exposed port: %s/tcp, got: %v", exposeVal, resMap)
	}

	res = inspectField(c, imgName, "Config.User")
	if res != userVal {
		c.Fatalf("Config.User value mismatch. Expected: %s, got: %s", userVal, res)
	}

	inspectFieldAndMarshall(c, imgName, "Config.Volumes", &resMap)
	if _, ok := resMap[volVal]; !ok {
		c.Fatalf("Config.Volumes value mismatch. Expected volume: %s, got: %v", volVal, resMap)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgExpansionOverride(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support ARG
	imgName := "bldvarstest"
	envKey := "foo"
	envVal := "bar"
	envKey1 := "foo1"
	envValOveride := "barOverride"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		ENV %s %s
		ENV %s ${%s}
		RUN echo $%s
		CMD echo $%s`, envKey, envKey, envValOveride, envKey1, envKey, envKey1, envKey1)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envValOveride) != 2 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envValOveride)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); !strings.Contains(out, envValOveride) {
		c.Fatalf("run produced invalid output: %q, expected %q", out, envValOveride)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgUntrustedDefinedAfterUse(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support ARG
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		RUN echo $%s
		ARG %s
		CMD echo $%s`, envKey, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Contains(out, envVal) {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("able to access environment variable in output: %q expected to be missing", out)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); out != "\n" {
		c.Fatalf("run produced invalid output: %q, expected empty string", out)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgBuiltinArg(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support --build-arg
	imgName := "bldargtest"
	envKey := "HTTP_PROXY"
	envVal := "bar"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		RUN echo $%s
		CMD echo $%s`, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || !strings.Contains(out, envVal) {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envVal)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); out != "\n" {
		c.Fatalf("run produced invalid output: %q, expected empty string", out)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgDefaultOverride(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support ARG
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	envValOveride := "barOverride"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envValOveride),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s=%s
		ENV %s $%s
		RUN echo $%s
		CMD echo $%s`, envKey, envVal, envKey, envKey, envKey, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envValOveride) != 1 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("failed to access environment variable in output: %q expected: %q", out, envValOveride)
	}

	containerName := "bldargCont"
	if out, _ := dockerCmd(c, "run", "--name", containerName, imgName); !strings.Contains(out, envValOveride) {
		c.Fatalf("run produced invalid output: %q, expected %q", out, envValOveride)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgUnconsumedArg(c *check.C) {
	imgName := "bldargtest"
	envKey := "foo"
	envVal := "bar"
	args := []string{
		"--build-arg", fmt.Sprintf("%s=%s", envKey, envVal),
	}
	dockerfile := fmt.Sprintf(`FROM busybox
		RUN echo $%s
		CMD echo $%s`, envKey, envKey)

	errStr := "One or more build-args"
	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err == nil {
		c.Fatalf("build succeeded, expected to fail. Output: %v", out)
	} else if !strings.Contains(out, errStr) {
		c.Fatalf("Unexpected error. output: %q, expected error: %q", out, errStr)
	}

}

func (s *DockerSuite) TestBuildBuildTimeArgQuotedValVariants(c *check.C) {
	imgName := "bldargtest"
	envKey := "foo"
	envKey1 := "foo1"
	envKey2 := "foo2"
	envKey3 := "foo3"
	args := []string{}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s=""
		ARG %s=''
		ARG %s="''"
		ARG %s='""'
		RUN [ "$%s" != "$%s" ]
		RUN [ "$%s" != "$%s" ]
		RUN [ "$%s" != "$%s" ]
		RUN [ "$%s" != "$%s" ]
		RUN [ "$%s" != "$%s" ]`, envKey, envKey1, envKey2, envKey3,
		envKey, envKey2, envKey, envKey3, envKey1, envKey2, envKey1, envKey3,
		envKey2, envKey3)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil {
		c.Fatalf("build failed to complete: %q %q", out, err)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgEmptyValVariants(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows does not support ARG
	imgName := "bldargtest"
	envKey := "foo"
	envKey1 := "foo1"
	envKey2 := "foo2"
	args := []string{}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s=
		ARG %s=""
		ARG %s=''
		RUN [ "$%s" == "$%s" ]
		RUN [ "$%s" == "$%s" ]
		RUN [ "$%s" == "$%s" ]`, envKey, envKey1, envKey2, envKey, envKey1, envKey1, envKey2, envKey, envKey2)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil {
		c.Fatalf("build failed to complete: %q %q", out, err)
	}
}

func (s *DockerSuite) TestBuildBuildTimeArgDefintionWithNoEnvInjection(c *check.C) {
	imgName := "bldargtest"
	envKey := "foo"
	args := []string{}
	dockerfile := fmt.Sprintf(`FROM busybox
		ARG %s
		RUN env`, envKey)

	if _, out, err := buildImageWithOut(imgName, dockerfile, true, args...); err != nil || strings.Count(out, envKey) != 1 {
		if err != nil {
			c.Fatalf("build failed to complete: %q %q", out, err)
		}
		c.Fatalf("unexpected number of occurrences of the arg in output: %q expected: 1", out)
	}
}

func (s *DockerSuite) TestBuildNoNamedVolume(c *check.C) {
	volName := "testname:/foo"

	if daemonPlatform == "windows" {
		volName = "testname:C:\\foo"
	}
	dockerCmd(c, "run", "-v", volName, "busybox", "sh", "-c", "touch /foo/oops")

	dockerFile := `FROM busybox
	VOLUME ` + volName + `
	RUN ls /foo/oops
	`
	_, err := buildImage("test", dockerFile, false)
	c.Assert(err, check.NotNil, check.Commentf("image build should have failed"))
}

func (s *DockerSuite) TestBuildTagEvent(c *check.C) {
	since := daemonUnixTime(c)

	dockerFile := `FROM busybox
	RUN echo events
	`
	_, err := buildImage("test", dockerFile, false)
	c.Assert(err, check.IsNil)

	until := daemonUnixTime(c)
	out, _ := dockerCmd(c, "events", "--since", since, "--until", until, "--filter", "type=image")
	events := strings.Split(strings.TrimSpace(out), "\n")
	actions := eventActionsByIDAndType(c, events, "test:latest", "image")
	var foundTag bool
	for _, a := range actions {
		if a == "tag" {
			foundTag = true
			break
		}
	}

	c.Assert(foundTag, checker.True, check.Commentf("No tag event found:\n%s", out))
}

// #15780
func (s *DockerSuite) TestBuildMultipleTags(c *check.C) {
	dockerfile := `
	FROM busybox
	MAINTAINER test-15780
	`
	cmd := exec.Command(dockerBinary, "build", "-t", "tag1", "-t", "tag2:v2",
		"-t", "tag1:latest", "-t", "tag1", "--no-cache", "-")
	cmd.Stdin = strings.NewReader(dockerfile)
	_, err := runCommand(cmd)
	c.Assert(err, check.IsNil)

	id1, err := getIDByName("tag1")
	c.Assert(err, check.IsNil)
	id2, err := getIDByName("tag2:v2")
	c.Assert(err, check.IsNil)
	c.Assert(id1, check.Equals, id2)
}

// #17290
func (s *DockerSuite) TestBuildCacheBrokenSymlink(c *check.C) {
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

// #17827
func (s *DockerSuite) TestBuildCacheRootSource(c *check.C) {
	name := "testbuildrootsource"
	ctx, err := fakeContext(`
	FROM busybox
	COPY / /data`,
		map[string]string{
			"foo": "bar",
		})
	c.Assert(err, checker.IsNil)
	defer ctx.Close()

	// warm up cache
	_, err = buildImageFromContext(name, ctx, true)
	c.Assert(err, checker.IsNil)

	// change file, should invalidate cache
	err = ioutil.WriteFile(filepath.Join(ctx.Dir, "foo"), []byte("baz"), 0644)
	c.Assert(err, checker.IsNil)

	_, out, err := buildImageFromContextWithOut(name, ctx, true)
	c.Assert(err, checker.IsNil)

	c.Assert(out, checker.Not(checker.Contains), "Using cache")
}

// #19375
func (s *DockerSuite) TestBuildFailsGitNotCallable(c *check.C) {
	cmd := exec.Command(dockerBinary, "build", "github.com/docker/v1.10-migrator.git")
	cmd.Env = append(cmd.Env, "PATH=")
	out, _, err := runCommandWithOutput(cmd)
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "unable to prepare context: unable to find 'git': ")

	cmd = exec.Command(dockerBinary, "build", "https://github.com/docker/v1.10-migrator.git")
	cmd.Env = append(cmd.Env, "PATH=")
	out, _, err = runCommandWithOutput(cmd)
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "unable to prepare context: unable to find 'git': ")
}

// TestBuildWorkdirWindowsPath tests that a Windows style path works as a workdir
func (s *DockerSuite) TestBuildWorkdirWindowsPath(c *check.C) {
	testRequires(c, DaemonIsWindows)
	name := "testbuildworkdirwindowspath"

	_, err := buildImage(name, `
	FROM `+WindowsBaseImage+`
	RUN mkdir C:\\work
	WORKDIR C:\\work
	RUN if "%CD%" NEQ "C:\work" exit -1
	`, true)

	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestBuildLabel(c *check.C) {
	name := "testbuildlabel"
	testLabel := "foo"

	_, err := buildImage(name, `
  FROM `+minimalBaseImage()+`
  LABEL default foo
`, false, "--label", testLabel)

	c.Assert(err, checker.IsNil)

	res := inspectFieldJSON(c, name, "Config.Labels")

	var labels map[string]string

	if err := json.Unmarshal([]byte(res), &labels); err != nil {
		c.Fatal(err)
	}

	if _, ok := labels[testLabel]; !ok {
		c.Fatal("label not found in image")
	}
}

func (s *DockerSuite) TestBuildLabelOneNode(c *check.C) {
	name := "testbuildlabel"

	_, err := buildImage(name, "FROM busybox", false, "--label", "foo=bar")

	c.Assert(err, checker.IsNil)

	res, err := inspectImage(name, "json .Config.Labels")
	c.Assert(err, checker.IsNil)
	var labels map[string]string

	if err := json.Unmarshal([]byte(res), &labels); err != nil {
		c.Fatal(err)
	}

	v, ok := labels["foo"]
	if !ok {
		c.Fatal("label `foo` not found in image")
	}
	c.Assert(v, checker.Equals, "bar")
}

func (s *DockerSuite) TestBuildLabelCacheCommit(c *check.C) {
	name := "testbuildlabelcachecommit"
	testLabel := "foo"

	if _, err := buildImage(name, `
  FROM `+minimalBaseImage()+`
  LABEL default foo
  `, false); err != nil {
		c.Fatal(err)
	}

	_, err := buildImage(name, `
  FROM `+minimalBaseImage()+`
  LABEL default foo
`, true, "--label", testLabel)

	c.Assert(err, checker.IsNil)

	res := inspectFieldJSON(c, name, "Config.Labels")

	var labels map[string]string

	if err := json.Unmarshal([]byte(res), &labels); err != nil {
		c.Fatal(err)
	}

	if _, ok := labels[testLabel]; !ok {
		c.Fatal("label not found in image")
	}
}

func (s *DockerSuite) TestBuildLabelMultiple(c *check.C) {
	name := "testbuildlabelmultiple"
	testLabels := map[string]string{
		"foo": "bar",
		"123": "456",
	}

	labelArgs := []string{}

	for k, v := range testLabels {
		labelArgs = append(labelArgs, "--label", k+"="+v)
	}

	_, err := buildImage(name, `
  FROM `+minimalBaseImage()+`
  LABEL default foo
`, false, labelArgs...)

	if err != nil {
		c.Fatal("error building image with labels", err)
	}

	res := inspectFieldJSON(c, name, "Config.Labels")

	var labels map[string]string

	if err := json.Unmarshal([]byte(res), &labels); err != nil {
		c.Fatal(err)
	}

	for k, v := range testLabels {
		if x, ok := labels[k]; !ok || x != v {
			c.Fatalf("label %s=%s not found in image", k, v)
		}
	}
}

func (s *DockerSuite) TestBuildLabelOverwrite(c *check.C) {
	name := "testbuildlabeloverwrite"
	testLabel := "foo"
	testValue := "bar"

	_, err := buildImage(name, `
  FROM `+minimalBaseImage()+`
  LABEL `+testLabel+`+ foo
`, false, []string{"--label", testLabel + "=" + testValue}...)

	if err != nil {
		c.Fatal("error building image with labels", err)
	}

	res := inspectFieldJSON(c, name, "Config.Labels")

	var labels map[string]string

	if err := json.Unmarshal([]byte(res), &labels); err != nil {
		c.Fatal(err)
	}

	v, ok := labels[testLabel]
	if !ok {
		c.Fatal("label not found in image")
	}

	if v != testValue {
		c.Fatal("label not overwritten")
	}
}

func (s *DockerRegistryAuthHtpasswdSuite) TestBuildFromAuthenticatedRegistry(c *check.C) {
	dockerCmd(c, "login", "-u", s.reg.username, "-p", s.reg.password, privateRegistryURL)

	baseImage := privateRegistryURL + "/baseimage"

	_, err := buildImage(baseImage, `
	FROM busybox
	ENV env1 val1
	`, true)

	c.Assert(err, checker.IsNil)

	dockerCmd(c, "push", baseImage)
	dockerCmd(c, "rmi", baseImage)

	_, err = buildImage(baseImage, fmt.Sprintf(`
	FROM %s
	ENV env2 val2
	`, baseImage), true)

	c.Assert(err, checker.IsNil)
}

func (s *DockerRegistryAuthHtpasswdSuite) TestBuildWithExternalAuth(c *check.C) {
	osPath := os.Getenv("PATH")
	defer os.Setenv("PATH", osPath)

	workingDir, err := os.Getwd()
	c.Assert(err, checker.IsNil)
	absolute, err := filepath.Abs(filepath.Join(workingDir, "fixtures", "auth"))
	c.Assert(err, checker.IsNil)
	testPath := fmt.Sprintf("%s%c%s", osPath, filepath.ListSeparator, absolute)

	os.Setenv("PATH", testPath)

	repoName := fmt.Sprintf("%v/dockercli/busybox:authtest", privateRegistryURL)

	tmp, err := ioutil.TempDir("", "integration-cli-")
	c.Assert(err, checker.IsNil)

	externalAuthConfig := `{ "credsStore": "shell-test" }`

	configPath := filepath.Join(tmp, "config.json")
	err = ioutil.WriteFile(configPath, []byte(externalAuthConfig), 0644)
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "--config", tmp, "login", "-u", s.reg.username, "-p", s.reg.password, privateRegistryURL)

	b, err := ioutil.ReadFile(configPath)
	c.Assert(err, checker.IsNil)
	c.Assert(string(b), checker.Not(checker.Contains), "\"auth\":")

	dockerCmd(c, "--config", tmp, "tag", "busybox", repoName)
	dockerCmd(c, "--config", tmp, "push", repoName)

	// make sure the image is pulled when building
	dockerCmd(c, "rmi", repoName)

	buildCmd := exec.Command(dockerBinary, "--config", tmp, "build", "-")
	buildCmd.Stdin = strings.NewReader(fmt.Sprintf("FROM %s", repoName))

	out, _, err := runCommandWithOutput(buildCmd)
	c.Assert(err, check.IsNil, check.Commentf(out))
}

// Test cases in #22036
func (s *DockerSuite) TestBuildLabelsOverride(c *check.C) {
	// Command line option labels will always override
	name := "scratchy"
	expected := `{"bar":"from-flag","foo":"from-flag"}`
	_, err := buildImage(name,
		`FROM `+minimalBaseImage()+`
                LABEL foo=from-dockerfile`,
		true, "--label", "foo=from-flag", "--label", "bar=from-flag")
	c.Assert(err, check.IsNil)

	res := inspectFieldJSON(c, name, "Config.Labels")
	if res != expected {
		c.Fatalf("Labels %s, expected %s", res, expected)
	}

	name = "from"
	expected = `{"foo":"from-dockerfile"}`
	_, err = buildImage(name,
		`FROM `+minimalBaseImage()+`
                LABEL foo from-dockerfile`,
		true)
	c.Assert(err, check.IsNil)

	res = inspectFieldJSON(c, name, "Config.Labels")
	if res != expected {
		c.Fatalf("Labels %s, expected %s", res, expected)
	}

	// Command line option label will override even via `FROM`
	name = "new"
	expected = `{"bar":"from-dockerfile2","foo":"new"}`
	_, err = buildImage(name,
		`FROM from
                LABEL bar from-dockerfile2`,
		true, "--label", "foo=new")
	c.Assert(err, check.IsNil)

	res = inspectFieldJSON(c, name, "Config.Labels")
	if res != expected {
		c.Fatalf("Labels %s, expected %s", res, expected)
	}

	// Command line option without a value set (--label foo, --label bar=)
	// will be treated as --label foo="", --label bar=""
	name = "scratchy2"
	expected = `{"bar":"","foo":""}`
	_, err = buildImage(name,
		`FROM `+minimalBaseImage()+`
                LABEL foo=from-dockerfile`,
		true, "--label", "foo", "--label", "bar=")
	c.Assert(err, check.IsNil)

	res = inspectFieldJSON(c, name, "Config.Labels")
	if res != expected {
		c.Fatalf("Labels %s, expected %s", res, expected)
	}

	// Command line option without a value set (--label foo, --label bar=)
	// will be treated as --label foo="", --label bar=""
	// This time is for inherited images
	name = "new2"
	expected = `{"bar":"","foo":""}`
	_, err = buildImage(name,
		`FROM from
                LABEL bar from-dockerfile2`,
		true, "--label", "foo=", "--label", "bar")
	c.Assert(err, check.IsNil)

	res = inspectFieldJSON(c, name, "Config.Labels")
	if res != expected {
		c.Fatalf("Labels %s, expected %s", res, expected)
	}

	// Command line option labels with only `FROM`
	name = "scratchy"
	expected = `{"bar":"from-flag","foo":"from-flag"}`
	_, err = buildImage(name,
		`FROM `+minimalBaseImage(),
		true, "--label", "foo=from-flag", "--label", "bar=from-flag")
	c.Assert(err, check.IsNil)

	res = inspectFieldJSON(c, name, "Config.Labels")
	if res != expected {
		c.Fatalf("Labels %s, expected %s", res, expected)
	}

	// Command line option labels with env var
	name = "scratchz"
	expected = `{"bar":"$PATH"}`
	_, err = buildImage(name,
		`FROM scratch`,
		true, "--label", "bar=$PATH")
	c.Assert(err, check.IsNil)

	res = inspectFieldJSON(c, name, "Config.Labels")
	if res != expected {
		c.Fatalf("Labels %s, expected %s", res, expected)
	}

}

// Test case for #22855
func (s *DockerSuite) TestBuildDeleteCommittedFile(c *check.C) {
	name := "test-delete-committed-file"

	_, err := buildImage(name,
		`FROM busybox
		RUN echo test > file
		RUN test -e file
		RUN rm file
		RUN sh -c "! test -e file"`, false)
	if err != nil {
		c.Fatal(err)
	}
}

// #20083
func (s *DockerSuite) TestBuildDockerignoreComment(c *check.C) {
	// TODO Windows: Figure out why this test is flakey on TP5. If you add
	// something like RUN sleep 5, or even RUN ls /tmp after the ADD line,
	// it is more reliable, but that's not a good fix.
	testRequires(c, DaemonIsLinux)

	name := "testbuilddockerignorecleanpaths"
	dockerfile := `
        FROM busybox
        ADD . /tmp/
        RUN sh -c "(ls -la /tmp/#1)"
        RUN sh -c "(! ls -la /tmp/#2)"
        RUN sh -c "(! ls /tmp/foo) && (! ls /tmp/foo2) && (ls /tmp/dir1/foo)"`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo":      "foo",
		"foo2":     "foo2",
		"dir1/foo": "foo in dir1",
		"#1":       "# file 1",
		"#2":       "# file 2",
		".dockerignore": `# Visual C++ cache files
# because we have git ;-)
# The above comment is from #20083
foo
#dir1/foo
foo2
# The following is considered as comment as # is at the beginning
#1
# The following is not considered as comment as # is not at the beginning
  #2
`,
	})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		c.Fatal(err)
	}
}

// Test case for #23221
func (s *DockerSuite) TestBuildWithUTF8BOM(c *check.C) {
	name := "test-with-utf8-bom"
	dockerfile := []byte(`FROM busybox`)
	bomDockerfile := append([]byte{0xEF, 0xBB, 0xBF}, dockerfile...)
	ctx, err := fakeContextFromNewTempDir()
	c.Assert(err, check.IsNil)
	defer ctx.Close()
	err = ctx.addFile("Dockerfile", bomDockerfile)
	c.Assert(err, check.IsNil)
	_, err = buildImageFromContext(name, ctx, true)
	c.Assert(err, check.IsNil)
}

// Test case for UTF-8 BOM in .dockerignore, related to #23221
func (s *DockerSuite) TestBuildWithUTF8BOMDockerignore(c *check.C) {
	name := "test-with-utf8-bom-dockerignore"
	dockerfile := `
        FROM busybox
		ADD . /tmp/
		RUN ls -la /tmp
		RUN sh -c "! ls /tmp/Dockerfile"
		RUN ls /tmp/.dockerignore`
	dockerignore := []byte("./Dockerfile\n")
	bomDockerignore := append([]byte{0xEF, 0xBB, 0xBF}, dockerignore...)
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile": dockerfile,
	})
	c.Assert(err, check.IsNil)
	defer ctx.Close()
	err = ctx.addFile(".dockerignore", bomDockerignore)
	c.Assert(err, check.IsNil)
	_, err = buildImageFromContext(name, ctx, true)
	if err != nil {
		c.Fatal(err)
	}
}

// #22489 Shell test to confirm config gets updated correctly
func (s *DockerSuite) TestBuildShellUpdatesConfig(c *check.C) {
	name := "testbuildshellupdatesconfig"

	expected := `["foo","-bar","#(nop) ","SHELL [foo -bar]"]`
	_, err := buildImage(name,
		`FROM `+minimalBaseImage()+`
        SHELL ["foo", "-bar"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectFieldJSON(c, name, "ContainerConfig.Cmd")
	if res != expected {
		c.Fatalf("%s, expected %s", res, expected)
	}
	res = inspectFieldJSON(c, name, "ContainerConfig.Shell")
	if res != `["foo","-bar"]` {
		c.Fatalf(`%s, expected ["foo","-bar"]`, res)
	}
}

// #22489 Changing the shell multiple times and CMD after.
func (s *DockerSuite) TestBuildShellMultiple(c *check.C) {
	name := "testbuildshellmultiple"

	_, out, _, err := buildImageWithStdoutStderr(name,
		`FROM busybox
		RUN echo defaultshell
		SHELL ["echo"]
		RUN echoshell
		SHELL ["ls"]
		RUN -l
		CMD -l`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	// Must contain 'defaultshell' twice
	if len(strings.Split(out, "defaultshell")) != 3 {
		c.Fatalf("defaultshell should have appeared twice in %s", out)
	}

	// Must contain 'echoshell' twice
	if len(strings.Split(out, "echoshell")) != 3 {
		c.Fatalf("echoshell should have appeared twice in %s", out)
	}

	// Must contain "total " (part of ls -l)
	if !strings.Contains(out, "total ") {
		c.Fatalf("%s should have contained 'total '", out)
	}

	// A container started from the image uses the shell-form CMD.
	// Last shell is ls. CMD is -l. So should contain 'total '.
	outrun, _ := dockerCmd(c, "run", "--rm", name)
	if !strings.Contains(outrun, "total ") {
		c.Fatalf("Expected started container to run ls -l. %s", outrun)
	}
}

// #22489. Changed SHELL with ENTRYPOINT
func (s *DockerSuite) TestBuildShellEntrypoint(c *check.C) {
	name := "testbuildshellentrypoint"

	_, err := buildImage(name,
		`FROM busybox
		SHELL ["ls"]
		ENTRYPOINT -l`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	// A container started from the image uses the shell-form ENTRYPOINT.
	// Shell is ls. ENTRYPOINT is -l. So should contain 'total '.
	outrun, _ := dockerCmd(c, "run", "--rm", name)
	if !strings.Contains(outrun, "total ") {
		c.Fatalf("Expected started container to run ls -l. %s", outrun)
	}
}

// #22489 Shell test to confirm shell is inherited in a subsequent build
func (s *DockerSuite) TestBuildShellInherited(c *check.C) {
	name1 := "testbuildshellinherited1"
	_, err := buildImage(name1,
		`FROM busybox
        SHELL ["ls"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	name2 := "testbuildshellinherited2"
	_, out, _, err := buildImageWithStdoutStderr(name2,
		`FROM `+name1+`
        RUN -l`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	// ls -l has "total " followed by some number in it, ls without -l does not.
	if !strings.Contains(out, "total ") {
		c.Fatalf("Should have seen total in 'ls -l'.\n%s", out)
	}
}

// #22489 Shell test to confirm non-JSON doesn't work
func (s *DockerSuite) TestBuildShellNotJSON(c *check.C) {
	name := "testbuildshellnotjson"

	_, err := buildImage(name,
		`FROM `+minimalBaseImage()+`
        sHeLl exec -form`, // Casing explicit to ensure error is upper-cased.
		true)
	if err == nil {
		c.Fatal("Image build should have failed")
	}
	if !strings.Contains(err.Error(), "SHELL requires the arguments to be in JSON form") {
		c.Fatal("Error didn't indicate that arguments must be in JSON form")
	}
}

// #22489 Windows shell test to confirm native is powershell if executing a PS command
// This would error if the default shell were still cmd.
func (s *DockerSuite) TestBuildShellWindowsPowershell(c *check.C) {
	testRequires(c, DaemonIsWindows)
	name := "testbuildshellpowershell"
	_, out, err := buildImageWithOut(name,
		`FROM `+minimalBaseImage()+`
        SHELL ["powershell", "-command"]
		RUN Write-Host John`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(out, "\nJohn\n") {
		c.Fatalf("Line with 'John' not found in output %q", out)
	}
}

// #22868. Make sure shell-form CMD is marked as escaped in the config of the image
func (s *DockerSuite) TestBuildCmdShellArgsEscaped(c *check.C) {
	testRequires(c, DaemonIsWindows)
	name := "testbuildcmdshellescaped"
	_, err := buildImage(name, `
  FROM `+minimalBaseImage()+`
  CMD "ipconfig"
  `, true)
	if err != nil {
		c.Fatal(err)
	}
	res := inspectFieldJSON(c, name, "Config.ArgsEscaped")
	if res != "true" {
		c.Fatalf("CMD did not update Config.ArgsEscaped on image: %v", res)
	}
	dockerCmd(c, "run", "--name", "inspectme", name)
	dockerCmd(c, "wait", "inspectme")
	res = inspectFieldJSON(c, name, "Config.Cmd")

	if res != `["cmd","/S","/C","\"ipconfig\""]` {
		c.Fatalf("CMD was not escaped Config.Cmd: got %v", res)
	}
}

func (s *DockerSuite) TestContinueCharSpace(c *check.C) {
	// Test to make sure that we don't treat a \ as a continuation
	// character IF there are spaces (or tabs) after it on the same line
	name := "testbuildcont"
	_, err := buildImage(name, "FROM busybox\nRUN echo hi \\\t\nbye", true)
	c.Assert(err, check.NotNil, check.Commentf("Build 1 should fail - didn't"))

	_, err = buildImage(name, "FROM busybox\nRUN echo hi \\ \nbye", true)
	c.Assert(err, check.NotNil, check.Commentf("Build 2 should fail - didn't"))
}

// Test case for #24912.
func (s *DockerSuite) TestBuildStepsWithProgress(c *check.C) {
	name := "testbuildstepswithprogress"

	totalRun := 5
	_, out, err := buildImageWithOut(name, "FROM busybox\n"+strings.Repeat("RUN echo foo\n", totalRun), true)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, fmt.Sprintf("Step 1/%d : FROM busybox", 1+totalRun))
	for i := 2; i <= 1+totalRun; i++ {
		c.Assert(out, checker.Contains, fmt.Sprintf("Step %d/%d : RUN echo foo", i, 1+totalRun))
	}
}

func (s *DockerSuite) TestBuildWithFailure(c *check.C) {
	name := "testbuildwithfailure"

	// First test case can only detect `nobody` in runtime so all steps will show up
	buildCmd := "FROM busybox\nRUN nobody"
	_, stdout, _, err := buildImageWithStdoutStderr(name, buildCmd, false, "--force-rm", "--rm")
	c.Assert(err, checker.NotNil)
	c.Assert(stdout, checker.Contains, "Step 1/2 : FROM busybox")
	c.Assert(stdout, checker.Contains, "Step 2/2 : RUN nobody")

	// Second test case `FFOM` should have been detected before build runs so no steps
	buildCmd = "FFOM nobody\nRUN nobody"
	_, stdout, _, err = buildImageWithStdoutStderr(name, buildCmd, false, "--force-rm", "--rm")
	c.Assert(err, checker.NotNil)
	c.Assert(stdout, checker.Not(checker.Contains), "Step 1/2 : FROM busybox")
	c.Assert(stdout, checker.Not(checker.Contains), "Step 2/2 : RUN nobody")
}

func (s *DockerSuite) TestBuildCacheFrom(c *check.C) {
	testRequires(c, DaemonIsLinux) // All tests that do save are skipped in windows
	dockerfile := `
		FROM busybox
		ENV FOO=bar
		ADD baz /
		RUN touch bax`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile": dockerfile,
		"baz":        "baz",
	})
	c.Assert(err, checker.IsNil)
	defer ctx.Close()

	id1, err := buildImageFromContext("build1", ctx, true)
	c.Assert(err, checker.IsNil)

	// rebuild with cache-from
	id2, out, err := buildImageFromContextWithOut("build2", ctx, true, "--cache-from=build1")
	c.Assert(err, checker.IsNil)
	c.Assert(id1, checker.Equals, id2)
	c.Assert(strings.Count(out, "Using cache"), checker.Equals, 3)
	dockerCmd(c, "rmi", "build2")

	// no cache match with unknown source
	id2, out, err = buildImageFromContextWithOut("build2", ctx, true, "--cache-from=nosuchtag")
	c.Assert(err, checker.IsNil)
	c.Assert(id1, checker.Not(checker.Equals), id2)
	c.Assert(strings.Count(out, "Using cache"), checker.Equals, 0)
	dockerCmd(c, "rmi", "build2")

	// clear parent images
	tempDir, err := ioutil.TempDir("", "test-build-cache-from-")
	if err != nil {
		c.Fatalf("failed to create temporary directory: %s", tempDir)
	}
	defer os.RemoveAll(tempDir)
	tempFile := filepath.Join(tempDir, "img.tar")
	dockerCmd(c, "save", "-o", tempFile, "build1")
	dockerCmd(c, "rmi", "build1")
	dockerCmd(c, "load", "-i", tempFile)
	parentID, _ := dockerCmd(c, "inspect", "-f", "{{.Parent}}", "build1")
	c.Assert(strings.TrimSpace(parentID), checker.Equals, "")

	// cache still applies without parents
	id2, out, err = buildImageFromContextWithOut("build2", ctx, true, "--cache-from=build1")
	c.Assert(err, checker.IsNil)
	c.Assert(id1, checker.Equals, id2)
	c.Assert(strings.Count(out, "Using cache"), checker.Equals, 3)
	history1, _ := dockerCmd(c, "history", "-q", "build2")

	// Retry, no new intermediate images
	id3, out, err := buildImageFromContextWithOut("build3", ctx, true, "--cache-from=build1")
	c.Assert(err, checker.IsNil)
	c.Assert(id1, checker.Equals, id3)
	c.Assert(strings.Count(out, "Using cache"), checker.Equals, 3)
	history2, _ := dockerCmd(c, "history", "-q", "build3")

	c.Assert(history1, checker.Equals, history2)
	dockerCmd(c, "rmi", "build2")
	dockerCmd(c, "rmi", "build3")
	dockerCmd(c, "rmi", "build1")
	dockerCmd(c, "load", "-i", tempFile)

	// Modify file, everything up to last command and layers are reused
	dockerfile = `
		FROM busybox
		ENV FOO=bar
		ADD baz /
		RUN touch newfile`
	err = ioutil.WriteFile(filepath.Join(ctx.Dir, "Dockerfile"), []byte(dockerfile), 0644)
	c.Assert(err, checker.IsNil)

	id2, out, err = buildImageFromContextWithOut("build2", ctx, true, "--cache-from=build1")
	c.Assert(err, checker.IsNil)
	c.Assert(id1, checker.Not(checker.Equals), id2)
	c.Assert(strings.Count(out, "Using cache"), checker.Equals, 2)

	layers1Str, _ := dockerCmd(c, "inspect", "-f", "{{json .RootFS.Layers}}", "build1")
	layers2Str, _ := dockerCmd(c, "inspect", "-f", "{{json .RootFS.Layers}}", "build2")

	var layers1 []string
	var layers2 []string
	c.Assert(json.Unmarshal([]byte(layers1Str), &layers1), checker.IsNil)
	c.Assert(json.Unmarshal([]byte(layers2Str), &layers2), checker.IsNil)

	c.Assert(len(layers1), checker.Equals, len(layers2))
	for i := 0; i < len(layers1)-1; i++ {
		c.Assert(layers1[i], checker.Equals, layers2[i])
	}
	c.Assert(layers1[len(layers1)-1], checker.Not(checker.Equals), layers2[len(layers1)-1])
}
