//go:build windows

package types

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type FsHandle uintptr
type StreamHandle uintptr

type CimFsFileMetadata struct {
	Attributes uint32
	FileSize   int64

	CreationTime   windows.Filetime
	LastWriteTime  windows.Filetime
	ChangeTime     windows.Filetime
	LastAccessTime windows.Filetime

	SecurityDescriptorBuffer unsafe.Pointer
	SecurityDescriptorSize   uint32

	ReparseDataBuffer unsafe.Pointer
	ReparseDataSize   uint32

	ExtendedAttributes unsafe.Pointer
	EACount            uint32
}

type CimFsImagePath struct {
	ImageDir  *uint16
	ImageName *uint16
}
