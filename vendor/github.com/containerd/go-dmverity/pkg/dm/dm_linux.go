/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package dm

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Ioctl encoding constants (see <asm-generic/ioctl.h>).
const (
	iocNRBits   = 8
	iocTypeBits = 8
	iocSizeBits = 14
	iocDirBits  = 2

	iocNRShift   = 0
	iocTypeShift = iocNRShift + iocNRBits
	iocSizeShift = iocTypeShift + iocTypeBits
	iocDirShift  = iocSizeShift + iocSizeBits

	iocWrite = 1
	iocRead  = 2
)

// Device-mapper ioctl constants (see <linux/dm-ioctl.h>).
// Ioctl type ("magic").
const DMIOCTLType = 0xfd // matches Linux uapi header

const (
	DMNameLen     = 128
	DMUUIDLen     = 129
	DMMaxTypeName = 16
)

// DM ioctl command numbers (subset) per <linux/dm-ioctl.h>.
const (
	DMDevCreateCMD    = 3  // DM_DEV_CREATE
	DMDevRemoveCMD    = 4  // DM_DEV_REMOVE
	DMDevSuspendCMD   = 6  // DM_DEV_SUSPEND
	DMDevStatusCMD    = 7  // DM_DEV_STATUS
	DMTableLoadCMD    = 9  // DM_TABLE_LOAD
	DMTableClearCMD   = 10 // DM_TABLE_CLEAR
	DMTableStatusCMD  = 12 // DM_TABLE_STATUS
	DMListVersionsCMD = 13 // DM_LIST_VERSIONS
)

const (
	DMVersionMajor = 4
	DMVersionMinor = 0
	DMVersionPatch = 0
)

const (
	DMReadOnlyFlag        = 1 << 0
	DMSuspendFlag         = 1 << 1
	DMStatusTableFlag     = 1 << 4
	DMActivePresentFlag   = 1 << 5
	DMInactivePresentFlag = 1 << 6
)

type dmIoctl struct {
	Version     [3]uint32
	DataSize    uint32
	DataStart   uint32
	TargetCount uint32
	OpenCount   int32
	Flags       uint32
	EventNr     uint32
	Padding     uint32
	Dev         uint64
	Name        [DMNameLen]byte
	UUID        [DMUUIDLen]byte
	Data        [7]byte
}

type dmTargetSpec struct {
	SectorStart uint64
	Length      uint64
	Status      int32
	Next        uint32
	TargetType  [DMMaxTypeName]byte
}

type dmTargetVersions struct {
	Next    uint32
	Version [3]uint32
	Name    [DMMaxTypeName]byte
}

type Control struct {
	fd *os.File
}

type Target struct {
	SectorStart uint64
	Length      uint64
	Type        string
	Params      string
}

type DeviceStatus struct {
	OpenCount       int32
	TargetCount     uint32
	EventNr         uint32
	Flags           uint32
	Dev             uint64
	Major           uint32
	Minor           uint32
	Name            string
	UUID            string
	ActivePresent   bool
	InactivePresent bool
}

func Open() (*Control, error) {
	fd, err := os.OpenFile("/dev/mapper/control", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &Control{fd: fd}, nil
}

func (c *Control) Close() error {
	if c == nil || c.fd == nil {
		return nil
	}
	return c.fd.Close()
}

var ioctlSyscall = func(fd, req, arg uintptr) (uintptr, uintptr, unix.Errno) {
	return unix.Syscall(unix.SYS_IOCTL, fd, req, arg)
}

func dmReq(nr uintptr) uintptr {
	return iowr(DMIOCTLType, nr, unsafe.Sizeof(dmIoctl{}))
}

func (c *Control) rawIoctl(nr uintptr, buf unsafe.Pointer) error {
	_, _, errno := ioctlSyscall(c.fd.Fd(), dmReq(nr), uintptr(buf))
	if errno != 0 {
		return errno
	}
	return nil
}

func makeBaseIoctl(name, uuid string, totalDataSize int) dmIoctl {
	var io dmIoctl
	io.Version[0] = DMVersionMajor
	io.Version[1] = DMVersionMinor
	io.Version[2] = DMVersionPatch
	io.DataSize = uint32(totalDataSize)
	io.DataStart = uint32(unsafe.Sizeof(dmIoctl{}))
	copy(io.Name[:], []byte(name))
	copy(io.UUID[:], []byte(uuid))
	return io
}

func (c *Control) CreateDevice(name string) (uint64, error) {
	buf := make([]byte, unsafe.Sizeof(dmIoctl{}))
	io := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	*io = makeBaseIoctl(name, "", len(buf))
	if err := c.rawIoctl(DMDevCreateCMD, unsafe.Pointer(io)); err != nil {
		return 0, fmt.Errorf("dm create '%s': %w", name, err)
	}
	return io.Dev, nil
}

func (c *Control) RemoveDevice(name string) error {
	buf := make([]byte, unsafe.Sizeof(dmIoctl{}))
	io := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	*io = makeBaseIoctl(name, "", len(buf))
	if err := c.rawIoctl(DMDevRemoveCMD, unsafe.Pointer(io)); err != nil {
		return fmt.Errorf("dm remove '%s': %w", name, err)
	}
	return nil
}

func (c *Control) SuspendDevice(name string, suspend bool) error {
	buf := make([]byte, unsafe.Sizeof(dmIoctl{}))
	io := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	*io = makeBaseIoctl(name, "", len(buf))
	if suspend {
		io.Flags |= DMSuspendFlag
	}
	if err := c.rawIoctl(DMDevSuspendCMD, unsafe.Pointer(io)); err != nil {
		return fmt.Errorf("dm suspend/resume '%s': %w", name, err)
	}
	return nil
}

func (c *Control) LoadTable(name string, targets []Target) error {
	if len(targets) == 0 {
		return errors.New("no targets provided")
	}

	headerSize := int(unsafe.Sizeof(dmIoctl{}))
	payload := make([]byte, 0, headerSize+len(targets)*(int(unsafe.Sizeof(dmTargetSpec{}))+256))
	body := make([]byte, 0, cap(payload)-headerSize)
	for i, t := range targets {
		start := len(body)
		body = append(body, make([]byte, int(unsafe.Sizeof(dmTargetSpec{})))...)
		spec := (*dmTargetSpec)(unsafe.Pointer(&body[start]))
		spec.SectorStart = t.SectorStart
		spec.Length = t.Length
		spec.Status = 0
		spec.Next = 0
		copy(spec.TargetType[:], []byte(t.Type))

		paramsBytes := append([]byte(t.Params), 0)
		body = append(body, paramsBytes...)
		rel := len(body) - start
		pad := ((rel + 7) &^ 7) - rel
		if pad > 0 {
			body = append(body, make([]byte, pad)...)
		}
		if i < len(targets)-1 {
			spec.Next = uint32(len(body) - start)
		}
	}

	buf := make([]byte, headerSize+len(body))
	io := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	*io = makeBaseIoctl(name, "", len(buf))
	io.Flags |= DMReadOnlyFlag
	io.TargetCount = uint32(len(targets))
	copy(buf[headerSize:], body)

	if err := c.rawIoctl(DMTableLoadCMD, unsafe.Pointer(io)); err != nil {
		return fmt.Errorf("dm table load '%s': %w", name, err)
	}
	return nil
}

func (c *Control) ClearTable(name string) error {
	buf := make([]byte, unsafe.Sizeof(dmIoctl{}))
	io := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	*io = makeBaseIoctl(name, "", len(buf))
	if err := c.rawIoctl(DMTableClearCMD, unsafe.Pointer(io)); err != nil {
		if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENXIO) {
			return nil
		}
		return fmt.Errorf("dm table clear '%s': %w", name, err)
	}
	return nil
}

func (c *Control) DeviceStatus(name string) (DeviceStatus, error) {
	buf := make([]byte, unsafe.Sizeof(dmIoctl{}))
	io := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	*io = makeBaseIoctl(name, "", len(buf))
	if err := c.rawIoctl(DMDevStatusCMD, unsafe.Pointer(io)); err != nil {
		return DeviceStatus{}, fmt.Errorf("dm dev status '%s': %w", name, err)
	}
	nlen := 0
	for nlen < len(io.Name) && io.Name[nlen] != 0 {
		nlen++
	}
	ulen := 0
	for ulen < len(io.UUID) && io.UUID[ulen] != 0 {
		ulen++
	}
	maj := unix.Major(io.Dev)
	minor := unix.Minor(io.Dev)
	return DeviceStatus{
		OpenCount:       io.OpenCount,
		TargetCount:     io.TargetCount,
		EventNr:         io.EventNr,
		Flags:           io.Flags,
		Dev:             io.Dev,
		Major:           maj,
		Minor:           minor,
		Name:            string(io.Name[:nlen]),
		UUID:            string(io.UUID[:ulen]),
		ActivePresent:   (io.Flags & DMActivePresentFlag) != 0,
		InactivePresent: (io.Flags & DMInactivePresentFlag) != 0,
	}, nil
}

func (c *Control) TableStatus(name string, inactive bool) (string, error) {
	bufSz := 16 * 1024
	for tries := 0; tries < 3; tries++ {
		buf := make([]byte, bufSz)
		io := (*dmIoctl)(unsafe.Pointer(&buf[0]))
		*io = makeBaseIoctl(name, "", bufSz)
		if inactive {
			io.Flags |= DMStatusTableFlag
		}
		if err := c.rawIoctl(DMTableStatusCMD, unsafe.Pointer(io)); err != nil {
			if errors.Is(err, unix.ENOSPC) || errors.Is(err, unix.EINVAL) {
				bufSz *= 2
				continue
			}
			return "", fmt.Errorf("dm table status '%s': %w", name, err)
		}
		i := int(io.DataStart)
		end := int(io.DataSize)
		if end == 0 || end > len(buf) {
			end = len(buf)
		}
		var out []byte
		first := true
		for i+int(unsafe.Sizeof(dmTargetSpec{})) <= end {
			start := i
			spec := (*dmTargetSpec)(unsafe.Pointer(&buf[i]))
			i += int(unsafe.Sizeof(dmTargetSpec{}))
			j := i
			for j < end && buf[j] != 0 {
				j++
			}
			if !first {
				out = append(out, '\n')
			}
			first = false
			out = append(out, buf[i:j]...)
			if spec.Next == 0 {
				break
			}
			i = start + int(spec.Next)
		}
		return string(out), nil
	}
	return "", fmt.Errorf("dm table status '%s': insufficient buffer after retries", name)
}

func ioc(dir, typ, nr, size uintptr) uintptr {
	return (dir << iocDirShift) | (typ << iocTypeShift) | (nr << iocNRShift) | (size << iocSizeShift)
}

func iowr(typ, nr, size uintptr) uintptr { return ioc(iocRead|iocWrite, typ, nr, size) }

func CheckVeritySignatureSupport() error {
	fd, err := os.OpenFile("/dev/mapper/control", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open /dev/mapper/control: %w", err)
	}
	defer fd.Close()

	bufSize := int(unsafe.Sizeof(dmIoctl{})) + 4096
	buf := make([]byte, bufSize)
	io := (*dmIoctl)(unsafe.Pointer(&buf[0]))
	io.Version[0] = DMVersionMajor
	io.Version[1] = DMVersionMinor
	io.Version[2] = DMVersionPatch
	io.DataSize = uint32(bufSize)
	io.DataStart = uint32(unsafe.Sizeof(dmIoctl{}))

	_, _, errno := ioctlSyscall(fd.Fd(), dmReq(DMListVersionsCMD), uintptr(unsafe.Pointer(io)))
	if errno != 0 {
		return fmt.Errorf("DM_LIST_VERSIONS ioctl failed: %w", errno)
	}

	offset := int(io.DataStart)
	for offset < int(io.DataSize) {
		if offset+int(unsafe.Sizeof(dmTargetVersions{})) > len(buf) {
			break
		}

		tv := (*dmTargetVersions)(unsafe.Pointer(&buf[offset]))
		nameLen := 0
		for nameLen < len(tv.Name) && tv.Name[nameLen] != 0 {
			nameLen++
		}
		name := string(tv.Name[:nameLen])

		if name == "verity" {
			major := tv.Version[0]
			minor := tv.Version[1]

			if major < 1 || (major == 1 && minor < 5) {
				return fmt.Errorf("dm-verity signature not supported (requires >= 1.5.0, found %d.%d.%d)",
					major, minor, tv.Version[2])
			}
			return nil
		}

		if tv.Next == 0 {
			break
		}
		offset += int(tv.Next)
	}

	return fmt.Errorf("dm-verity target not found")
}
