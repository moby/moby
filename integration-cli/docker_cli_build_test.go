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
	"testing"
	"text/template"
	"time"

	"github.com/docker/docker/builder/command"
	"github.com/docker/docker/pkg/archive"
)

func TestBuildJSONEmptyRun(t *testing.T) {
	name := "testbuildjsonemptyrun"
	defer deleteImages(name)

	_, err := buildImage(
		name,
		`
    FROM busybox
    RUN []
    `,
		true)

	if err != nil {
		t.Fatal("error when dealing with a RUN statement with empty JSON array")
	}

	logDone("build - RUN with an empty array should not panic")
}

func TestBuildEmptyWhitespace(t *testing.T) {
	name := "testbuildemptywhitespace"
	defer deleteImages(name)

	_, err := buildImage(
		name,
		`
    FROM busybox
    COPY
      quux \
      bar
    `,
		true)

	if err == nil {
		t.Fatal("no error when dealing with a COPY statement with no content on the same line")
	}

	logDone("build - statements with whitespace and no content should generate a parse error")
}

func TestBuildShCmdJSONEntrypoint(t *testing.T) {
	name := "testbuildshcmdjsonentrypoint"
	defer deleteImages(name)

	_, err := buildImage(
		name,
		`
    FROM busybox
    ENTRYPOINT ["/bin/echo"]
    CMD echo test
    `,
		true)

	if err != nil {
		t.Fatal(err)
	}

	out, _, err := runCommandWithOutput(
		exec.Command(
			dockerBinary,
			"run",
			"--rm",
			name))

	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(out) != "/bin/sh -c echo test" {
		t.Fatal("CMD did not contain /bin/sh -c")
	}

	logDone("build - CMD should always contain /bin/sh -c when specified without JSON")
}

func TestBuildEnvironmentReplacementUser(t *testing.T) {
	name := "testbuildenvironmentreplacement"
	defer deleteImages(name)

	_, err := buildImage(name, `
  FROM scratch
  ENV user foo
  USER ${user}
  `, true)
	if err != nil {
		t.Fatal(err)
	}

	res, err := inspectFieldJSON(name, "Config.User")
	if err != nil {
		t.Fatal(err)
	}

	if res != `"foo"` {
		t.Fatal("User foo from environment not in Config.User on image")
	}

	logDone("build - user environment replacement")
}

func TestBuildEnvironmentReplacementVolume(t *testing.T) {
	name := "testbuildenvironmentreplacement"
	defer deleteImages(name)

	_, err := buildImage(name, `
  FROM scratch
  ENV volume /quux
  VOLUME ${volume}
  `, true)
	if err != nil {
		t.Fatal(err)
	}

	res, err := inspectFieldJSON(name, "Config.Volumes")
	if err != nil {
		t.Fatal(err)
	}

	var volumes map[string]interface{}

	if err := json.Unmarshal([]byte(res), &volumes); err != nil {
		t.Fatal(err)
	}

	if _, ok := volumes["/quux"]; !ok {
		t.Fatal("Volume /quux from environment not in Config.Volumes on image")
	}

	logDone("build - volume environment replacement")
}

func TestBuildEnvironmentReplacementExpose(t *testing.T) {
	name := "testbuildenvironmentreplacement"
	defer deleteImages(name)

	_, err := buildImage(name, `
  FROM scratch
  ENV port 80
  EXPOSE ${port}
  `, true)
	if err != nil {
		t.Fatal(err)
	}

	res, err := inspectFieldJSON(name, "Config.ExposedPorts")
	if err != nil {
		t.Fatal(err)
	}

	var exposedPorts map[string]interface{}

	if err := json.Unmarshal([]byte(res), &exposedPorts); err != nil {
		t.Fatal(err)
	}

	if _, ok := exposedPorts["80/tcp"]; !ok {
		t.Fatal("Exposed port 80 from environment not in Config.ExposedPorts on image")
	}

	logDone("build - expose environment replacement")
}

func TestBuildEnvironmentReplacementWorkdir(t *testing.T) {
	name := "testbuildenvironmentreplacement"
	defer deleteImages(name)

	_, err := buildImage(name, `
  FROM busybox
  ENV MYWORKDIR /work
  RUN mkdir ${MYWORKDIR}
  WORKDIR ${MYWORKDIR}
  `, true)

	if err != nil {
		t.Fatal(err)
	}

	logDone("build - workdir environment replacement")
}

func TestBuildEnvironmentReplacementAddCopy(t *testing.T) {
	name := "testbuildenvironmentreplacement"
	defer deleteImages(name)

	ctx, err := fakeContext(`
  FROM scratch
  ENV baz foo
  ENV quux bar
  ENV dot .

  ADD ${baz} ${dot}
  COPY ${quux} ${dot}
  `,
		map[string]string{
			"foo": "test1",
			"bar": "test2",
		})

	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}

	logDone("build - add/copy environment replacement")
}

func TestBuildEnvironmentReplacementEnv(t *testing.T) {
	name := "testbuildenvironmentreplacement"

	defer deleteImages(name)

	_, err := buildImage(name,
		`
  FROM scratch
  ENV foo foo
  ENV bar ${foo}
  `, true)

	if err != nil {
		t.Fatal(err)
	}

	res, err := inspectFieldJSON(name, "Config.Env")
	if err != nil {
		t.Fatal(err)
	}

	envResult := []string{}

	if err = unmarshalJSON([]byte(res), &envResult); err != nil {
		t.Fatal(err)
	}

	found := false

	for _, env := range envResult {
		parts := strings.SplitN(env, "=", 2)
		if parts[0] == "bar" {
			found = true
			if parts[1] != "foo" {
				t.Fatalf("Could not find replaced var for env `bar`: got %q instead of `foo`", parts[1])
			}
		}
	}

	if !found {
		t.Fatal("Never found the `bar` env variable")
	}

	logDone("build - env environment replacement")
}

func TestBuildHandleEscapes(t *testing.T) {
	name := "testbuildhandleescapes"

	defer deleteImages(name)

	_, err := buildImage(name,
		`
  FROM scratch
  ENV FOO bar
  VOLUME ${FOO}
  `, true)

	if err != nil {
		t.Fatal(err)
	}

	var result map[string]map[string]struct{}

	res, err := inspectFieldJSON(name, "Config.Volumes")
	if err != nil {
		t.Fatal(err)
	}

	if err = unmarshalJSON([]byte(res), &result); err != nil {
		t.Fatal(err)
	}

	if _, ok := result["bar"]; !ok {
		t.Fatal("Could not find volume bar set from env foo in volumes table")
	}

	deleteImages(name)

	_, err = buildImage(name,
		`
  FROM scratch
  ENV FOO bar
  VOLUME \${FOO}
  `, true)

	if err != nil {
		t.Fatal(err)
	}

	res, err = inspectFieldJSON(name, "Config.Volumes")
	if err != nil {
		t.Fatal(err)
	}

	if err = unmarshalJSON([]byte(res), &result); err != nil {
		t.Fatal(err)
	}

	if _, ok := result["${FOO}"]; !ok {
		t.Fatal("Could not find volume ${FOO} set from env foo in volumes table")
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
		t.Fatal(err)
	}

	res, err = inspectFieldJSON(name, "Config.Volumes")
	if err != nil {
		t.Fatal(err)
	}

	if err = unmarshalJSON([]byte(res), &result); err != nil {
		t.Fatal(err)
	}

	if _, ok := result[`\\\\\\${FOO}`]; !ok {
		t.Fatal(`Could not find volume \\\\\\${FOO} set from env foo in volumes table`)
	}

	logDone("build - handle escapes")
}

func TestBuildOnBuildLowercase(t *testing.T) {
	name := "testbuildonbuildlowercase"
	name2 := "testbuildonbuildlowercase2"

	defer deleteImages(name, name2)

	_, err := buildImage(name,
		`
  FROM busybox
  onbuild run echo quux
  `, true)

	if err != nil {
		t.Fatal(err)
	}

	_, out, err := buildImageWithOut(name2, fmt.Sprintf(`
  FROM %s
  `, name), true)

	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "quux") {
		t.Fatalf("Did not receive the expected echo text, got %s", out)
	}

	if strings.Contains(out, "ONBUILD ONBUILD") {
		t.Fatalf("Got an ONBUILD ONBUILD error with no error: got %s", out)
	}

	logDone("build - handle case-insensitive onbuild statement")
}

func TestBuildEnvEscapes(t *testing.T) {
	name := "testbuildenvescapes"
	defer deleteImages(name)
	defer deleteAllContainers()
	_, err := buildImage(name,
		`
    FROM busybox
    ENV TEST foo
    CMD echo \$
    `,
		true)

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-t", name))

	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(out) != "$" {
		t.Fatalf("Env TEST was not overwritten with bar when foo was supplied to dockerfile: was %q", strings.TrimSpace(out))
	}

	logDone("build - env should handle \\$ properly")
}

func TestBuildEnvOverwrite(t *testing.T) {
	name := "testbuildenvoverwrite"
	defer deleteImages(name)
	defer deleteAllContainers()

	_, err := buildImage(name,
		`
    FROM busybox
    ENV TEST foo
    CMD echo ${TEST}
    `,
		true)

	if err != nil {
		t.Fatal(err)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-e", "TEST=bar", "-t", name))

	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(out) != "bar" {
		t.Fatalf("Env TEST was not overwritten with bar when foo was supplied to dockerfile: was %q", strings.TrimSpace(out))
	}

	logDone("build - env should overwrite builder ENV during run")
}

func TestBuildOnBuildForbiddenMaintainerInSourceImage(t *testing.T) {
	name := "testbuildonbuildforbiddenmaintainerinsourceimage"
	defer deleteImages("onbuild")
	defer deleteImages(name)
	defer deleteAllContainers()

	createCmd := exec.Command(dockerBinary, "create", "busybox", "true")
	out, _, _, err := runCommandWithStdoutStderr(createCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	commitCmd := exec.Command(dockerBinary, "commit", "--run", "{\"OnBuild\":[\"MAINTAINER docker.io\"]}", cleanedContainerID, "onbuild")

	if _, err := runCommand(commitCmd); err != nil {
		t.Fatal(err)
	}

	_, err = buildImage(name,
		`FROM onbuild`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "maintainer isn't allowed as an ONBUILD trigger") {
			t.Fatalf("Wrong error %v, must be about MAINTAINER and ONBUILD in source image", err)
		}
	} else {
		t.Fatal("Error must not be nil")
	}
	logDone("build - onbuild forbidden maintainer in source image")

}

func TestBuildOnBuildForbiddenFromInSourceImage(t *testing.T) {
	name := "testbuildonbuildforbiddenfrominsourceimage"
	defer deleteImages("onbuild")
	defer deleteImages(name)
	defer deleteAllContainers()

	createCmd := exec.Command(dockerBinary, "create", "busybox", "true")
	out, _, _, err := runCommandWithStdoutStderr(createCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	commitCmd := exec.Command(dockerBinary, "commit", "--run", "{\"OnBuild\":[\"FROM busybox\"]}", cleanedContainerID, "onbuild")

	if _, err := runCommand(commitCmd); err != nil {
		t.Fatal(err)
	}

	_, err = buildImage(name,
		`FROM onbuild`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "from isn't allowed as an ONBUILD trigger") {
			t.Fatalf("Wrong error %v, must be about FROM and ONBUILD in source image", err)
		}
	} else {
		t.Fatal("Error must not be nil")
	}
	logDone("build - onbuild forbidden from in source image")

}

func TestBuildOnBuildForbiddenChainedInSourceImage(t *testing.T) {
	name := "testbuildonbuildforbiddenchainedinsourceimage"
	defer deleteImages("onbuild")
	defer deleteImages(name)
	defer deleteAllContainers()

	createCmd := exec.Command(dockerBinary, "create", "busybox", "true")
	out, _, _, err := runCommandWithStdoutStderr(createCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	commitCmd := exec.Command(dockerBinary, "commit", "--run", "{\"OnBuild\":[\"ONBUILD RUN ls\"]}", cleanedContainerID, "onbuild")

	if _, err := runCommand(commitCmd); err != nil {
		t.Fatal(err)
	}

	_, err = buildImage(name,
		`FROM onbuild`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed") {
			t.Fatalf("Wrong error %v, must be about chaining ONBUILD in source image", err)
		}
	} else {
		t.Fatal("Error must not be nil")
	}
	logDone("build - onbuild forbidden chained in source image")

}

func TestBuildOnBuildCmdEntrypointJSON(t *testing.T) {
	name1 := "onbuildcmd"
	name2 := "onbuildgenerated"

	defer deleteImages(name2)
	defer deleteImages(name1)
	defer deleteAllContainers()

	_, err := buildImage(name1, `
FROM busybox
ONBUILD CMD ["hello world"]
ONBUILD ENTRYPOINT ["echo"]
ONBUILD RUN ["true"]`,
		false)

	if err != nil {
		t.Fatal(err)
	}

	_, err = buildImage(name2, fmt.Sprintf(`FROM %s`, name1), false)

	if err != nil {
		t.Fatal(err)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-t", name2))
	if err != nil {
		t.Fatal(err)
	}

	if !regexp.MustCompile(`(?m)^hello world`).MatchString(out) {
		t.Fatal("did not get echo output from onbuild", out)
	}

	logDone("build - onbuild with json entrypoint/cmd")
}

func TestBuildOnBuildEntrypointJSON(t *testing.T) {
	name1 := "onbuildcmd"
	name2 := "onbuildgenerated"

	defer deleteImages(name2)
	defer deleteImages(name1)
	defer deleteAllContainers()

	_, err := buildImage(name1, `
FROM busybox
ONBUILD ENTRYPOINT ["echo"]`,
		false)

	if err != nil {
		t.Fatal(err)
	}

	_, err = buildImage(name2, fmt.Sprintf("FROM %s\nCMD [\"hello world\"]\n", name1), false)

	if err != nil {
		t.Fatal(err)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-t", name2))
	if err != nil {
		t.Fatal(err)
	}

	if !regexp.MustCompile(`(?m)^hello world`).MatchString(out) {
		t.Fatal("got malformed output from onbuild", out)
	}

	logDone("build - onbuild with json entrypoint")
}

func TestBuildCacheADD(t *testing.T) {
	name := "testbuildtwoimageswithadd"
	defer deleteImages(name)
	server, err := fakeStorage(map[string]string{
		"robots.txt": "hello",
		"index.html": "world",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	if _, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
		ADD %s/robots.txt /`, server.URL()),
		true); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	deleteImages(name)
	_, out, err := buildImageWithOut(name,
		fmt.Sprintf(`FROM scratch
		ADD %s/index.html /`, server.URL()),
		true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Using cache") {
		t.Fatal("2nd build used cache on ADD, it shouldn't")
	}

	logDone("build - build two images with remote ADD")
}

func TestBuildLastModified(t *testing.T) {
	name := "testbuildlastmodified"
	defer deleteImages(name)

	server, err := fakeStorage(map[string]string{
		"file": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	var out, out2 string

	dFmt := `FROM busybox
ADD %s/file /
RUN ls -le /file`

	dockerfile := fmt.Sprintf(dFmt, server.URL())

	if _, out, err = buildImageWithOut(name, dockerfile, false); err != nil {
		t.Fatal(err)
	}

	originMTime := regexp.MustCompile(`root.*/file.*\n`).FindString(out)
	// Make sure our regexp is correct
	if strings.Index(originMTime, "/file") < 0 {
		t.Fatalf("Missing ls info on 'file':\n%s", out)
	}

	// Build it again and make sure the mtime of the file didn't change.
	// Wait a few seconds to make sure the time changed enough to notice
	time.Sleep(2 * time.Second)

	if _, out2, err = buildImageWithOut(name, dockerfile, false); err != nil {
		t.Fatal(err)
	}

	newMTime := regexp.MustCompile(`root.*/file.*\n`).FindString(out2)
	if newMTime != originMTime {
		t.Fatalf("MTime changed:\nOrigin:%s\nNew:%s", originMTime, newMTime)
	}

	// Now 'touch' the file and make sure the timestamp DID change this time
	// Create a new fakeStorage instead of just using Add() to help windows
	server, err = fakeStorage(map[string]string{
		"file": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	dockerfile = fmt.Sprintf(dFmt, server.URL())

	if _, out2, err = buildImageWithOut(name, dockerfile, false); err != nil {
		t.Fatal(err)
	}

	newMTime = regexp.MustCompile(`root.*/file.*\n`).FindString(out2)
	if newMTime == originMTime {
		t.Fatalf("MTime didn't change:\nOrigin:%s\nNew:%s", originMTime, newMTime)
	}

	logDone("build - use Last-Modified header")
}

func TestBuildSixtySteps(t *testing.T) {
	name := "foobuildsixtysteps"
	defer deleteImages(name)
	ctx, err := fakeContext("FROM scratch\n"+strings.Repeat("ADD foo /\n", 60),
		map[string]string{
			"foo": "test1",
		})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - build an image with sixty build steps")
}

func TestBuildAddSingleFileToRoot(t *testing.T) {
	name := "testaddimg"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - add single file to root")
}

// Issue #3960: "ADD src ." hangs
func TestBuildAddSingleFileToWorkdir(t *testing.T) {
	name := "testaddsinglefiletoworkdir"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM busybox
ADD test_file .`,
		map[string]string{
			"test_file": "test1",
		})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	done := make(chan struct{})
	go func() {
		if _, err := buildImageFromContext(name, ctx, true); err != nil {
			t.Fatal(err)
		}
		close(done)
	}()
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("Build with adding to workdir timed out")
	case <-done:
	}
	logDone("build - add single file to workdir")
}

func TestBuildAddSingleFileToExistDir(t *testing.T) {
	name := "testaddsinglefiletoexistdir"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - add single file to existing dir")
}

func TestBuildCopyAddMultipleFiles(t *testing.T) {
	server, err := fakeStorage(map[string]string{
		"robots.txt": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	name := "testcopymultiplefilestofile"
	defer deleteImages(name)
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
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - multiple file copy/add tests")
}

func TestBuildAddMultipleFilesToFile(t *testing.T) {
	name := "testaddmultiplefilestofile"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
	ADD file1.txt file2.txt test
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

	logDone("build - multiple add files to file")
}

func TestBuildJSONAddMultipleFilesToFile(t *testing.T) {
	name := "testjsonaddmultiplefilestofile"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
	ADD ["file1.txt", "file2.txt", "test"]
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

	logDone("build - multiple add files to file json syntax")
}

func TestBuildAddMultipleFilesToFileWild(t *testing.T) {
	name := "testaddmultiplefilestofilewild"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
	ADD file*.txt test
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

	logDone("build - multiple add files to file wild")
}

func TestBuildJSONAddMultipleFilesToFileWild(t *testing.T) {
	name := "testjsonaddmultiplefilestofilewild"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
	ADD ["file*.txt", "test"]
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

	logDone("build - multiple add files to file wild json syntax")
}

func TestBuildCopyMultipleFilesToFile(t *testing.T) {
	name := "testcopymultiplefilestofile"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
	COPY file1.txt file2.txt test
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := "When using COPY with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

	logDone("build - multiple copy files to file")
}

func TestBuildJSONCopyMultipleFilesToFile(t *testing.T) {
	name := "testjsoncopymultiplefilestofile"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
	COPY ["file1.txt", "file2.txt", "test"]
	`,
		map[string]string{
			"file1.txt": "test1",
			"file2.txt": "test1",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := "When using COPY with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

	logDone("build - multiple copy files to file json syntax")
}

func TestBuildAddFileWithWhitespace(t *testing.T) {
	name := "testaddfilewithwhitespace"
	defer deleteImages(name)
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
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - add file with whitespace")
}

func TestBuildCopyFileWithWhitespace(t *testing.T) {
	name := "testcopyfilewithwhitespace"
	defer deleteImages(name)
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
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - copy file with whitespace")
}

func TestBuildAddMultipleFilesToFileWithWhitespace(t *testing.T) {
	name := "testaddmultiplefilestofilewithwhitespace"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM busybox
	ADD [ "test file1", "test file2", "test" ]
    `,
		map[string]string{
			"test file1": "test1",
			"test file2": "test2",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := "When using ADD with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

	logDone("build - multiple add files to file with whitespace")
}

func TestBuildCopyMultipleFilesToFileWithWhitespace(t *testing.T) {
	name := "testcopymultiplefilestofilewithwhitespace"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM busybox
	COPY [ "test file1", "test file2", "test" ]
        `,
		map[string]string{
			"test file1": "test1",
			"test file2": "test2",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := "When using COPY with more than one source file, the destination must be a directory and end with a /"
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Wrong error: (should contain %q) got:\n%v", expected, err)
	}

	logDone("build - multiple copy files to file with whitespace")
}

func TestBuildCopyWildcard(t *testing.T) {
	name := "testcopywildcard"
	defer deleteImages(name)
	server, err := fakeStorage(map[string]string{
		"robots.txt": "hello",
		"index.html": "world",
	})
	if err != nil {
		t.Fatal(err)
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
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	// Now make sure we use a cache the 2nd time
	id2, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	if id1 != id2 {
		t.Fatal("didn't use the cache")
	}

	logDone("build - copy wild card")
}

func TestBuildCopyWildcardNoFind(t *testing.T) {
	name := "testcopywildcardnofind"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM busybox
	COPY file*.txt /tmp/
	`, nil)
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, err = buildImageFromContext(name, ctx, true)
	if err == nil {
		t.Fatal("should have failed to find a file")
	}
	if !strings.Contains(err.Error(), "No source files were specified") {
		t.Fatalf("Wrong error %v, must be about no source files", err)
	}

	logDone("build - copy wild card no find")
}

func TestBuildCopyWildcardCache(t *testing.T) {
	name := "testcopywildcardcache"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM busybox
	COPY file1.txt /tmp/`,
		map[string]string{
			"file1.txt": "test1",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	// Now make sure we use a cache the 2nd time even with wild cards.
	// Use the same context so the file is the same and the checksum will match
	ctx.Add("Dockerfile", `FROM busybox
	COPY file*.txt /tmp/`)

	id2, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	if id1 != id2 {
		t.Fatal("didn't use the cache")
	}

	logDone("build - copy wild card cache")
}

func TestBuildAddSingleFileToNonExistingDir(t *testing.T) {
	name := "testaddsinglefiletononexistingdir"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}

	logDone("build - add single file to non-existing dir")
}

func TestBuildAddDirContentToRoot(t *testing.T) {
	name := "testadddircontenttoroot"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - add directory contents to root")
}

func TestBuildAddDirContentToExistingDir(t *testing.T) {
	name := "testadddircontenttoexistingdir"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - add directory contents to existing dir")
}

func TestBuildAddWholeDirToRoot(t *testing.T) {
	name := "testaddwholedirtoroot"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - add whole directory to root")
}

// Testing #5941
func TestBuildAddEtcToRoot(t *testing.T) {
	name := "testaddetctoroot"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
ADD . /`,
		map[string]string{
			"etc/test_file": "test1",
		})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - add etc directory to root")
}

// Testing #9401
func TestBuildAddPreservesFilesSpecialBits(t *testing.T) {
	name := "testaddpreservesfilesspecialbits"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - add preserves files special bits")
}

func TestBuildCopySingleFileToRoot(t *testing.T) {
	name := "testcopysinglefiletoroot"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - copy single file to root")
}

// Issue #3960: "ADD src ." hangs - adapted for COPY
func TestBuildCopySingleFileToWorkdir(t *testing.T) {
	name := "testcopysinglefiletoworkdir"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM busybox
COPY test_file .`,
		map[string]string{
			"test_file": "test1",
		})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	done := make(chan struct{})
	go func() {
		if _, err := buildImageFromContext(name, ctx, true); err != nil {
			t.Fatal(err)
		}
		close(done)
	}()
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("Build with adding to workdir timed out")
	case <-done:
	}
	logDone("build - copy single file to workdir")
}

func TestBuildCopySingleFileToExistDir(t *testing.T) {
	name := "testcopysinglefiletoexistdir"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - copy single file to existing dir")
}

func TestBuildCopySingleFileToNonExistDir(t *testing.T) {
	name := "testcopysinglefiletononexistdir"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - copy single file to non-existing dir")
}

func TestBuildCopyDirContentToRoot(t *testing.T) {
	name := "testcopydircontenttoroot"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - copy directory contents to root")
}

func TestBuildCopyDirContentToExistDir(t *testing.T) {
	name := "testcopydircontenttoexistdir"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - copy directory contents to existing dir")
}

func TestBuildCopyWholeDirToRoot(t *testing.T) {
	name := "testcopywholedirtoroot"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - copy whole directory to root")
}

func TestBuildCopyEtcToRoot(t *testing.T) {
	name := "testcopyetctoroot"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
COPY . /`,
		map[string]string{
			"etc/test_file": "test1",
		})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - copy etc directory to root")
}

func TestBuildCopyDisallowRemote(t *testing.T) {
	name := "testcopydisallowremote"
	defer deleteImages(name)
	_, out, err := buildImageWithOut(name, `FROM scratch
COPY https://index.docker.io/robots.txt /`,
		true)
	if err == nil || !strings.Contains(out, "Source can't be a URL for COPY") {
		t.Fatalf("Error should be about disallowed remote source, got err: %s, out: %q", err, out)
	}
	logDone("build - copy - disallow copy from remote")
}

func TestBuildAddBadLinks(t *testing.T) {
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
	defer deleteImages(name)
	ctx, err := fakeContext(dockerfile, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	tempDir, err := ioutil.TempDir("", "test-link-absolute-temp-")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", tempDir)
	}
	defer os.RemoveAll(tempDir)

	var symlinkTarget string
	if runtime.GOOS == "windows" {
		var driveLetter string
		if abs, err := filepath.Abs(tempDir); err != nil {
			t.Fatal(err)
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
		t.Fatal(err)
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
		t.Fatal(err)
	}

	tarWriter.Close()
	tarOut.Close()

	foo, err := os.Create(fooPath)
	if err != nil {
		t.Fatal(err)
	}
	defer foo.Close()

	if _, err := foo.WriteString("test"); err != nil {
		t.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(nonExistingFile); err == nil || err != nil && !os.IsNotExist(err) {
		t.Fatalf("%s shouldn't have been written and it shouldn't exist", nonExistingFile)
	}

	logDone("build - ADD must add files in container")
}

func TestBuildAddBadLinksVolume(t *testing.T) {
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
	defer deleteImages(name)

	tempDir, err := ioutil.TempDir("", "test-link-absolute-volume-temp-")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", tempDir)
	}
	defer os.RemoveAll(tempDir)

	dockerfile = fmt.Sprintf(dockerfileTemplate, tempDir)
	nonExistingFile := filepath.Join(tempDir, targetFile)

	ctx, err := fakeContext(dockerfile, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()
	fooPath := filepath.Join(ctx.Dir, targetFile)

	foo, err := os.Create(fooPath)
	if err != nil {
		t.Fatal(err)
	}
	defer foo.Close()

	if _, err := foo.WriteString("test"); err != nil {
		t.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(nonExistingFile); err == nil || err != nil && !os.IsNotExist(err) {
		t.Fatalf("%s shouldn't have been written and it shouldn't exist", nonExistingFile)
	}

	logDone("build - ADD should add files in volume")
}

// Issue #5270 - ensure we throw a better error than "unexpected EOF"
// when we can't access files in the context.
func TestBuildWithInaccessibleFilesInContext(t *testing.T) {
	testRequires(t, UnixCli) // test uses chown/chmod: not available on windows

	{
		name := "testbuildinaccessiblefiles"
		defer deleteImages(name)
		ctx, err := fakeContext("FROM scratch\nADD . /foo/", map[string]string{"fileWithoutReadAccess": "foo"})
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Close()
		// This is used to ensure we detect inaccessible files early during build in the cli client
		pathToFileWithoutReadAccess := filepath.Join(ctx.Dir, "fileWithoutReadAccess")

		if err = os.Chown(pathToFileWithoutReadAccess, 0, 0); err != nil {
			t.Fatalf("failed to chown file to root: %s", err)
		}
		if err = os.Chmod(pathToFileWithoutReadAccess, 0700); err != nil {
			t.Fatalf("failed to chmod file to 700: %s", err)
		}
		buildCmd := exec.Command("su", "unprivilegeduser", "-c", fmt.Sprintf("%s build -t %s .", dockerBinary, name))
		buildCmd.Dir = ctx.Dir
		out, _, err := runCommandWithOutput(buildCmd)
		if err == nil {
			t.Fatalf("build should have failed: %s %s", err, out)
		}

		// check if we've detected the failure before we started building
		if !strings.Contains(out, "no permission to read from ") {
			t.Fatalf("output should've contained the string: no permission to read from but contained: %s", out)
		}

		if !strings.Contains(out, "Error checking context is accessible") {
			t.Fatalf("output should've contained the string: Error checking context is accessible")
		}
	}
	{
		name := "testbuildinaccessibledirectory"
		defer deleteImages(name)
		ctx, err := fakeContext("FROM scratch\nADD . /foo/", map[string]string{"directoryWeCantStat/bar": "foo"})
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Close()
		// This is used to ensure we detect inaccessible directories early during build in the cli client
		pathToDirectoryWithoutReadAccess := filepath.Join(ctx.Dir, "directoryWeCantStat")
		pathToFileInDirectoryWithoutReadAccess := filepath.Join(pathToDirectoryWithoutReadAccess, "bar")

		if err = os.Chown(pathToDirectoryWithoutReadAccess, 0, 0); err != nil {
			t.Fatalf("failed to chown directory to root: %s", err)
		}
		if err = os.Chmod(pathToDirectoryWithoutReadAccess, 0444); err != nil {
			t.Fatalf("failed to chmod directory to 444: %s", err)
		}
		if err = os.Chmod(pathToFileInDirectoryWithoutReadAccess, 0700); err != nil {
			t.Fatalf("failed to chmod file to 700: %s", err)
		}

		buildCmd := exec.Command("su", "unprivilegeduser", "-c", fmt.Sprintf("%s build -t %s .", dockerBinary, name))
		buildCmd.Dir = ctx.Dir
		out, _, err := runCommandWithOutput(buildCmd)
		if err == nil {
			t.Fatalf("build should have failed: %s %s", err, out)
		}

		// check if we've detected the failure before we started building
		if !strings.Contains(out, "can't stat") {
			t.Fatalf("output should've contained the string: can't access %s", out)
		}

		if !strings.Contains(out, "Error checking context is accessible") {
			t.Fatalf("output should've contained the string: Error checking context is accessible")
		}

	}
	{
		name := "testlinksok"
		defer deleteImages(name)
		ctx, err := fakeContext("FROM scratch\nADD . /foo/", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Close()

		target := "../../../../../../../../../../../../../../../../../../../azA"
		if err := os.Symlink(filepath.Join(ctx.Dir, "g"), target); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(target)
		// This is used to ensure we don't follow links when checking if everything in the context is accessible
		// This test doesn't require that we run commands as an unprivileged user
		if _, err := buildImageFromContext(name, ctx, true); err != nil {
			t.Fatal(err)
		}
	}
	{
		name := "testbuildignoredinaccessible"
		defer deleteImages(name)
		ctx, err := fakeContext("FROM scratch\nADD . /foo/",
			map[string]string{
				"directoryWeCantStat/bar": "foo",
				".dockerignore":           "directoryWeCantStat",
			})
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Close()
		// This is used to ensure we don't try to add inaccessible files when they are ignored by a .dockerignore pattern
		pathToDirectoryWithoutReadAccess := filepath.Join(ctx.Dir, "directoryWeCantStat")
		pathToFileInDirectoryWithoutReadAccess := filepath.Join(pathToDirectoryWithoutReadAccess, "bar")
		if err = os.Chown(pathToDirectoryWithoutReadAccess, 0, 0); err != nil {
			t.Fatalf("failed to chown directory to root: %s", err)
		}
		if err = os.Chmod(pathToDirectoryWithoutReadAccess, 0444); err != nil {
			t.Fatalf("failed to chmod directory to 755: %s", err)
		}
		if err = os.Chmod(pathToFileInDirectoryWithoutReadAccess, 0700); err != nil {
			t.Fatalf("failed to chmod file to 444: %s", err)
		}

		buildCmd := exec.Command("su", "unprivilegeduser", "-c", fmt.Sprintf("%s build -t %s .", dockerBinary, name))
		buildCmd.Dir = ctx.Dir
		if out, _, err := runCommandWithOutput(buildCmd); err != nil {
			t.Fatalf("build should have worked: %s %s", err, out)
		}

	}
	logDone("build - ADD from context with inaccessible files must not pass")
	logDone("build - ADD from context with accessible links must work")
	logDone("build - ADD from context with ignored inaccessible files must work")
}

func TestBuildForceRm(t *testing.T) {
	containerCountBefore, err := getContainerCount()
	if err != nil {
		t.Fatalf("failed to get the container count: %s", err)
	}
	name := "testbuildforcerm"
	defer deleteImages(name)
	ctx, err := fakeContext("FROM scratch\nRUN true\nRUN thiswillfail", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	buildCmd := exec.Command(dockerBinary, "build", "-t", name, "--force-rm", ".")
	buildCmd.Dir = ctx.Dir
	if out, _, err := runCommandWithOutput(buildCmd); err == nil {
		t.Fatalf("failed to build the image: %s, %v", out, err)
	}

	containerCountAfter, err := getContainerCount()
	if err != nil {
		t.Fatalf("failed to get the container count: %s", err)
	}

	if containerCountBefore != containerCountAfter {
		t.Fatalf("--force-rm shouldn't have left containers behind")
	}

	logDone("build - ensure --force-rm doesn't leave containers behind")
}

func TestBuildRm(t *testing.T) {
	name := "testbuildrm"
	defer deleteImages(name)
	ctx, err := fakeContext("FROM scratch\nADD foo /\nADD foo /", map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()
	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		out, _, err := dockerCmdInDir(t, ctx.Dir, "build", "--rm", "-t", name, ".")

		if err != nil {
			t.Fatal("failed to build the image", out)
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore != containerCountAfter {
			t.Fatalf("-rm shouldn't have left containers behind")
		}
		deleteImages(name)
	}

	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		out, _, err := dockerCmdInDir(t, ctx.Dir, "build", "-t", name, ".")

		if err != nil {
			t.Fatal("failed to build the image", out)
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore != containerCountAfter {
			t.Fatalf("--rm shouldn't have left containers behind")
		}
		deleteImages(name)
	}

	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		out, _, err := dockerCmdInDir(t, ctx.Dir, "build", "--rm=false", "-t", name, ".")

		if err != nil {
			t.Fatal("failed to build the image", out)
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore == containerCountAfter {
			t.Fatalf("--rm=false should have left containers behind")
		}
		deleteAllContainers()
		deleteImages(name)

	}

	logDone("build - ensure --rm doesn't leave containers behind and that --rm=true is the default")
	logDone("build - ensure --rm=false overrides the default")
}

func TestBuildWithVolumes(t *testing.T) {
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
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "Config.Volumes")
	if err != nil {
		t.Fatal(err)
	}

	err = unmarshalJSON([]byte(res), &result)
	if err != nil {
		t.Fatal(err)
	}

	equal := reflect.DeepEqual(&result, &expected)

	if !equal {
		t.Fatalf("Volumes %s, expected %s", result, expected)
	}

	logDone("build - with volumes")
}

func TestBuildMaintainer(t *testing.T) {
	name := "testbuildmaintainer"
	expected := "dockerio"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM scratch
        MAINTAINER dockerio`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Author")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Maintainer %s, expected %s", res, expected)
	}
	logDone("build - maintainer")
}

func TestBuildUser(t *testing.T) {
	name := "testbuilduser"
	expected := "dockerio"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
		RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
		USER dockerio
		RUN [ $(whoami) = 'dockerio' ]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.User")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("User %s, expected %s", res, expected)
	}
	logDone("build - user")
}

func TestBuildRelativeWorkdir(t *testing.T) {
	name := "testbuildrelativeworkdir"
	expected := "/test2/test3"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
		RUN [ "$PWD" = '/' ]
		WORKDIR test1
		RUN [ "$PWD" = '/test1' ]
		WORKDIR /test2
		RUN [ "$PWD" = '/test2' ]
		WORKDIR test3
		RUN [ "$PWD" = '/test2/test3' ]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.WorkingDir")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Workdir %s, expected %s", res, expected)
	}
	logDone("build - relative workdir")
}

func TestBuildWorkdirWithEnvVariables(t *testing.T) {
	name := "testbuildworkdirwithenvvariables"
	expected := "/test1/test2/$MISSING_VAR"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
		ENV DIRPATH /test1
		ENV SUBDIRNAME test2
		WORKDIR $DIRPATH
		WORKDIR $SUBDIRNAME/$MISSING_VAR`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.WorkingDir")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Workdir %s, expected %s", res, expected)
	}
	logDone("build - workdir with env variables")
}

func TestBuildRelativeCopy(t *testing.T) {
	name := "testbuildrelativecopy"
	defer deleteImages(name)
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
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = buildImageFromContext(name, ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	logDone("build - relative copy/add")
}

func TestBuildEnv(t *testing.T) {
	name := "testbuildenv"
	expected := "[PATH=/test:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin PORT=2375]"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
		ENV PATH /test:$PATH
        ENV PORT 2375
		RUN [ $(env | grep PORT) = 'PORT=2375' ]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.Env")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Env %s, expected %s", res, expected)
	}
	logDone("build - env")
}

func TestBuildContextCleanup(t *testing.T) {
	testRequires(t, SameHostDaemon)

	name := "testbuildcontextcleanup"
	defer deleteImages(name)
	entries, err := ioutil.ReadDir("/var/lib/docker/tmp")
	if err != nil {
		t.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	_, err = buildImage(name,
		`FROM scratch
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	entriesFinal, err := ioutil.ReadDir("/var/lib/docker/tmp")
	if err != nil {
		t.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	if err = compareDirectoryEntries(entries, entriesFinal); err != nil {
		t.Fatalf("context should have been deleted, but wasn't")
	}

	logDone("build - verify context cleanup works properly")
}

func TestBuildContextCleanupFailedBuild(t *testing.T) {
	testRequires(t, SameHostDaemon)

	name := "testbuildcontextcleanup"
	defer deleteImages(name)
	defer deleteAllContainers()
	entries, err := ioutil.ReadDir("/var/lib/docker/tmp")
	if err != nil {
		t.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	_, err = buildImage(name,
		`FROM scratch
	RUN /non/existing/command`,
		true)
	if err == nil {
		t.Fatalf("expected build to fail, but it didn't")
	}
	entriesFinal, err := ioutil.ReadDir("/var/lib/docker/tmp")
	if err != nil {
		t.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	if err = compareDirectoryEntries(entries, entriesFinal); err != nil {
		t.Fatalf("context should have been deleted, but wasn't")
	}

	logDone("build - verify context cleanup works properly after an unsuccessful build")
}

func TestBuildCmd(t *testing.T) {
	name := "testbuildcmd"
	expected := "[/bin/echo Hello World]"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM scratch
        CMD ["/bin/echo", "Hello World"]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.Cmd")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Cmd %s, expected %s", res, expected)
	}
	logDone("build - cmd")
}

func TestBuildExpose(t *testing.T) {
	name := "testbuildexpose"
	expected := "map[2375/tcp:map[]]"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM scratch
        EXPOSE 2375`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.ExposedPorts")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Exposed ports %s, expected %s", res, expected)
	}
	logDone("build - expose")
}

func TestBuildExposeMorePorts(t *testing.T) {
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
	defer deleteImages(name)
	_, err := buildImage(name, buf.String(), true)
	if err != nil {
		t.Fatal(err)
	}

	// check if all the ports are saved inside Config.ExposedPorts
	res, err := inspectFieldJSON(name, "Config.ExposedPorts")
	if err != nil {
		t.Fatal(err)
	}
	var exposedPorts map[string]interface{}
	if err := json.Unmarshal([]byte(res), &exposedPorts); err != nil {
		t.Fatal(err)
	}

	for _, p := range expectedPorts {
		ep := fmt.Sprintf("%d/tcp", p)
		if _, ok := exposedPorts[ep]; !ok {
			t.Errorf("Port(%s) is not exposed", ep)
		} else {
			delete(exposedPorts, ep)
		}
	}
	if len(exposedPorts) != 0 {
		t.Errorf("Unexpected extra exposed ports %v", exposedPorts)
	}
	logDone("build - expose large number of ports")
}

func TestBuildExposeOrder(t *testing.T) {
	buildID := func(name, exposed string) string {
		_, err := buildImage(name, fmt.Sprintf(`FROM scratch
		EXPOSE %s`, exposed), true)
		if err != nil {
			t.Fatal(err)
		}
		id, err := inspectField(name, "Id")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	id1 := buildID("testbuildexpose1", "80 2375")
	id2 := buildID("testbuildexpose2", "2375 80")
	defer deleteImages("testbuildexpose1", "testbuildexpose2")
	if id1 != id2 {
		t.Errorf("EXPOSE should invalidate the cache only when ports actually changed")
	}
	logDone("build - expose order")
}

func TestBuildExposeUpperCaseProto(t *testing.T) {
	name := "testbuildexposeuppercaseproto"
	expected := "map[5678/udp:map[]]"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM scratch
        EXPOSE 5678/UDP`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.ExposedPorts")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Exposed ports %s, expected %s", res, expected)
	}
	logDone("build - expose port with upper case proto")
}

func TestBuildExposeHostPort(t *testing.T) {
	// start building docker file with ip:hostPort:containerPort
	name := "testbuildexpose"
	expected := "map[5678/tcp:map[]]"
	defer deleteImages(name)
	_, out, err := buildImageWithOut(name,
		`FROM scratch
        EXPOSE 192.168.1.2:2375:5678`,
		true)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "to map host ports to container ports (ip:hostPort:containerPort) is deprecated.") {
		t.Fatal("Missing warning message")
	}

	res, err := inspectField(name, "Config.ExposedPorts")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Exposed ports %s, expected %s", res, expected)
	}
	logDone("build - ignore exposing host's port")
}

func TestBuildEmptyEntrypointInheritance(t *testing.T) {
	name := "testbuildentrypointinheritance"
	name2 := "testbuildentrypointinheritance2"
	defer deleteImages(name, name2)

	_, err := buildImage(name,
		`FROM busybox
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.Entrypoint")
	if err != nil {
		t.Fatal(err)
	}

	expected := "[/bin/echo]"
	if res != expected {
		t.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

	_, err = buildImage(name2,
		fmt.Sprintf(`FROM %s
        ENTRYPOINT []`, name),
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err = inspectField(name2, "Config.Entrypoint")
	if err != nil {
		t.Fatal(err)
	}

	expected = "[]"

	if res != expected {
		t.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

	logDone("build - empty entrypoint inheritance")
}

func TestBuildEmptyEntrypoint(t *testing.T) {
	name := "testbuildentrypoint"
	defer deleteImages(name)
	expected := "[]"

	_, err := buildImage(name,
		`FROM busybox
        ENTRYPOINT []`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.Entrypoint")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

	logDone("build - empty entrypoint")
}

func TestBuildEntrypoint(t *testing.T) {
	name := "testbuildentrypoint"
	expected := "[/bin/echo]"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM scratch
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.Entrypoint")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Entrypoint %s, expected %s", res, expected)
	}

	logDone("build - entrypoint")
}

// #6445 ensure ONBUILD triggers aren't committed to grandchildren
func TestBuildOnBuildLimitedInheritence(t *testing.T) {
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
			t.Fatal(err)
		}
		defer ctx.Close()

		out1, _, err := dockerCmdInDir(t, ctx.Dir, "build", "-t", name1, ".")
		if err != nil {
			t.Fatalf("build failed to complete: %s, %v", out1, err)
		}
		defer deleteImages(name1)
	}
	{
		name2 := "testonbuildtrigger2"
		dockerfile2 := `
		FROM testonbuildtrigger1
		`
		ctx, err := fakeContext(dockerfile2, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Close()

		out2, _, err = dockerCmdInDir(t, ctx.Dir, "build", "-t", name2, ".")
		if err != nil {
			t.Fatalf("build failed to complete: %s, %v", out2, err)
		}
		defer deleteImages(name2)
	}
	{
		name3 := "testonbuildtrigger3"
		dockerfile3 := `
		FROM testonbuildtrigger2
		`
		ctx, err := fakeContext(dockerfile3, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Close()

		out3, _, err = dockerCmdInDir(t, ctx.Dir, "build", "-t", name3, ".")
		if err != nil {
			t.Fatalf("build failed to complete: %s, %v", out3, err)
		}

		defer deleteImages(name3)
	}

	// ONBUILD should be run in second build.
	if !strings.Contains(out2, "ONBUILD PARENT") {
		t.Fatalf("ONBUILD instruction did not run in child of ONBUILD parent")
	}

	// ONBUILD should *not* be run in third build.
	if strings.Contains(out3, "ONBUILD PARENT") {
		t.Fatalf("ONBUILD instruction ran in grandchild of ONBUILD parent")
	}

	logDone("build - onbuild")
}

func TestBuildWithCache(t *testing.T) {
	name := "testbuildwithcache"
	defer deleteImages(name)
	id1, err := buildImage(name,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImage(name,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("The cache should have been used but hasn't.")
	}
	logDone("build - with cache")
}

func TestBuildWithoutCache(t *testing.T) {
	name := "testbuildwithoutcache"
	name2 := "testbuildwithoutcache2"
	defer deleteImages(name, name2)
	id1, err := buildImage(name,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		t.Fatal(err)
	}

	id2, err := buildImage(name2,
		`FROM scratch
		MAINTAINER dockerio
		EXPOSE 5432
        ENTRYPOINT ["/bin/echo"]`,
		false)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	logDone("build - without cache")
}

func TestBuildConditionalCache(t *testing.T) {
	name := "testbuildconditionalcache"
	name2 := "testbuildconditionalcache2"
	defer deleteImages(name, name2)

	dockerfile := `
		FROM busybox
        ADD foo /tmp/`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatalf("Error building #1: %s", err)
	}

	if err := ctx.Add("foo", "bye"); err != nil {
		t.Fatalf("Error modifying foo: %s", err)
	}

	id2, err := buildImageFromContext(name, ctx, false)
	if err != nil {
		t.Fatalf("Error building #2: %s", err)
	}
	if id2 == id1 {
		t.Fatal("Should not have used the cache")
	}

	id3, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatalf("Error building #3: %s", err)
	}
	if id3 != id2 {
		t.Fatal("Should have used the cache")
	}

	logDone("build - conditional cache")
}

func TestBuildADDLocalFileWithCache(t *testing.T) {
	name := "testbuildaddlocalfilewithcache"
	name2 := "testbuildaddlocalfilewithcache2"
	defer deleteImages(name, name2)
	dockerfile := `
		FROM busybox
        MAINTAINER dockerio
        ADD foo /usr/lib/bla/bar
		RUN [ "$(cat /usr/lib/bla/bar)" = "hello" ]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("The cache should have been used but hasn't.")
	}
	logDone("build - add local file with cache")
}

func TestBuildADDMultipleLocalFileWithCache(t *testing.T) {
	name := "testbuildaddmultiplelocalfilewithcache"
	name2 := "testbuildaddmultiplelocalfilewithcache2"
	defer deleteImages(name, name2)
	dockerfile := `
		FROM busybox
        MAINTAINER dockerio
        ADD foo Dockerfile /usr/lib/bla/
		RUN [ "$(cat /usr/lib/bla/foo)" = "hello" ]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("The cache should have been used but hasn't.")
	}
	logDone("build - add multiple local files with cache")
}

func TestBuildADDLocalFileWithoutCache(t *testing.T) {
	name := "testbuildaddlocalfilewithoutcache"
	name2 := "testbuildaddlocalfilewithoutcache2"
	defer deleteImages(name, name2)
	dockerfile := `
		FROM busybox
        MAINTAINER dockerio
        ADD foo /usr/lib/bla/bar
		RUN [ "$(cat /usr/lib/bla/bar)" = "hello" ]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	logDone("build - add local file without cache")
}

func TestBuildCopyDirButNotFile(t *testing.T) {
	name := "testbuildcopydirbutnotfile"
	name2 := "testbuildcopydirbutnotfile2"
	defer deleteImages(name, name2)
	dockerfile := `
        FROM scratch
        COPY dir /tmp/`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"dir/foo": "hello",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	// Check that adding file with similar name doesn't mess with cache
	if err := ctx.Add("dir_file", "hello2"); err != nil {
		t.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("The cache should have been used but wasn't")
	}
	logDone("build - add current directory but not file")
}

func TestBuildADDCurrentDirWithCache(t *testing.T) {
	name := "testbuildaddcurrentdirwithcache"
	name2 := name + "2"
	name3 := name + "3"
	name4 := name + "4"
	name5 := name + "5"
	defer deleteImages(name, name2, name3, name4, name5)
	dockerfile := `
        FROM scratch
        MAINTAINER dockerio
        ADD . /usr/lib/bla`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	// Check that adding file invalidate cache of "ADD ."
	if err := ctx.Add("bar", "hello2"); err != nil {
		t.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	// Check that changing file invalidate cache of "ADD ."
	if err := ctx.Add("foo", "hello1"); err != nil {
		t.Fatal(err)
	}
	id3, err := buildImageFromContext(name3, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id2 == id3 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	// Check that changing file to same content invalidate cache of "ADD ."
	time.Sleep(1 * time.Second) // wait second because of mtime precision
	if err := ctx.Add("foo", "hello1"); err != nil {
		t.Fatal(err)
	}
	id4, err := buildImageFromContext(name4, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id3 == id4 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	id5, err := buildImageFromContext(name5, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id4 != id5 {
		t.Fatal("The cache should have been used but hasn't.")
	}
	logDone("build - add current directory with cache")
}

func TestBuildADDCurrentDirWithoutCache(t *testing.T) {
	name := "testbuildaddcurrentdirwithoutcache"
	name2 := "testbuildaddcurrentdirwithoutcache2"
	defer deleteImages(name, name2)
	dockerfile := `
        FROM scratch
        MAINTAINER dockerio
        ADD . /usr/lib/bla`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"foo": "hello",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	logDone("build - add current directory without cache")
}

func TestBuildADDRemoteFileWithCache(t *testing.T) {
	name := "testbuildaddremotefilewithcache"
	defer deleteImages(name)
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	id1, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("The cache should have been used but hasn't.")
	}
	logDone("build - add remote file with cache")
}

func TestBuildADDRemoteFileWithoutCache(t *testing.T) {
	name := "testbuildaddremotefilewithoutcache"
	name2 := "testbuildaddremotefilewithoutcache2"
	defer deleteImages(name, name2)
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	id1, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImage(name2,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()),
		false)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	logDone("build - add remote file without cache")
}

func TestBuildADDRemoteFileMTime(t *testing.T) {
	name := "testbuildaddremotefilemtime"
	name2 := name + "2"
	name3 := name + "3"
	name4 := name + "4"

	defer deleteImages(name, name2, name3, name4)

	files := map[string]string{"baz": "hello"}
	server, err := fakeStorage(files)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL()), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	id2, err := buildImageFromContext(name2, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("The cache should have been used but wasn't - #1")
	}

	// Now create a different server withsame contents (causes different mtim)
	// This time the cache should not be used

	// allow some time for clock to pass as mtime precision is only 1s
	time.Sleep(2 * time.Second)

	server2, err := fakeStorage(files)
	if err != nil {
		t.Fatal(err)
	}
	defer server2.Close()

	ctx2, err := fakeContext(fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server2.URL()), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx2.Close()
	id3, err := buildImageFromContext(name3, ctx2, true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id3 {
		t.Fatal("The cache should not have been used but was")
	}

	// And for good measure do it again and make sure cache is used this time
	id4, err := buildImageFromContext(name4, ctx2, true)
	if err != nil {
		t.Fatal(err)
	}
	if id3 != id4 {
		t.Fatal("The cache should have been used but wasn't - #2")
	}
	logDone("build - add remote file testing mtime")
}

func TestBuildADDLocalAndRemoteFilesWithCache(t *testing.T) {
	name := "testbuildaddlocalandremotefilewithcache"
	defer deleteImages(name)
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("The cache should have been used but hasn't.")
	}
	logDone("build - add local and remote file with cache")
}

func testContextTar(t *testing.T, compression archive.Compression) {
	ctx, err := fakeContext(
		`FROM busybox
ADD foo /foo
CMD ["cat", "/foo"]`,
		map[string]string{
			"foo": "bar",
		},
	)
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	context, err := archive.Tar(ctx.Dir, compression)
	if err != nil {
		t.Fatalf("failed to build context tar: %v", err)
	}
	name := "contexttar"
	buildCmd := exec.Command(dockerBinary, "build", "-t", name, "-")
	defer deleteImages(name)
	buildCmd.Stdin = context

	if out, _, err := runCommandWithOutput(buildCmd); err != nil {
		t.Fatalf("build failed to complete: %v %v", out, err)
	}
	logDone(fmt.Sprintf("build - build an image with a context tar, compression: %v", compression))
}

func TestBuildContextTarGzip(t *testing.T) {
	testContextTar(t, archive.Gzip)
}

func TestBuildContextTarNoCompression(t *testing.T) {
	testContextTar(t, archive.Uncompressed)
}

func TestBuildNoContext(t *testing.T) {
	buildCmd := exec.Command(dockerBinary, "build", "-t", "nocontext", "-")
	buildCmd.Stdin = strings.NewReader("FROM busybox\nCMD echo ok\n")

	if out, _, err := runCommandWithOutput(buildCmd); err != nil {
		t.Fatalf("build failed to complete: %v %v", out, err)
	}

	if out, _, err := dockerCmd(t, "run", "--rm", "nocontext"); out != "ok\n" || err != nil {
		t.Fatalf("run produced invalid output: %q, expected %q", out, "ok")
	}

	deleteImages("nocontext")
	logDone("build - build an image with no context")
}

// TODO: TestCaching
func TestBuildADDLocalAndRemoteFilesWithoutCache(t *testing.T) {
	name := "testbuildaddlocalandremotefilewithoutcache"
	name2 := "testbuildaddlocalandremotefilewithoutcache2"
	defer deleteImages(name, name2)
	server, err := fakeStorage(map[string]string{
		"baz": "hello",
	})
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}
	defer ctx.Close()
	id1, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImageFromContext(name2, ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	logDone("build - add local and remote file without cache")
}

func TestBuildWithVolumeOwnership(t *testing.T) {
	name := "testbuildimg"
	defer deleteImages(name)

	_, err := buildImage(name,
		`FROM busybox:latest
        RUN mkdir /test && chown daemon:daemon /test && chmod 0600 /test
        VOLUME /test`,
		true)

	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "--rm", "testbuildimg", "ls", "-la", "/test")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if expected := "drw-------"; !strings.Contains(out, expected) {
		t.Fatalf("expected %s received %s", expected, out)
	}

	if expected := "daemon   daemon"; !strings.Contains(out, expected) {
		t.Fatalf("expected %s received %s", expected, out)
	}

	logDone("build - volume ownership")
}

// testing #1405 - config.Cmd does not get cleaned up if
// utilizing cache
func TestBuildEntrypointRunCleanup(t *testing.T) {
	name := "testbuildcmdcleanup"
	defer deleteImages(name)
	if _, err := buildImage(name,
		`FROM busybox
        RUN echo "hello"`,
		true); err != nil {
		t.Fatal(err)
	}

	ctx, err := fakeContext(`FROM busybox
        RUN echo "hello"
        ADD foo /foo
        ENTRYPOINT ["/bin/echo"]`,
		map[string]string{
			"foo": "hello",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.Cmd")
	if err != nil {
		t.Fatal(err)
	}
	// Cmd must be cleaned up
	if expected := "<no value>"; res != expected {
		t.Fatalf("Cmd %s, expected %s", res, expected)
	}
	logDone("build - cleanup cmd after RUN")
}

func TestBuildForbiddenContextPath(t *testing.T) {
	name := "testbuildforbidpath"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
        ADD ../../ test/
        `,
		map[string]string{
			"test.txt":  "test1",
			"other.txt": "other",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected := "Forbidden path outside the build context: ../../ "
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Wrong error: (should contain \"%s\") got:\n%v", expected, err)
	}

	logDone("build - forbidden context path")
}

func TestBuildADDFileNotFound(t *testing.T) {
	name := "testbuildaddnotfound"
	defer deleteImages(name)
	ctx, err := fakeContext(`FROM scratch
        ADD foo /usr/local/bar`,
		map[string]string{"bar": "hello"})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		if !strings.Contains(err.Error(), "foo: no such file or directory") {
			t.Fatalf("Wrong error %v, must be about missing foo file or directory", err)
		}
	} else {
		t.Fatal("Error must not be nil")
	}
	logDone("build - add file not found")
}

func TestBuildInheritance(t *testing.T) {
	name := "testbuildinheritance"
	defer deleteImages(name)

	_, err := buildImage(name,
		`FROM scratch
		EXPOSE 2375`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	ports1, err := inspectField(name, "Config.ExposedPorts")
	if err != nil {
		t.Fatal(err)
	}

	_, err = buildImage(name,
		fmt.Sprintf(`FROM %s
		ENTRYPOINT ["/bin/echo"]`, name),
		true)
	if err != nil {
		t.Fatal(err)
	}

	res, err := inspectField(name, "Config.Entrypoint")
	if err != nil {
		t.Fatal(err)
	}
	if expected := "[/bin/echo]"; res != expected {
		t.Fatalf("Entrypoint %s, expected %s", res, expected)
	}
	ports2, err := inspectField(name, "Config.ExposedPorts")
	if err != nil {
		t.Fatal(err)
	}
	if ports1 != ports2 {
		t.Fatalf("Ports must be same: %s != %s", ports1, ports2)
	}
	logDone("build - inheritance")
}

func TestBuildFails(t *testing.T) {
	name := "testbuildfails"
	defer deleteImages(name)
	defer deleteAllContainers()
	_, err := buildImage(name,
		`FROM busybox
		RUN sh -c "exit 23"`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "returned a non-zero code: 23") {
			t.Fatalf("Wrong error %v, must be about non-zero code 23", err)
		}
	} else {
		t.Fatal("Error must not be nil")
	}
	logDone("build - unsuccessful")
}

func TestBuildFailsDockerfileEmpty(t *testing.T) {
	name := "testbuildfails"
	defer deleteImages(name)
	_, err := buildImage(name, ``, true)
	if err != nil {
		if !strings.Contains(err.Error(), "Dockerfile cannot be empty") {
			t.Fatalf("Wrong error %v, must be about empty Dockerfile", err)
		}
	} else {
		t.Fatal("Error must not be nil")
	}
	logDone("build - unsuccessful with empty dockerfile")
}

func TestBuildOnBuild(t *testing.T) {
	name := "testbuildonbuild"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
		ONBUILD RUN touch foobar`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = buildImage(name,
		fmt.Sprintf(`FROM %s
		RUN [ -f foobar ]`, name),
		true)
	if err != nil {
		t.Fatal(err)
	}
	logDone("build - onbuild")
}

func TestBuildOnBuildForbiddenChained(t *testing.T) {
	name := "testbuildonbuildforbiddenchained"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
		ONBUILD ONBUILD RUN touch foobar`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed") {
			t.Fatalf("Wrong error %v, must be about chaining ONBUILD", err)
		}
	} else {
		t.Fatal("Error must not be nil")
	}
	logDone("build - onbuild forbidden chained")
}

func TestBuildOnBuildForbiddenFrom(t *testing.T) {
	name := "testbuildonbuildforbiddenfrom"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
		ONBUILD FROM scratch`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "FROM isn't allowed as an ONBUILD trigger") {
			t.Fatalf("Wrong error %v, must be about FROM forbidden", err)
		}
	} else {
		t.Fatal("Error must not be nil")
	}
	logDone("build - onbuild forbidden from")
}

func TestBuildOnBuildForbiddenMaintainer(t *testing.T) {
	name := "testbuildonbuildforbiddenmaintainer"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
		ONBUILD MAINTAINER docker.io`,
		true)
	if err != nil {
		if !strings.Contains(err.Error(), "MAINTAINER isn't allowed as an ONBUILD trigger") {
			t.Fatalf("Wrong error %v, must be about MAINTAINER forbidden", err)
		}
	} else {
		t.Fatal("Error must not be nil")
	}
	logDone("build - onbuild forbidden maintainer")
}

// gh #2446
func TestBuildAddToSymlinkDest(t *testing.T) {
	name := "testbuildaddtosymlinkdest"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - add to symlink destination")
}

func TestBuildEscapeWhitespace(t *testing.T) {
	name := "testbuildescaping"
	defer deleteImages(name)

	_, err := buildImage(name, `
  FROM busybox
  MAINTAINER "Docker \
IO <io@\
docker.com>"
  `, true)

	res, err := inspectField(name, "Author")

	if err != nil {
		t.Fatal(err)
	}

	if res != "\"Docker IO <io@docker.com>\"" {
		t.Fatalf("Parsed string did not match the escaped string. Got: %q", res)
	}

	logDone("build - validate escaping whitespace")
}

func TestBuildVerifyIntString(t *testing.T) {
	// Verify that strings that look like ints are still passed as strings
	name := "testbuildstringing"
	defer deleteImages(name)

	_, err := buildImage(name, `
  FROM busybox
  MAINTAINER 123
  `, true)

	out, rc, err := runCommandWithOutput(exec.Command(dockerBinary, "inspect", name))
	if rc != 0 || err != nil {
		t.Fatalf("Unexcepted error from inspect: rc: %v  err: %v", rc, err)
	}

	if !strings.Contains(out, "\"123\"") {
		t.Fatalf("Output does not contain the int as a string:\n%s", out)
	}

	logDone("build - verify int/strings as strings")
}

func TestBuildDockerignore(t *testing.T) {
	name := "testbuilddockerignore"
	defer deleteImages(name)
	dockerfile := `
        FROM busybox
        ADD . /bla
		RUN [[ -f /bla/src/x.go ]]
		RUN [[ -f /bla/Makefile ]]
		RUN [[ ! -e /bla/src/_vendor ]]
		RUN [[ ! -e /bla/.gitignore ]]
		RUN [[ ! -e /bla/README.md ]]
		RUN [[ ! -e /bla/.git ]]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Makefile":         "all:",
		".git/HEAD":        "ref: foo",
		"src/x.go":         "package main",
		"src/_vendor/v.go": "package main",
		".gitignore":       "",
		"README.md":        "readme",
		".dockerignore":    ".git\npkg\n.gitignore\nsrc/_vendor\n*.md",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - test .dockerignore")
}

func TestBuildDockerignoreCleanPaths(t *testing.T) {
	name := "testbuilddockerignorecleanpaths"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()
	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - test .dockerignore with clean paths")
}

func TestBuildDockerignoringDockerfile(t *testing.T) {
	name := "testbuilddockerignoredockerfile"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("Didn't ignore Dockerfile correctly:%s", err)
	}

	// now try it with ./Dockerfile
	ctx.Add(".dockerignore", "./Dockerfile\n")
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("Didn't ignore ./Dockerfile correctly:%s", err)
	}

	logDone("build - test .dockerignore of Dockerfile")
}

func TestBuildDockerignoringRenamedDockerfile(t *testing.T) {
	name := "testbuilddockerignoredockerfile"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("Didn't ignore MyDockerfile correctly:%s", err)
	}

	// now try it with ./MyDockerfile
	ctx.Add(".dockerignore", "./MyDockerfile\n")
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("Didn't ignore ./MyDockerfile correctly:%s", err)
	}

	logDone("build - test .dockerignore of renamed Dockerfile")
}

func TestBuildDockerignoringDockerignore(t *testing.T) {
	name := "testbuilddockerignoredockerignore"
	defer deleteImages(name)
	dockerfile := `
        FROM busybox
		ADD . /tmp/
		RUN ! ls /tmp/.dockerignore
		RUN ls /tmp/Dockerfile`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    dockerfile,
		".dockerignore": ".dockerignore\n",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("Didn't ignore .dockerignore correctly:%s", err)
	}
	logDone("build - test .dockerignore of .dockerignore")
}

func TestBuildDockerignoreTouchDockerfile(t *testing.T) {
	var id1 string
	var id2 string

	name := "testbuilddockerignoretouchdockerfile"
	defer deleteImages(name)
	dockerfile := `
        FROM busybox
		ADD . /tmp/`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    dockerfile,
		".dockerignore": "Dockerfile\n",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	if id1, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("Didn't build it correctly:%s", err)
	}

	if id2, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("Didn't build it correctly:%s", err)
	}
	if id1 != id2 {
		t.Fatalf("Didn't use the cache - 1")
	}

	// Now make sure touching Dockerfile doesn't invalidate the cache
	if err = ctx.Add("Dockerfile", dockerfile+"\n# hi"); err != nil {
		t.Fatalf("Didn't add Dockerfile: %s", err)
	}
	if id2, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("Didn't build it correctly:%s", err)
	}
	if id1 != id2 {
		t.Fatalf("Didn't use the cache - 2")
	}

	// One more time but just 'touch' it instead of changing the content
	if err = ctx.Add("Dockerfile", dockerfile+"\n# hi"); err != nil {
		t.Fatalf("Didn't add Dockerfile: %s", err)
	}
	if id2, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("Didn't build it correctly:%s", err)
	}
	if id1 != id2 {
		t.Fatalf("Didn't use the cache - 3")
	}

	logDone("build - test .dockerignore touch dockerfile")
}

func TestBuildDockerignoringWholeDir(t *testing.T) {
	name := "testbuilddockerignorewholedir"
	defer deleteImages(name)
	dockerfile := `
        FROM busybox
		COPY . /
		RUN [[ ! -e /.gitignore ]]
		RUN [[ -f /Makefile ]]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    "FROM scratch",
		"Makefile":      "all:",
		".dockerignore": ".*\n",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}
	logDone("build - test .dockerignore whole dir with .*")
}

func TestBuildLineBreak(t *testing.T) {
	name := "testbuildlinebreak"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM  busybox
RUN    sh -c 'echo root:testpass \
	> /tmp/passwd'
RUN    mkdir -p /var/run/sshd
RUN    [ "$(cat /tmp/passwd)" = "root:testpass" ]
RUN    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	logDone("build - line break with \\")
}

func TestBuildEOLInLine(t *testing.T) {
	name := "testbuildeolinline"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM   busybox
RUN    sh -c 'echo root:testpass > /tmp/passwd'
RUN    echo "foo \n bar"; echo "baz"
RUN    mkdir -p /var/run/sshd
RUN    [ "$(cat /tmp/passwd)" = "root:testpass" ]
RUN    [ "$(ls -d /var/run/sshd)" = "/var/run/sshd" ]`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	logDone("build - end of line in dockerfile instruction")
}

func TestBuildCommentsShebangs(t *testing.T) {
	name := "testbuildcomments"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	logDone("build - comments and shebangs")
}

func TestBuildUsersAndGroups(t *testing.T) {
	name := "testbuildusers"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox

# Make sure our defaults work
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)" = '0:0/root:root' ]

# TODO decide if "args.user = strconv.Itoa(syscall.Getuid())" is acceptable behavior for changeUser in sysvinit instead of "return nil" when "USER" isn't specified (so that we get the proper group list even if that is the empty list, even in the default case of not supplying an explicit USER to run as, which implies USER 0)
USER root
RUN [ "$(id -G):$(id -Gn)" = '0 10:root wheel' ]

# Setup dockerio user and group
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group

# Make sure we can switch to our user and all the information is exactly as we expect it to be
USER dockerio
RUN id -G
RUN id -Gn
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '1001:1001/dockerio:dockerio/1001:dockerio' ]

# Switch back to root and double check that worked exactly as we might expect it to
USER root
RUN [ "$(id -u):$(id -g)/$(id -un):$(id -gn)/$(id -G):$(id -Gn)" = '0:0/root:root/0 10:root wheel' ]

# Add a "supplementary" group for our dockerio user
RUN echo 'supplementary:x:1002:dockerio' >> /etc/group

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
		t.Fatal(err)
	}
	logDone("build - users and groups")
}

func TestBuildEnvUsage(t *testing.T) {
	name := "testbuildenvusage"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	_, err = buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	logDone("build - environment variables usage")
}

func TestBuildEnvUsage2(t *testing.T) {
	name := "testbuildenvusage2"
	defer deleteImages(name)
	dockerfile := `FROM busybox
ENV    abc=def
RUN    [ "$abc" = "def" ]
ENV    def="hello world"
RUN    [ "$def" = "hello world" ]
ENV    def=hello\ world
RUN    [ "$def" = "hello world" ]
ENV    v1=abc v2="hi there"
RUN    [ "$v1" = "abc" ]
RUN    [ "$v2" = "hi there" ]
ENV    v3='boogie nights' v4="with'quotes too"
RUN    [ "$v3" = "boogie nights" ]
RUN    [ "$v4" = "with'quotes too" ]
ENV    abc=zzz FROM=hello/docker/world
ENV    abc=zzz TO=/docker/world/hello
ADD    $FROM $TO
RUN    [ "$(cat $TO)" = "hello" ]
ENV    abc "zzz"
RUN    [ $abc = \"zzz\" ]
ENV    abc 'yyy'
RUN    [ $abc = \'yyy\' ]
ENV    abc=
RUN    [ "$abc" = "" ]

# use grep to make sure if the builder substitutes \$foo by mistake
# we don't get a false positive
ENV    abc=\$foo
RUN    [ "$abc" = "\$foo" ] && (echo "$abc" | grep foo)
ENV    abc \$foo
RUN    [ "$abc" = "\$foo" ] && (echo "$abc" | grep foo)

ENV    abc=\'foo\'
RUN    [ "$abc" = "'foo'" ]
ENV    abc=\"foo\"
RUN    [ "$abc" = "\"foo\"" ]
ENV    abc "foo"
RUN    [ "$abc" = "\"foo\"" ]
ENV    abc 'foo'
RUN    [ "$abc" = "'foo'" ]
ENV    abc \'foo\'
RUN    [ "$abc" = "\\'foo\\'" ]
ENV    abc \"foo\"
RUN    [ "$abc" = "\\\"foo\\\"" ]
`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"hello/docker/world": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	_, err = buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	logDone("build - environment variables usage2")
}

func TestBuildAddScript(t *testing.T) {
	name := "testbuildaddscript"
	defer deleteImages(name)
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
		t.Fatal(err)
	}
	defer ctx.Close()

	_, err = buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	logDone("build - add and run script")
}

func TestBuildAddTar(t *testing.T) {
	name := "testbuildaddtar"
	defer deleteImages(name)

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
		testTar, err := os.Create(filepath.Join(tmpDir, "test.tar"))
		if err != nil {
			t.Fatalf("failed to create test.tar archive: %v", err)
		}
		defer testTar.Close()

		tw := tar.NewWriter(testTar)

		if err := tw.WriteHeader(&tar.Header{
			Name: "test/foo",
			Size: 2,
		}); err != nil {
			t.Fatalf("failed to write tar file header: %v", err)
		}
		if _, err := tw.Write([]byte("Hi")); err != nil {
			t.Fatalf("failed to write tar file content: %v", err)
		}
		if err := tw.Close(); err != nil {
			t.Fatalf("failed to close tar archive: %v", err)
		}

		if err := ioutil.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
			t.Fatalf("failed to open destination dockerfile: %v", err)
		}
		return fakeContextFromDir(tmpDir)
	}()
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("build failed to complete for TestBuildAddTar: %v", err)
	}

	logDone("build - ADD tar")
}

func TestBuildAddTarXz(t *testing.T) {
	name := "testbuildaddtarxz"
	defer deleteImages(name)

	ctx := func() *FakeContext {
		dockerfile := `
			FROM busybox
			ADD test.tar.xz /
			RUN cat /test/foo | grep Hi`
		tmpDir, err := ioutil.TempDir("", "fake-context")
		testTar, err := os.Create(filepath.Join(tmpDir, "test.tar"))
		if err != nil {
			t.Fatalf("failed to create test.tar archive: %v", err)
		}
		defer testTar.Close()

		tw := tar.NewWriter(testTar)

		if err := tw.WriteHeader(&tar.Header{
			Name: "test/foo",
			Size: 2,
		}); err != nil {
			t.Fatalf("failed to write tar file header: %v", err)
		}
		if _, err := tw.Write([]byte("Hi")); err != nil {
			t.Fatalf("failed to write tar file content: %v", err)
		}
		if err := tw.Close(); err != nil {
			t.Fatalf("failed to close tar archive: %v", err)
		}
		xzCompressCmd := exec.Command("xz", "-k", "test.tar")
		xzCompressCmd.Dir = tmpDir
		out, _, err := runCommandWithOutput(xzCompressCmd)
		if err != nil {
			t.Fatal(err, out)
		}

		if err := ioutil.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
			t.Fatalf("failed to open destination dockerfile: %v", err)
		}
		return fakeContextFromDir(tmpDir)
	}()

	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("build failed to complete for TestBuildAddTarXz: %v", err)
	}

	logDone("build - ADD tar.xz")
}

func TestBuildAddTarXzGz(t *testing.T) {
	name := "testbuildaddtarxzgz"
	defer deleteImages(name)

	ctx := func() *FakeContext {
		dockerfile := `
			FROM busybox
			ADD test.tar.xz.gz /
			RUN ls /test.tar.xz.gz`
		tmpDir, err := ioutil.TempDir("", "fake-context")
		testTar, err := os.Create(filepath.Join(tmpDir, "test.tar"))
		if err != nil {
			t.Fatalf("failed to create test.tar archive: %v", err)
		}
		defer testTar.Close()

		tw := tar.NewWriter(testTar)

		if err := tw.WriteHeader(&tar.Header{
			Name: "test/foo",
			Size: 2,
		}); err != nil {
			t.Fatalf("failed to write tar file header: %v", err)
		}
		if _, err := tw.Write([]byte("Hi")); err != nil {
			t.Fatalf("failed to write tar file content: %v", err)
		}
		if err := tw.Close(); err != nil {
			t.Fatalf("failed to close tar archive: %v", err)
		}

		xzCompressCmd := exec.Command("xz", "-k", "test.tar")
		xzCompressCmd.Dir = tmpDir
		out, _, err := runCommandWithOutput(xzCompressCmd)
		if err != nil {
			t.Fatal(err, out)
		}

		gzipCompressCmd := exec.Command("gzip", "test.tar.xz")
		gzipCompressCmd.Dir = tmpDir
		out, _, err = runCommandWithOutput(gzipCompressCmd)
		if err != nil {
			t.Fatal(err, out)
		}

		if err := ioutil.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
			t.Fatalf("failed to open destination dockerfile: %v", err)
		}
		return fakeContextFromDir(tmpDir)
	}()

	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("build failed to complete for TestBuildAddTarXz: %v", err)
	}

	logDone("build - ADD tar.xz.gz")
}

func TestBuildFromGIT(t *testing.T) {
	name := "testbuildfromgit"
	defer deleteImages(name)
	git, err := fakeGIT("repo", map[string]string{
		"Dockerfile": `FROM busybox
					ADD first /first
					RUN [ -f /first ]
					MAINTAINER docker`,
		"first": "test git data",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer git.Close()

	_, err = buildImageFromPath(name, git.RepoURL, true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Author")
	if err != nil {
		t.Fatal(err)
	}
	if res != "docker" {
		t.Fatalf("Maintainer should be docker, got %s", res)
	}
	logDone("build - build from GIT")
}

func TestBuildCleanupCmdOnEntrypoint(t *testing.T) {
	name := "testbuildcmdcleanuponentrypoint"
	defer deleteImages(name)
	if _, err := buildImage(name,
		`FROM scratch
        CMD ["test"]
		ENTRYPOINT ["echo"]`,
		true); err != nil {
		t.Fatal(err)
	}
	if _, err := buildImage(name,
		fmt.Sprintf(`FROM %s
		ENTRYPOINT ["cat"]`, name),
		true); err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.Cmd")
	if err != nil {
		t.Fatal(err)
	}
	if expected := "<no value>"; res != expected {
		t.Fatalf("Cmd %s, expected %s", res, expected)
	}
	res, err = inspectField(name, "Config.Entrypoint")
	if err != nil {
		t.Fatal(err)
	}
	if expected := "[cat]"; res != expected {
		t.Fatalf("Entrypoint %s, expected %s", res, expected)
	}
	logDone("build - cleanup cmd on ENTRYPOINT")
}

func TestBuildClearCmd(t *testing.T) {
	name := "testbuildclearcmd"
	defer deleteImages(name)
	_, err := buildImage(name,
		`From scratch
   ENTRYPOINT ["/bin/bash"]
   CMD []`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "Config.Cmd")
	if err != nil {
		t.Fatal(err)
	}
	if res != "[]" {
		t.Fatalf("Cmd %s, expected %s", res, "[]")
	}
	logDone("build - clearcmd")
}

func TestBuildEmptyCmd(t *testing.T) {
	name := "testbuildemptycmd"
	defer deleteImages(name)
	if _, err := buildImage(name, "FROM scratch\nMAINTAINER quux\n", true); err != nil {
		t.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "Config.Cmd")
	if err != nil {
		t.Fatal(err)
	}
	if res != "null" {
		t.Fatalf("Cmd %s, expected %s", res, "null")
	}
	logDone("build - empty cmd")
}

func TestBuildOnBuildOutput(t *testing.T) {
	name := "testbuildonbuildparent"
	defer deleteImages(name)
	if _, err := buildImage(name, "FROM busybox\nONBUILD RUN echo foo\n", true); err != nil {
		t.Fatal(err)
	}

	childname := "testbuildonbuildchild"
	defer deleteImages(childname)

	_, out, err := buildImageWithOut(name, "FROM "+name+"\nMAINTAINER quux\n", true)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "Trigger 0, RUN echo foo") {
		t.Fatal("failed to find the ONBUILD output", out)
	}

	logDone("build - onbuild output")
}

func TestBuildInvalidTag(t *testing.T) {
	name := "abcd:" + makeRandomString(200)
	defer deleteImages(name)
	_, out, err := buildImageWithOut(name, "FROM scratch\nMAINTAINER quux\n", true)
	// if the error doesnt check for illegal tag name, or the image is built
	// then this should fail
	if !strings.Contains(out, "Illegal tag name") || strings.Contains(out, "Sending build context to Docker daemon") {
		t.Fatalf("failed to stop before building. Error: %s, Output: %s", err, out)
	}
	logDone("build - invalid tag")
}

func TestBuildCmdShDashC(t *testing.T) {
	name := "testbuildcmdshc"
	defer deleteImages(name)
	if _, err := buildImage(name, "FROM busybox\nCMD echo cmd\n", true); err != nil {
		t.Fatal(err)
	}

	res, err := inspectFieldJSON(name, "Config.Cmd")
	if err != nil {
		t.Fatal(err, res)
	}

	expected := `["/bin/sh","-c","echo cmd"]`

	if res != expected {
		t.Fatalf("Expected value %s not in Config.Cmd: %s", expected, res)
	}

	logDone("build - cmd should have sh -c for non-json")
}

func TestBuildCmdSpaces(t *testing.T) {
	// Test to make sure that when we strcat arrays we take into account
	// the arg separator to make sure ["echo","hi"] and ["echo hi"] don't
	// look the same
	name := "testbuildcmdspaces"
	defer deleteImages(name)
	var id1 string
	var id2 string
	var err error

	if id1, err = buildImage(name, "FROM busybox\nCMD [\"echo hi\"]\n", true); err != nil {
		t.Fatal(err)
	}

	if id2, err = buildImage(name, "FROM busybox\nCMD [\"echo\", \"hi\"]\n", true); err != nil {
		t.Fatal(err)
	}

	if id1 == id2 {
		t.Fatal("Should not have resulted in the same CMD")
	}

	// Now do the same with ENTRYPOINT
	if id1, err = buildImage(name, "FROM busybox\nENTRYPOINT [\"echo hi\"]\n", true); err != nil {
		t.Fatal(err)
	}

	if id2, err = buildImage(name, "FROM busybox\nENTRYPOINT [\"echo\", \"hi\"]\n", true); err != nil {
		t.Fatal(err)
	}

	if id1 == id2 {
		t.Fatal("Should not have resulted in the same ENTRYPOINT")
	}

	logDone("build - cmd with spaces")
}

func TestBuildCmdJSONNoShDashC(t *testing.T) {
	name := "testbuildcmdjson"
	defer deleteImages(name)
	if _, err := buildImage(name, "FROM busybox\nCMD [\"echo\", \"cmd\"]", true); err != nil {
		t.Fatal(err)
	}

	res, err := inspectFieldJSON(name, "Config.Cmd")
	if err != nil {
		t.Fatal(err, res)
	}

	expected := `["echo","cmd"]`

	if res != expected {
		t.Fatalf("Expected value %s not in Config.Cmd: %s", expected, res)
	}

	logDone("build - cmd should not have /bin/sh -c for json")
}

func TestBuildErrorInvalidInstruction(t *testing.T) {
	name := "testbuildignoreinvalidinstruction"
	defer deleteImages(name)

	out, _, err := buildImageWithOut(name, "FROM busybox\nfoo bar", true)
	if err == nil {
		t.Fatalf("Should have failed: %s", out)
	}

	logDone("build - error invalid Dockerfile instruction")
}

func TestBuildEntrypointInheritance(t *testing.T) {
	defer deleteImages("parent", "child")
	defer deleteAllContainers()

	if _, err := buildImage("parent", `
    FROM busybox
    ENTRYPOINT exit 130
    `, true); err != nil {
		t.Fatal(err)
	}

	status, _ := runCommand(exec.Command(dockerBinary, "run", "parent"))

	if status != 130 {
		t.Fatalf("expected exit code 130 but received %d", status)
	}

	if _, err := buildImage("child", `
    FROM parent
    ENTRYPOINT exit 5
    `, true); err != nil {
		t.Fatal(err)
	}

	status, _ = runCommand(exec.Command(dockerBinary, "run", "child"))

	if status != 5 {
		t.Fatalf("expected exit code 5 but received %d", status)
	}

	logDone("build - clear entrypoint")
}

func TestBuildEntrypointInheritanceInspect(t *testing.T) {
	var (
		name     = "testbuildepinherit"
		name2    = "testbuildepinherit2"
		expected = `["/bin/sh","-c","echo quux"]`
	)

	defer deleteImages(name, name2)
	defer deleteAllContainers()

	if _, err := buildImage(name, "FROM busybox\nENTRYPOINT /foo/bar", true); err != nil {
		t.Fatal(err)
	}

	if _, err := buildImage(name2, fmt.Sprintf("FROM %s\nENTRYPOINT echo quux", name), true); err != nil {
		t.Fatal(err)
	}

	res, err := inspectFieldJSON(name2, "Config.Entrypoint")
	if err != nil {
		t.Fatal(err, res)
	}

	if res != expected {
		t.Fatalf("Expected value %s not in Config.Entrypoint: %s", expected, res)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-t", name2))
	if err != nil {
		t.Fatal(err, out)
	}

	expected = "quux"

	if strings.TrimSpace(out) != expected {
		t.Fatalf("Expected output is %s, got %s", expected, out)
	}

	logDone("build - entrypoint override inheritance properly")
}

func TestBuildRunShEntrypoint(t *testing.T) {
	name := "testbuildentrypoint"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
                                ENTRYPOINT /bin/echo`,
		true)
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--rm", name))

	if err != nil {
		t.Fatal(err, out)
	}

	logDone("build - entrypoint with /bin/echo running successfully")
}

func TestBuildExoticShellInterpolation(t *testing.T) {
	name := "testbuildexoticshellinterpolation"
	defer deleteImages(name)

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
		t.Fatal(err)
	}

	logDone("build - exotic shell interpolation")
}

func TestBuildVerifySingleQuoteFails(t *testing.T) {
	// This testcase is supposed to generate an error because the
	// JSON array we're passing in on the CMD uses single quotes instead
	// of double quotes (per the JSON spec). This means we interpret it
	// as a "string" insead of "JSON array" and pass it on to "sh -c" and
	// it should barf on it.
	name := "testbuildsinglequotefails"
	defer deleteImages(name)
	defer deleteAllContainers()

	_, err := buildImage(name,
		`FROM busybox
		CMD [ '/bin/sh', '-c', 'echo hi' ]`,
		true)
	_, _, err = runCommandWithOutput(exec.Command(dockerBinary, "run", "--rm", name))

	if err == nil {
		t.Fatal("The image was not supposed to be able to run")
	}

	logDone("build - verify single quotes break the build")
}

func TestBuildVerboseOut(t *testing.T) {
	name := "testbuildverboseout"
	defer deleteImages(name)

	_, out, err := buildImageWithOut(name,
		`FROM busybox
RUN echo 123`,
		false)

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\n123\n") {
		t.Fatalf("Output should contain %q: %q", "123", out)
	}

	logDone("build - verbose output from commands")
}

func TestBuildWithTabs(t *testing.T) {
	name := "testbuildwithtabs"
	defer deleteImages(name)
	_, err := buildImage(name,
		"FROM busybox\nRUN echo\tone\t\ttwo", true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "ContainerConfig.Cmd")
	if err != nil {
		t.Fatal(err)
	}
	expected1 := `["/bin/sh","-c","echo\tone\t\ttwo"]`
	expected2 := `["/bin/sh","-c","echo\u0009one\u0009\u0009two"]` // syntactically equivalent, and what Go 1.3 generates
	if res != expected1 && res != expected2 {
		t.Fatalf("Missing tabs.\nGot: %s\nExp: %s or %s", res, expected1, expected2)
	}
	logDone("build - with tabs")
}

func TestBuildLabels(t *testing.T) {
	name := "testbuildlabel"
	expected := `{"License":"GPL","Vendor":"Acme"}`
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme
                LABEL License GPL`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectFieldJSON(name, "Config.Labels")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Labels %s, expected %s", res, expected)
	}
	logDone("build - label")
}

func TestBuildLabelsCache(t *testing.T) {
	name := "testbuildlabelcache"
	defer deleteImages(name)

	id1, err := buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme`, false)
	if err != nil {
		t.Fatalf("Build 1 should have worked: %v", err)
	}

	id2, err := buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme`, true)
	if err != nil || id1 != id2 {
		t.Fatalf("Build 2 should have worked & used cache(%s,%s): %v", id1, id2, err)
	}

	id2, err = buildImage(name,
		`FROM busybox
		LABEL Vendor=Acme1`, true)
	if err != nil || id1 == id2 {
		t.Fatalf("Build 3 should have worked & NOT used cache(%s,%s): %v", id1, id2, err)
	}

	id2, err = buildImage(name,
		`FROM busybox
		LABEL Vendor Acme`, true) // Note: " " and "=" should be same
	if err != nil || id1 != id2 {
		t.Fatalf("Build 4 should have worked & used cache(%s,%s): %v", id1, id2, err)
	}

	// Now make sure the cache isn't used by mistake
	id1, err = buildImage(name,
		`FROM busybox
       LABEL f1=b1 f2=b2`, false)
	if err != nil {
		t.Fatalf("Build 5 should have worked: %q", err)
	}

	id2, err = buildImage(name,
		`FROM busybox
       LABEL f1="b1 f2=b2"`, true)
	if err != nil || id1 == id2 {
		t.Fatalf("Build 6 should have worked & NOT used the cache(%s,%s): %q", id1, id2, err)
	}

	logDone("build - label cache")
}

func TestBuildStderr(t *testing.T) {
	// This test just makes sure that no non-error output goes
	// to stderr
	name := "testbuildstderr"
	defer deleteImages(name)
	_, _, stderr, err := buildImageWithStdoutStderr(name,
		"FROM busybox\nRUN echo one", true)
	if err != nil {
		t.Fatal(err)
	}

	if runtime.GOOS == "windows" {
		// stderr might contain a security warning on windows
		lines := strings.Split(stderr, "\n")
		for _, v := range lines {
			if v != "" && !strings.Contains(v, "SECURITY WARNING:") {
				t.Fatalf("Stderr contains unexpected output line: %q", v)
			}
		}
	} else {
		if stderr != "" {
			t.Fatalf("Stderr should have been empty, instead its: %q", stderr)
		}
	}
	logDone("build - testing stderr")
}

func TestBuildChownSingleFile(t *testing.T) {
	testRequires(t, UnixCli) // test uses chown: not available on windows

	name := "testbuildchownsinglefile"
	defer deleteImages(name)

	ctx, err := fakeContext(`
FROM busybox
COPY test /
RUN ls -l /test
RUN [ $(ls -l /test | awk '{print $3":"$4}') = 'root:root' ]
`, map[string]string{
		"test": "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	if err := os.Chown(filepath.Join(ctx.Dir, "test"), 4242, 4242); err != nil {
		t.Fatal(err)
	}

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}

	logDone("build - change permission on single file")
}

func TestBuildSymlinkBreakout(t *testing.T) {
	name := "testbuildsymlinkbreakout"
	tmpdir, err := ioutil.TempDir("", name)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	ctx := filepath.Join(tmpdir, "context")
	if err := os.MkdirAll(ctx, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(ctx, "Dockerfile"), []byte(`
	from busybox
	add symlink.tar /
	add inject /symlink/
	`), 0644); err != nil {
		t.Fatal(err)
	}
	inject := filepath.Join(ctx, "inject")
	if err := ioutil.WriteFile(inject, nil, 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(ctx, "symlink.tar"))
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(tmpdir, "inject")); err == nil {
		t.Fatal("symlink breakout - inject")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	logDone("build - symlink breakout")
}

func TestBuildXZHost(t *testing.T) {
	name := "testbuildxzhost"
	defer deleteImages(name)

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
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatal(err)
	}

	logDone("build - xz host is being used")
}

func TestBuildVolumesRetainContents(t *testing.T) {
	var (
		name     = "testbuildvolumescontent"
		expected = "some text"
	)
	defer deleteImages(name)
	ctx, err := fakeContext(`
FROM busybox
COPY content /foo/file
VOLUME /foo
CMD cat /foo/file`,
		map[string]string{
			"content": expected,
		})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err := buildImageFromContext(name, ctx, false); err != nil {
		t.Fatal(err)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--rm", name))
	if err != nil {
		t.Fatal(err)
	}
	if out != expected {
		t.Fatalf("expected file contents for /foo/file to be %q but received %q", expected, out)
	}

	logDone("build - volumes retain contents in build")
}

func TestBuildRenamedDockerfile(t *testing.T) {
	defer deleteAllContainers()

	ctx, err := fakeContext(`FROM busybox
	RUN echo from Dockerfile`,
		map[string]string{
			"Dockerfile":       "FROM busybox\nRUN echo from Dockerfile",
			"files/Dockerfile": "FROM busybox\nRUN echo from files/Dockerfile",
			"files/dFile":      "FROM busybox\nRUN echo from files/dFile",
			"dFile":            "FROM busybox\nRUN echo from dFile",
			"files/dFile2":     "FROM busybox\nRUN echo from files/dFile2",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := dockerCmdInDir(t, ctx.Dir, "build", "-t", "test1", ".")
	if err != nil {
		t.Fatalf("Failed to build: %s\n%s", out, err)
	}
	if !strings.Contains(out, "from Dockerfile") {
		t.Fatalf("test1 should have used Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(t, ctx.Dir, "build", "-f", filepath.Join("files", "Dockerfile"), "-t", "test2", ".")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "from files/Dockerfile") {
		t.Fatalf("test2 should have used files/Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(t, ctx.Dir, "build", fmt.Sprintf("--file=%s", filepath.Join("files", "dFile")), "-t", "test3", ".")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "from files/dFile") {
		t.Fatalf("test3 should have used files/dFile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(t, ctx.Dir, "build", "--file=dFile", "-t", "test4", ".")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "from dFile") {
		t.Fatalf("test4 should have used dFile, output:%s", out)
	}

	dirWithNoDockerfile, _ := ioutil.TempDir(os.TempDir(), "test5")
	nonDockerfileFile := filepath.Join(dirWithNoDockerfile, "notDockerfile")
	if _, err = os.Create(nonDockerfileFile); err != nil {
		t.Fatal(err)
	}
	out, _, err = dockerCmdInDir(t, ctx.Dir, "build", fmt.Sprintf("--file=%s", nonDockerfileFile), "-t", "test5", ".")

	if err == nil {
		t.Fatalf("test5 was supposed to fail to find passwd")
	}

	if expected := fmt.Sprintf("The Dockerfile (%s) must be within the build context (.)", strings.Replace(nonDockerfileFile, `\`, `\\`, -1)); !strings.Contains(out, expected) {
		t.Fatalf("wrong error messsage:%v\nexpected to contain=%v", out, expected)
	}

	out, _, err = dockerCmdInDir(t, filepath.Join(ctx.Dir, "files"), "build", "-f", filepath.Join("..", "Dockerfile"), "-t", "test6", "..")
	if err != nil {
		t.Fatalf("test6 failed: %s", err)
	}
	if !strings.Contains(out, "from Dockerfile") {
		t.Fatalf("test6 should have used root Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(t, filepath.Join(ctx.Dir, "files"), "build", "-f", filepath.Join(ctx.Dir, "files", "Dockerfile"), "-t", "test7", "..")
	if err != nil {
		t.Fatalf("test7 failed: %s", err)
	}
	if !strings.Contains(out, "from files/Dockerfile") {
		t.Fatalf("test7 should have used files Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(t, filepath.Join(ctx.Dir, "files"), "build", "-f", filepath.Join("..", "Dockerfile"), "-t", "test8", ".")
	if err == nil || !strings.Contains(out, "must be within the build context") {
		t.Fatalf("test8 should have failed with Dockerfile out of context: %s", err)
	}

	tmpDir := os.TempDir()
	out, _, err = dockerCmdInDir(t, tmpDir, "build", "-t", "test9", ctx.Dir)
	if err != nil {
		t.Fatalf("test9 - failed: %s", err)
	}
	if !strings.Contains(out, "from Dockerfile") {
		t.Fatalf("test9 should have used root Dockerfile, output:%s", out)
	}

	out, _, err = dockerCmdInDir(t, filepath.Join(ctx.Dir, "files"), "build", "-f", "dFile2", "-t", "test10", ".")
	if err != nil {
		t.Fatalf("test10 should have worked: %s", err)
	}
	if !strings.Contains(out, "from files/dFile2") {
		t.Fatalf("test10 should have used files/dFile2, output:%s", out)
	}

	logDone("build - rename dockerfile")
}

func TestBuildFromMixedcaseDockerfile(t *testing.T) {
	testRequires(t, UnixCli) // Dockerfile overwrites dockerfile on windows
	defer deleteImages("test1")

	ctx, err := fakeContext(`FROM busybox
	RUN echo from dockerfile`,
		map[string]string{
			"dockerfile": "FROM busybox\nRUN echo from dockerfile",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := dockerCmdInDir(t, ctx.Dir, "build", "-t", "test1", ".")
	if err != nil {
		t.Fatalf("Failed to build: %s\n%s", out, err)
	}

	if !strings.Contains(out, "from dockerfile") {
		t.Fatalf("Missing proper output: %s", out)
	}

	logDone("build - mixedcase Dockerfile")
}

func TestBuildWithTwoDockerfiles(t *testing.T) {
	testRequires(t, UnixCli) // Dockerfile overwrites dockerfile on windows
	defer deleteImages("test1")

	ctx, err := fakeContext(`FROM busybox
RUN echo from Dockerfile`,
		map[string]string{
			"dockerfile": "FROM busybox\nRUN echo from dockerfile",
		})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := dockerCmdInDir(t, ctx.Dir, "build", "-t", "test1", ".")
	if err != nil {
		t.Fatalf("Failed to build: %s\n%s", out, err)
	}

	if !strings.Contains(out, "from Dockerfile") {
		t.Fatalf("Missing proper output: %s", out)
	}

	logDone("build - two Dockerfiles")
}

func TestBuildFromURLWithF(t *testing.T) {
	defer deleteImages("test1")

	server, err := fakeStorage(map[string]string{"baz": `FROM busybox
RUN echo from baz
COPY * /tmp/
RUN find /tmp/`})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, err := fakeContext(`FROM busybox
RUN echo from Dockerfile`,
		map[string]string{})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure that -f is ignored and that we don't use the Dockerfile
	// that's in the current dir
	out, _, err := dockerCmdInDir(t, ctx.Dir, "build", "-f", "baz", "-t", "test1", server.URL()+"/baz")
	if err != nil {
		t.Fatalf("Failed to build: %s\n%s", out, err)
	}

	if !strings.Contains(out, "from baz") ||
		strings.Contains(out, "/tmp/baz") ||
		!strings.Contains(out, "/tmp/Dockerfile") {
		t.Fatalf("Missing proper output: %s", out)
	}

	logDone("build - from URL with -f")
}

func TestBuildFromStdinWithF(t *testing.T) {
	defer deleteImages("test1")

	ctx, err := fakeContext(`FROM busybox
RUN echo from Dockerfile`,
		map[string]string{})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure that -f is ignored and that we don't use the Dockerfile
	// that's in the current dir
	dockerCommand := exec.Command(dockerBinary, "build", "-f", "baz", "-t", "test1", "-")
	dockerCommand.Dir = ctx.Dir
	dockerCommand.Stdin = strings.NewReader(`FROM busybox
RUN echo from baz
COPY * /tmp/
RUN find /tmp/`)
	out, status, err := runCommandWithOutput(dockerCommand)
	if err != nil || status != 0 {
		t.Fatalf("Error building: %s", err)
	}

	if !strings.Contains(out, "from baz") ||
		strings.Contains(out, "/tmp/baz") ||
		!strings.Contains(out, "/tmp/Dockerfile") {
		t.Fatalf("Missing proper output: %s", out)
	}

	logDone("build - from stdin with -f")
}

func TestBuildFromOfficialNames(t *testing.T) {
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
			t.Errorf("Build failed using FROM %s: %s", fromName, err)
		}
		deleteImages(imgName)
	}
	logDone("build - from official names")
}

func TestBuildDockerfileOutsideContext(t *testing.T) {
	testRequires(t, UnixCli) // uses os.Symlink: not implemented in windows at the time of writing (go-1.4.2)

	name := "testbuilddockerfileoutsidecontext"
	tmpdir, err := ioutil.TempDir("", name)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	ctx := filepath.Join(tmpdir, "context")
	if err := os.MkdirAll(ctx, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(ctx, "Dockerfile"), []byte("FROM scratch\nENV X Y"), 0644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(ctx); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(tmpdir, "outsideDockerfile"), []byte("FROM scratch\nENV x y"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "outsideDockerfile"), filepath.Join(ctx, "dockerfile1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(tmpdir, "outsideDockerfile"), filepath.Join(ctx, "dockerfile2")); err != nil {
		t.Fatal(err)
	}

	for _, dockerfilePath := range []string{
		filepath.Join("..", "outsideDockerfile"),
		filepath.Join(ctx, "dockerfile1"),
		filepath.Join(ctx, "dockerfile2"),
	} {
		out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "build", "-t", name, "--no-cache", "-f", dockerfilePath, "."))
		if err == nil {
			t.Fatalf("Expected error with %s. Out: %s", dockerfilePath, out)
		}
		if !strings.Contains(out, "must be within the build context") && !strings.Contains(out, "Cannot locate Dockerfile") {
			t.Fatalf("Unexpected error with %s. Out: %s", dockerfilePath, out)
		}
		deleteImages(name)
	}

	os.Chdir(tmpdir)

	// Path to Dockerfile should be resolved relative to working directory, not relative to context.
	// There is a Dockerfile in the context, but since there is no Dockerfile in the current directory, the following should fail
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "build", "-t", name, "--no-cache", "-f", "Dockerfile", ctx))
	if err == nil {
		t.Fatalf("Expected error. Out: %s", out)
	}
	deleteImages(name)

	logDone("build - Dockerfile outside context")
}

func TestBuildSpaces(t *testing.T) {
	// Test to make sure that leading/trailing spaces on a command
	// doesn't change the error msg we get
	var (
		err1 error
		err2 error
	)

	name := "testspaces"
	defer deleteImages(name)
	ctx, err := fakeContext("FROM busybox\nCOPY\n",
		map[string]string{
			"Dockerfile": "FROM busybox\nCOPY\n",
		})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err1 = buildImageFromContext(name, ctx, false); err1 == nil {
		t.Fatal("Build 1 was supposed to fail, but didn't")
	}

	ctx.Add("Dockerfile", "FROM busybox\nCOPY    ")
	if _, err2 = buildImageFromContext(name, ctx, false); err2 == nil {
		t.Fatal("Build 2 was supposed to fail, but didn't")
	}

	removeLogTimestamps := func(s string) string {
		return regexp.MustCompile(`time="(.*?)"`).ReplaceAllString(s, `time=[TIMESTAMP]`)
	}

	// Skip over the times
	e1 := removeLogTimestamps(err1.Error())
	e2 := removeLogTimestamps(err2.Error())

	// Ignore whitespace since that's what were verifying doesn't change stuff
	if strings.Replace(e1, " ", "", -1) != strings.Replace(e2, " ", "", -1) {
		t.Fatalf("Build 2's error wasn't the same as build 1's\n1:%s\n2:%s", err1, err2)
	}

	ctx.Add("Dockerfile", "FROM busybox\n   COPY")
	if _, err2 = buildImageFromContext(name, ctx, false); err2 == nil {
		t.Fatal("Build 3 was supposed to fail, but didn't")
	}

	// Skip over the times
	e1 = removeLogTimestamps(err1.Error())
	e2 = removeLogTimestamps(err2.Error())

	// Ignore whitespace since that's what were verifying doesn't change stuff
	if strings.Replace(e1, " ", "", -1) != strings.Replace(e2, " ", "", -1) {
		t.Fatalf("Build 3's error wasn't the same as build 1's\n1:%s\n3:%s", err1, err2)
	}

	ctx.Add("Dockerfile", "FROM busybox\n   COPY    ")
	if _, err2 = buildImageFromContext(name, ctx, false); err2 == nil {
		t.Fatal("Build 4 was supposed to fail, but didn't")
	}

	// Skip over the times
	e1 = removeLogTimestamps(err1.Error())
	e2 = removeLogTimestamps(err2.Error())

	// Ignore whitespace since that's what were verifying doesn't change stuff
	if strings.Replace(e1, " ", "", -1) != strings.Replace(e2, " ", "", -1) {
		t.Fatalf("Build 4's error wasn't the same as build 1's\n1:%s\n4:%s", err1, err2)
	}

	logDone("build - test spaces")
}

func TestBuildSpacesWithQuotes(t *testing.T) {
	// Test to make sure that spaces in quotes aren't lost
	name := "testspacesquotes"
	defer deleteImages(name)

	dockerfile := `FROM busybox
RUN echo "  \
  foo  "`

	_, out, err := buildImageWithOut(name, dockerfile, false)
	if err != nil {
		t.Fatal("Build failed:", err)
	}

	expecting := "\n    foo  \n"
	if !strings.Contains(out, expecting) {
		t.Fatalf("Bad output: %q expecting to contian %q", out, expecting)
	}

	logDone("build - test spaces with quotes")
}

// #4393
func TestBuildVolumeFileExistsinContainer(t *testing.T) {
	buildCmd := exec.Command(dockerBinary, "build", "-t", "docker-test-errcreatevolumewithfile", "-")
	buildCmd.Stdin = strings.NewReader(`
	FROM busybox
	RUN touch /foo
	VOLUME /foo
	`)

	out, _, err := runCommandWithOutput(buildCmd)
	if err == nil || !strings.Contains(out, "file exists") {
		t.Fatalf("expected build to fail when file exists in container at requested volume path")
	}

	logDone("build - errors when volume is specified where a file exists")
}

func TestBuildMissingArgs(t *testing.T) {
	// Test to make sure that all Dockerfile commands (except the ones listed
	// in skipCmds) will generate an error if no args are provided.
	// Note: INSERT is deprecated so we exclude it because of that.
	skipCmds := map[string]struct{}{
		"CMD":        {},
		"RUN":        {},
		"ENTRYPOINT": {},
		"INSERT":     {},
	}

	defer deleteAllContainers()

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
			t.Fatal(err)
		}
		defer ctx.Close()
		var out string
		if out, err = buildImageFromContext("args", ctx, true); err == nil {
			t.Fatalf("%s was supposed to fail. Out:%s", cmd, out)
		}
		if !strings.Contains(err.Error(), cmd+" requires") {
			t.Fatalf("%s returned the wrong type of error:%s", cmd, err)
		}
	}

	logDone("build - verify missing args")
}

func TestBuildEmptyScratch(t *testing.T) {
	defer deleteImages("sc")
	_, out, err := buildImageWithOut("sc", "FROM scratch", true)
	if err == nil {
		t.Fatalf("Build was supposed to fail")
	}
	if !strings.Contains(out, "No image was generated") {
		t.Fatalf("Wrong error message: %v", out)
	}
	logDone("build - empty scratch Dockerfile")
}

func TestBuildDotDotFile(t *testing.T) {
	defer deleteImages("sc")
	ctx, err := fakeContext("FROM busybox\n",
		map[string]string{
			"..gitme": "",
		})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	if _, err = buildImageFromContext("sc", ctx, false); err != nil {
		t.Fatalf("Build was supposed to work: %s", err)
	}
	logDone("build - ..file")
}

func TestBuildNotVerbose(t *testing.T) {
	defer deleteAllContainers()
	defer deleteImages("verbose")

	ctx, err := fakeContext("FROM busybox\nENV abc=hi\nRUN echo $abc there", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	// First do it w/verbose - baseline
	buildCmd := exec.Command(dockerBinary, "build", "--no-cache", "-t", "verbose", ".")
	buildCmd.Dir = ctx.Dir
	out, _, err := runCommandWithOutput(buildCmd)
	if err != nil {
		t.Fatalf("failed to build the image w/o -q: %s, %v", out, err)
	}
	if !strings.Contains(out, "hi there") {
		t.Fatalf("missing output:%s\n", out)
	}

	// Now do it w/o verbose
	buildCmd = exec.Command(dockerBinary, "build", "--no-cache", "-q", "-t", "verbose", ".")
	buildCmd.Dir = ctx.Dir
	out, _, err = runCommandWithOutput(buildCmd)
	if err != nil {
		t.Fatalf("failed to build the image w/ -q: %s, %v", out, err)
	}
	if strings.Contains(out, "hi there") {
		t.Fatalf("Bad output, should not contain 'hi there':%s", out)
	}

	logDone("build - not verbose")
}

func TestBuildRUNoneJSON(t *testing.T) {
	name := "testbuildrunonejson"

	defer deleteAllContainers()
	defer deleteImages(name)

	ctx, err := fakeContext(`FROM hello-world:frozen
RUN [ "/hello" ]`, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Close()

	buildCmd := exec.Command(dockerBinary, "build", "--no-cache", "-t", name, ".")
	buildCmd.Dir = ctx.Dir
	out, _, err := runCommandWithOutput(buildCmd)
	if err != nil {
		t.Fatalf("failed to build the image: %s, %v", out, err)
	}

	if !strings.Contains(out, "Hello from Docker") {
		t.Fatalf("bad output: %s", out)
	}

	logDone("build - RUN with one JSON arg")
}

func TestBuildResourceConstraintsAreUsed(t *testing.T) {
	name := "testbuildresourceconstraints"
	defer deleteAllContainers()
	defer deleteImages(name)

	ctx, err := fakeContext(`
	FROM hello-world:frozen
	RUN ["/hello"]
	`, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "build", "--rm=false", "--memory=64m", "--memory-swap=-1", "--cpuset-cpus=1", "--cpu-shares=100", "-t", name, ".")
	cmd.Dir = ctx.Dir

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	out, _, err = dockerCmd(t, "ps", "-lq")
	if err != nil {
		t.Fatal(err, out)
	}

	cID := stripTrailingCharacters(out)

	type hostConfig struct {
		Memory     float64 // Use float64 here since the json decoder sees it that way
		MemorySwap int
		CpusetCpus string
		CpuShares  int
	}

	cfg, err := inspectFieldJSON(cID, "HostConfig")
	if err != nil {
		t.Fatal(err)
	}

	var c1 hostConfig
	if err := json.Unmarshal([]byte(cfg), &c1); err != nil {
		t.Fatal(err, cfg)
	}
	mem := int64(c1.Memory)
	if mem != 67108864 || c1.MemorySwap != -1 || c1.CpusetCpus != "1" || c1.CpuShares != 100 {
		t.Fatalf("resource constraints not set properly:\nMemory: %d, MemSwap: %d, CpusetCpus: %s, CpuShares: %d",
			mem, c1.MemorySwap, c1.CpusetCpus, c1.CpuShares)
	}

	// Make sure constraints aren't saved to image
	_, _, err = dockerCmd(t, "run", "--name=test", name)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = inspectFieldJSON("test", "HostConfig")
	if err != nil {
		t.Fatal(err)
	}
	var c2 hostConfig
	if err := json.Unmarshal([]byte(cfg), &c2); err != nil {
		t.Fatal(err, cfg)
	}
	mem = int64(c2.Memory)
	if mem == 67108864 || c2.MemorySwap == -1 || c2.CpusetCpus == "1" || c2.CpuShares == 100 {
		t.Fatalf("resource constraints leaked from build:\nMemory: %d, MemSwap: %d, CpusetCpus: %s, CpuShares: %d",
			mem, c2.MemorySwap, c2.CpusetCpus, c2.CpuShares)
	}

	logDone("build - resource constraints applied")
}
