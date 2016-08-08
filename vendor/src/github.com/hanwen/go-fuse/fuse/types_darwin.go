package fuse

type Attr struct {
	Ino         uint64
	Size        uint64
	Blocks      uint64
	Atime       uint64
	Mtime       uint64
	Ctime       uint64
	Crtime_     uint64 // OS X
	Atimensec   uint32
	Mtimensec   uint32
	Ctimensec   uint32
	Crtimensec_ uint32 // OS X
	Mode        uint32
	Nlink       uint32
	Owner
	Rdev   uint32
	Flags_ uint32 //  OS X
}

const (
	FATTR_CRTIME   = (1 << 28)
	FATTR_CHGTIME  = (1 << 29)
	FATTR_BKUPTIME = (1 << 30)
	FATTR_FLAGS    = (1 << 31)
)

type SetAttrIn struct {
	SetAttrInCommon

	// OS X only
	Bkuptime_    uint64
	Chgtime_     uint64
	Crtime       uint64
	BkuptimeNsec uint32
	ChgtimeNsec  uint32
	CrtimeNsec   uint32
	Flags_       uint32 // see chflags(2)
}

const (
	FOPEN_PURGE_ATTR = (1 << 30)
	FOPEN_PURGE_UBC  = (1 << 31)
)

// compat with linux.
const (
	// Mask for GetAttrIn.Flags. If set, GetAttrIn has a file handle set.
	FUSE_GETATTR_FH = (1 << 0)
)

type GetAttrIn struct {
	InHeader
}

func (g *GetAttrIn) Flags() uint32 {
	return 0
}

func (g *GetAttrIn) Fh() uint64 {
	return 0
}

// Uses OpenIn struct for create.
type CreateIn struct {
	InHeader

	Flags uint32
	Mode  uint32
}

type MknodIn struct {
	InHeader

	Mode uint32
	Rdev uint32
}

type ReadIn struct {
	InHeader

	Fh        uint64
	Offset    uint64
	Size      uint32
	ReadFlags uint32
}

type WriteIn struct {
	InHeader
	Fh         uint64
	Offset     uint64
	Size       uint32
	WriteFlags uint32
}

type SetXAttrIn struct {
	InHeader
	Size     uint32
	Flags    uint32
	Position uint32
	Padding  uint32
}

type GetXAttrIn struct {
	InHeader
	Size     uint32
	Padding  uint32
	Position uint32
	Padding2 uint32
}

const (
	CAP_CASE_INSENSITIVE = (1 << 29)
	CAP_VOL_RENAME       = (1 << 30)
	CAP_XTIMES           = (1 << 31)
)

type GetxtimesOut struct {
	Bkuptime     uint64
	Crtime       uint64
	Bkuptimensec uint32
	Crtimensec   uint32
}

type ExchangeIn struct {
	InHeader
	Olddir  uint64
	Newdir  uint64
	Options uint64
}
