package integration

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestIsKilledFalseWithNonKilledProcess(c *check.C) {
	var lsCmd *exec.Cmd
	if runtime.GOOS != "windows" {
		lsCmd = exec.Command("ls")
	} else {
		lsCmd = exec.Command("cmd", "/c", "dir")
	}

	err := lsCmd.Run()
	if IsKilled(err) {
		c.Fatalf("Expected the ls command to not be killed, was.")
	}
}

func (s *DockerSuite) TestIsKilledTrueWithKilledProcess(c *check.C) {
	var longCmd *exec.Cmd
	if runtime.GOOS != "windows" {
		longCmd = exec.Command("top")
	} else {
		longCmd = exec.Command("powershell", "while ($true) { sleep 1 }")
	}

	// Start a command
	err := longCmd.Start()
	if err != nil {
		c.Fatal(err)
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
		c.Fatalf("Expected the command to be killed, was not.")
	}
}

func (s *DockerSuite) TestRunCommandPipelineWithOutputWithNotEnoughCmds(c *check.C) {
	_, _, err := RunCommandPipelineWithOutput(exec.Command("ls"))
	expectedError := "pipeline does not have multiple cmds"
	if err == nil || err.Error() != expectedError {
		c.Fatalf("Expected an error with %s, got err:%s", expectedError, err)
	}
}

func (s *DockerSuite) TestRunCommandPipelineWithOutputErrors(c *check.C) {
	p := "$PATH"
	if runtime.GOOS == "windows" {
		p = "%PATH%"
	}
	cmd1 := exec.Command("ls")
	cmd1.Stdout = os.Stdout
	cmd2 := exec.Command("anything really")
	_, _, err := RunCommandPipelineWithOutput(cmd1, cmd2)
	if err == nil || err.Error() != "cannot set stdout pipe for anything really: exec: Stdout already set" {
		c.Fatalf("Expected an error, got %v", err)
	}

	cmdWithError := exec.Command("doesnotexists")
	cmdCat := exec.Command("cat")
	_, _, err = RunCommandPipelineWithOutput(cmdWithError, cmdCat)
	if err == nil || err.Error() != `starting doesnotexists failed with error: exec: "doesnotexists": executable file not found in `+p {
		c.Fatalf("Expected an error, got %v", err)
	}
}

func (s *DockerSuite) TestRunCommandPipelineWithOutput(c *check.C) {
	cmds := []*exec.Cmd{
		// Print 2 characters
		exec.Command("echo", "-n", "11"),
		// Count the number or char from stdin (previous command)
		exec.Command("wc", "-m"),
	}
	out, exitCode, err := RunCommandPipelineWithOutput(cmds...)
	expectedOutput := "2\n"
	if out != expectedOutput || exitCode != 0 || err != nil {
		c.Fatalf("Expected %s for commands %v, got out:%s, exitCode:%d, err:%v", expectedOutput, cmds, out, exitCode, err)
	}
}

func (s *DockerSuite) TestConvertSliceOfStringsToMap(c *check.C) {
	input := []string{"a", "b"}
	actual := ConvertSliceOfStringsToMap(input)
	for _, key := range input {
		if _, ok := actual[key]; !ok {
			c.Fatalf("Expected output to contains key %s, did not: %v", key, actual)
		}
	}
}

func (s *DockerSuite) TestCompareDirectoryEntries(c *check.C) {
	tmpFolder, err := ioutil.TempDir("", "integration-cli-utils-compare-directories")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpFolder)

	file1 := filepath.Join(tmpFolder, "file1")
	file2 := filepath.Join(tmpFolder, "file2")
	os.Create(file1)
	os.Create(file2)

	fi1, err := os.Stat(file1)
	if err != nil {
		c.Fatal(err)
	}
	fi1bis, err := os.Stat(file1)
	if err != nil {
		c.Fatal(err)
	}
	fi2, err := os.Stat(file2)
	if err != nil {
		c.Fatal(err)
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
			c.Fatalf("Should have return an error, did not with %v and %v", elt.e1, elt.e2)
		}
		if !elt.shouldError && err != nil {
			c.Fatalf("Should have not returned an error, but did : %v with %v and %v", err, elt.e1, elt.e2)
		}
	}
}

// FIXME make an "unhappy path" test for ListTar without "panicking" :-)
func (s *DockerSuite) TestListTar(c *check.C) {
	// TODO Windows: Figure out why this fails. Should be portable.
	if runtime.GOOS == "windows" {
		c.Skip("Failing on Windows - needs further investigation")
	}
	tmpFolder, err := ioutil.TempDir("", "integration-cli-utils-list-tar")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpFolder)

	// Let's create a Tar file
	srcFile := filepath.Join(tmpFolder, "src")
	tarFile := filepath.Join(tmpFolder, "src.tar")
	os.Create(srcFile)
	cmd := exec.Command("sh", "-c", "tar cf "+tarFile+" "+srcFile)
	_, err = cmd.CombinedOutput()
	if err != nil {
		c.Fatal(err)
	}

	reader, err := os.Open(tarFile)
	if err != nil {
		c.Fatal(err)
	}
	defer reader.Close()

	entries, err := ListTar(reader)
	if err != nil {
		c.Fatal(err)
	}
	if len(entries) != 1 && entries[0] != "src" {
		c.Fatalf("Expected a tar file with 1 entry (%s), got %v", srcFile, entries)
	}
}

func (s *DockerSuite) TestRandomTmpDirPath(c *check.C) {
	path := RandomTmpDirPath("something", runtime.GOOS)

	prefix := "/tmp/something"
	if runtime.GOOS == "windows" {
		prefix = os.Getenv("TEMP") + `\something`
	}
	expectedSize := len(prefix) + 11

	if !strings.HasPrefix(path, prefix) {
		c.Fatalf("Expected generated path to have '%s' as prefix, got %s'", prefix, path)
	}
	if len(path) != expectedSize {
		c.Fatalf("Expected generated path to be %d, got %d", expectedSize, len(path))
	}
}

func (s *DockerSuite) TestConsumeWithSpeed(c *check.C) {
	reader := strings.NewReader("1234567890")
	chunksize := 2

	bytes1, err := ConsumeWithSpeed(reader, chunksize, 1*time.Second, nil)
	if err != nil {
		c.Fatal(err)
	}

	if bytes1 != 10 {
		c.Fatalf("Expected to have read 10 bytes, got %d", bytes1)
	}

}

func (s *DockerSuite) TestConsumeWithSpeedWithStop(c *check.C) {
	reader := strings.NewReader("1234567890")
	chunksize := 2

	stopIt := make(chan bool)

	go func() {
		time.Sleep(1 * time.Millisecond)
		stopIt <- true
	}()

	bytes1, err := ConsumeWithSpeed(reader, chunksize, 20*time.Millisecond, stopIt)
	if err != nil {
		c.Fatal(err)
	}

	if bytes1 != 2 {
		c.Fatalf("Expected to have read 2 bytes, got %d", bytes1)
	}

}

func (s *DockerSuite) TestParseCgroupPathsEmpty(c *check.C) {
	cgroupMap := ParseCgroupPaths("")
	if len(cgroupMap) != 0 {
		c.Fatalf("Expected an empty map, got %v", cgroupMap)
	}
	cgroupMap = ParseCgroupPaths("\n")
	if len(cgroupMap) != 0 {
		c.Fatalf("Expected an empty map, got %v", cgroupMap)
	}
	cgroupMap = ParseCgroupPaths("something:else\nagain:here")
	if len(cgroupMap) != 0 {
		c.Fatalf("Expected an empty map, got %v", cgroupMap)
	}
}

func (s *DockerSuite) TestParseCgroupPaths(c *check.C) {
	cgroupMap := ParseCgroupPaths("2:memory:/a\n1:cpuset:/b")
	if len(cgroupMap) != 2 {
		c.Fatalf("Expected a map with 2 entries, got %v", cgroupMap)
	}
	if value, ok := cgroupMap["memory"]; !ok || value != "/a" {
		c.Fatalf("Expected cgroupMap to contains an entry for 'memory' with value '/a', got %v", cgroupMap)
	}
	if value, ok := cgroupMap["cpuset"]; !ok || value != "/b" {
		c.Fatalf("Expected cgroupMap to contains an entry for 'cpuset' with value '/b', got %v", cgroupMap)
	}
}

func (s *DockerSuite) TestChannelBufferTimeout(c *check.C) {
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
		c.Fatalf("Expected an error, got %s", err)
	}
	<-done
}

func (s *DockerSuite) TestChannelBuffer(c *check.C) {
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
		c.Fatal(err)
	}

	if string(b) != expected {
		c.Fatalf("Expected '%s', got '%s'", expected, string(b))
	}
}

// FIXME doesn't work
// func (s *DockerSuite) TestRunAtDifferentDate(c *check.C) {
// 	var date string

// 	// Layout for date. MMDDhhmmYYYY
// 	const timeLayout = "20060102"
// 	expectedDate := "20100201"
// 	theDate, err := time.Parse(timeLayout, expectedDate)
// 	if err != nil {
// 		c.Fatal(err)
// 	}

// 	RunAtDifferentDate(theDate, func() {
// 		cmd := exec.Command("date", "+%Y%M%d")
// 		out, err := cmd.Output()
// 		if err != nil {
// 			c.Fatal(err)
// 		}
// 		date = string(out)
// 	})
// }
