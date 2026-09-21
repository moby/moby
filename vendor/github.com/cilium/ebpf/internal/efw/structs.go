//go:build windows

package efw

import "golang.org/x/sys/windows"

// https://github.com/microsoft/ebpf-for-windows/blob/95267a53b26c68a94145d1731e2a4c8b546034c3/include/ebpf_structs.h#L366
const _BPF_OBJ_NAME_LEN = 64

// See https://github.com/microsoft/ebpf-for-windows/blob/95267a53b26c68a94145d1731e2a4c8b546034c3/include/ebpf_structs.h#L372-L386
type BpfMapInfo struct {
	_    uint32                  ///< Map ID.
	_    uint32                  ///< Type of map.
	_    uint32                  ///< Size in bytes of a map key.
	_    uint32                  ///< Size in bytes of a map value.
	_    uint32                  ///< Maximum number of entries allowed in the map.
	Name [_BPF_OBJ_NAME_LEN]byte ///< Null-terminated map name.
	_    uint32                  ///< Map flags.

	_ uint32 ///< ID of inner map template.
	_ uint32 ///< Number of pinned paths.
}

// See https://github.com/microsoft/ebpf-for-windows/blob/95267a53b26c68a94145d1731e2a4c8b546034c3/include/ebpf_structs.h#L396-L410
type BpfProgInfo struct {
	_    uint32                  ///< Program ID.
	_    uint32                  ///< Program type, if a cross-platform type.
	_    uint32                  ///< Number of maps associated with this program.
	_    uintptr                 ///< Pointer to caller-allocated array to fill map IDs into.
	Name [_BPF_OBJ_NAME_LEN]byte ///< Null-terminated map name.

	_ windows.GUID ///< Program type UUID.
	_ windows.GUID ///< Attach type UUID.
	_ uint32       ///< Number of pinned paths.
	_ uint32       ///< Number of attached links.
}
