// Shim for the Host Compute Service (HSC) to manage Windows Server
// containers and Hyper-V containers.

package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"
)

//go:generate go run mksyscall_windows.go -output zhcsshim.go hcsshim.go

//sys coTaskMemFree(buffer unsafe.Pointer) = ole32.CoTaskMemFree

//sys activateLayer(info *driverInfo, id string) (hr error) = vmcompute.ActivateLayer?
//sys copyLayer(info *driverInfo, srcId string, dstId string, descriptors []WC_LAYER_DESCRIPTOR) (hr error) = vmcompute.CopyLayer?
//sys createLayer(info *driverInfo, id string, parent string) (hr error) = vmcompute.CreateLayer?
//sys createSandboxLayer(info *driverInfo, id string, parent string, descriptors []WC_LAYER_DESCRIPTOR) (hr error) = vmcompute.CreateSandboxLayer?
//sys deactivateLayer(info *driverInfo, id string) (hr error) = vmcompute.DeactivateLayer?
//sys destroyLayer(info *driverInfo, id string) (hr error) = vmcompute.DestroyLayer?
//sys exportLayer(info *driverInfo, id string, path string, descriptors []WC_LAYER_DESCRIPTOR) (hr error) = vmcompute.ExportLayer?
//sys getLayerMountPath(info *driverInfo, id string, length *uintptr, buffer *uint16) (hr error) = vmcompute.GetLayerMountPath?
//sys getBaseImages(buffer **uint16) (hr error) = vmcompute.GetBaseImages?
//sys importLayer(info *driverInfo, id string, path string, descriptors []WC_LAYER_DESCRIPTOR) (hr error) = vmcompute.ImportLayer?
//sys layerExists(info *driverInfo, id string, exists *uint32) (hr error) = vmcompute.LayerExists?
//sys nameToGuid(name string, guid *GUID) (hr error) = vmcompute.NameToGuid?
//sys prepareLayer(info *driverInfo, id string, descriptors []WC_LAYER_DESCRIPTOR) (hr error) = vmcompute.PrepareLayer?
//sys unprepareLayer(info *driverInfo, id string) (hr error) = vmcompute.UnprepareLayer?

//sys importLayerBegin(info *driverInfo, id string, descriptors []WC_LAYER_DESCRIPTOR, context *uintptr) (hr error) = vmcompute.ImportLayerBegin?
//sys importLayerNext(context uintptr, fileName string, fileInfo *winio.FileBasicInfo) (hr error) = vmcompute.ImportLayerNext?
//sys importLayerWrite(context uintptr, buffer []byte) (hr error) = vmcompute.ImportLayerWrite?
//sys importLayerEnd(context uintptr) (hr error) = vmcompute.ImportLayerEnd?

//sys exportLayerBegin(info *driverInfo, id string, descriptors []WC_LAYER_DESCRIPTOR, context *uintptr) (hr error) = vmcompute.ExportLayerBegin?
//sys exportLayerNext(context uintptr, fileName **uint16, fileInfo *winio.FileBasicInfo, fileSize *int64, deleted *uint32) (hr error) = vmcompute.ExportLayerNext?
//sys exportLayerRead(context uintptr, buffer []byte, bytesRead *uint32) (hr error) = vmcompute.ExportLayerRead?
//sys exportLayerEnd(context uintptr) (hr error) = vmcompute.ExportLayerEnd?

//sys createComputeSystem(id string, configuration string) (hr error) = vmcompute.CreateComputeSystem?
//sys createProcessWithStdHandlesInComputeSystem(id string, paramsJson string, pid *uint32, stdin *syscall.Handle, stdout *syscall.Handle, stderr *syscall.Handle) (hr error) = vmcompute.CreateProcessWithStdHandlesInComputeSystem?
//sys resizeConsoleInComputeSystem(id string, pid uint32, height uint16, width uint16, flags uint32) (hr error) = vmcompute.ResizeConsoleInComputeSystem?
//sys shutdownComputeSystem(id string, timeout uint32) (hr error) = vmcompute.ShutdownComputeSystem?
//sys startComputeSystem(id string) (hr error) = vmcompute.StartComputeSystem?
//sys terminateComputeSystem(id string) (hr error) = vmcompute.TerminateComputeSystem?
//sys terminateProcessInComputeSystem(id string, pid uint32) (hr error) = vmcompute.TerminateProcessInComputeSystem?
//sys waitForProcessInComputeSystem(id string, pid uint32, timeout uint32, exitCode *uint32) (hr error) = vmcompute.WaitForProcessInComputeSystem?

//sys _hnsCall(method string, path string, object string, response **uint16) (hr error) = vmcompute.HNSCall?

const (
	// Specific user-visible exit codes
	WaitErrExecFailed = 32767

	ERROR_GEN_FAILURE          = syscall.Errno(31)
	ERROR_SHUTDOWN_IN_PROGRESS = syscall.Errno(1115)
	WSAEINVAL                  = syscall.Errno(10022)

	// Timeout on wait calls
	TimeoutInfinite = 0xFFFFFFFF
)

type HcsError struct {
	title string
	rest  string
	Err   error
}

func makeError(err error, title, rest string) error {
	// Pass through DLL errors directly since they do not originate from HCS.
	if _, ok := err.(*syscall.DLLError); ok {
		return err
	}
	return &HcsError{title, rest, err}
}

func makeErrorf(err error, title, format string, a ...interface{}) error {
	return makeError(err, title, fmt.Sprintf(format, a...))
}

func win32FromError(err error) uint32 {
	if herr, ok := err.(*HcsError); ok {
		return win32FromError(herr.Err)
	}
	if code, ok := err.(syscall.Errno); ok {
		return uint32(code)
	}
	return uint32(ERROR_GEN_FAILURE)
}

func win32FromHresult(hr uintptr) uintptr {
	if hr&0x1fff0000 == 0x00070000 {
		return hr & 0xffff
	}
	return hr
}

func (e *HcsError) Error() string {
	s := e.title
	if len(s) > 0 && s[len(s)-1] != ' ' {
		s += " "
	}
	s += fmt.Sprintf("failed in Win32: %s (0x%x)", e.Err, win32FromError(e.Err))
	if e.rest != "" {
		if e.rest[0] != ' ' {
			s += " "
		}
		s += e.rest
	}
	return s
}

func convertAndFreeCoTaskMemString(buffer *uint16) string {
	str := syscall.UTF16ToString((*[1 << 30]uint16)(unsafe.Pointer(buffer))[:])
	coTaskMemFree(unsafe.Pointer(buffer))
	return str
}
