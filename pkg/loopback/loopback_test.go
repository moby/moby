// +build linux,cgo

package loopback

import (
	"flag"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"testing"

	"fmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const filePath = "/tmp/"

var rootEnabled bool

func init() {
	flag.BoolVar(&rootEnabled, "test.root", false, "enable tests that require root")
}

func RequiresRoot(t *testing.T) {
	if !rootEnabled {
		t.Skip("skipping test that requires root")
		return
	}
	require.Equal(t, 0, os.Getuid(), "This test must be run as root.")
}

func createSparseFile() (string, error) {
	size := int64(10 * 1024 * 1024)
	fd, err := ioutil.TempFile(filePath, "sparseFile")
	if err != nil {
		return "", errors.Wrap(err, "Error creating file")
	}
	_, err = fd.Seek(size-1, 0)
	if err != nil {
		return "", errors.Wrap(err, "Failed to seek")
	}
	_, err = fd.Write([]byte{0})
	if err != nil {
		return "", errors.Wrap(err, "Write failed")
	}
	err = fd.Close()
	if err != nil {
		return "", errors.Wrap(err, "Close failed")
	}
	return fd.Name(), nil
}

func TestAttachLoopDevice(t *testing.T) {
	RequiresRoot(t)
	fd, err := AttachLoopDevice(filePath + "test.log")
	require.Equal(t, ErrAttachLoopbackDevice, err)

	fileName, err := createSparseFile()
	require.NoError(t, err, "Error creating sparse file")
	defer os.Remove(fileName)
	fd, err = os.Open(fileName)
	defer fd.Close()
	require.NoError(t, err, "Error opening file")

	loopDevice, err := AttachLoopDevice(fileName)
	require.NoError(t, err, "Error attaching loop device")

	if !strings.Contains(loopDevice.Name(), "/dev/loop") {
		t.Fatal("Invalid loop file")
	}

	stat, err := fd.Stat()

	targetINode := stat.Sys().(*syscall.Stat_t).Ino
	targetDevice := stat.Sys().(*syscall.Stat_t).Dev
	loopInfo, err := ioctlLoopGetStatus64(loopDevice.Fd())
	require.NoError(t, err, "Error get loopback backing file")

	if loopInfo.loDevice != targetDevice || loopInfo.loInode != targetINode {
		t.Fatal("File not properly attached")
	}
}

func TestFindLoopDeviceFor(t *testing.T) {
	RequiresRoot(t)
	fileName, err := createSparseFile()
	require.NoError(t, err, "Error creating sparse file")
	defer os.Remove(fileName)

	fd, err := os.Open(fileName)
	require.NoError(t, err, "Error opening file")
	defer fd.Close()

	loopDevice, err := AttachLoopDevice(fileName)
	require.NoError(t, err, "Error attaching loop device")

	confirmLoopDevice := FindLoopDeviceFor(fd)
	require.NotNil(t, confirmLoopDevice, "Cannot find Loop Device.")
	require.Equal(t, loopDevice.Name(), confirmLoopDevice.Name())
}

func TestSetCapacity(t *testing.T) {
	RequiresRoot(t)
	fileName, err := createSparseFile()
	require.NoError(t, err, "Error creating sparse file")
	defer os.Remove(fileName)

	count := 9
	_, err = os.Stat(fmt.Sprintf("/dev/loop%d", count))

	for os.IsNotExist(err) {
		count--
		if count < 0 {
			break
		}
		_, err = os.Stat(fmt.Sprintf("/dev/loop%d", count))

	}
	if !os.IsNotExist(err) {
		loopDevice, err := os.Open(fmt.Sprintf("/dev/loop%d", count))
		_, err = ioctlLoopGetStatus64(loopDevice.Fd())
		if err != nil {
			err = SetCapacity(loopDevice)
			assert.Equal(t, ErrSetCapacity, err)
		}
	}

	loopDevice, err := AttachLoopDevice(fileName)
	require.NoError(t, err, "Error attaching loop device")

	err = SetCapacity(loopDevice)
	require.NoError(t, err, "Could not set capacity.")
}
