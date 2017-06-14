// +build windows

package opengcs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// DefaultSandboxSizeMB is the size of the default sandbox size in MB
const DefaultSandboxSizeMB = 20 * 1024 * 1024

var (
	modkernel32   = syscall.NewLazyDLL("kernel32.dll")
	procCopyFileW = modkernel32.NewProc("CopyFileW")
	cacheLock     sync.Mutex
)

// copyFile is a utility for copying a file - used for the sandbox cache.
// Uses CopyFileW win32 API for performance
func copyFile(srcFile, destFile string) error {
	var bFailIfExists uint32 = 1

	lpExistingFileName, err := syscall.UTF16PtrFromString(srcFile)
	if err != nil {
		return err
	}
	lpNewFileName, err := syscall.UTF16PtrFromString(destFile)
	if err != nil {
		return err
	}
	r1, _, err := syscall.Syscall(
		procCopyFileW.Addr(),
		3,
		uintptr(unsafe.Pointer(lpExistingFileName)),
		uintptr(unsafe.Pointer(lpNewFileName)),
		uintptr(bFailIfExists))
	if r1 == 0 {
		return fmt.Errorf("failed CopyFileW Win32 call from '%s' to %s: %s", srcFile, destFile, err)
	}
	return nil

}

// CreateSandbox does what it says on the tin. This is done by copying a prebuilt-sandbox from the ServiceVM
// TODO: @jhowardmsft maxSizeInMB isn't hooked up in GCS. Needs a platform change which is in flight.
func CreateSandbox(uvm hcsshim.Container, destFile string, maxSizeInMB uint32, cacheFile string) error {
	// Smallest we can accept is the default sandbox size as we can't size down, only expand.
	if maxSizeInMB < DefaultSandboxSizeMB {
		maxSizeInMB = DefaultSandboxSizeMB
	}

	logrus.Debugf("opengcs: CreateSandbox: %s size:%dMB cache:%s", destFile, maxSizeInMB, cacheFile)

	// Retrieve from cache if the default size and already on disk
	if maxSizeInMB == DefaultSandboxSizeMB {
		cacheLock.Lock()
		if _, err := os.Stat(cacheFile); err == nil {
			if err := copyFile(cacheFile, destFile); err != nil {
				cacheLock.Unlock()
				return fmt.Errorf("opengcs: CreateSandbox: Failed to copy cached sandbox '%s' to '%s': %s", cacheFile, destFile, err)
			}
			cacheLock.Unlock()
			logrus.Debugf("opengcs: CreateSandbox: %s fulfilled from cache", destFile)
			return nil
		}
		cacheLock.Unlock()
	}

	if uvm == nil {
		return fmt.Errorf("opengcs: CreateSandbox: No utility VM was supplied")
	}

	process, err := createUtilsProcess(uvm)
	if err != nil {
		return fmt.Errorf("opengcs: CreateSandbox: %s: failed to create utils process: %s", destFile, err)
	}

	defer func() {
		//process.Process.Kill() // TODO @jhowardmsft - Add a timeout?
		process.Process.Close()
	}()

	// Prepare payload data for CreateSandboxCmd command
	hdr := &protocolCommandHeader{
		Command:     cmdCreateSandbox,
		Version:     version1,
		PayloadSize: sandboxInfoHeaderSize,
	}

	hdrSandboxInfo := &sandboxInfoHeader{maxSandboxSizeInMB: maxSizeInMB}

	// Send protocolCommandHeader and SandboxInfoHeader to the Service VM
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.BigEndian, hdr); err != nil {
		return fmt.Errorf("opengcs: CreateSandbox: %s: failed sending protocol header to utility VM: %s", destFile, err)
	}
	if err := binary.Write(buf, binary.BigEndian, hdrSandboxInfo); err != nil {
		return fmt.Errorf("opengcs: CreateSandbox: %s: failed sending sandbox info header to utility VM: %s", destFile, err)
	}

	logrus.Debugf("opengcs: CreateSandbox: %s: Writing %d bytes to utility VM", destFile, buf.Bytes())
	if _, err = process.Stdin.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("opengcs: CreateSandbox: %s: %dMB: failed to send %d bytes to utility VM: %s", destFile, maxSizeInMB, buf.Bytes(), err)
	}

	logrus.Debugf("opengcs: CreateSandbox: %s: waiting for utility VM to respond", destFile)
	resultSize, err := waitForResponse(process.Stdout)
	if err != nil {
		return fmt.Errorf("opengcs: CreateSandbox: %s: failed waiting for a response from utility VM: %s", destFile, err)
	}

	logrus.Debugf("opengcs: CreateSandbox: %s: writing %d bytes", destFile, resultSize)
	// Get back the sandbox VHDx stream from the service VM and write it to file
	err = writeFileFromReader(destFile, resultSize, process.Stdout)
	if err != nil {
		return fmt.Errorf("opengcs: CreateSandbox: %s: failed writing %d bytes to target file: %s", destFile, resultSize, err)
	}

	// Populate the cache
	if maxSizeInMB == DefaultSandboxSizeMB {
		cacheLock.Lock()
		if err := copyFile(destFile, cacheFile); err != nil {
			cacheLock.Unlock()
			return fmt.Errorf("opengcs: CreateSandbox: Failed to seed sandbox cache '%s' from '%s': %s", destFile, cacheFile, err)
		}
		cacheLock.Unlock()
	}

	logrus.Debugf("opengcs: CreateSandbox: %s created (non-cache)", destFile)
	return nil
}
