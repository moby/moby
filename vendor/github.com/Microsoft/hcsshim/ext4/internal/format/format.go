package format

type SuperBlock struct {
	InodesCount          uint32
	BlocksCountLow       uint32
	RootBlocksCountLow   uint32
	FreeBlocksCountLow   uint32
	FreeInodesCount      uint32
	FirstDataBlock       uint32
	LogBlockSize         uint32
	LogClusterSize       uint32
	BlocksPerGroup       uint32
	ClustersPerGroup     uint32
	InodesPerGroup       uint32
	Mtime                uint32
	Wtime                uint32
	MountCount           uint16
	MaxMountCount        uint16
	Magic                uint16
	State                uint16
	Errors               uint16
	MinorRevisionLevel   uint16
	LastCheck            uint32
	CheckInterval        uint32
	CreatorOS            uint32
	RevisionLevel        uint32
	DefaultReservedUid   uint16
	DefaultReservedGid   uint16
	FirstInode           uint32
	InodeSize            uint16
	BlockGroupNr         uint16
	FeatureCompat        CompatFeature
	FeatureIncompat      IncompatFeature
	FeatureRoCompat      RoCompatFeature
	UUID                 [16]uint8
	VolumeName           [16]byte
	LastMounted          [64]byte
	AlgorithmUsageBitmap uint32
	PreallocBlocks       uint8
	PreallocDirBlocks    uint8
	ReservedGdtBlocks    uint16
	JournalUUID          [16]uint8
	JournalInum          uint32
	JournalDev           uint32
	LastOrphan           uint32
	HashSeed             [4]uint32
	DefHashVersion       uint8
	JournalBackupType    uint8
	DescSize             uint16
	DefaultMountOpts     uint32
	FirstMetaBg          uint32
	MkfsTime             uint32
	JournalBlocks        [17]uint32
	BlocksCountHigh      uint32
	RBlocksCountHigh     uint32
	FreeBlocksCountHigh  uint32
	MinExtraIsize        uint16
	WantExtraIsize       uint16
	Flags                uint32
	RaidStride           uint16
	MmpInterval          uint16
	MmpBlock             uint64
	RaidStripeWidth      uint32
	LogGroupsPerFlex     uint8
	ChecksumType         uint8
	ReservedPad          uint16
	KbytesWritten        uint64
	SnapshotInum         uint32
	SnapshotID           uint32
	SnapshotRBlocksCount uint64
	SnapshotList         uint32
	ErrorCount           uint32
	FirstErrorTime       uint32
	FirstErrorInode      uint32
	FirstErrorBlock      uint64
	FirstErrorFunc       [32]uint8
	FirstErrorLine       uint32
	LastErrorTime        uint32
	LastErrorInode       uint32
	LastErrorLine        uint32
	LastErrorBlock       uint64
	LastErrorFunc        [32]uint8
	MountOpts            [64]uint8
	UserQuotaInum        uint32
	GroupQuotaInum       uint32
	OverheadBlocks       uint32
	BackupBgs            [2]uint32
	EncryptAlgos         [4]uint8
	EncryptPwSalt        [16]uint8
	LpfInode             uint32
	ProjectQuotaInum     uint32
	ChecksumSeed         uint32
	WtimeHigh            uint8
	MtimeHigh            uint8
	MkfsTimeHigh         uint8
	LastcheckHigh        uint8
	FirstErrorTimeHigh   uint8
	LastErrorTimeHigh    uint8
	Pad                  [2]uint8
	Reserved             [96]uint32
	Checksum             uint32
}

const SuperBlockMagic uint16 = 0xef53

type CompatFeature uint32
type IncompatFeature uint32
type RoCompatFeature uint32

const (
	CompatDirPrealloc   CompatFeature = 0x1
	CompatImagicInodes  CompatFeature = 0x2
	CompatHasJournal    CompatFeature = 0x4
	CompatExtAttr       CompatFeature = 0x8
	CompatResizeInode   CompatFeature = 0x10
	CompatDirIndex      CompatFeature = 0x20
	CompatLazyBg        CompatFeature = 0x40
	CompatExcludeInode  CompatFeature = 0x80
	CompatExcludeBitmap CompatFeature = 0x100
	CompatSparseSuper2  CompatFeature = 0x200

	IncompatCompression IncompatFeature = 0x1
	IncompatFiletype    IncompatFeature = 0x2
	IncompatRecover     IncompatFeature = 0x4
	IncompatJournalDev  IncompatFeature = 0x8
	IncompatMetaBg      IncompatFeature = 0x10
	IncompatExtents     IncompatFeature = 0x40
	Incompat_64Bit      IncompatFeature = 0x80
	IncompatMmp         IncompatFeature = 0x100
	IncompatFlexBg      IncompatFeature = 0x200
	IncompatEaInode     IncompatFeature = 0x400
	IncompatDirdata     IncompatFeature = 0x1000
	IncompatCsumSeed    IncompatFeature = 0x2000
	IncompatLargedir    IncompatFeature = 0x4000
	IncompatInlineData  IncompatFeature = 0x8000
	IncompatEncrypt     IncompatFeature = 0x10000

	RoCompatSparseSuper  RoCompatFeature = 0x1
	RoCompatLargeFile    RoCompatFeature = 0x2
	RoCompatBtreeDir     RoCompatFeature = 0x4
	RoCompatHugeFile     RoCompatFeature = 0x8
	RoCompatGdtCsum      RoCompatFeature = 0x10
	RoCompatDirNlink     RoCompatFeature = 0x20
	RoCompatExtraIsize   RoCompatFeature = 0x40
	RoCompatHasSnapshot  RoCompatFeature = 0x80
	RoCompatQuota        RoCompatFeature = 0x100
	RoCompatBigalloc     RoCompatFeature = 0x200
	RoCompatMetadataCsum RoCompatFeature = 0x400
	RoCompatReplica      RoCompatFeature = 0x800
	RoCompatReadonly     RoCompatFeature = 0x1000
	RoCompatProject      RoCompatFeature = 0x2000
)

type BlockGroupFlag uint16

const (
	BlockGroupInodeUninit BlockGroupFlag = 0x1
	BlockGroupBlockUninit BlockGroupFlag = 0x2
	BlockGroupInodeZeroed BlockGroupFlag = 0x4
)

type GroupDescriptor struct {
	BlockBitmapLow     uint32
	InodeBitmapLow     uint32
	InodeTableLow      uint32
	FreeBlocksCountLow uint16
	FreeInodesCountLow uint16
	UsedDirsCountLow   uint16
	Flags              BlockGroupFlag
	ExcludeBitmapLow   uint32
	BlockBitmapCsumLow uint16
	InodeBitmapCsumLow uint16
	ItableUnusedLow    uint16
	Checksum           uint16
}

type GroupDescriptor64 struct {
	GroupDescriptor
	BlockBitmapHigh     uint32
	InodeBitmapHigh     uint32
	InodeTableHigh      uint32
	FreeBlocksCountHigh uint16
	FreeInodesCountHigh uint16
	UsedDirsCountHigh   uint16
	ItableUnusedHigh    uint16
	ExcludeBitmapHigh   uint32
	BlockBitmapCsumHigh uint16
	InodeBitmapCsumHigh uint16
	Reserved            uint32
}

const (
	S_IXOTH  = 0x1
	S_IWOTH  = 0x2
	S_IROTH  = 0x4
	S_IXGRP  = 0x8
	S_IWGRP  = 0x10
	S_IRGRP  = 0x20
	S_IXUSR  = 0x40
	S_IWUSR  = 0x80
	S_IRUSR  = 0x100
	S_ISVTX  = 0x200
	S_ISGID  = 0x400
	S_ISUID  = 0x800
	S_IFIFO  = 0x1000
	S_IFCHR  = 0x2000
	S_IFDIR  = 0x4000
	S_IFBLK  = 0x6000
	S_IFREG  = 0x8000
	S_IFLNK  = 0xA000
	S_IFSOCK = 0xC000

	TypeMask uint16 = 0xF000
)

type InodeNumber uint32

const (
	InodeRoot = 2
)

type Inode struct {
	Mode                 uint16
	Uid                  uint16
	SizeLow              uint32
	Atime                uint32
	Ctime                uint32
	Mtime                uint32
	Dtime                uint32
	Gid                  uint16
	LinksCount           uint16
	BlocksLow            uint32
	Flags                InodeFlag
	Version              uint32
	Block                [60]byte
	Generation           uint32
	XattrBlockLow        uint32
	SizeHigh             uint32
	ObsoleteFragmentAddr uint32
	BlocksHigh           uint16
	XattrBlockHigh       uint16
	UidHigh              uint16
	GidHigh              uint16
	ChecksumLow          uint16
	Reserved             uint16
	ExtraIsize           uint16
	ChecksumHigh         uint16
	CtimeExtra           uint32
	MtimeExtra           uint32
	AtimeExtra           uint32
	Crtime               uint32
	CrtimeExtra          uint32
	VersionHigh          uint32
	Projid               uint32
}

type InodeFlag uint32

const (
	InodeFlagSecRm              InodeFlag = 0x1
	InodeFlagUnRm               InodeFlag = 0x2
	InodeFlagCompressed         InodeFlag = 0x4
	InodeFlagSync               InodeFlag = 0x8
	InodeFlagImmutable          InodeFlag = 0x10
	InodeFlagAppend             InodeFlag = 0x20
	InodeFlagNoDump             InodeFlag = 0x40
	InodeFlagNoAtime            InodeFlag = 0x80
	InodeFlagDirtyCompressed    InodeFlag = 0x100
	InodeFlagCompressedClusters InodeFlag = 0x200
	InodeFlagNoCompress         InodeFlag = 0x400
	InodeFlagEncrypted          InodeFlag = 0x800
	InodeFlagHashedIndex        InodeFlag = 0x1000
	InodeFlagMagic              InodeFlag = 0x2000
	InodeFlagJournalData        InodeFlag = 0x4000
	InodeFlagNoTail             InodeFlag = 0x8000
	InodeFlagDirSync            InodeFlag = 0x10000
	InodeFlagTopDir             InodeFlag = 0x20000
	InodeFlagHugeFile           InodeFlag = 0x40000
	InodeFlagExtents            InodeFlag = 0x80000
	InodeFlagEaInode            InodeFlag = 0x200000
	InodeFlagEOFBlocks          InodeFlag = 0x400000
	InodeFlagSnapfile           InodeFlag = 0x01000000
	InodeFlagSnapfileDeleted    InodeFlag = 0x04000000
	InodeFlagSnapfileShrunk     InodeFlag = 0x08000000
	InodeFlagInlineData         InodeFlag = 0x10000000
	InodeFlagProjectIDInherit   InodeFlag = 0x20000000
	InodeFlagReserved           InodeFlag = 0x80000000
)

const (
	MaxLinks = 65000
)

type ExtentHeader struct {
	Magic      uint16
	Entries    uint16
	Max        uint16
	Depth      uint16
	Generation uint32
}

const ExtentHeaderMagic uint16 = 0xf30a

type ExtentIndexNode struct {
	Block    uint32
	LeafLow  uint32
	LeafHigh uint16
	Unused   uint16
}

type ExtentLeafNode struct {
	Block     uint32
	Length    uint16
	StartHigh uint16
	StartLow  uint32
}

type ExtentTail struct {
	Checksum uint32
}

type DirectoryEntry struct {
	Inode        InodeNumber
	RecordLength uint16
	NameLength   uint8
	FileType     FileType
	//Name         []byte
}

type FileType uint8

const (
	FileTypeUnknown      FileType = 0x0
	FileTypeRegular      FileType = 0x1
	FileTypeDirectory    FileType = 0x2
	FileTypeCharacter    FileType = 0x3
	FileTypeBlock        FileType = 0x4
	FileTypeFIFO         FileType = 0x5
	FileTypeSocket       FileType = 0x6
	FileTypeSymbolicLink FileType = 0x7
)

type DirectoryEntryTail struct {
	ReservedZero1 uint32
	RecordLength  uint16
	ReservedZero2 uint8
	FileType      uint8
	Checksum      uint32
}

type DirectoryTreeRoot struct {
	Dot            DirectoryEntry
	DotName        [4]byte
	DotDot         DirectoryEntry
	DotDotName     [4]byte
	ReservedZero   uint32
	HashVersion    uint8
	InfoLength     uint8
	IndirectLevels uint8
	UnusedFlags    uint8
	Limit          uint16
	Count          uint16
	Block          uint32
	//Entries        []DirectoryTreeEntry
}

type DirectoryTreeNode struct {
	FakeInode        uint32
	FakeRecordLength uint16
	NameLength       uint8
	FileType         uint8
	Limit            uint16
	Count            uint16
	Block            uint32
	//Entries          []DirectoryTreeEntry
}

type DirectoryTreeEntry struct {
	Hash  uint32
	Block uint32
}

type DirectoryTreeTail struct {
	Reserved uint32
	Checksum uint32
}

type XAttrInodeBodyHeader struct {
	Magic uint32
}

type XAttrHeader struct {
	Magic          uint32
	ReferenceCount uint32
	Blocks         uint32
	Hash           uint32
	Checksum       uint32
	Reserved       [3]uint32
}

const XAttrHeaderMagic uint32 = 0xea020000

type XAttrEntry struct {
	NameLength  uint8
	NameIndex   uint8
	ValueOffset uint16
	ValueInum   uint32
	ValueSize   uint32
	Hash        uint32
	//Name        []byte
}
