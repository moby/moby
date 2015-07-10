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
	procCreateComputeSystem             = "CreateComputeSystem"
	procStartComputeSystem              = "StartComputeSystem"
	procCreateProcessInComputeSystem    = "CreateProcessInComputeSystem"
	procWaitForProcessInComputeSystem   = "WaitForProcessInComputeSystem"
	procShutdownComputeSystem           = "ShutdownComputeSystem"
	procTerminateComputeSystem          = "TerminateComputeSystem"
	procTerminateProcessInComputeSystem = "TerminateProcessInComputeSystem"
	procResizeConsoleInComputeSystem    = "ResizeConsoleInComputeSystem"

	// Storage related functions in the shim DLL
	procLayerExists        = "LayerExists"
	procCreateLayer        = "CreateLayer"
	procDestroyLayer       = "DestroyLayer"
	procActivateLayer      = "ActivateLayer"
	procDeactivateLayer    = "DeactivateLayer"
	procGetLayerMountPath  = "GetLayerMountPath"
	procCopyLayer          = "CopyLayer"
	procCreateSandboxLayer = "CreateSandboxLayer"
	procPrepareLayer       = "PrepareLayer"
	procUnprepareLayer     = "UnprepareLayer"
	procExportLayer        = "ExportLayer"
	procImportLayer        = "ImportLayer"
)

// loadAndFind finds a procedure in the DLL. Note we do NOT do lazy loading as
// go is particularly unfriendly in the case of a mismatch. By that - it panics
// if a function can't be found. By explicitly loading, we can control error
// handling gracefully without the daemon terminating.
func loadAndFind(procedure string) (dll *syscall.DLL, proc *syscall.Proc, err error) {

	logrus.Debugf("hcsshim::loadAndFind %s", procedure)

	dll, err = syscall.LoadDLL(shimDLLName)
	if err != nil {
		err = fmt.Errorf("Failed to load %s - error %s", shimDLLName, err)
		logrus.Error(err)
		return nil, nil, err
	}

	proc, err = dll.FindProc(procedure)
	if err != nil {
		err = fmt.Errorf("Failed to find %s in %s", procedure, shimDLLName)
		logrus.Error(err)
		return nil, nil, err
	}

	return dll, proc, nil
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
