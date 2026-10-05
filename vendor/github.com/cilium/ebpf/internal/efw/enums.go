//go:build windows

package efw

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

/*
Converts an attach type enum into a GUID.

	ebpf_result_t ebpf_get_ebpf_attach_type(
		bpf_attach_type_t bpf_attach_type,
		_Out_ ebpf_attach_type_t* ebpf_attach_type_t *ebpf_attach_type)
*/
var ebpfGetEbpfAttachTypeProc = newProc("ebpf_get_ebpf_attach_type")

func EbpfGetEbpfAttachType(attachType uint32) (windows.GUID, error) {
	addr, err := ebpfGetEbpfAttachTypeProc.Find()
	if err != nil {
		return windows.GUID{}, err
	}

	var attachTypeGUID windows.GUID
	err = errorResult(syscall.SyscallN(addr,
		uintptr(attachType),
		uintptr(unsafe.Pointer(&attachTypeGUID)),
	))
	return attachTypeGUID, err
}

/*
Retrieve a program type given a GUID.

	bpf_prog_type_t ebpf_get_bpf_program_type(_In_ const ebpf_program_type_t* program_type)
*/
var ebpfGetBpfProgramTypeProc = newProc("ebpf_get_bpf_program_type")

func EbpfGetBpfProgramType(programType windows.GUID) (uint32, error) {
	addr, err := ebpfGetBpfProgramTypeProc.Find()
	if err != nil {
		return 0, err
	}

	return uint32Result(syscall.SyscallN(addr, uintptr(unsafe.Pointer(&programType)))), nil
}

/*
Retrieve an attach type given a GUID.

	bpf_attach_type_t ebpf_get_bpf_attach_type(_In_ const ebpf_attach_type_t* ebpf_attach_type)
*/
var ebpfGetBpfAttachTypeProc = newProc("ebpf_get_bpf_attach_type")

func EbpfGetBpfAttachType(attachType windows.GUID) (uint32, error) {
	addr, err := ebpfGetBpfAttachTypeProc.Find()
	if err != nil {
		return 0, err
	}

	return uint32Result(syscall.SyscallN(addr, uintptr(unsafe.Pointer(&attachType)))), nil
}
