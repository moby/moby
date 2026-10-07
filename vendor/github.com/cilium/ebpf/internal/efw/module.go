//go:build windows

// Package efw contains support code for eBPF for Windows.
package efw

import (
	"golang.org/x/sys/windows"
)

// module is the global handle for the eBPF for Windows user-space API.
var module = windows.NewLazyDLL("ebpfapi.dll")

// FD is the equivalent of fd_t.
//
// See https://github.com/microsoft/ebpf-for-windows/blob/54632eb360c560ebef2f173be1a4a4625d540744/include/ebpf_api.h#L24
type FD int32

// Size is the equivalent of size_t.
//
// This is correct on amd64 and arm64 according to tests on godbolt.org.
type Size uint64

// Int is the equivalent of int on MSVC (am64, arm64) and MinGW (gcc, clang).
type Int int32

// ObjectType is the equivalent of ebpf_object_type_t.
//
// See https://github.com/microsoft/ebpf-for-windows/blob/44f5de09ec0f3f7ad176c00a290c1cb7106cdd5e/include/ebpf_core_structs.h#L41
type ObjectType uint32

const (
	EBPF_OBJECT_UNKNOWN ObjectType = iota
	EBPF_OBJECT_MAP
	EBPF_OBJECT_LINK
	EBPF_OBJECT_PROGRAM
)
