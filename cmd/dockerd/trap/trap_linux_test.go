//go:build linux

package trap // import "github.com/docker/docker/cmd/dockerd/trap"

import (
	"os"
	"os/exec"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func buildTestBinary(t *testing.T, prefix string) string {
	t.Helper()
	exePath := t.TempDir() + "/" + prefix
	wd, _ := os.Getwd()
	testHelperCode := wd + "/testfiles/main.go"
	cmd := exec.Command("go", "build", "-o", exePath, testHelperCode)
	assert.NilError(t, cmd.Run())
	return exePath
}

func TestTrap(t *testing.T) {
	sigmap := []struct {
		name     string
		signal   os.Signal
		multiple bool
	}{
		{"TERM", syscall.SIGTERM, false},
		{"INT", os.Interrupt, false},
		{"TERM", syscall.SIGTERM, true},
		{"INT", os.Interrupt, true},
	}
	exePath := buildTestBinary(t, "main")

	for _, v := range sigmap {
		t.Run(v.name, func(t *testing.T) {
			cmd := exec.Command(exePath)
			cmd.Env = append(os.Environ(), "SIGNAL_TYPE="+v.name)
			if v.multiple {
				cmd.Env = append(cmd.Env, "IF_MULTIPLE=1")
			}
			err := cmd.Start()
			assert.NilError(t, err)
			err = cmd.Wait()
			e, ok := err.(*exec.ExitError)
			assert.Assert(t, ok, "expected exec.ExitError, got %T", e)

			code := e.Sys().(syscall.WaitStatus).ExitStatus()
			if v.multiple {
				assert.Check(t, is.DeepEqual(128+int(v.signal.(syscall.Signal)), code))
			} else {
				assert.Check(t, is.Equal(99, code))
			}
		})
	}
}
