package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
)

func getExitCode(err error) (int, error) {
	exitCode := 0
	if exiterr, ok := err.(*exec.ExitError); ok {
		if procExit := exiterr.Sys().(syscall.WaitStatus); ok {
			return procExit.ExitStatus(), nil
		}
	}
	return exitCode, fmt.Errorf("failed to get exit code")
}

func processExitCode(err error) (exitCode int) {
	if err != nil {
		var exiterr error
		if exitCode, exiterr = getExitCode(err); exiterr != nil {
			// TODO: Fix this so we check the error's text.
			// we've failed to retrieve exit code, so we set it to 127
			exitCode = 127
		}
	}
	return
}

func runCommandWithOutput(cmd *exec.Cmd) (output string, exitCode int, err error) {
	exitCode = 0
	out, err := cmd.CombinedOutput()
	exitCode = processExitCode(err)
	output = string(out)
	return
}

func runCommandWithStdoutStderr(cmd *exec.Cmd) (stdout string, stderr string, exitCode int, err error) {
	var (
		stderrBuffer, stdoutBuffer bytes.Buffer
	)
	exitCode = 0
	cmd.Stderr = &stderrBuffer
	cmd.Stdout = &stdoutBuffer
	err = cmd.Run()
	exitCode = processExitCode(err)

	stdout = stdoutBuffer.String()
	stderr = stderrBuffer.String()
	return
}

var ErrCmdTimeout = fmt.Errorf("command timed out")

func runCommandWithOutputAndTimeout(cmd *exec.Cmd, timeout time.Duration) (output string, exitCode int, err error) {
	done := make(chan error)
	go func() {
		output, exitCode, err = runCommandWithOutput(cmd)
		if err != nil || exitCode != 0 {
			done <- fmt.Errorf("failed to run command: %s", err)
			return
		}
		done <- nil
	}()
	select {
	case <-time.After(timeout):
		killFailed := cmd.Process.Kill()
		if killFailed == nil {
			fmt.Printf("failed to kill (pid=%d): %v\n", cmd.Process.Pid, err)
		}
		err = ErrCmdTimeout
	case <-done:
		break
	}
	return
}

func runCommand(cmd *exec.Cmd) (exitCode int, err error) {
	exitCode = 0
	err = cmd.Run()
	exitCode = processExitCode(err)
	return
}

func logDone(message string) {
	fmt.Printf("[PASSED]: %s\n", message)
}

func stripTrailingCharacters(target string) string {
	target = strings.Trim(target, "\n")
	target = strings.Trim(target, " ")
	return target
}

func unmarshalJSON(data []byte, result interface{}) error {
	err := json.Unmarshal(data, result)
	if err != nil {
		return err
	}

	return nil
}

func convertSliceOfStringsToMap(input []string) map[string]struct{} {
	output := make(map[string]struct{})
	for _, v := range input {
		output[v] = struct{}{}
	}
	return output
}

func waitForContainer(contID string, args ...string) error {
	args = append([]string{"run", "--name", contID}, args...)
	cmd := exec.Command(dockerBinary, args...)
	if _, err := runCommand(cmd); err != nil {
		return err
	}

	if err := waitRun(contID); err != nil {
		return err
	}

	return nil
}

func waitRun(contID string) error {
	after := time.After(5 * time.Second)

	for {
		cmd := exec.Command(dockerBinary, "inspect", "-f", "{{.State.Running}}", contID)
		out, _, err := runCommandWithOutput(cmd)
		if err != nil {
			return fmt.Errorf("error executing docker inspect: %v", err)
		}

		if strings.Contains(out, "true") {
			break
		}

		select {
		case <-after:
			return fmt.Errorf("container did not come up in time")
		default:
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func compareDirectoryEntries(e1 []os.FileInfo, e2 []os.FileInfo) error {
	var (
		e1Entries = make(map[string]struct{})
		e2Entries = make(map[string]struct{})
	)
	for _, e := range e1 {
		e1Entries[e.Name()] = struct{}{}
	}
	for _, e := range e2 {
		e2Entries[e.Name()] = struct{}{}
	}
	if !reflect.DeepEqual(e1Entries, e2Entries) {
		return fmt.Errorf("entries differ")
	}
	return nil
}

func ListTar(f io.Reader) ([]string, error) {
	tr := tar.NewReader(f)
	var entries []string

	for {
		th, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			return entries, nil
		}
		if err != nil {
			return entries, err
		}
		entries = append(entries, th.Name)
	}
}

type FileServer struct {
	*httptest.Server
}

func fileServer(files map[string]string) (*FileServer, error) {
	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		if filePath, found := files[r.URL.Path]; found {
			http.ServeFile(w, r, filePath)
		} else {
			http.Error(w, http.StatusText(404), 404)
		}
	}

	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			return nil, err
		}
	}
	server := httptest.NewServer(handler)
	return &FileServer{
		Server: server,
	}, nil
}

func copyWithCP(source, target string) error {
	copyCmd := exec.Command("cp", "-rp", source, target)
	out, exitCode, err := runCommandWithOutput(copyCmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to copy: error: %q ,output: %q", err, out)
	}
	return nil
}

func makeRandomString(n int) string {
	// make a really long string
	letters := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// Reads chunkSize bytes from reader after every interval.
// Returns total read bytes.
func consumeWithSpeed(reader io.Reader, chunkSize int, interval time.Duration, stop chan bool) (n int, err error) {
	buffer := make([]byte, chunkSize)
	for {
		select {
		case <-stop:
			return
		default:
			var readBytes int
			readBytes, err = reader.Read(buffer)
			n += readBytes
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				return
			}
			time.Sleep(interval)
		}
	}
}
