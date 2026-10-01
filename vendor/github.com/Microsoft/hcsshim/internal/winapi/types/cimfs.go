//go:build windows

package types

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type FsHandle uintptr
type StreamHandle uintptr

type CimGetVerificationInfoFlags uint32

const (
	CimGetVerificationInfoNone            CimGetVerificationInfoFlags = 0
	CimGetVerificationInfoFromSkeletonCim CimGetVerificationInfoFlags = 1
)

type CimSignatureType uint32

const (
	CimSignatureNone  CimSignatureType = 0
	CimSignaturePKCS7 CimSignatureType = 1
)

type CimHashAlgorithm uint32

const (
	CimHashAlgorithmSHA256 CimHashAlgorithm = 0
	CimHashAlgorithmSHA512 CimHashAlgorithm = 1
)

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

type CimFsCimInfo struct {
	NextEntryOffset uint32
	CimNameLength   uint16
	CimName         [1]uint16
}
