//go:build windows

package efw

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// https://github.com/microsoft/ebpf-for-windows/blob/9d9003c39c3fd75be5225ac0fce30077d6bf0604/include/ebpf_core_structs.h#L15
const _EBPF_MAX_PIN_PATH_LENGTH = 256

/*
Retrieve object info and type from a fd.

	ebpf_result_t ebpf_object_get_info_by_fd(
		fd_t bpf_fd,
		_Inout_updates_bytes_to_opt_(*info_size, *info_size) void* info,
		_Inout_opt_ uint32_t* info_size,
		_Out_opt_ ebpf_object_type_t* type)
*/
var ebpfObjectGetInfoByFdProc = newProc("ebpf_object_get_info_by_fd")

func EbpfObjectGetInfoByFd(fd int, info unsafe.Pointer, info_size *uint32) (ObjectType, error) {
	addr, err := ebpfObjectGetInfoByFdProc.Find()
	if err != nil {
		return 0, err
	}

	var objectType ObjectType
	err = errorResult(syscall.SyscallN(addr,
		uintptr(fd),
		uintptr(info),
		uintptr(unsafe.Pointer(info_size)),
		uintptr(unsafe.Pointer(&objectType)),
	))
	return objectType, err
}

// ebpf_result_t ebpf_object_unpin(_In_z_ const char* path)
var ebpfObjectUnpinProc = newProc("ebpf_object_unpin")

func EbpfObjectUnpin(path string) error {
	addr, err := ebpfObjectUnpinProc.Find()
	if err != nil {
		return err
	}

	pathBytes, err := windows.ByteSliceFromString(path)
	if err != nil {
		return err
	}

	return errorResult(syscall.SyscallN(addr, uintptr(unsafe.Pointer(&pathBytes[0]))))
}

/*
Retrieve the next pinned object path.

	ebpf_result_t ebpf_get_next_pinned_object_path(
		_In_opt_z_ const char* start_path,
		_Out_writes_z_(next_path_len) char* next_path,
		size_t next_path_len,
		_Inout_opt_ ebpf_object_type_t* type)
*/
var ebpfGetNextPinnedObjectPath = newProc("ebpf_get_next_pinned_object_path")

func EbpfGetNextPinnedObjectPath(startPath string, objectType ObjectType) (string, ObjectType, error) {
	addr, err := ebpfGetNextPinnedObjectPath.Find()
	if err != nil {
		return "", 0, err
	}

	ptr, err := windows.BytePtrFromString(startPath)
	if err != nil {
		return "", 0, err
	}

	tmp := make([]byte, _EBPF_MAX_PIN_PATH_LENGTH)
	err = errorResult(syscall.SyscallN(addr,
		uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(&tmp[0])),
		uintptr(len(tmp)),
		uintptr(unsafe.Pointer(&objectType)),
	))
	return windows.ByteSliceToString(tmp), objectType, err
}

/*
Canonicalize a path using filesystem canonicalization rules.

	_Must_inspect_result_ ebpf_result_t
		ebpf_canonicalize_pin_path(_Out_writes_(output_size) char* output, size_t output_size, _In_z_ const char* input)
*/
var ebpfCanonicalizePinPath = newProc("ebpf_canonicalize_pin_path")

func EbpfCanonicalizePinPath(input string) (string, error) {
	addr, err := ebpfCanonicalizePinPath.Find()
	if err != nil {
		return "", err
	}

	inputBytes, err := windows.ByteSliceFromString(input)
	if err != nil {
		return "", err
	}

	output := make([]byte, _EBPF_MAX_PIN_PATH_LENGTH)
	err = errorResult(syscall.SyscallN(addr,
		uintptr(unsafe.Pointer(&output[0])),
		uintptr(len(output)),
		uintptr(unsafe.Pointer(&inputBytes[0])),
	))
	return windows.ByteSliceToString(output), err
}
