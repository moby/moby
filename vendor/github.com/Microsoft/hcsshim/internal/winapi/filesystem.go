package winapi

//sys NtCreateFile(handle *uintptr, accessMask uint32, oa *ObjectAttributes, iosb *IOStatusBlock, allocationSize *uint64, fileAttributes uint32, shareAccess uint32, createDisposition uint32, createOptions uint32, eaBuffer *byte, eaLength uint32) (status uint32) = ntdll.NtCreateFile
//sys NtSetInformationFile(handle uintptr, iosb *IOStatusBlock, information uintptr, length uint32, class uint32) (status uint32) = ntdll.NtSetInformationFile

//sys NtOpenDirectoryObject(handle *uintptr, accessMask uint32, oa *ObjectAttributes) (status uint32) = ntdll.NtOpenDirectoryObject
//sys NtQueryDirectoryObject(handle uintptr, buffer *byte, length uint32, singleEntry bool, restartScan bool, context *uint32, returnLength *uint32)(status uint32) = ntdll.NtQueryDirectoryObject

const (
	FileLinkInformationClass          = 11
	FileDispositionInformationExClass = 64

	FILE_READ_ATTRIBUTES  = 0x0080
	FILE_WRITE_ATTRIBUTES = 0x0100
	DELETE                = 0x10000

	FILE_OPEN   = 1
	FILE_CREATE = 2

	FILE_LIST_DIRECTORY          = 0x00000001
	FILE_DIRECTORY_FILE          = 0x00000001
	FILE_SYNCHRONOUS_IO_NONALERT = 0x00000020
	FILE_OPEN_FOR_BACKUP_INTENT  = 0x00004000
	FILE_OPEN_REPARSE_POINT      = 0x00200000

	FILE_DISPOSITION_DELETE = 0x00000001

	OBJ_DONT_REPARSE = 0x1000

	STATUS_MORE_ENTRIES    = 0x105
	STATUS_NO_MORE_ENTRIES = 0x8000001a
)

type FileDispositionInformationEx struct {
	Flags uintptr
}

type IOStatusBlock struct {
	Status, Information uintptr
}

type ObjectAttributes struct {
	Length             uintptr
	RootDirectory      uintptr
	ObjectName         uintptr
	Attributes         uintptr
	SecurityDescriptor uintptr
	SecurityQoS        uintptr
}

type ObjectDirectoryInformation struct {
	Name     UnicodeString
	TypeName UnicodeString
}

type FileLinkInformation struct {
	ReplaceIfExists bool
	RootDirectory   uintptr
	FileNameLength  uint32
	FileName        [1]uint16
}
