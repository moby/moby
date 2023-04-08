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
	processorArchitecture64    = 9  // PROCESSOR_ARCHITECTURE_AMD64
	processorArchitectureIA64  = 6  // PROCESSOR_ARCHITECTURE_IA64
	processorArchitecture32    = 0  // PROCESSOR_ARCHITECTURE_INTEL
	processorArchitectureArm   = 5  // PROCESSOR_ARCHITECTURE_ARM
	processorArchitectureArm64 = 12 // PROCESSOR_ARCHITECTURE_ARM64
)

// runtimeArchitecture gets the name of the current architecture (x86, x86_64, …)
func runtimeArchitecture() (string, error) {
	// TODO(thaJeztah): rewrite this to use "GetNativeSystemInfo" instead.
	// See: https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-getsysteminfo
	// See: https://github.com/shirou/gopsutil/blob/v3.23.3/host/host_windows.go#L267-L297
	// > To retrieve accurate information for an application running on WOW64,
	// > call the GetNativeSystemInfo function.
	var sysinfo systeminfo
	_, _, _ = syscall.SyscallN(procGetSystemInfo.Addr(), uintptr(unsafe.Pointer(&sysinfo)))
	switch sysinfo.wProcessorArchitecture {
	case processorArchitecture64, processorArchitectureIA64:
		return "x86_64", nil
	case processorArchitecture32:
		return "i686", nil
	case processorArchitectureArm:
		return "arm", nil
	case processorArchitectureArm64:
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
