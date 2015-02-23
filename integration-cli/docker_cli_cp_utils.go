package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/archive"
)

type FileType uint32

const (
	Regular FileType = iota
	Dir
	Symlink
)

type FileData struct {
	filetype FileType
	path     string
	contents string
}

func (fd FileData) creationCommand() string {
	var command string

	switch fd.filetype {
	case Regular:
		// Don't overwrite the file if it already exists!
		command = fmt.Sprintf("if [ ! -f %s ]; then echo %q > %s; fi", fd.path, fd.contents, fd.path)
	case Dir:
		command = fmt.Sprintf("mkdir -p %s", fd.path)
	case Symlink:
		command = fmt.Sprintf("ln -fs %s %s", fd.contents, fd.path)
	}

	return command
}

func mkFilesCommand(fds []FileData) string {
	commands := make([]string, len(fds))

	for i, fd := range fds {
		commands[i] = fd.creationCommand()
	}

	return strings.Join(commands, " && ")
}

func defaultMkContentCommand() string {
	fds := []FileData{
		{Regular, "file1", "file1"},
		{Regular, "file2", "file2"},
		{Regular, "file3", "file3"},
		{Regular, "file4", "file4"},
		{Regular, "file5", "file5"},
		{Regular, "file6", "file6"},
		{Regular, "file7", "file7"},
		{Dir, "dir1", ""},
		{Regular, "dir1/file1-1", "file1-1"},
		{Regular, "dir1/file1-2", "file1-2"},
		{Dir, "dir2", ""},
		{Regular, "dir2/file2-1", "file2-1"},
		{Regular, "dir2/file2-2", "file2-2"},
		{Dir, "dir3", ""},
		{Regular, "dir3/file3-1", "file3-1"},
		{Regular, "dir3/file3-2", "file3-2"},
		{Dir, "dir4", ""},
		{Regular, "dir4/file3-1", "file4-1"},
		{Regular, "dir4/file3-2", "file4-2"},
		{Dir, "dir5", ""},
		{Symlink, "symlink1", "target1"},
		{Symlink, "symlink2", "target2"},
	}

	return mkFilesCommand(fds)
}

func makeTestContentInDir(t *testing.T, dir string) {
	changeDirCmd := strings.Join([]string{"cd", dir}, " ")
	mkContentCmd := strings.Join([]string{changeDirCmd, defaultMkContentCommand()}, " && ")

	out, status, err := runCommandWithOutput(exec.Command("/bin/sh", "-c", mkContentCmd))
	if err != nil {
		err = fmt.Errorf("unable to execute make content command: %s", err)
	} else if status != 0 {
		err = fmt.Errorf("exit code %d on make content command: %s", status, out)
	}
}

type testContainerOptions struct {
	addContent bool
	readOnly   bool
	volumes    []string
	workDir    string
	command    string
}

func makeTestContainer(t *testing.T, options testContainerOptions) (containerID string) {
	if options.addContent {
		mkContentCmd := defaultMkContentCommand()
		if options.command == "" {
			options.command = mkContentCmd
		} else {
			options.command = fmt.Sprintf("%s && %s", defaultMkContentCommand(), options.command)
		}
	}

	if options.command == "" {
		options.command = "#(nop)"
	}

	args := []string{"run", "-d"}

	for _, volume := range options.volumes {
		args = append(args, "-v", volume)
	}

	if options.workDir != "" {
		args = append(args, "-w", options.workDir)
	}

	if options.readOnly {
		args = append(args, "--read-only")
	}

	args = append(args, "busybox", "/bin/sh", "-c", options.command)

	out, exitCode, err := dockerCmd(t, args...)
	if err != nil || exitCode != 0 {
		t.Fatal("failed to create a container", out, err)
	}

	containerID = stripTrailingCharacters(out)

	out, _, err = dockerCmd(t, "wait", containerID)
	if err != nil || stripTrailingCharacters(out) != "0" {
		t.Fatal("failed to set up container", out, err)
	}

	return
}

func makeCatFileCommand(path string) string {
	return fmt.Sprintf("if [ -f %s ]; then cat %s; fi", path, path)
}

func cpPath(pathElements ...string) string {
	return strings.Join(pathElements, string(filepath.Separator))
}

func cpPathTrailingSep(pathElements ...string) string {
	joined := strings.Join(pathElements, string(filepath.Separator))
	return fmt.Sprintf("%s%c", joined, filepath.Separator)
}

func containerCpPath(containerID string, pathElements ...string) string {
	joined := strings.Join(pathElements, string(filepath.Separator))
	return fmt.Sprintf("%s:%s", containerID, joined)
}

func containerCpPathTrailingSep(containerID string, pathElements ...string) string {
	joined := strings.Join(pathElements, string(filepath.Separator))
	return fmt.Sprintf("%s:%s%c", containerID, joined, filepath.Separator)
}

func runDockerCp(t *testing.T, src, dst string) (err error) {
	t.Logf("running `docker cp %s %s`", src, dst)

	args := []string{"cp", src, dst}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, args...))
	if err != nil {
		err = fmt.Errorf("error executing `docker cp` command: %s: %s", err, out)
	}

	return
}

func startContainerGetOutput(t *testing.T, cID string) (out string, err error) {
	t.Logf("running `docker start -a %s`", cID)

	args := []string{"start", "-a", cID}

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, args...))
	if err != nil {
		err = fmt.Errorf("error executing `docker start` command: %s: %s", err, out)
	}

	return
}

func getTestDir(t *testing.T, label string) (tmpDir string) {
	var err error

	if tmpDir, err = ioutil.TempDir("", label); err != nil {
		t.Fatalf("unable to make temporary directory: %s", err)
	}

	return
}

func isCpNotExist(err error) bool {
	return strings.Contains(err.Error(), "no such file or directory")
}

func isCpDirNotExist(err error) bool {
	return strings.Contains(err.Error(), archive.ErrDirNotExists.Error())
}

func isCpNotDir(err error) bool {
	return strings.Contains(err.Error(), archive.ErrNotDirectory.Error())
}

func isCpCannotCopyDir(err error) bool {
	return strings.Contains(err.Error(), archive.ErrCannotCopyDir.Error())
}

func isCpCannotCopyReadOnly(err error) bool {
	return strings.Contains(err.Error(), "marked read-only")
}

func fileContentEquals(t *testing.T, filename, contents string) (err error) {
	t.Logf("checking that file %q contains %q\n", filename, contents)

	fileBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	expectedBytes, err := ioutil.ReadAll(strings.NewReader(contents))
	if err != nil {
		return
	}

	if !bytes.Equal(fileBytes, expectedBytes) {
		err = fmt.Errorf("file content not equal - expected %q, got %q", string(expectedBytes), string(fileBytes))
	}

	return
}

func containerStartOutputEquals(t *testing.T, cID, contents string) (err error) {
	t.Logf("checking that container %q start output contains %q\n", cID, contents)

	out, err := startContainerGetOutput(t, cID)
	if err != nil {
		return err
	}

	if out != contents {
		err = fmt.Errorf("output contents not equal - expected %q, got %q", contents, out)
	}

	return
}

func defaultVolumes(tmpDir string) []string {
	return []string{
		"/vol1",
		fmt.Sprintf("%s:/vol2", tmpDir),
		fmt.Sprintf("%s:/vol3", filepath.Join(tmpDir, "vol3")),
		fmt.Sprintf("%s:/vol_ro:ro", filepath.Join(tmpDir, "vol_ro")),
	}
}
