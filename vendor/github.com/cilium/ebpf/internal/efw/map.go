//go:build windows

package efw

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

/*
ebpf_ring_buffer_map_map_buffer(

	fd_t map_fd,
	_Outptr_result_maybenull_ void** consumer,
	_Outptr_result_maybenull_ const void** producer,
	_Outptr_result_buffer_maybenull_(*data_size) const uint8_t** data,
	_Out_ size_t* data_size) EBPF_NO_EXCEPT;
*/
var ebpfRingBufferMapMapBufferProc = newProc("ebpf_ring_buffer_map_map_buffer")

func EbpfRingBufferMapMapBuffer(mapFd int) (consumer, producer, data *uint8, dataLen Size, _ error) {
	addr, err := ebpfRingBufferMapMapBufferProc.Find()
	if err != nil {
		return nil, nil, nil, 0, err
	}

	err = errorResult(syscall.SyscallN(addr,
		uintptr(mapFd),
		uintptr(unsafe.Pointer(&consumer)),
		uintptr(unsafe.Pointer(&producer)),
		uintptr(unsafe.Pointer(&data)),
		uintptr(unsafe.Pointer(&dataLen)),
	))
	if err != nil {
		return nil, nil, nil, 0, err
	}

	return consumer, producer, data, dataLen, nil
}

/*
ebpf_ring_buffer_map_unmap_buffer(

	fd_t map_fd, _In_ void* consumer, _In_ const void* producer, _In_ const void* data) EBPF_NO_EXCEPT;
*/
var ebpfRingBufferMapUnmapBufferProc = newProc("ebpf_ring_buffer_map_unmap_buffer")

func EbpfRingBufferMapUnmapBuffer(mapFd int, consumer, producer, data *uint8) error {
	addr, err := ebpfRingBufferMapUnmapBufferProc.Find()
	if err != nil {
		return err
	}

	return errorResult(syscall.SyscallN(addr,
		uintptr(mapFd),
		uintptr(unsafe.Pointer(consumer)),
		uintptr(unsafe.Pointer(producer)),
		uintptr(unsafe.Pointer(data)),
	))
}

/*
ebpf_result_t ebpf_map_set_wait_handle(

	fd_t map_fd,
	uint64_t index,
	ebpf_handle_t handle)
*/
var ebpfMapSetWaitHandleProc = newProc("ebpf_map_set_wait_handle")

func EbpfMapSetWaitHandle(mapFd int, index uint64, handle windows.Handle) error {
	addr, err := ebpfMapSetWaitHandleProc.Find()
	if err != nil {
		return err
	}

	return errorResult(syscall.SyscallN(addr,
		uintptr(mapFd),
		uintptr(index),
		uintptr(handle),
	))
}

/*
ebpf_result_t ebpf_ring_buffer_map_write(

	fd_t ring_buffer_map_fd,
	const void* data,
	size_t data_length)
*/
var ebpfRingBufferMapWriteProc = newProc("ebpf_ring_buffer_map_write")

func EbpfRingBufferMapWrite(ringBufferMapFd int, data []byte) error {
	addr, err := ebpfRingBufferMapWriteProc.Find()
	if err != nil {
		return err
	}

	err = errorResult(syscall.SyscallN(addr,
		uintptr(ringBufferMapFd),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
	))
	runtime.KeepAlive(data)
	return err
}
