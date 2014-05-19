package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", "SingleFileToRoot")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add single file to root")
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
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestAdd")
	buildCmd := exec.Command(dockerBinary, "build", "-t", "testaddimg", "WholeDirToRoot")
	buildCmd.Dir = buildDirectory
	out, exitCode, err := runCommandWithOutput(buildCmd)
	errorOut(err, t, fmt.Sprintf("build failed to complete: %v %v", out, err))

	if err != nil || exitCode != 0 {
		t.Fatal("failed to build the image")
	}

	deleteImages("testaddimg")

	logDone("build - add whole directory to root")
}

// Issue #5270 - ensure we throw a better error than "unexpected EOF"
// when we can't access files in the context.
func TestBuildWithInaccessibleFilesInContext(t *testing.T) {
	buildDirectory := filepath.Join(workingDirectory, "build_tests", "TestBuildWithInaccessibleFilesInContext")
	addUserCmd := exec.Command("adduser", "unprivilegeduser")
	out, _, err := runCommandWithOutput(addUserCmd)
	errorOut(err, t, fmt.Sprintf("failed to add user: %v %v", out, err))

	{
		// This is used to ensure we detect inaccessible files early during build in the cli client
		pathToInaccessibleFileBuildDirectory := filepath.Join(buildDirectory, "inaccessiblefile")
		pathToFileWithoutReadAccess := filepath.Join(pathToInaccessibleFileBuildDirectory, "fileWithoutReadAccess")

		err = os.Chown(pathToFileWithoutReadAccess, 0, 0)
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
			t.Fatalf("output should've contained the string: no permission to read from ")
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

		err = os.Chown(pathToDirectoryWithoutReadAccess, 0, 0)
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

// TODO: TestCaching

// TODO: TestADDCacheInvalidation
