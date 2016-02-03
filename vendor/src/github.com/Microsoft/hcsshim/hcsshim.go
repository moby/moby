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

	// Known Win32 RC values which should be trapped
	Win32PipeHasBeenEnded                 = 0x8007006d // WaitForProcessInComputeSystem: The pipe has been ended
	Win32SystemShutdownIsInProgress       = 0x8007045B // ShutdownComputeSystem: A system shutdown is in progress
	Win32SpecifiedPathInvalid             = 0x800700A1 // ShutdownComputeSystem: The specified path is invalid
	Win32SystemCannotFindThePathSpecified = 0x80070003 // ShutdownComputeSystem: The system cannot find the path specified
	Win32InvalidArgument                  = 0x80072726 // CreateProcessInComputeSystem: An invalid argument was supplied
	EFail                                 = 0x80004005

	// Timeout on wait calls
	TimeoutInfinite = 0xFFFFFFFF
)

type hcsError struct {
	title string
	rest  string
	err   error
}

type Win32Error interface {
	error
	HResult() uint32
}

func makeError(err error, title, rest string) Win32Error {
	return &hcsError{title, rest, err}
}

func makeErrorf(err error, title, format string, a ...interface{}) Win32Error {
	return makeError(err, title, fmt.Sprintf(format, a...))
}

func (e *hcsError) HResult() uint32 {
	if hr, ok := e.err.(syscall.Errno); ok {
		return uint32(hr)
	} else {
		return EFail
	}
}

func (e *hcsError) Error() string {
	return fmt.Sprintf("%s- Win32 API call returned error r1=0x%x err=%s%s", e.title, e.HResult(), e.err, e.rest)
}

func convertAndFreeCoTaskMemString(buffer *uint16) string {
	str := syscall.UTF16ToString((*[1 << 30]uint16)(unsafe.Pointer(buffer))[:])
	coTaskMemFree(unsafe.Pointer(buffer))
	return str
}
