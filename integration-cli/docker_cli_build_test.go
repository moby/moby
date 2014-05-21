package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

// TODO: TestCaching

// TODO: TestADDCacheInvalidation
