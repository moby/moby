package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	cpTestPathParent = "/some"
	cpTestPath       = "/some/path"
	cpTestName       = "test"
	cpFullPath       = "/some/path/test"

	cpContainerContents = "holla, i am the container"
	cpHostContents      = "hello, i am the host"
)

// Test for #5656
// Check that garbage paths don't escape the container's rootfs
func TestCpGarbagePath(t *testing.T) {
	out, exitCode, err := cmd(t, "run", "-d", "busybox", "/bin/sh", "-c", "mkdir -p '"+cpTestPath+"' && echo -n '"+cpContainerContents+"' > "+cpFullPath)
	if err != nil || exitCode != 0 {
		t.Fatal("failed to create a container", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteContainer(cleanedContainerID)

	out, _, err = cmd(t, "wait", cleanedContainerID)
	if err != nil || stripTrailingCharacters(out) != "0" {
		t.Fatal("failed to set up container", out, err)
	}

	if err := os.MkdirAll(cpTestPath, os.ModeDir); err != nil {
		t.Fatal(err)
	}

	hostFile, err := os.Create(cpFullPath)
	if err != nil {
		t.Fatal(err)
	}
	defer hostFile.Close()
	defer os.RemoveAll(cpTestPathParent)

	fmt.Fprintf(hostFile, "%s", cpHostContents)

	tmpdir, err := ioutil.TempDir("", "docker-integration")
	if err != nil {
		t.Fatal(err)
	}

	tmpname := filepath.Join(tmpdir, cpTestName)
	defer os.RemoveAll(tmpdir)

	path := filepath.Join("../../../../../../../../../../../../", cpFullPath)

	_, _, err = cmd(t, "cp", cleanedContainerID+":"+path, tmpdir)
	if err != nil {
		t.Fatalf("couldn't copy from garbage path: %s:%s %s", cleanedContainerID, path, err)
	}

	file, _ := os.Open(tmpname)
	defer file.Close()

	test, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	if string(test) == cpHostContents {
		t.Errorf("output matched host file -- garbage path can escape container rootfs")
	}

	if string(test) != cpContainerContents {
		t.Errorf("output doesn't match the input for garbage path")
	}

	logDone("cp - garbage paths relative to container's rootfs")
}

// Check that relative paths are relative to the container's rootfs
func TestCpRelativePath(t *testing.T) {
	out, exitCode, err := cmd(t, "run", "-d", "busybox", "/bin/sh", "-c", "mkdir -p '"+cpTestPath+"' && echo -n '"+cpContainerContents+"' > "+cpFullPath)
	if err != nil || exitCode != 0 {
		t.Fatal("failed to create a container", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteContainer(cleanedContainerID)

	out, _, err = cmd(t, "wait", cleanedContainerID)
	if err != nil || stripTrailingCharacters(out) != "0" {
		t.Fatal("failed to set up container", out, err)
	}

	if err := os.MkdirAll(cpTestPath, os.ModeDir); err != nil {
		t.Fatal(err)
	}

	hostFile, err := os.Create(cpFullPath)
	if err != nil {
		t.Fatal(err)
	}
	defer hostFile.Close()
	defer os.RemoveAll(cpTestPathParent)

	fmt.Fprintf(hostFile, "%s", cpHostContents)

	tmpdir, err := ioutil.TempDir("", "docker-integration")

	if err != nil {
		t.Fatal(err)
	}

	tmpname := filepath.Join(tmpdir, cpTestName)
	defer os.RemoveAll(tmpdir)

	path, _ := filepath.Rel("/", cpFullPath)

	_, _, err = cmd(t, "cp", cleanedContainerID+":"+path, tmpdir)
	if err != nil {
		t.Fatalf("couldn't copy from relative path: %s:%s %s", cleanedContainerID, path, err)
	}

	file, _ := os.Open(tmpname)
	defer file.Close()

	test, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	if string(test) == cpHostContents {
		t.Errorf("output matched host file -- relative path can escape container rootfs")
	}

	if string(test) != cpContainerContents {
		t.Errorf("output doesn't match the input for relative path")
	}

	logDone("cp - relative paths relative to container's rootfs")
}

// Check that absolute paths are relative to the container's rootfs
func TestCpAbsolutePath(t *testing.T) {
	out, exitCode, err := cmd(t, "run", "-d", "busybox", "/bin/sh", "-c", "mkdir -p '"+cpTestPath+"' && echo -n '"+cpContainerContents+"' > "+cpFullPath)
	if err != nil || exitCode != 0 {
		t.Fatal("failed to create a container", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteContainer(cleanedContainerID)

	out, _, err = cmd(t, "wait", cleanedContainerID)
	if err != nil || stripTrailingCharacters(out) != "0" {
		t.Fatal("failed to set up container", out, err)
	}

	if err := os.MkdirAll(cpTestPath, os.ModeDir); err != nil {
		t.Fatal(err)
	}

	hostFile, err := os.Create(cpFullPath)
	if err != nil {
		t.Fatal(err)
	}
	defer hostFile.Close()
	defer os.RemoveAll(cpTestPathParent)

	fmt.Fprintf(hostFile, "%s", cpHostContents)

	tmpdir, err := ioutil.TempDir("", "docker-integration")

	if err != nil {
		t.Fatal(err)
	}

	tmpname := filepath.Join(tmpdir, cpTestName)
	defer os.RemoveAll(tmpdir)

	path := cpFullPath

	_, _, err = cmd(t, "cp", cleanedContainerID+":"+path, tmpdir)
	if err != nil {
		t.Fatalf("couldn't copy from absolute path: %s:%s %s", cleanedContainerID, path, err)
	}

	file, _ := os.Open(tmpname)
	defer file.Close()

	test, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	if string(test) == cpHostContents {
		t.Errorf("output matched host file -- absolute path can escape container rootfs")
	}

	if string(test) != cpContainerContents {
		t.Errorf("output doesn't match the input for absolute path")
	}

	logDone("cp - absolute paths relative to container's rootfs")
}

// Test for #5619
// Check that absolute symlinks are still relative to the container's rootfs
func TestCpAbsoluteSymlink(t *testing.T) {
	out, exitCode, err := cmd(t, "run", "-d", "busybox", "/bin/sh", "-c", "mkdir -p '"+cpTestPath+"' && echo -n '"+cpContainerContents+"' > "+cpFullPath+" && ln -s "+cpFullPath+" container_path")
	if err != nil || exitCode != 0 {
		t.Fatal("failed to create a container", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteContainer(cleanedContainerID)

	out, _, err = cmd(t, "wait", cleanedContainerID)
	if err != nil || stripTrailingCharacters(out) != "0" {
		t.Fatal("failed to set up container", out, err)
	}

	if err := os.MkdirAll(cpTestPath, os.ModeDir); err != nil {
		t.Fatal(err)
	}

	hostFile, err := os.Create(cpFullPath)
	if err != nil {
		t.Fatal(err)
	}
	defer hostFile.Close()
	defer os.RemoveAll(cpTestPathParent)

	fmt.Fprintf(hostFile, "%s", cpHostContents)

	tmpdir, err := ioutil.TempDir("", "docker-integration")

	if err != nil {
		t.Fatal(err)
	}

	tmpname := filepath.Join(tmpdir, cpTestName)
	defer os.RemoveAll(tmpdir)

	path := filepath.Join("/", "container_path")

	_, _, err = cmd(t, "cp", cleanedContainerID+":"+path, tmpdir)
	if err != nil {
		t.Fatalf("couldn't copy from absolute path: %s:%s %s", cleanedContainerID, path, err)
	}

	file, _ := os.Open(tmpname)
	defer file.Close()

	test, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	if string(test) == cpHostContents {
		t.Errorf("output matched host file -- absolute symlink can escape container rootfs")
	}

	if string(test) != cpContainerContents {
		t.Errorf("output doesn't match the input for absolute symlink")
	}

	logDone("cp - absolute symlink relative to container's rootfs")
}

// Test for #5619
// Check that symlinks which are part of the resource path are still relative to the container's rootfs
func TestCpSymlinkComponent(t *testing.T) {
	out, exitCode, err := cmd(t, "run", "-d", "busybox", "/bin/sh", "-c", "mkdir -p '"+cpTestPath+"' && echo -n '"+cpContainerContents+"' > "+cpFullPath+" && ln -s "+cpTestPath+" container_path")
	if err != nil || exitCode != 0 {
		t.Fatal("failed to create a container", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteContainer(cleanedContainerID)

	out, _, err = cmd(t, "wait", cleanedContainerID)
	if err != nil || stripTrailingCharacters(out) != "0" {
		t.Fatal("failed to set up container", out, err)
	}

	if err := os.MkdirAll(cpTestPath, os.ModeDir); err != nil {
		t.Fatal(err)
	}

	hostFile, err := os.Create(cpFullPath)
	if err != nil {
		t.Fatal(err)
	}
	defer hostFile.Close()
	defer os.RemoveAll(cpTestPathParent)

	fmt.Fprintf(hostFile, "%s", cpHostContents)

	tmpdir, err := ioutil.TempDir("", "docker-integration")

	if err != nil {
		t.Fatal(err)
	}

	tmpname := filepath.Join(tmpdir, cpTestName)
	defer os.RemoveAll(tmpdir)

	path := filepath.Join("/", "container_path", cpTestName)

	_, _, err = cmd(t, "cp", cleanedContainerID+":"+path, tmpdir)
	if err != nil {
		t.Fatalf("couldn't copy from symlink path component: %s:%s %s", cleanedContainerID, path, err)
	}

	file, _ := os.Open(tmpname)
	defer file.Close()

	test, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	if string(test) == cpHostContents {
		t.Errorf("output matched host file -- symlink path component can escape container rootfs")
	}

	if string(test) != cpContainerContents {
		t.Errorf("output doesn't match the input for symlink path component")
	}

	logDone("cp - symlink path components relative to container's rootfs")
}

// Check that cp with unprivileged user doesn't return any error
func TestCpUnprivilegedUser(t *testing.T) {
	out, exitCode, err := cmd(t, "run", "-d", "busybox", "/bin/sh", "-c", "touch "+cpTestName)
	if err != nil || exitCode != 0 {
		t.Fatal("failed to create a container", out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)
	defer deleteContainer(cleanedContainerID)

	out, _, err = cmd(t, "wait", cleanedContainerID)
	if err != nil || stripTrailingCharacters(out) != "0" {
		t.Fatal("failed to set up container", out, err)
	}

	tmpdir, err := ioutil.TempDir("", "docker-integration")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmpdir)

	if err = os.Chmod(tmpdir, 0777); err != nil {
		t.Fatal(err)
	}

	path := cpTestName

	_, _, err = runCommandWithOutput(exec.Command("su", "unprivilegeduser", "-c", dockerBinary+" cp "+cleanedContainerID+":"+path+" "+tmpdir))
	if err != nil {
		t.Fatalf("couldn't copy with unprivileged user: %s:%s %s", cleanedContainerID, path, err)
	}

	logDone("cp - unprivileged user")
}

func TestCopyContainerHost(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		errorOut(err, t, fmt.Sprintf("failed to create temp dir: %v", err))
	}
	defer os.RemoveAll(tmpDir)

	containerCmd := `echo -n test > /foo`
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", containerCmd)
	cid, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		errorOut(err, t, fmt.Sprintf("failed to start the container: %v", err))
	}
	cleanCID := stripTrailingCharacters(cid)
	time.Sleep(time.Second)
	copyCmd := exec.Command(dockerBinary, "cp", fmt.Sprintf("%s:/foo", cleanCID), tmpDir+"/bar")

	_, _, err = runCommandWithOutput(copyCmd)
	if err != nil {
		errorOut(err, t, fmt.Sprintf("failed to copy from the container: %v", err))
	}
	file, err := os.Open(tmpDir + "/bar/foo")
	if err != nil {
		errorOut(err, t, fmt.Sprintf("failed to open the temp file: %v", err))
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		errorOut(err, t, fmt.Sprintf("failed to read the temp file: %v", err))
	}
	if string(content) != "test" {
		t.Errorf("the file wasn't copied")
	}
	go deleteContainer(cleanCID)

	logDone("copy - check copy from container to host")
}

func TestCopyHostContainer(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		errorOut(err, t, fmt.Sprintf("failed to create temp file: %v", err))
	}
	fmt.Fprintf(tmpFile, "test")
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "ls")
	cid, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		errorOut(err, t, fmt.Sprintf("failed to start the container: %v", err))
	}
	cleanCID := stripTrailingCharacters(cid)
	time.Sleep(time.Second)
	copyCmd := exec.Command(dockerBinary, "cp", tmpFile.Name(), fmt.Sprintf("%s:/foo", cleanCID))

	_, _, err = runCommandWithOutput(copyCmd)
	if err != nil {
		errorOut(err, t, fmt.Sprintf("failed to copy from the container: %v", err))
	}

	diffCmd := exec.Command(dockerBinary, "diff", cleanCID)
	out, _, err := runCommandWithOutput(diffCmd)
	errorOut(err, t, fmt.Sprintf("failed to run diff: %v %v", out, err))

	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains("A /foo", line) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("couldn't find the new file in docker diff's output: %v", out)
	}
	go deleteContainer(cleanCID)

	logDone("copy - check copy from host to container")
}
