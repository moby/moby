//+build linux

package term // import "github.com/docker/docker/pkg/term"

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gotestyourself/gotestyourself/assert"
)

// RequiresRoot skips tests that require root, unless the test.root flag has
// been set
func RequiresRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root")
		return
	}
}

func newTtyForTest(t *testing.T) (*os.File, error) {
	RequiresRoot(t)
	return os.OpenFile("/dev/tty", os.O_RDWR, os.ModeDevice)
}

func newTempFile() (*os.File, error) {
	return ioutil.TempFile(os.TempDir(), "temp")
}

func TestGetWinsize(t *testing.T) {
	tty, err := newTtyForTest(t)
	defer tty.Close()
	assert.NilError(t, err)
	winSize, err := GetWinsize(tty.Fd())
	assert.NilError(t, err)
	assert.Assert(t, winSize != nil)

	newSize := Winsize{Width: 200, Height: 200, x: winSize.x, y: winSize.y}
	err = SetWinsize(tty.Fd(), &newSize)
	assert.NilError(t, err)
	winSize, err = GetWinsize(tty.Fd())
	assert.NilError(t, err)
	assert.DeepEqual(t, *winSize, newSize, cmpWinsize)
}

var cmpWinsize = cmp.AllowUnexported(Winsize{})

func TestSetWinsize(t *testing.T) {
	tty, err := newTtyForTest(t)
	defer tty.Close()
	assert.NilError(t, err)
	winSize, err := GetWinsize(tty.Fd())
	assert.NilError(t, err)
	assert.Assert(t, winSize != nil)
	newSize := Winsize{Width: 200, Height: 200, x: winSize.x, y: winSize.y}
	err = SetWinsize(tty.Fd(), &newSize)
	assert.NilError(t, err)
	winSize, err = GetWinsize(tty.Fd())
	assert.NilError(t, err)
	assert.DeepEqual(t, *winSize, newSize, cmpWinsize)
}

func TestGetFdInfo(t *testing.T) {
	tty, err := newTtyForTest(t)
	defer tty.Close()
	assert.NilError(t, err)
	inFd, isTerminal := GetFdInfo(tty)
	assert.Equal(t, inFd, tty.Fd())
	assert.Equal(t, isTerminal, true)
	tmpFile, err := newTempFile()
	assert.NilError(t, err)
	defer tmpFile.Close()
	inFd, isTerminal = GetFdInfo(tmpFile)
	assert.Equal(t, inFd, tmpFile.Fd())
	assert.Equal(t, isTerminal, false)
}

func TestIsTerminal(t *testing.T) {
	tty, err := newTtyForTest(t)
	defer tty.Close()
	assert.NilError(t, err)
	isTerminal := IsTerminal(tty.Fd())
	assert.Equal(t, isTerminal, true)
	tmpFile, err := newTempFile()
	assert.NilError(t, err)
	defer tmpFile.Close()
	isTerminal = IsTerminal(tmpFile.Fd())
	assert.Equal(t, isTerminal, false)
}

func TestSaveState(t *testing.T) {
	tty, err := newTtyForTest(t)
	defer tty.Close()
	assert.NilError(t, err)
	state, err := SaveState(tty.Fd())
	assert.NilError(t, err)
	assert.Assert(t, state != nil)
	tty, err = newTtyForTest(t)
	assert.NilError(t, err)
	defer tty.Close()
	err = RestoreTerminal(tty.Fd(), state)
	assert.NilError(t, err)
}

func TestDisableEcho(t *testing.T) {
	tty, err := newTtyForTest(t)
	defer tty.Close()
	assert.NilError(t, err)
	state, err := SetRawTerminal(tty.Fd())
	defer RestoreTerminal(tty.Fd(), state)
	assert.NilError(t, err)
	assert.Assert(t, state != nil)
	err = DisableEcho(tty.Fd(), state)
	assert.NilError(t, err)
}
