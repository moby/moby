//go:build linux
// +build linux

package trap // import "github.com/docker/docker/cmd/dockerd/trap"

import (
	"os"
	"os/exec"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func buildTestBinary(t *testing.T, tmpdir string, prefix string) (string, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp(tmpdir, prefix)
	assert.NilError(t, err)
	exePath := tmpDir + "/" + prefix
	wd, _ := os.Getwd()
	testHelperCode := wd + "/testfiles/main.go"
	cmd := exec.Command("go", "build", "-o", exePath, testHelperCode)
	err = cmd.Run()
	assert.NilError(t, err)
	return exePath, tmpDir
}

func TestTrap(t *testing.T) {
	var sigmap = []struct {
		name     string
		signal   os.Signal
		multiple bool
	}{
		{"TERM", syscall.SIGTERM, false},
		{"QUIT", syscall.SIGQUIT, true},
		{"INT", os.Interrupt, false},
		{"TERM", syscall.SIGTERM, true},
		{"INT", os.Interrupt, true},
	}
	exePath, tmpDir := buildTestBinary(t, "", "main")
	defer os.RemoveAll(tmpDir)

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
