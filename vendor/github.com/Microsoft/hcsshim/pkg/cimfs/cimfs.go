//go:build windows
// +build windows

package cimfs

import (
	"path/filepath"

	"github.com/Microsoft/hcsshim/osversion"
	"github.com/sirupsen/logrus"
)

func IsCimFSSupported() bool {
	rv, err := osversion.BuildRevision()
	if err != nil {
		logrus.WithError(err).Warn("get build revision")
	}
	build := osversion.Build()
	// CimFS support is backported to LTSC2022 starting with revision 2031 and should
	// otherwise be available on all builds >= V25H1Server
	return build >= osversion.V25H1Server || (build == osversion.V21H2Server && rv >= 2031)
}

// IsBlockCimSupported returns true if block formatted CIMs (i.e block device CIM &
// single file CIM) are supported on the current OS build.
func IsBlockCimSupported() bool {
	build := osversion.Build()
	// TODO(ambarve): Currently we are checking against a higher build number since there is no
	// official build with block CIM support yet. Once we have that build, we should
	// update the build number here.
	return build >= 27766
}

// IsVerifiedCimSupported returns true if block CIM format supports also writing verification information in the CIM.
func IsVerifiedCimSupported() bool {
	build := osversion.Build()
	// TODO(ambarve): Currently we are checking against a higher build number since there is no
	// official build with block CIM support yet. Once we have that build, we should
	// update the build number here.
	return build >= 27800
}

func IsMergedCimSupported() bool {
	// The merged CIM support was originally added before block CIM support.  However,
	// some of the merged CIM features that we use (e.g. merged hard links) were added
	// later along with block CIM support. So use the same check as block CIM here.
	return IsBlockCimSupported()
}

type BlockCIMType uint32

const (
	BlockCIMTypeNone BlockCIMType = iota
	BlockCIMTypeSingleFile
	BlockCIMTypeDevice

	CimMountFlagNone       uint32 = 0x0
	CimMountFlagEnableDax  uint32 = 0x2
	CimMountBlockDeviceCim uint32 = 0x10
	CimMountSingleFileCim  uint32 = 0x20
	CimMountVerifiedCim    uint32 = 0x80

	CimCreateFlagNone                uint32 = 0x0
	CimCreateFlagDoNotExpandPEImages uint32 = 0x1
	CimCreateFlagFixedSizeChunks     uint32 = 0x2
	CimCreateFlagBlockDeviceCim      uint32 = 0x4
	CimCreateFlagSingleFileCim       uint32 = 0x8
	CimCreateFlagConsistentCim       uint32 = 0x10
	CimCreateFlagVerifiedCim         uint32 = 0x40

	CimMergeFlagNone        uint32 = 0x0
	CimMergeFlagSingleFile  uint32 = 0x1
	CimMergeFlagBlockDevice uint32 = 0x2
	CimMergeFlagVerifiedCim uint32 = 0x4
)

// BlockCIM represents a CIM stored in a block formatted way.
//
// A CIM usually is made up of a .cim file and multiple region & objectID
// files. Currently, all of these files are stored together in the same directory. To
// refer to such a CIM, we provide the path to the `.cim` file and the corresponding
// region & objectID files are assumed to be present right next to it. In this case the
// directory on the host's filesystem which holds one or more such CIMs is the container
// for those CIMs.
//
// Using multiple files for a single CIM can be very limiting. (For example, if you want
// to do a remote mount for a CIM layer, you now need to mount multiple files for a single
// layer). In such cases having a single container which contains all of the CIM related
// data is a great option. For this reason, CimFS has added support for a new type of a
// CIM named BlockCIM. A BlockCIM is a CIM for which the container used to store all of
// the CIM files is a block device or a binary file formatted like a block device. Such a
// block device (or a binary file) doesn't have a separate filesystem (like NTFS or FAT32)
// on it. Instead it is formatted in such a way that CimFS driver can read the blocks and
// find out which CIMs are present on that block device. The CIMs stored on a raw block
// device are sometimes referred to as block device CIMs and CIMs stored on the block
// formatted single file are referred as single file CIMs.
type BlockCIM struct {
	Type BlockCIMType
	// BlockPath is a path to the block device or the single file which contains the
	// CIM.
	BlockPath string
	// Since a block device CIM or a single file CIM can container multiple CIMs, we
	// refer to an individual CIM using its name.
	CimName string
}

// added for logging convenience
func (b *BlockCIM) String() string {
	return filepath.Join(b.BlockPath, b.CimName)
}
