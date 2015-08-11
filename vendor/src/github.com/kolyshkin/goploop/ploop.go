package ploop

import "strings"
import "os/exec"
import "sync"

// #cgo pkg-config: --static ploop
// #include <ploop/libploop.h>
import "C"

// Possible SetVerboseLevel arguments
const (
	NoConsole  = C.PLOOP_LOG_NOCONSOLE
	NoStdout   = C.PLOOP_LOG_NOSTDOUT
	Timestamps = C.PLOOP_LOG_TIMESTAMPS
)

// SetVerboseLevel sets a level of verbosity when logging to stdout/stderr
func SetVerboseLevel(v int) {
	C.ploop_set_verbose_level(C.int(v))
}

// SetLogFile enables logging to a file and sets log file name
func SetLogFile(file string) error {
	cfile := C.CString(file)
	defer cfree(cfile)

	ret := C.ploop_set_log_file(cfile)

	return mkerr(ret)
}

// SetLogLevel sets a level of verbosity when logging to a file
func SetLogLevel(v int) {
	C.ploop_set_log_level(C.int(v))
}

// Ploop is a type containing DiskDescriptor.xml opened by the library
type Ploop struct {
	d *C.struct_ploop_disk_images_data
}

var once sync.Once

// load ploop modules
func loadKmod() {
	// try to load ploop modules
	modules := []string{"ploop", "pfmt_ploop1", "pfmt_raw", "pio_direct", "pio_nfs", "pio_kaio"}
	for _, m := range modules {
		exec.Command("modprobe", m).Run()
	}
}

// Open opens a ploop DiskDescriptor.xml, most ploop operations require it
func Open(file string) (Ploop, error) {
	var d Ploop

	once.Do(loadKmod)

	cfile := C.CString(file)
	defer cfree(cfile)

	ret := C.ploop_open_dd(&d.d, cfile)

	return d, mkerr(ret)
}

// Close closes a ploop disk descriptor when it is no longer needed
func (d Ploop) Close() {
	C.ploop_close_dd(d.d)
}

type ImageMode int

// Possible values for ImageMode
const (
	Expanded     ImageMode = C.PLOOP_EXPANDED_MODE
	Preallocated ImageMode = C.PLOOP_EXPANDED_PREALLOCATED_MODE
	Raw          ImageMode = C.PLOOP_RAW_MODE
)

// ParseImageMode converts a string to ImageMode value
func ParseImageMode(s string) (ImageMode, error) {
	switch strings.ToLower(s) {
	case "expanded":
		return Expanded, nil
	case "preallocated":
		return Preallocated, nil
	case "raw":
		return Raw, nil
	default:
		return Expanded, mkerr(E_PARAM)
	}
}

// String converts an ImageMode value to string
func (m ImageMode) String() string {
	switch m {
	case Expanded:
		return "Expanded"
	case Preallocated:
		return "Preallocated"
	case Raw:
		return "Raw"
	}
	return "<unknown>"
}

// CreateFlags is a type for CreateParam.Flags
type CreateFlags uint

// Possible values for CreateFlags
const (
	NoLazy CreateFlags = C.PLOOP_CREATE_NOLAZY
)

// CreateParam is a set of parameters for a newly created ploop
type CreateParam struct {
	Size  uint64      // image size, in kilobytes (FS size is about 10% smaller)
	Mode  ImageMode   // image mode
	File  string      // path to and a file name for base delta image
	CLog  uint        // cluster block size log (6 to 15, default 11)
	Flags CreateFlags // flags
}

// Create creates a ploop image and its DiskDescriptor.xml
func Create(p *CreateParam) error {
	var a C.struct_ploop_create_param

	once.Do(loadKmod)

	// default image file name
	if p.File == "" {
		p.File = "root.hdd"
	}

	a.size = convertSize(p.Size)
	a.mode = C.int(p.Mode)
	if p.CLog != 0 {
		// ploop cluster block size, in 512-byte sectors
		// default is 1M cluster block size (CLog=11)
		// 2^11 = 2048 sectors, 2048*512 = 1M
		a.blocksize = 1 << p.CLog
	}
	a.flags = C.uint(p.Flags)
	a.image = C.CString(p.File)
	defer cfree(a.image)
	a.fstype = C.CString("ext4")
	defer cfree(a.fstype)

	ret := C.ploop_create_image(&a)
	return mkerr(ret)
}

// MountParam is a set of parameters to pass to Mount()
type MountParam struct {
	UUID     string // snapshot uuid (empty for top delta)
	Target   string // mount point (empty if no mount is needed)
	Flags    int    // bit mount flags such as MS_NOATIME
	Data     string // auxiliary mount options
	Readonly bool   // mount read-only
	Fsck     bool   // do fsck before mounting inner FS
	Quota    bool   // enable quota for inner FS
}

// Mount creates a ploop device and (optionally) mounts it
func (d Ploop) Mount(p *MountParam) (string, error) {
	var a C.struct_ploop_mount_param
	var device string

	if p.UUID != "" {
		a.guid = C.CString(p.UUID)
		defer cfree(a.guid)
	}
	if p.Target != "" {
		a.target = C.CString(p.Target)
		defer cfree(a.target)
	}

	// mount_data should not be NULL
	a.mount_data = C.CString(p.Data)
	defer cfree(a.mount_data)

	a.flags = C.int(p.Flags)
	a.ro = boolToC(p.Readonly)
	a.fsck = boolToC(p.Fsck)
	a.quota = boolToC(p.Quota)

	ret := C.ploop_mount_image(d.d, &a)
	if ret == 0 {
		device = C.GoString(&a.device[0])
		// TODO? fsck_code = C.GoString(a.fsck_rc)
	}
	return device, mkerr(ret)
}

// Umount unmounts the ploop filesystem and dismantles the device
func (d Ploop) Umount() error {
	ret := C.ploop_umount_image(d.d)

	return mkerr(ret)
}

// UmountByDevice unmounts the ploop filesystem and dismantles the device.
// Unlike Umount(), this is a lower-level function meaning it can be less
// safe and should generally not be used.
func UmountByDevice(dev string) error {
	cdev := C.CString(dev)
	defer cfree(cdev)

	ret := C.ploop_umount(cdev, nil)

	return mkerr(ret)
}

// Resize changes the ploop size. Online resize is recommended.
func (d Ploop) Resize(size uint64, offline bool) error {
	var p C.struct_ploop_resize_param

	p.size = convertSize(size)
	p.offline_resize = boolToC(offline)

	ret := C.ploop_resize_image(d.d, &p)
	return mkerr(ret)
}

// Snapshot creates a ploop snapshot, returning its uuid
func (d Ploop) Snapshot() (string, error) {
	var p C.struct_ploop_snapshot_param
	var uuid, err = UUID()
	if err != nil {
		return "", err
	}
	p.guid = C.CString(uuid)
	defer cfree(p.guid)

	ret := C.ploop_create_snapshot(d.d, &p)
	if ret == 0 {
		uuid = C.GoString(p.guid)
	}

	return uuid, mkerr(ret)
}

// SwitchSnapshot switches to a specified snapshot,
// creates a new empty delta on top of it, and makes it a top one
// (i.e. the one new data will be written to).
// Old top delta (i.e. data modified since the last snapshot) is lost.
func (d Ploop) SwitchSnapshot(uuid string) error {
	var p C.struct_ploop_snapshot_switch_param

	p.guid = C.CString(uuid)
	defer cfree(p.guid)

	ret := C.ploop_switch_snapshot_ex(d.d, &p)

	return mkerr(ret)
}

// Possible values for SwitchSnapshotExtended flags argument
type SwitchFlag uint

const (
	// SkipDestroy, if set, modifies the behavior of
	// SwitchSnapshotExtended to not delete the old top delta, but
	// make it a snapshot and return its uuid. Without this flag,
	// old top delta (i.e. data modified since the last snapshot)
	// is lost.
	SkipDestroy SwitchFlag = C.PLOOP_SNAP_SKIP_TOPDELTA_DESTROY
	// SkipCreate flag, if set, modifies the behavior of
	// SwitchSnapshotExtended to not create a new top delta,
	// but rather transform the specified snapshot itself to be
	// the new top delta), so all new changes will be written
	// right to it. Snapshot UUID is lost in this case.
	SkipCreate SwitchFlag = C.PLOOP_SNAP_SKIP_TOPDELTA_CREATE
)

// SwitchSnapshotExtended is same as SwitchSnapshot but with additional
// flags modifying its behavior. Please see individual flags description.
// Returns uuid of what was the old top delta if SkipDestroy flag is set.
func (d Ploop) SwitchSnapshotExtended(uuid string, flags SwitchFlag) (string, error) {
	var p C.struct_ploop_snapshot_switch_param
	oldUUID := ""

	p.guid = C.CString(uuid)
	defer cfree(p.guid)

	p.flags = C.int(flags)

	if flags&SkipDestroy != 0 {
		oldUUID, err := UUID()
		if err != nil {
			return "", err
		}
		p.guid_old = C.CString(oldUUID)
		defer cfree(p.guid_old)
	}

	ret := C.ploop_switch_snapshot_ex(d.d, &p)

	return oldUUID, mkerr(ret)
}

// DeleteSnapshot deletes a snapshot (merging it down if necessary)
func (d Ploop) DeleteSnapshot(uuid string) error {
	cuuid := C.CString(uuid)
	defer cfree(cuuid)

	ret := C.ploop_delete_snapshot(d.d, cuuid)

	return mkerr(ret)
}

type ReplaceFlag int

// Possible values for ReplaceParam.flags
const (
	// KeepName renames the new file to old file name after replace;
	// note that if this option is used the old file is removed.
	KeepName ReplaceFlag = C.PLOOP_REPLACE_KEEP_NAME
)

// ReplaceParam is a set of parameters to Replace()
type ReplaceParam struct {
	File string // new image file name
	// Image to be replaced is specified by either
	// uuid, current file name, or level,
	// in the above order of preference.
	UUID    string
	CurFile string
	Level   int
	Flags   ReplaceFlag
}

// Replace replaces a ploop image to a different (but identical) one
func (d Ploop) Replace(p *ReplaceParam) error {
	var a C.struct_ploop_replace_param

	a.file = C.CString(p.File)
	defer cfree(a.file)

	if p.UUID != "" {
		a.guid = C.CString(p.UUID)
		defer cfree(a.guid)
	} else if p.CurFile != "" {
		a.cur_file = C.CString(p.CurFile)
		defer cfree(a.cur_file)
	} else {
		a.level = C.int(p.Level)
	}

	a.flags = C.int(p.Flags)

	ret := C.ploop_replace_image(d.d, &a)

	return mkerr(ret)
}

// IsMounted returns true if ploop is mounted
func (d Ploop) IsMounted() (bool, error) {
	ret := C.ploop_is_mounted(d.d)
	if ret == 0 {
		return false, nil
	} else if ret == 1 {
		return true, nil
	} else {
		// error, but no code, make our own
		return false, mkerr(E_SYS)
	}
}

// FSInfoData holds information about ploop inner file system
type FSInfoData struct {
	BlockSize  uint64
	Blocks     uint64
	BlocksFree uint64
	Inodes     uint64
	InodesFree uint64
}

// FSInfo gets info of ploop's inner file system
func FSInfo(file string) (FSInfoData, error) {
	var cinfo C.struct_ploop_info
	var info FSInfoData
	cfile := C.CString(file)
	defer cfree(cfile)

	once.Do(loadKmod)

	ret := C.ploop_get_info_by_descr(cfile, &cinfo)
	if ret == 0 {
		info.BlockSize = uint64(cinfo.fs_bsize)
		info.Blocks = uint64(cinfo.fs_blocks)
		info.BlocksFree = uint64(cinfo.fs_bfree)
		info.Inodes = uint64(cinfo.fs_inodes)
		info.InodesFree = uint64(cinfo.fs_ifree)
	}

	return info, mkerr(ret)
}

// ImageInfoData holds information about ploop image
type ImageInfoData struct {
	Blocks    uint64
	BlockSize uint32
	Version   int
}

// ImageInfo gets information about a ploop image
func (d Ploop) ImageInfo() (ImageInfoData, error) {
	var cinfo C.struct_ploop_spec
	var info ImageInfoData

	ret := C.ploop_get_spec(d.d, &cinfo)
	if ret == 0 {
		info.Blocks = uint64(cinfo.size)
		info.BlockSize = uint32(cinfo.blocksize)
		info.Version = int(cinfo.fmt_version)
	}

	return info, mkerr(ret)
}

// TopDeltaFile returns file name of top delta
func (d Ploop) TopDeltaFile() (string, error) {
	const len = 4096 // PATH_MAX
	var out [len]C.char

	ret := C.ploop_get_top_delta_fname(d.d, &out[0], len)
	if ret != 0 {
		// error, but no code, make our own
		return "", mkerr(E_SYS)
	}

	file := C.GoString(&out[0])
	return file, nil
}

// UUID generates a ploop UUID
func UUID() (string, error) {
	var cuuid [39]C.char

	ret := C.ploop_uuid_generate(&cuuid[0], 39)
	if ret != 0 {
		return "", mkerr(ret)
	}

	uuid := C.GoString(&cuuid[0])
	return uuid, nil
}
