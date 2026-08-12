package launcher

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestProcessLifetimeKillsProcessWhenClosed(t *testing.T) {
	if os.Getenv("MOBY_TEST_PROCESS_LIFETIME_HELPER") == "1" {
		time.Sleep(time.Hour)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestProcessLifetimeKillsProcessWhenClosed")
	cmd.Env = append(os.Environ(), "MOBY_TEST_PROCESS_LIFETIME_HELPER=1")
	lifetime, err := startProcess(cmd)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	assert.NilError(t, lifetime.Close())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("extension process survived closing its lifetime")
	}
}
