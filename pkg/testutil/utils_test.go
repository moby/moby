package testutil

import (
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestConsumeWithSpeed(t *testing.T) {
	reader := strings.NewReader("1234567890")
	chunksize := 2

	bytes1, err := ConsumeWithSpeed(reader, chunksize, 10*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}

	if bytes1 != 10 {
		t.Fatalf("Expected to have read 10 bytes, got %d", bytes1)
	}

}

func TestConsumeWithSpeedWithStop(t *testing.T) {
	reader := strings.NewReader("1234567890")
	chunksize := 2

	stopIt := make(chan bool)

	go func() {
		time.Sleep(1 * time.Millisecond)
		stopIt <- true
	}()

	bytes1, err := ConsumeWithSpeed(reader, chunksize, 20*time.Millisecond, stopIt)
	if err != nil {
		t.Fatal(err)
	}

	if bytes1 != 2 {
		t.Fatalf("Expected to have read 2 bytes, got %d", bytes1)
	}

}

func TestParseCgroupPathsEmpty(t *testing.T) {
	cgroupMap := ParseCgroupPaths("")
	if len(cgroupMap) != 0 {
		t.Fatalf("Expected an empty map, got %v", cgroupMap)
	}
	cgroupMap = ParseCgroupPaths("\n")
	if len(cgroupMap) != 0 {
		t.Fatalf("Expected an empty map, got %v", cgroupMap)
	}
	cgroupMap = ParseCgroupPaths("something:else\nagain:here")
	if len(cgroupMap) != 0 {
		t.Fatalf("Expected an empty map, got %v", cgroupMap)
	}
}

func TestParseCgroupPaths(t *testing.T) {
	cgroupMap := ParseCgroupPaths("2:memory:/a\n1:cpuset:/b")
	if len(cgroupMap) != 2 {
		t.Fatalf("Expected a map with 2 entries, got %v", cgroupMap)
	}
	if value, ok := cgroupMap["memory"]; !ok || value != "/a" {
		t.Fatalf("Expected cgroupMap to contains an entry for 'memory' with value '/a', got %v", cgroupMap)
	}
	if value, ok := cgroupMap["cpuset"]; !ok || value != "/b" {
		t.Fatalf("Expected cgroupMap to contains an entry for 'cpuset' with value '/b', got %v", cgroupMap)
	}
}

func TestChannelBufferTimeout(t *testing.T) {
	expected := "11"

	buf := &ChannelBuffer{make(chan []byte, 1)}
	defer buf.Close()

	done := make(chan struct{}, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		io.Copy(buf, strings.NewReader(expected))
		done <- struct{}{}
	}()

	// Wait long enough
	b := make([]byte, 2)
	_, err := buf.ReadTimeout(b, 50*time.Millisecond)
	if err == nil && err.Error() != "timeout reading from channel" {
		t.Fatalf("Expected an error, got %s", err)
	}
	<-done
}

func TestChannelBuffer(t *testing.T) {
	expected := "11"

	buf := &ChannelBuffer{make(chan []byte, 1)}
	defer buf.Close()

	go func() {
		time.Sleep(100 * time.Millisecond)
		io.Copy(buf, strings.NewReader(expected))
	}()

	// Wait long enough
	b := make([]byte, 2)
	_, err := buf.ReadTimeout(b, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != expected {
		t.Fatalf("Expected '%s', got '%s'", expected, string(b))
	}
}
