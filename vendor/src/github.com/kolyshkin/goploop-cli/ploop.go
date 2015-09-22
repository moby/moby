package ploop

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Ploop is a class representing a ploop
type Ploop struct {
	dd string
}

// Possible SetVerboseLevel arguments
const (
	unsetVerbosity int = -111
	NoConsole          = -2
	NoStdout           = -1
	Timestamps         = 4
	ShowCommands       = 5
)

var verbosity = unsetVerbosity
var verbosityOpt = []string{""}

// SetVerboseLevel sets a level of verbosity when logging to stdout/stderr
func SetVerboseLevel(v int) {
	verbosity = v
	verbosityOpt = []string{"-v" + strconv.Itoa(verbosity)}
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

	d.dd = file
	return d, nil
}

// Close closes a ploop disk descriptor when it is no longer needed
func (d Ploop) Close() {
	d.dd = ""
}

// ImageMode is a type for CreateParam.Mode field
type ImageMode string

// Possible values for ImageMode
const (
	Expanded     ImageMode = "expanded"
	Preallocated ImageMode = "preallocated"
	Raw          ImageMode = "raw"
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
		return Expanded, &Err{c: E_PARAM, s: "ParseImageMode: unknown mode " + s}
	}
}

// String converts an ImageMode value to string
func (m ImageMode) String() string {
	return string(m)
}

// CreateFlags is a type for CreateParam.Flags
type CreateFlags int

// Possible values for CreateFlags
const (
	NoLazy CreateFlags = 1 << iota
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
	once.Do(loadKmod)

	// default image file name
	if p.File == "" {
		p.File = "root.hdd"
	}

	args := []string{"init", "-s", strconv.FormatUint(p.Size, 10) + "K"}
	if p.Mode != "" {
		args = append(args, "-f", string(p.Mode))
	}
	if p.CLog != 0 {
		// ploop cluster block size, in 512-byte sectors
		// default is 1M cluster block size (CLog=11)
		// 2^11 = 2048 sectors, 2048*512 = 1M
		blocksize := 1 << p.CLog
		args = append(args, "-b", strconv.Itoa(blocksize))
	}
	if p.Flags != 0 {
		if p.Flags&NoLazy == NoLazy {
			args = append(args, "--nolazy")
		}
	}
	args = append(args, p.File)

	return ploop(args...)
}

// MountParam is a set of parameters to pass to Mount()
type MountParam struct {
	UUID     string // snapshot uuid (empty for top delta)
	Target   string // mount point (empty if no mount is needed)
	Data     string // auxiliary mount options
	Readonly bool   // mount read-only
	Fsck     bool   // do fsck before mounting inner FS
}

// Input is like this (with or without a timestamp, depending on verbosity):
// [ 0.001234] Adding delta dev=/dev/ploop12345 ...
// Adding delta dev=/dev/ploop12345 ...
var reAddDelta = regexp.MustCompile(`(?m)^(?:\[[0-9. ]+\] )*Adding delta dev=(/dev/ploop\d+) `)

// Mount creates a ploop device and (optionally) mounts it
func (d Ploop) Mount(p *MountParam) (string, error) {
	args := []string{"mount"}
	if p.Readonly {
		args = append(args, "-r")
	}
	if p.Fsck {
		args = append(args, "-F")
	}
	if p.Target != "" {
		args = append(args, "-m", p.Target)
	}
	if p.Data != "" {
		args = append(args, "-o", p.Data)
	}
	args = append(args, d.dd)

	dev := ""
	out, err := ploopOut(args...)
	if err == nil {
		// Figure out what device we have
		m := reAddDelta.FindStringSubmatch(out)
		if len(m) != 2 {
			return "", &Err{c: -1,
				s: "Can't parse ploop mount output:\n" + out}
		}
		dev = m[1]
	}

	return dev, err
}

// Umount unmounts the ploop filesystem and dismantles the device
func (d Ploop) Umount() error {
	return ploop("umount", d.dd)
}

// UmountByDevice unmounts the ploop filesystem and dismantles the device.
// Unlike Umount(), this is a lower-level function meaning it can be less
// safe and should generally not be used.
func UmountByDevice(dev string) error {
	return ploop("umount", "-d", dev)
}

// Resize changes the ploop size. Offline flag is ignored
// by this implementation, as ploop tool automatically chooses
// whether do to offline resize (if device is not mounted).
func (d Ploop) Resize(size uint64, offline bool) error {
	return ploop("resize", "-s", strconv.FormatUint(size, 10)+"K", d.dd)
}

// Snapshot creates a ploop snapshot, returning its uuid
func (d Ploop) Snapshot() (string, error) {
	uuid, err := UUID()
	if err != nil {
		return "", err
	}

	return uuid, ploop("snapshot", "-u", uuid, d.dd)
}

// SwitchSnapshot switches to a specified snapshot,
// creates a new empty delta on top of it, and makes it a top one
// (i.e. the one new data will be written to).
// Old top delta (i.e. data modified since the last snapshot) is lost.
func (d Ploop) SwitchSnapshot(uuid string) error {
	return ploop("snapshot-switch", "-u", uuid, d.dd)
}

// DeleteSnapshot deletes a snapshot (merging it down if necessary)
func (d Ploop) DeleteSnapshot(uuid string) error {
	return ploop("snapshot-delete", "-u", uuid, d.dd)
}

// ReplaceFlag is a type for ReplaceParam.Flags field
type ReplaceFlag int

// Possible values for ReplaceParam.Flags field
const (
	_ = iota
	// KeepName renames the new file to old file name after replace;
	// note that if this option is used the old file is removed.
	KeepName ReplaceFlag = iota
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
	args := []string{"replace"}
	if p.UUID != "" {
		args = append(args, "-u", p.UUID)
	} else if p.CurFile != "" {
		args = append(args, "-o", p.CurFile)
	} else {
		args = append(args, "-l", strconv.Itoa(p.Level))
	}

	if p.Flags != 0 {
		if p.Flags&KeepName == KeepName {
			args = append(args, "-k")
		}
	}
	args = append(args, "-i", p.File)
	args = append(args, d.dd)

	return ploop(args...)
}

// device:	/dev/ploop25579
var reDevice = regexp.MustCompile(`(?m)^device:\s+(/dev/ploop\d+)$`)

func (d Ploop) getDevice() (string, error) {
	dev := ""
	out, err := ploopOut("-v-1", "info", "-d", d.dd)
	if err == nil {
		// Figure out what device we have
		m := reDevice.FindStringSubmatch(out)
		if len(m) > 1 {
			dev = m[1]
		}
	}
	return dev, err
}

// IsMounted returns true if ploop is mounted
func (d Ploop) IsMounted() (bool, error) {
	dev, err := d.getDevice()
	return dev != "", err
}

// FSInfoData holds information about ploop inner file system
type FSInfoData struct {
	BlockSize  uint64
	Blocks     uint64
	BlocksFree uint64
	Inodes     uint64
	InodesFree uint64
}

//   resource           Size           Used
//  1k-blocks       10188052          36888
//     inodes         655360             12
var reFSInfo = regexp.MustCompile(`
\s+1k-blocks\s+(\d+)\s+(\d+)
\s+inodes\s+(\d+)\s+(\d+)
`)

// FSInfo gets info of ploop's inner file system
func FSInfo(file string) (FSInfoData, error) {
	once.Do(loadKmod)
	var info FSInfoData

	out, err := ploopOut("-v-1", "info", file)
	if err == nil {
		info.BlockSize = 1024 // ploop info reports in 1-k blocks
		i := reFSInfo.FindStringSubmatch(out)
		if len(i) != 5 {
			return info, &Err{c: -1,
				s: "Can't parse ploop info output:\n" + out}
		}

		info.Blocks, _ = strconv.ParseUint(i[1], 10, 64)
		blocksUsed, _ := strconv.ParseUint(i[2], 10, 64)
		info.BlocksFree = info.Blocks - blocksUsed

		info.Inodes, _ = strconv.ParseUint(i[3], 10, 64)
		inodesUsed, _ := strconv.ParseUint(i[4], 10, 64)
		info.InodesFree = info.Inodes - inodesUsed
	}

	return info, err
}

// ImageInfoData holds information about ploop image
type ImageInfoData struct {
	Blocks    uint64
	BlockSize uint32
	Version   int
}

// size:	20971520
// blocksize:	2048
// fmt_version:	2
var reImageInfo = regexp.MustCompile(`\n*size:\s+(\d+)
blocksize:\s+(\d+)
fmt_version:\s+(\d*)
`)

// ImageInfo gets information about a ploop image
func (d Ploop) ImageInfo() (ImageInfoData, error) {
	var info ImageInfoData

	out, err := ploopOut("-v-1", "info", "-s", d.dd)
	if err == nil {
		i := reImageInfo.FindStringSubmatch(out)
		if len(i) != 4 {
			return info, &Err{c: -1,
				s: "Can't parse ploop info -s output:\n" + out}
		}
		info.Blocks, _ = strconv.ParseUint(i[1], 10, 64)
		bs, _ := strconv.ParseUint(i[2], 10, 32)
		info.BlockSize = uint32(bs)
		info.Version, _ = strconv.Atoi(i[3])
	}

	return info, err
}
