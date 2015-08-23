// Shim for the Host Compute Service (HSC) to manage Windows Server
// containers and Hyper-V containers.

package hcsshim

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

const (
	// Name of the shim DLL for access to the HCS
	shimDLLName = "vmcompute.dll"

	// Container related functions in the shim DLL
	procCreateComputeSystem                        = "CreateComputeSystem"
	procStartComputeSystem                         = "StartComputeSystem"
	procCreateProcessWithStdHandlesInComputeSystem = "CreateProcessWithStdHandlesInComputeSystem"
	procWaitForProcessInComputeSystem              = "WaitForProcessInComputeSystem"
	procShutdownComputeSystem                      = "ShutdownComputeSystem"
	procTerminateComputeSystem                     = "TerminateComputeSystem"
	procTerminateProcessInComputeSystem            = "TerminateProcessInComputeSystem"
	procResizeConsoleInComputeSystem               = "ResizeConsoleInComputeSystem"

	// Storage related functions in the shim DLL
	procLayerExists         = "LayerExists"
	procCreateLayer         = "CreateLayer"
	procDestroyLayer        = "DestroyLayer"
	procActivateLayer       = "ActivateLayer"
	procDeactivateLayer     = "DeactivateLayer"
	procGetLayerMountPath   = "GetLayerMountPath"
	procCopyLayer           = "CopyLayer"
	procCreateSandboxLayer  = "CreateSandboxLayer"
	procPrepareLayer        = "PrepareLayer"
	procUnprepareLayer      = "UnprepareLayer"
	procExportLayer         = "ExportLayer"
	procImportLayer         = "ImportLayer"
	procGetSharedBaseImages = "GetBaseImages"
	procNameToGuid          = "NameToGuid"

	// Name of the standard OLE dll
	oleDLLName = "Ole32.dll"

	// Utility functions
	procCoTaskMemFree = "CoTaskMemFree"
)

// loadAndFindFromDll finds a procedure in the given DLL. Note we do NOT do lazy loading as
// go is particularly unfriendly in the case of a mismatch. By that - it panics
// if a function can't be found. By explicitly loading, we can control error
// handling gracefully without the daemon terminating.
func loadAndFindFromDll(dllName, procedure string) (dll *syscall.DLL, proc *syscall.Proc, err error) {
	logrus.Debugf("hcsshim::loadAndFindFromDll %s %s", dllName, procedure)

	dll, err = syscall.LoadDLL(dllName)
	if err != nil {
		err = fmt.Errorf("Failed to load %s - error %s", dllName, err)
		logrus.Error(err)
		return
	}

	proc, err = dll.FindProc(procedure)
	if err != nil {
		err = fmt.Errorf("Failed to find %s in %s", procedure, dllName)
		logrus.Error(err)
		return
	}

	return
}

// loadAndFind finds a procedure in the shim DLL.
func loadAndFind(procedure string) (*syscall.DLL, *syscall.Proc, error) {

	return loadAndFindFromDll(shimDLLName, procedure)
}

// use is a no-op, but the compiler cannot see that it is.
// Calling use(p) ensures that p is kept live until that point.
/*
//go:noescape
func use(p unsafe.Pointer)
*/

// Alternate without using //go:noescape and asm.s
var temp unsafe.Pointer

func use(p unsafe.Pointer) {
	temp = p
}
