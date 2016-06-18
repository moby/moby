package integration

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestIsKilledFalseWithNonKilledProcess(t *testing.T) {
	var lsCmd *exec.Cmd
	if runtime.GOOS != "windows" {
		lsCmd = exec.Command("ls")
	} else {
		lsCmd = exec.Command("cmd", "/c", "dir")
	}

	err := lsCmd.Run()
	if IsKilled(err) {
		t.Fatalf("Expected the ls command to not be killed, was.")
	}
}

func TestIsKilledTrueWithKilledProcess(t *testing.T) {
	var longCmd *exec.Cmd
	if runtime.GOOS != "windows" {
		longCmd = exec.Command("top")
	} else {
		longCmd = exec.Command("powershell", "while ($true) { sleep 1 }")
	}

	// Start a command
	err := longCmd.Start()
	if err != nil {
		t.Fatal(err)
	}
	// Capture the error when *dying*
	done := make(chan error, 1)
	go func() {
		done <- longCmd.Wait()
	}()
	// Then kill it
	longCmd.Process.Kill()
	// Get the error
	err = <-done
	if !IsKilled(err) {
		t.Fatalf("Expected the command to be killed, was not.")
	}
}

func TestRunCommandWithOutput(t *testing.T) {
	var (
		echoHelloWorldCmd *exec.Cmd
		expected          string
	)
	if runtime.GOOS != "windows" {
		echoHelloWorldCmd = exec.Command("echo", "hello", "world")
		expected = "hello world\n"
	} else {
		echoHelloWorldCmd = exec.Command("cmd", "/s", "/c", "echo", "hello", "world")
		expected = "hello world\r\n"
	}

	out, exitCode, err := RunCommandWithOutput(echoHelloWorldCmd)
	if out != expected || exitCode != 0 || err != nil {
		t.Fatalf("Expected command to output %s, got %s, %v with exitCode %v", expected, out, err, exitCode)
	}
}

func TestRunCommandWithOutputError(t *testing.T) {
	var (
		p                string
		wrongCmd         *exec.Cmd
		expected         string
		expectedExitCode int
	)

	if runtime.GOOS != "windows" {
		p = "$PATH"
		wrongCmd = exec.Command("ls", "-z")
		expected = `ls: invalid option -- 'z'
Try 'ls --help' for more information.
`
		expectedExitCode = 2
	} else {
		p = "%PATH%"
		wrongCmd = exec.Command("cmd", "/s", "/c", "dir", "/Z")
		expected = "Invalid switch - " + strconv.Quote("Z") + ".\r\n"
		expectedExitCode = 1
	}
	cmd := exec.Command("doesnotexists")
	out, exitCode, err := RunCommandWithOutput(cmd)
	expectedError := `exec: "doesnotexists": executable file not found in ` + p
	if out != "" || exitCode != 127 || err == nil || err.Error() != expectedError {
		t.Fatalf("Expected command to output %s, got %s, %v with exitCode %v", expectedError, out, err, exitCode)
	}

	out, exitCode, err = RunCommandWithOutput(wrongCmd)

	if out != expected || exitCode != expectedExitCode || err == nil || !strings.Contains(err.Error(), "exit status "+strconv.Itoa(expectedExitCode)) {
		t.Fatalf("Expected command to output %s, got out:xxx%sxxx, err:%v with exitCode %v", expected, out, err, exitCode)
	}
}

func TestRunCommandWithStdoutStderr(t *testing.T) {
	echoHelloWorldCmd := exec.Command("echo", "hello", "world")
	stdout, stderr, exitCode, err := RunCommandWithStdoutStderr(echoHelloWorldCmd)
	expected := "hello world\n"
	if stdout != expected || stderr != "" || exitCode != 0 || err != nil {
		t.Fatalf("Expected command to output %s, got stdout:%s, stderr:%s, err:%v with exitCode %v", expected, stdout, stderr, err, exitCode)
	}
}

func TestRunCommandWithStdoutStderrError(t *testing.T) {
	p := "$PATH"
	if runtime.GOOS == "windows" {
		p = "%PATH%"
	}
	cmd := exec.Command("doesnotexists")
	stdout, stderr, exitCode, err := RunCommandWithStdoutStderr(cmd)
	expectedError := `exec: "doesnotexists": executable file not found in ` + p
	if stdout != "" || stderr != "" || exitCode != 127 || err == nil || err.Error() != expectedError {
		t.Fatalf("Expected command to output out:%s, stderr:%s, got stdout:%s, stderr:%s, err:%v with exitCode %v", "", "", stdout, stderr, err, exitCode)
	}

	wrongLsCmd := exec.Command("ls", "-z")
	expected := `ls: invalid option -- 'z'
Try 'ls --help' for more information.
`

	stdout, stderr, exitCode, err = RunCommandWithStdoutStderr(wrongLsCmd)
	if stdout != "" && stderr != expected || exitCode != 2 || err == nil || err.Error() != "exit status 2" {
		t.Fatalf("Expected command to output out:%s, stderr:%s, got stdout:%s, stderr:%s, err:%v with exitCode %v", "", expectedError, stdout, stderr, err, exitCode)
	}
}

func TestRunCommandWithOutputForDurationFinished(t *testing.T) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		t.Skip("Needs porting to Windows")
	}

	cmd := exec.Command("ls")
	out, exitCode, timedOut, err := RunCommandWithOutputForDuration(cmd, 50*time.Millisecond)
	if out == "" || exitCode != 0 || timedOut || err != nil {
		t.Fatalf("Expected the command to run for less 50 milliseconds and thus not time out, but did not : out:[%s], exitCode:[%d], timedOut:[%v], err:[%v]", out, exitCode, timedOut, err)
	}
}

func TestRunCommandWithOutputForDurationKilled(t *testing.T) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		t.Skip("Needs porting to Windows")
	}
	cmd := exec.Command("sh", "-c", "while true ; do echo 1 ; sleep .1 ; done")
	out, exitCode, timedOut, err := RunCommandWithOutputForDuration(cmd, 500*time.Millisecond)
	ones := strings.Split(out, "\n")
	if len(ones) != 6 || exitCode != 0 || !timedOut || err != nil {
		t.Fatalf("Expected the command to run for 500 milliseconds (and thus print six lines (five with 1, one empty) and time out, but did not : out:[%s], exitCode:%d, timedOut:%v, err:%v", out, exitCode, timedOut, err)
	}
}

func TestRunCommandWithOutputForDurationErrors(t *testing.T) {
	cmd := exec.Command("ls")
	cmd.Stdout = os.Stdout
	if _, _, _, err := RunCommandWithOutputForDuration(cmd, 1*time.Millisecond); err == nil || err.Error() != "cmd.Stdout already set" {
		t.Fatalf("Expected an error as cmd.Stdout was already set, did not (err:%s).", err)
	}
	cmd = exec.Command("ls")
	cmd.Stderr = os.Stderr
	if _, _, _, err := RunCommandWithOutputForDuration(cmd, 1*time.Millisecond); err == nil || err.Error() != "cmd.Stderr already set" {
		t.Fatalf("Expected an error as cmd.Stderr was already set, did not (err:%s).", err)
	}
}

func TestRunCommandWithOutputAndTimeoutFinished(t *testing.T) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		t.Skip("Needs porting to Windows")
	}

	cmd := exec.Command("ls")
	out, exitCode, err := RunCommandWithOutputAndTimeout(cmd, 50*time.Millisecond)
	if out == "" || exitCode != 0 || err != nil {
		t.Fatalf("Expected the command to run for less 50 milliseconds and thus not time out, but did not : out:[%s], exitCode:[%d], err:[%v]", out, exitCode, err)
	}
}

func TestRunCommandWithOutputAndTimeoutKilled(t *testing.T) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		t.Skip("Needs porting to Windows")
	}

	cmd := exec.Command("sh", "-c", "while true ; do echo 1 ; sleep .1 ; done")
	out, exitCode, err := RunCommandWithOutputAndTimeout(cmd, 500*time.Millisecond)
	ones := strings.Split(out, "\n")
	if len(ones) != 6 || exitCode != 0 || err == nil || err.Error() != "command timed out" {
		t.Fatalf("Expected the command to run for 500 milliseconds (and thus print six lines (five with 1, one empty) and time out with an error 'command timed out', but did not : out:[%s], exitCode:%d, err:%v", out, exitCode, err)
	}
}

func TestRunCommandWithOutputAndTimeoutErrors(t *testing.T) {
	cmd := exec.Command("ls")
	cmd.Stdout = os.Stdout
	if _, _, err := RunCommandWithOutputAndTimeout(cmd, 1*time.Millisecond); err == nil || err.Error() != "cmd.Stdout already set" {
		t.Fatalf("Expected an error as cmd.Stdout was already set, did not (err:%s).", err)
	}
	cmd = exec.Command("ls")
	cmd.Stderr = os.Stderr
	if _, _, err := RunCommandWithOutputAndTimeout(cmd, 1*time.Millisecond); err == nil || err.Error() != "cmd.Stderr already set" {
		t.Fatalf("Expected an error as cmd.Stderr was already set, did not (err:%s).", err)
	}
}

func TestRunCommand(t *testing.T) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		t.Skip("Needs porting to Windows")
	}

	p := "$PATH"
	if runtime.GOOS == "windows" {
		p = "%PATH%"
	}
	lsCmd := exec.Command("ls")
	exitCode, err := RunCommand(lsCmd)
	if exitCode != 0 || err != nil {
		t.Fatalf("Expected runCommand to run the command successfully, got: exitCode:%d, err:%v", exitCode, err)
	}

	var expectedError string

	exitCode, err = RunCommand(exec.Command("doesnotexists"))
	expectedError = `exec: "doesnotexists": executable file not found in ` + p
	if exitCode != 127 || err == nil || err.Error() != expectedError {
		t.Fatalf("Expected runCommand to run the command successfully, got: exitCode:%d, err:%v", exitCode, err)
	}
	wrongLsCmd := exec.Command("ls", "-z")
	expected := 2
	expectedError = `exit status 2`
	exitCode, err = RunCommand(wrongLsCmd)
	if exitCode != expected || err == nil || err.Error() != expectedError {
		t.Fatalf("Expected runCommand to run the command successfully, got: exitCode:%d, err:%v", exitCode, err)
	}
}

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

// Simple simple test as it is just a passthrough for json.Unmarshal
func TestUnmarshalJSON(t *testing.T) {
	emptyResult := struct{}{}
	if err := UnmarshalJSON([]byte(""), &emptyResult); err == nil {
		t.Fatalf("Expected an error, got nothing")
	}
	result := struct{ Name string }{}
	if err := UnmarshalJSON([]byte(`{"name": "name"}`), &result); err != nil {
		t.Fatal(err)
	}
	if result.Name != "name" {
		t.Fatalf("Expected result.name to be 'name', was '%s'", result.Name)
	}
}

func TestConvertSliceOfStringsToMap(t *testing.T) {
	input := []string{"a", "b"}
	actual := ConvertSliceOfStringsToMap(input)
	for _, key := range input {
		if _, ok := actual[key]; !ok {
			t.Fatalf("Expected output to contains key %s, did not: %v", key, actual)
		}
	}
}

func TestCompareDirectoryEntries(t *testing.T) {
	tmpFolder, err := ioutil.TempDir("", "integration-cli-utils-compare-directories")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpFolder)

	file1 := filepath.Join(tmpFolder, "file1")
	file2 := filepath.Join(tmpFolder, "file2")
	os.Create(file1)
	os.Create(file2)

	fi1, err := os.Stat(file1)
	if err != nil {
		t.Fatal(err)
	}
	fi1bis, err := os.Stat(file1)
	if err != nil {
		t.Fatal(err)
	}
	fi2, err := os.Stat(file2)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		e1          []os.FileInfo
		e2          []os.FileInfo
		shouldError bool
	}{
		// Empty directories
		{
			[]os.FileInfo{},
			[]os.FileInfo{},
			false,
		},
		// Same FileInfos
		{
			[]os.FileInfo{fi1},
			[]os.FileInfo{fi1},
			false,
		},
		// Different FileInfos but same names
		{
			[]os.FileInfo{fi1},
			[]os.FileInfo{fi1bis},
			false,
		},
		// Different FileInfos, different names
		{
			[]os.FileInfo{fi1},
			[]os.FileInfo{fi2},
			true,
		},
	}
	for _, elt := range cases {
		err := CompareDirectoryEntries(elt.e1, elt.e2)
		if elt.shouldError && err == nil {
			t.Fatalf("Should have return an error, did not with %v and %v", elt.e1, elt.e2)
		}
		if !elt.shouldError && err != nil {
			t.Fatalf("Should have not returned an error, but did : %v with %v and %v", err, elt.e1, elt.e2)
		}
	}
}

// FIXME make an "unhappy path" test for ListTar without "panicking" :-)
func TestListTar(t *testing.T) {
	// TODO Windows: Figure out why this fails. Should be portable.
	if runtime.GOOS == "windows" {
		t.Skip("Failing on Windows - needs further investigation")
	}
	tmpFolder, err := ioutil.TempDir("", "integration-cli-utils-list-tar")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpFolder)

	// Let's create a Tar file
	srcFile := filepath.Join(tmpFolder, "src")
	tarFile := filepath.Join(tmpFolder, "src.tar")
	os.Create(srcFile)
	cmd := exec.Command("sh", "-c", "tar cf "+tarFile+" "+srcFile)
	_, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}

	reader, err := os.Open(tarFile)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	entries, err := ListTar(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 && entries[0] != "src" {
		t.Fatalf("Expected a tar file with 1 entry (%s), got %v", srcFile, entries)
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

	bytes1, err := ConsumeWithSpeed(reader, chunksize, 1*time.Second, nil)
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

// FIXME doesn't work
// func TestRunAtDifferentDate(t *testing.T) {
// 	var date string

// 	// Layout for date. MMDDhhmmYYYY
// 	const timeLayout = "20060102"
// 	expectedDate := "20100201"
// 	theDate, err := time.Parse(timeLayout, expectedDate)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	RunAtDifferentDate(theDate, func() {
// 		cmd := exec.Command("date", "+%Y%M%d")
// 		out, err := cmd.Output()
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		date = string(out)
// 	})
// }
