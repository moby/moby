package btf

import (
	"bytes"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/unix"
)

// info describes a BTF object.
type info struct {
	BTF *Spec
	ID  ID
	// Name is an identifying name for the BTF, currently only used by the
	// kernel.
	Name string
	// KernelBTF is true if the BTf originated with the kernel and not
	// userspace.
	KernelBTF bool
}

func newInfoFromFd(fd *sys.FD) (*info, error) {
	// We invoke the syscall once with a empty BTF and name buffers to get size
	// information to allocate buffers. Then we invoke it a second time with
	// buffers to receive the data.
	var btfInfo sys.BtfInfo
	if err := sys.ObjInfo(fd, &btfInfo); err != nil {
		return nil, err
	}

	btfBuffer := make([]byte, btfInfo.BtfSize)
	nameBuffer := make([]byte, btfInfo.NameLen)
	btfInfo.Btf, btfInfo.BtfSize = sys.NewSlicePointerLen(btfBuffer)
	btfInfo.Name, btfInfo.NameLen = sys.NewSlicePointerLen(nameBuffer)
	if err := sys.ObjInfo(fd, &btfInfo); err != nil {
		return nil, err
	}

	spec, err := loadRawSpec(bytes.NewReader(btfBuffer), internal.NativeEndian, nil, nil)
	if err != nil {
		return nil, err
	}

	return &info{
		BTF:       spec,
		ID:        ID(btfInfo.Id),
		Name:      unix.ByteSliceToString(nameBuffer),
		KernelBTF: btfInfo.KernelBtf != 0,
	}, nil
}
