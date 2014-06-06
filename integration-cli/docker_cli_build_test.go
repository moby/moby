package main

import (
	"fmt"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildCacheADD(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildCacheADD", "1")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcacheadd1", ".")
	buildCmd.Dir = buildDirectory
	exitCode, err := runCommand(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v", err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	buildDirectory = filepath.Join(workingDirectory, "build_tests", "TestBuildCacheADD", "2")
	buildCmd = exec.Command(dockerBinary, "build", "-t", "testcacheadd2", ".")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	if strings.Contains(out, "Using cache") {
		t.Fatal("2nd build used cache on ADD, it shouldn't")
	}

	deleteImages("testcacheadd1")
	deleteImages("testcacheadd2")

	logDone("build - build two images with ADD")
}

func TestBuildSixtySteps(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildSixtySteps")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "foobuildsixtysteps", ".")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
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
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", ".")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
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
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", "SingleFileToExistDir")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add single file to existing dir")
}

func TestAddSingleFileToNonExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", "SingleFileToNonExistDir")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add single file to non-existing dir")
}

func TestAddDirContentToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", "DirContentToRoot")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add directory contents to root")
}

func TestAddDirContentToExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", "DirContentToExistDir")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
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
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", ".")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add whole directory to root")
}

func TestAddEtcToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", "EtcToRoot")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
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
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcopyimg", ".")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
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
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcopyimg", "SingleFileToExistDir")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - add single file to existing dir")
}

func TestCopySingleFileToNonExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcopyimg", "SingleFileToNonExistDir")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - copy single file to non-existing dir")
}

func TestCopyDirContentToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcopyimg", "DirContentToRoot")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - copy directory contents to root")
}

func TestCopyDirContentToExistDir(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcopyimg", "DirContentToExistDir")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
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
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcopyimg", ".")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testcopyimg")

	logDone("build - copy whole directory to root")
}

func TestCopyEtcToRoot(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestCopy")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testcopyimg", "EtcToRoot")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
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

		buildCmd := exec.Command(dockerBinary, "build", "-t", "testlinksok", ".")
		buildCmd.Dir = pathToDirectoryWhichContainsLinks
		out, exitCode, err := runCommandWithOutput(buildCmd)
		if err != nil || exitCode != 0 {
			t.Fatalf("build should have worked: %s %s", err, out)
		}

		deleteImages("testlinksok")

	}
	deleteImages("inaccessiblefiles")
	logDone("build - ADD from context with inaccessible files must fail")
	logDone("build - ADD from context with accessible links must work")
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
		buildCmd := exec.Command(dockerBinary, "build", "--rm", "-t", "testbuildrm", ".")
		buildCmd.Dir = buildDirectory
		_, exitCode, err := runCommandWithOutput(buildCmd)

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
		buildCmd := exec.Command(dockerBinary, "build", "-t", "testbuildrm", ".")
		buildCmd.Dir = buildDirectory
		_, exitCode, err := runCommandWithOutput(buildCmd)

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
		buildCmd := exec.Command(dockerBinary, "build", "--rm=false", "-t", "testbuildrm", ".")
		buildCmd.Dir = buildDirectory
		_, exitCode, err := runCommandWithOutput(buildCmd)

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
	name := "testbuildvolumes"
	expected := "map[/test1:map[] /test2:map[]]"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM scratch
		VOLUME /test1
		VOLUME /test2`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inspectField(name, "Config.Volumes")
	if err != nil {
		t.Fatal(err)
	}
	if res != expected {
		t.Fatalf("Volumes %s, expected %s", res, expected)
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
	expected := "[HOME=/ PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin PORT=2375]"
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
