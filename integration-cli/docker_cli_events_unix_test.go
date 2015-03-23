// +build !windows

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"unicode"

	"github.com/kr/pty"
)

// #5979
func TestEventsRedirectStdout(t *testing.T) {
	since := daemonTime(t).Unix()
	dockerCmd(t, "run", "busybox", "true")
	defer deleteAllContainers()

	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("could not create temp file: %v", err)
	}
	defer os.Remove(file.Name())

	command := fmt.Sprintf("%s events --since=%d --until=%d > %s", dockerBinary, since, daemonTime(t).Unix(), file.Name())
	_, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("Could not open pty: %v", err)
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	if err := cmd.Run(); err != nil {
		t.Fatalf("run err for command %q: %v", command, err)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		for _, c := range scanner.Text() {
			if unicode.IsControl(c) {
				t.Fatalf("found control character %v", []byte(string(c)))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Scan err for command %q: %v", command, err)
	}

	logDone("events - redirect stdout")
}
