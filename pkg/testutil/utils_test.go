package testutil

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestRunCommandPipelineWithOutputWithNotEnoughCmds(t *testing.T) {
	_, _, err := RunCommandPipelineWithOutput(exec.Command("ls"))
	expectedError := "pipeline does not have multiple cmds"
	if err == nil || err.Error() != expectedError {
		t.Fatalf("Expected an error with %s, got err:%s", expectedError, err)
	}
}

func TestRunCommandPipelineWithOutputErrors(t *testing.T) {
	p := "$PATH"
	if runtime.GOOS == "windows" {
		p = "%PATH%"
	}
	cmd1 := exec.Command("ls")
	cmd1.Stdout = os.Stdout
	cmd2 := exec.Command("anything really")
	_, _, err := RunCommandPipelineWithOutput(cmd1, cmd2)
	if err == nil || err.Error() != "cannot set stdout pipe for anything really: exec: Stdout already set" {
		t.Fatalf("Expected an error, got %v", err)
	}

	cmdWithError := exec.Command("doesnotexists")
	cmdCat := exec.Command("cat")
	_, _, err = RunCommandPipelineWithOutput(cmdWithError, cmdCat)
	if err == nil || err.Error() != `starting doesnotexists failed with error: exec: "doesnotexists": executable file not found in `+p {
		t.Fatalf("Expected an error, got %v", err)
	}
}

func TestRunCommandPipelineWithOutput(t *testing.T) {
	//TODO: Should run on Solaris
	if runtime.GOOS == "solaris" {
		t.Skip()
	}
	cmds := []*exec.Cmd{
		// Print 2 characters
		exec.Command("echo", "-n", "11"),
		// Count the number or char from stdin (previous command)
		exec.Command("wc", "-m"),
	}
	out, exitCode, err := RunCommandPipelineWithOutput(cmds...)
	expectedOutput := "2\n"
	if out != expectedOutput || exitCode != 0 || err != nil {
		t.Fatalf("Expected %s for commands %v, got out:%s, exitCode:%d, err:%v", expectedOutput, cmds, out, exitCode, err)
	}
}

func TestRandomTmpDirPath(t *testing.T) {
	path := RandomTmpDirPath("something", runtime.GOOS)

	prefix := "/tmp/something"
	if runtime.GOOS == "windows" {
		prefix = os.Getenv("TEMP") + `\something`
	}
	expectedSize := len(prefix) + 11

	if !strings.HasPrefix(path, prefix) {
		t.Fatalf("Expected generated path to have '%s' as prefix, got %s'", prefix, path)
	}
	if len(path) != expectedSize {
		t.Fatalf("Expected generated path to be %d, got %d", expectedSize, len(path))
	}
}
