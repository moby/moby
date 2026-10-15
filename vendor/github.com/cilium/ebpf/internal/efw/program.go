//go:build windows

package efw

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

/*
Attach a program.

	ebpf_result_t ebpf_program_attach_by_fds(
		fd_t program_fd,
		_In_opt_ const ebpf_attach_type_t* attach_type,
		_In_reads_bytes_opt_(attach_parameters_size) void* attach_parameters,
		size_t attach_parameters_size,
		_Out_ fd_t* link)
*/
var ebpfProgramAttachByFdsProc = newProc("ebpf_program_attach_by_fds")

func EbpfProgramAttachFds(fd int, attachType windows.GUID, params unsafe.Pointer, params_size uintptr) (int, error) {
	addr, err := ebpfProgramAttachByFdsProc.Find()
	if err != nil {
		return 0, err
	}

	var link FD
	err = errorResult(syscall.SyscallN(addr,
		uintptr(fd),
		uintptr(unsafe.Pointer(&attachType)),
		uintptr(params),
		params_size,
		uintptr(unsafe.Pointer(&link)),
	))
	return int(link), err
}
