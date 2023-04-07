package platform // import "github.com/docker/docker/pkg/platform"

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32       = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemInfo = modkernel32.NewProc("GetSystemInfo")
)

// see https://learn.microsoft.com/en-gb/windows/win32/api/sysinfoapi/ns-sysinfoapi-system_info
type systeminfo struct {
	wProcessorArchitecture      uint16
	wReserved                   uint16
	dwPageSize                  uint32
	lpMinimumApplicationAddress uintptr
	lpMaximumApplicationAddress uintptr
	dwActiveProcessorMask       uintptr
	dwNumberOfProcessors        uint32
	dwProcessorType             uint32
	dwAllocationGranularity     uint32
	wProcessorLevel             uint16
	wProcessorRevision          uint16
}

// Windows processor architectures.
//
// see https://github.com/microsoft/go-winio/blob/v0.6.0/wim/wim.go#L48-L65
// see https://learn.microsoft.com/en-gb/windows/win32/api/sysinfoapi/ns-sysinfoapi-system_info
const (
	ProcessorArchitecture64    = 9  // PROCESSOR_ARCHITECTURE_AMD64
	ProcessorArchitectureIA64  = 6  // PROCESSOR_ARCHITECTURE_IA64
	ProcessorArchitecture32    = 0  // PROCESSOR_ARCHITECTURE_INTEL
	ProcessorArchitectureArm   = 5  // PROCESSOR_ARCHITECTURE_ARM
	ProcessorArchitectureArm64 = 12 // PROCESSOR_ARCHITECTURE_ARM64
)

// runtimeArchitecture gets the name of the current architecture (x86, x86_64, …)
func runtimeArchitecture() (string, error) {
	var sysinfo systeminfo
	_, _, _ = syscall.SyscallN(procGetSystemInfo.Addr(), uintptr(unsafe.Pointer(&sysinfo)))
	switch sysinfo.wProcessorArchitecture {
	case ProcessorArchitecture64, ProcessorArchitectureIA64:
		return "x86_64", nil
	case ProcessorArchitecture32:
		return "i686", nil
	case ProcessorArchitectureArm:
		return "arm", nil
	case ProcessorArchitectureArm64:
		return "arm64", nil
	default:
		return "", fmt.Errorf("unknown processor architecture %+v", sysinfo.wProcessorArchitecture)
	}
}

// NumProcs returns the number of processors on the system
func NumProcs() uint32 {
	var sysinfo systeminfo
	_, _, _ = syscall.SyscallN(procGetSystemInfo.Addr(), uintptr(unsafe.Pointer(&sysinfo)))
	return sysinfo.dwNumberOfProcessors
}
