package reexec

import (
	"os"
	"os/exec"
	"testing"
)

const testReExec = "test-reexec"

func init() {
	Register(testReExec, func() {
		panic("Return Error")
	})
	Init()
}

func TestRegister(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			const expected = `reexec func already registered under name "test-reexec"`
			if r != expected {
				t.Errorf("got %q, want %q", r, expected)
			}
		}
	}()
	Register(testReExec, func() {})
}

func TestCommand(t *testing.T) {
	cmd := Command(testReExec)
	w, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Error on pipe creation: %v", err)
	}
	defer w.Close()

	err = cmd.Start()
	if err != nil {
		t.Fatalf("Error on re-exec cmd: %v", err)
	}
	err = cmd.Wait()
	const expected = "exit status 2"
	if err == nil || err.Error() != expected {
		t.Fatalf("got %v, want %v", err, expected)
	}
}

func TestNaiveSelf(t *testing.T) {
	if os.Getenv("TEST_CHECK") == "1" {
		os.Exit(2)
	}
	cmd := exec.Command(naiveSelf(), "-test.run=TestNaiveSelf")
	cmd.Env = append(os.Environ(), "TEST_CHECK=1")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Unable to start command: %v", err)
	}
	err = cmd.Wait()
	const expected = "exit status 2"
	if err == nil || err.Error() != expected {
		t.Fatalf("got %v, want %v", err, expected)
	}

	os.Args[0] = "mkdir"
	if naiveSelf() == os.Args[0] {
		t.Fatalf("Expected naiveSelf to resolve the location of mkdir")
	}
}
