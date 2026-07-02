package tar2ext4

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
)

// Constants for the VHD footer
const (
	cookieMagic            = "conectix"
	featureMask            = 0x2
	fileFormatVersionMagic = 0x00010000
	fixedDataOffset        = -1
	creatorVersionMagic    = 0x000a0000
	diskTypeFixed          = 2
)

type vhdFooter struct {
	Cookie             [8]byte
	Features           uint32
	FileFormatVersion  uint32
	DataOffset         int64
	TimeStamp          uint32
	CreatorApplication [4]byte
	CreatorVersion     uint32
	CreatorHostOS      [4]byte
	OriginalSize       int64
	CurrentSize        int64
	DiskGeometry       uint32
	DiskType           uint32
	Checksum           uint32
	UniqueID           [16]uint8
	SavedState         uint8
	Reserved           [427]uint8
}

func makeFixedVHDFooter(size int64) *vhdFooter {
	footer := &vhdFooter{
		Features:          featureMask,
		FileFormatVersion: fileFormatVersionMagic,
		DataOffset:        fixedDataOffset,
		CreatorVersion:    creatorVersionMagic,
		OriginalSize:      size,
		CurrentSize:       size,
		DiskType:          diskTypeFixed,
		UniqueID:          generateUUID(),
	}
	copy(footer.Cookie[:], cookieMagic)
	footer.Checksum = calculateCheckSum(footer)
	return footer
}

func calculateCheckSum(footer *vhdFooter) uint32 {
	oldchk := footer.Checksum
	footer.Checksum = 0

	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, footer)

	var chk uint32
	bufBytes := buf.Bytes()
	for i := 0; i < len(bufBytes); i++ {
		chk += uint32(bufBytes[i])
	}
	footer.Checksum = oldchk
	return uint32(^chk)
}

func generateUUID() [16]byte {
	res := [16]byte{}
	if _, err := rand.Read(res[:]); err != nil {
		panic(err)
	}
	return res
}
