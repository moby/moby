package main

import (
	"archive/tar"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/archive"
)

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
		ADD %s/robots.txt /`, server.URL),
		true); err != nil {
		t.Fatal(err)
	}
	out, _, err := buildImageWithOut(name,
		fmt.Sprintf(`FROM scratch
		ADD %s/index.html /`, server.URL),
		true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Using cache") {
		t.Fatal("2nd build used cache on ADD, it shouldn't")
	}

	logDone("build - build two images with remote ADD")
}

func TestBuildSixtySteps(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildSixtySteps")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "foobuildsixtysteps", ".")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("foobuildsixtysteps")

	logDone("build - build an image with sixty build steps")
}

func TestAddSingleFileToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd", "SingleFileToRoot")
	f, err := os.OpenFile(filepath.Join(buildDirectory, "test_file"), os.O_CREATE, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testaddimg", ".")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add single file to root")
}

// Issue #3960: "ADD src ." hangs
func TestAddSingleFileToWorkdir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd", "SingleFileToWorkdir")
	f, err := os.OpenFile(filepath.Join(buildDirectory, "test_file"), os.O_CREATE, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", ".")
	buildCmd.Dir = buildDirectory
	done := make(chan error)
	go func() {
		out, exitCode, err := runCommandWithOutput(buildCmd)
		if err != nil || exitCode != 0 {
			done <- fmt.Errorf("build failed to complete: %s %v", out, err)
			return
		}
		done <- nil
	}()
	select {
	case <-time.After(5 * time.Second):
		if err := buildCmd.Process.Kill(); err != nil {
			fmt.Printf("could not kill build (pid=%d): %v\n", buildCmd.Process.Pid, err)
		}
		t.Fatal("build timed out")
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	}

	deleteImages("testaddimg")

	logDone("build - add single file to workdir")
}

func TestAddSingleFileToExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testaddimg", "SingleFileToExistDir")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add single file to existing dir")
}

func TestAddSingleFileToNonExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testaddimg", "SingleFileToNonExistDir")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add single file to non-existing dir")
}

func TestAddDirContentToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testaddimg", "DirContentToRoot")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add directory contents to root")
}

func TestAddDirContentToExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testaddimg", "DirContentToExistDir")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add directory contents to existing dir")
}

func TestAddWholeDirToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd", "WholeDirToRoot")
	test_dir := filepath.Join(buildDirectory, "test_dir")
	if err := os.MkdirAll(test_dir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(test_dir, "test_file"), os.O_CREATE, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testaddimg", ".")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add whole directory to root")
}

func TestAddEtcToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testaddimg", "EtcToRoot")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")
	logDone("build - add etc directory to root")
}

func TestCopySingleFileToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy", "SingleFileToRoot")
	f, err := os.OpenFile(filepath.Join(buildDirectory, "test_file"), os.O_CREATE, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testcopyimg", ".")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - copy single file to root")
}

// Issue #3960: "ADD src ." hangs - adapted for COPY
func TestCopySingleFileToWorkdir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy", "SingleFileToWorkdir")
	f, err := os.OpenFile(filepath.Join(buildDirectory, "test_file"), os.O_CREATE, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcopyimg", ".")
	buildCmd.Dir = buildDirectory
	done := make(chan error)
	go func() {
		out, exitCode, err := runCommandWithOutput(buildCmd)
		if err != nil || exitCode != 0 {
			done <- fmt.Errorf("build failed to complete: %s %v", out, err)
			return
		}
		done <- nil
	}()
	select {
	case <-time.After(5 * time.Second):
		if err := buildCmd.Process.Kill(); err != nil {
			fmt.Printf("could not kill build (pid=%d): %v\n", buildCmd.Process.Pid, err)
		}
		t.Fatal("build timed out")
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	}

	deleteImages("testcopyimg")

	logDone("build - copy single file to workdir")
}

func TestCopySingleFileToExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testcopyimg", "SingleFileToExistDir")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - add single file to existing dir")
}

func TestCopySingleFileToNonExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testcopyimg", "SingleFileToNonExistDir")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - copy single file to non-existing dir")
}

func TestCopyDirContentToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testcopyimg", "DirContentToRoot")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - copy directory contents to root")
}

func TestCopyDirContentToExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testcopyimg", "DirContentToExistDir")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - copy directory contents to existing dir")
}

func TestCopyWholeDirToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy", "WholeDirToRoot")
	test_dir := filepath.Join(buildDirectory, "test_dir")
	if err := os.MkdirAll(test_dir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(test_dir, "test_file"), os.O_CREATE, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testcopyimg", ".")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - copy whole directory to root")
}

func TestCopyEtcToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	out, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testcopyimg", "EtcToRoot")
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")
	logDone("build - copy etc directory to root")
}

func TestCopyDisallowRemote(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcopyimg", "DisallowRemote")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)

	if err == nil || exitCode == 0 {
		t.Fatalf("building the image should've failed; output: %s", out)
	}

	deleteImages("testcopyimg")
	logDone("build - copy - disallow copy from remote")
}

// Issue #5270 - ensure we throw a better error than "unexpected EOF"
// when we can't access files in the context.
func TestBuildWithInaccessibleFilesInContext(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildWithInaccessibleFilesInContext")

	{
		// This is used to ensure we detect inaccessible files early during build in the cli client
		pathToInaccessibleFileBuildDirectory := filepath.Join(buildDirectory, "inaccessiblefile")
		pathToFileWithoutReadAccess := filepath.Join(pathToInaccessibleFileBuildDirectory, "fileWithoutReadAccess")

		err := os.Chown(pathToFileWithoutReadAccess, 0, 0)
		errorOut(err, t, fmt.Sprintf("failed to chown file to root: %s", err))
		err = os.Chmod(pathToFileWithoutReadAccess, 0700)
		errorOut(err, t, fmt.Sprintf("failed to chmod file to 700: %s", err))

		buildCommandStatement := fmt.Sprintf("%s build -t inaccessiblefiles .", dockerBinary)
		buildCmd := exec.Command("su", "unprivilegeduser", "-c", buildCommandStatement)
		buildCmd.Dir = pathToInaccessibleFileBuildDirectory
		out, exitCode, err := runCommandWithOutput(buildCmd)
		if err == nil || exitCode == 0 {
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
		// This is used to ensure we detect inaccessible directories early during build in the cli client
		pathToInaccessibleDirectoryBuildDirectory := filepath.Join(buildDirectory, "inaccessibledirectory")
		pathToDirectoryWithoutReadAccess := filepath.Join(pathToInaccessibleDirectoryBuildDirectory, "directoryWeCantStat")
		pathToFileInDirectoryWithoutReadAccess := filepath.Join(pathToDirectoryWithoutReadAccess, "bar")

		err := os.Chown(pathToDirectoryWithoutReadAccess, 0, 0)
		errorOut(err, t, fmt.Sprintf("failed to chown directory to root: %s", err))
		err = os.Chmod(pathToDirectoryWithoutReadAccess, 0444)
		errorOut(err, t, fmt.Sprintf("failed to chmod directory to 755: %s", err))
		err = os.Chmod(pathToFileInDirectoryWithoutReadAccess, 0700)
		errorOut(err, t, fmt.Sprintf("failed to chmod file to 444: %s", err))

		buildCommandStatement := fmt.Sprintf("%s build -t inaccessiblefiles .", dockerBinary)
		buildCmd := exec.Command("su", "unprivilegeduser", "-c", buildCommandStatement)
		buildCmd.Dir = pathToInaccessibleDirectoryBuildDirectory
		out, exitCode, err := runCommandWithOutput(buildCmd)
		if err == nil || exitCode == 0 {
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
		// This is used to ensure we don't follow links when checking if everything in the context is accessible
		// This test doesn't require that we run commands as an unprivileged user
		pathToDirectoryWhichContainsLinks := filepath.Join(buildDirectory, "linksdirectory")

		out, exitCode, err := dockerCmdInDir(t, pathToDirectoryWhichContainsLinks, "build", "-t", "testlinksok", ".")
		if err != nil || exitCode != 0 {
			t.Fatalf("build should have worked: %s %s", err, out)
		}

		deleteImages("testlinksok")

	}
	{
		// This is used to ensure we don't try to add inaccessible files when they are ignored by a .dockerignore pattern
		pathToInaccessibleDirectoryBuildDirectory := filepath.Join(buildDirectory, "ignoredinaccessible")
		pathToDirectoryWithoutReadAccess := filepath.Join(pathToInaccessibleDirectoryBuildDirectory, "directoryWeCantStat")
		pathToFileInDirectoryWithoutReadAccess := filepath.Join(pathToDirectoryWithoutReadAccess, "bar")
		err := os.Chown(pathToDirectoryWithoutReadAccess, 0, 0)
		errorOut(err, t, fmt.Sprintf("failed to chown directory to root: %s", err))
		err = os.Chmod(pathToDirectoryWithoutReadAccess, 0444)
		errorOut(err, t, fmt.Sprintf("failed to chmod directory to 755: %s", err))
		err = os.Chmod(pathToFileInDirectoryWithoutReadAccess, 0700)
		errorOut(err, t, fmt.Sprintf("failed to chmod file to 444: %s", err))

		buildCommandStatement := fmt.Sprintf("%s build -t ignoredinaccessible .", dockerBinary)
		buildCmd := exec.Command("su", "unprivilegeduser", "-c", buildCommandStatement)
		buildCmd.Dir = pathToInaccessibleDirectoryBuildDirectory
		out, exitCode, err := runCommandWithOutput(buildCmd)
		if err != nil || exitCode != 0 {
			t.Fatalf("build should have worked: %s %s", err, out)
		}
		deleteImages("ignoredinaccessible")

	}
	deleteImages("inaccessiblefiles")
	logDone("build - ADD from context with inaccessible files must fail")
	logDone("build - ADD from context with accessible links must work")
	logDone("build - ADD from context with ignored inaccessible files must work")
}

func TestBuildForceRm(t *testing.T) {
	containerCountBefore, err := getContainerCount()
	if err != nil {
		t.Fatalf("failed to get the container count: %s", err)
	}

	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildForceRm")
	buildCmd := exec.Command(dockerBinary, "build", "--force-rm", ".")
	buildCmd.Dir = buildDirectory
	_, exitCode, err := runCommandWithOutput(buildCmd)

	if err == nil || exitCode == 0 {
		t.Fatal("failed to build the image")
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
	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildRm")
		_, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "--rm", "-t", "testbuildrm", ".")

		if err != nil || exitCode != 0 {
			t.Fatal("failed to build the image")
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore != containerCountAfter {
			t.Fatalf("-rm shouldn't have left containers behind")
		}
		deleteImages("testbuildrm")
	}

	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildRm")
		_, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "-t", "testbuildrm", ".")

		if err != nil || exitCode != 0 {
			t.Fatal("failed to build the image")
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore != containerCountAfter {
			t.Fatalf("--rm shouldn't have left containers behind")
		}
		deleteImages("testbuildrm")
	}

	{
		containerCountBefore, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildRm")
		_, exitCode, err := dockerCmdInDir(t, buildDirectory, "build", "--rm=false", "-t", "testbuildrm", ".")

		if err != nil || exitCode != 0 {
			t.Fatal("failed to build the image")
		}

		containerCountAfter, err := getContainerCount()
		if err != nil {
			t.Fatalf("failed to get the container count: %s", err)
		}

		if containerCountBefore == containerCountAfter {
			t.Fatalf("--rm=false should have left containers behind")
		}
		deleteAllContainers()
		deleteImages("testbuildrm")

	}

	logDone("build - ensure --rm doesn't leave containers behind and that --rm=true is the default")
	logDone("build - ensure --rm=false overrides the default")
}

func TestBuildWithVolumes(t *testing.T) {
	var (
		result   map[string]map[string]struct{}
		name     = "testbuildvolumes"
		emptyMap = make(map[string]struct{})
		expected = map[string]map[string]struct{}{"/test1": emptyMap, "/test2": emptyMap}
	)
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM scratch
		VOLUME /test1
		VOLUME /test2`,
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

	equal := deepEqual(&expected, &result)

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

func TestBuildEnv(t *testing.T) {
	name := "testbuildenv"
	expected := "[PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin PORT=2375]"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM busybox
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

		out1, _, err := dockerCmdInDir(t, ctx.Dir, "build", "-t", name1, ".")
		errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out1, err))
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

		out2, _, err = dockerCmdInDir(t, ctx.Dir, "build", "-t", name2, ".")
		errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out2, err))
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

		out3, _, err = dockerCmdInDir(t, ctx.Dir, "build", "-t", name3, ".")
		errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out3, err))
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
		false)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	logDone("build - without cache")
}

func TestBuildADDLocalFileWithCache(t *testing.T) {
	name := "testbuildaddlocalfilewithcache"
	defer deleteImages(name)
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
	id2, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("The cache should have been used but hasn't.")
	}
	logDone("build - add local file with cache")
}

func TestBuildADDLocalFileWithoutCache(t *testing.T) {
	name := "testbuildaddlocalfilewithoutcache"
	defer deleteImages(name)
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
	id2, err := buildImageFromContext(name, ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	logDone("build - add local file without cache")
}

func TestBuildADDCurrentDirWithCache(t *testing.T) {
	name := "testbuildaddcurrentdirwithcache"
	defer deleteImages(name)
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
	id2, err := buildImageFromContext(name, ctx, true)
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
	id3, err := buildImageFromContext(name, ctx, true)
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
	id4, err := buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if id3 == id4 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	id5, err := buildImageFromContext(name, ctx, true)
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
	defer deleteImages(name)
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
	id2, err := buildImageFromContext(name, ctx, false)
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
        ADD %s/baz /usr/lib/baz/quux`, server.URL),
		true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL),
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
        ADD %s/baz /usr/lib/baz/quux`, server.URL),
		true)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := buildImage(name,
		fmt.Sprintf(`FROM scratch
        MAINTAINER dockerio
        ADD %s/baz /usr/lib/baz/quux`, server.URL),
		false)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("The cache should have been invalided but hasn't.")
	}
	logDone("build - add remote file without cache")
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
        ADD %s/baz /usr/lib/baz/quux`, server.URL),
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
	contextDirectory := filepath.Join(workingDirectory, "build_tests", "TestContextTar")
	context, err := archive.Tar(contextDirectory, compression)

	if err != nil {
		t.Fatalf("failed to build context tar: %v", err)
	}
	buildCmd := exec.Command(dockerBinary, "build", "-t", "contexttar", "-")
	buildCmd.Stdin = context

	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("build failed to complete: %v %v", out, err)
	}
	deleteImages("contexttar")
	logDone(fmt.Sprintf("build - build an image with a context tar, compression: %v", compression))
}

func TestContextTarGzip(t *testing.T) {
	testContextTar(t, archive.Gzip)
}

func TestContextTarNoCompression(t *testing.T) {
	testContextTar(t, archive.Uncompressed)
}

func TestNoContext(t *testing.T) {
	buildCmd := exec.Command(dockerBinary, "build", "-t", "nocontext", "-")
	buildCmd.Stdin = strings.NewReader("FROM busybox\nCMD echo ok\n")

	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("build failed to complete: %v %v", out, err)
	}

	out, exitCode, err = cmd(t, "run", "nocontext")
	if out != "ok\n" {
		t.Fatalf("run produced invalid output: %q, expected %q", out, "ok")
	}

	deleteImages("nocontext")
	logDone("build - build an image with no context")
}

// TODO: TestCaching
func TestBuildADDLocalAndRemoteFilesWithoutCache(t *testing.T) {
	name := "testbuildaddlocalandremotefilewithoutcache"
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
        ADD %s/baz /usr/lib/baz/quux`, server.URL),
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
	id2, err := buildImageFromContext(name, ctx, false)
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
		t.Fatal(err)
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
	logDone("build - fails")
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
	logDone("build - fails with empty dockerfile")
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

	if res != "Docker IO <io@docker.com>" {
		t.Fatal("Parsed string did not match the escaped string")
	}

	logDone("build - validate escaping whitespace")
}

func TestDockerignore(t *testing.T) {
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

func TestDockerignoringDockerfile(t *testing.T) {
	name := "testbuilddockerignoredockerfile"
	defer deleteImages(name)
	dockerfile := `
        FROM scratch`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"Dockerfile":    "FROM scratch",
		".dockerignore": "Dockerfile\n",
	})
	defer ctx.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = buildImageFromContext(name, ctx, true); err == nil {
		t.Fatalf("Didn't get expected error from ignoring Dockerfile")
	}
	logDone("build - test .dockerignore of Dockerfile")
}

func TestDockerignoringWholeDir(t *testing.T) {
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
ENV    FOO /foo/baz
ENV    BAR /bar
ENV    BAZ $BAR
ENV    FOOPATH $PATH:$FOO
RUN    [ "$BAR" = "$BAZ" ]
RUN    [ "$FOOPATH" = "$PATH:/foo/baz" ]
ENV	   FROM hello/docker/world
ENV    TO /docker/world/hello
ADD    $FROM $TO
RUN    [ "$(cat $TO)" = "hello" ]`
	ctx, err := fakeContext(dockerfile, map[string]string{
		"hello/docker/world": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = buildImageFromContext(name, ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	logDone("build - environment variables usage")
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
		return &FakeContext{Dir: tmpDir}
	}()

	if _, err := buildImageFromContext(name, ctx, true); err != nil {
		t.Fatalf("build failed to complete for TestBuildAddTar: %v", err)
	}

	logDone("build - ADD tar")
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
	})
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
