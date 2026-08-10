//go:build windows
// +build windows

package format

import "github.com/Microsoft/go-winio/pkg/guid"

const (
	RegionFileName   = "region"
	ObjectIDFileName = "objectid"
)

// Magic specifies the magic number at the beginning of a file.
type Magic [8]uint8

var MagicValue = Magic([8]uint8{'c', 'i', 'm', 'f', 'i', 'l', 'e', '0'})

type Version struct {
	Major, Minor uint32
}

var CurrentVersion = Version{3, 0}

var MinSupportedVersion = Version{2, 0}

type FileType uint8

// RegionOffset encodes an offset to objects as index of the region file
// containing the object and the byte offset within that file.
type RegionOffset uint64

// CommonHeader is the common header for all CIM-related files.
type CommonHeader struct {
	Magic        Magic
	HeaderLength uint32
	Type         FileType
	Reserved     uint8
	Reserved2    uint16
	Version      Version
	Reserved3    uint64
}

type RegionSet struct {
	ID        guid.GUID
	Count     uint16
	Reserved  uint16
	Reserved1 uint32
}

// FilesystemHeader is the header for a filesystem file.
//
// The filesystem file points to the filesystem object inside a region
// file and specifies regions sets.
type FilesystemHeader struct {
	Common           CommonHeader
	Regions          RegionSet
	FilesystemOffset RegionOffset
	Reserved         uint32
	Reserved1        uint16
	ParentCount      uint16
}
