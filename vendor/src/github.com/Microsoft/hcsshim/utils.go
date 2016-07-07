package hcsshim

import (
	"syscall"
)

var (
	vmcomputedll          = syscall.NewLazyDLL("vmcompute.dll")
	hcsCallbackAPI        = vmcomputedll.NewProc("HcsRegisterComputeSystemCallback")
	hcsCallbacksSupported = hcsCallbackAPI.Find() == nil
)
