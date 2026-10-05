package ebpf

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/internal"
)

const structOpsValuePrefix = "bpf_struct_ops_"
const structOpsLinkSec = ".struct_ops.link"
const structOpsSec = ".struct_ops"
const structOpsKeySize = 4

// structOpsMemberLayout describes the location and type of a struct_ops member.
type structOpsMemberLayout struct {
	member btf.Member
	off    int
	size   int
	typ    btf.Type
}

// newStructOpsMemberLayout returns a layout information from a struct_ops member.
func newStructOpsMemberLayout(m btf.Member) (*structOpsMemberLayout, error) {
	if m.BitfieldSize > 0 {
		return nil, fmt.Errorf("bitfield %s not supported", m.Name)
	}

	size, err := btf.Sizeof(m.Type)
	if err != nil {
		return nil, fmt.Errorf("sizeof(%s): %w", m.Name, err)
	}

	off := int(m.Offset.Bytes())
	if off < 0 {
		return nil, fmt.Errorf("member %q: invalid offset", m.Name)
	}

	return &structOpsMemberLayout{
		member: m,
		off:    off,
		size:   size,
		typ:    btf.UnderlyingType(m.Type),
	}, nil
}

// bytes returns the portion of `buf` corresponding to the member.
func (ml *structOpsMemberLayout) bytes(buf []byte) ([]byte, error) {
	if ml.off < 0 || ml.off+ml.size > len(buf) {
		return nil, fmt.Errorf("member %q: value buffer too small", ml.member.Name)
	}
	return buf[ml.off : ml.off+ml.size], nil
}

// structOpsFuncPtrMember returns an error unless m is a func pointer member.
func structOpsFuncPtrMember(m btf.Member) error {
	kmPtr, ok := btf.As[*btf.Pointer](m.Type)
	if !ok {
		return fmt.Errorf("member %s is not a func pointer", m.Name)
	}
	if _, isFuncProto := btf.As[*btf.FuncProto](kmPtr.Target); !isFuncProto {
		return fmt.Errorf("member %s is not a func pointer", m.Name)
	}
	return nil
}

// structOpsFindInnerType returns the "inner" struct inside a value struct_ops type.
//
// Given a value like:
//
//	struct bpf_struct_ops_bpf_testmod_ops {
//	    struct bpf_struct_ops_common common;
//	    struct bpf_testmod_ops data;
//	};
//
// this function returns the *btf.Struct for "bpf_testmod_ops" along with the
// byte offset of the "data" member inside the value type.
//
// The inner struct name is derived by trimming the "bpf_struct_ops_" prefix
// from the value's name.
func structOpsFindInnerType(vType *btf.Struct) (*btf.Struct, uint32, error) {
	innerName := strings.TrimPrefix(vType.Name, structOpsValuePrefix)

	for _, m := range vType.Members {
		if st, ok := btf.As[*btf.Struct](m.Type); ok && st.Name == innerName {
			return st, m.Offset.Bytes(), nil
		}
	}

	return nil, 0, fmt.Errorf("inner struct %q not found in %s", innerName, vType.Name)
}

// structOpsFindTarget resolves the kernel-side "value struct" for a struct_ops map.
func structOpsFindTarget(userType *btf.Struct, cache *btf.Cache) (vType *btf.Struct, id btf.TypeID, module *btf.Handle, err error) {
	// the kernel value type name, e.g. "bpf_struct_ops_<name>"
	vTypeName := structOpsValuePrefix + userType.Name

	target := btf.Type((*btf.Struct)(nil))
	spec, module, err := findTargetInKernel(vTypeName, &target, cache)
	if errors.Is(err, btf.ErrNotFound) {
		return nil, 0, nil, fmt.Errorf("%q doesn't exist in kernel: %w", vTypeName, ErrNotSupported)
	}
	if err != nil {
		return nil, 0, nil, fmt.Errorf("lookup value type %q: %w", vTypeName, err)
	}

	id, err = spec.TypeID(target)
	if err != nil {
		return nil, 0, nil, err
	}

	return target.(*btf.Struct), id, module, nil
}

// structOpsPopulateValue writes a `prog FD` which references to `p` into the
// struct_ops value buffer `kernVData` at byte offset `dstOff` corresponding to
// the member `km`.
func structOpsPopulateValue(km btf.Member, kernVData []byte, p *Program) error {
	if err := structOpsFuncPtrMember(km); err != nil {
		return err
	}

	layout, err := newStructOpsMemberLayout(km)
	if err != nil {
		return err
	}

	dst, err := layout.bytes(kernVData)
	if err != nil || len(dst) != 8 {
		return fmt.Errorf("member %q: value buffer too small for func ptr", km.Name)
	}

	internal.NativeEndian.PutUint64(dst, uint64(p.FD()))
	return nil
}

// structOpsValidateMemberPair checks whether `m` can be copied into `km`.
func structOpsValidateMemberPair(m, km btf.Member) (int, error) {
	mLayout, err := newStructOpsMemberLayout(m)
	if err != nil {
		return 0, fmt.Errorf("user member %s: %w", m.Name, err)
	}

	kLayout, err := newStructOpsMemberLayout(km)
	if err != nil {
		return 0, fmt.Errorf("kernel member %s: %w", km.Name, err)
	}

	if mLayout.size != kLayout.size {
		return 0, fmt.Errorf("size mismatch for %s: user=%d kernel=%d", m.Name, mLayout.size, kLayout.size)
	}

	if reflect.TypeOf(mLayout.typ) != reflect.TypeOf(kLayout.typ) {
		return 0, fmt.Errorf("unmatched member type %s != %s (kernel)", m.Name, km.Name)
	}

	return mLayout.size, nil
}

// structOpsCopyMemberBytes copies the bytes of `m` into `km`.
func structOpsCopyMemberBytes(m, km btf.Member, data, kernVData []byte, size int) error {
	mLayout, err := newStructOpsMemberLayout(m)
	if err != nil {
		return fmt.Errorf("user member %s: %w", m.Name, err)
	}

	kLayout, err := newStructOpsMemberLayout(km)
	if err != nil {
		return fmt.Errorf("kernel member %s: %w", km.Name, err)
	}

	if mLayout.size != size {
		return fmt.Errorf("member %q: unexpected validated size %d, got %d", m.Name, size, mLayout.size)
	}
	if kLayout.size != size {
		return fmt.Errorf("member %q: unexpected validated size %d, got %d", km.Name, size, kLayout.size)
	}

	src, err := mLayout.bytes(data)
	if err != nil {
		return fmt.Errorf("member %q: userdata is too small", m.Name)
	}

	dst, err := kLayout.bytes(kernVData)
	if err != nil {
		return fmt.Errorf("member %q: value type is too small", km.Name)
	}

	switch mLayout.typ.(type) {
	case *btf.Struct, *btf.Union:
		if !structOpsIsMemZeroed(src) {
			return fmt.Errorf("non-zero nested struct %s: %w", m.Name, ErrNotSupported)
		}
		return nil
	}

	copy(dst, src)
	return nil
}

// structOpsCopyMember copies `m` into `km`.
func structOpsCopyMember(m, km btf.Member, data []byte, kernVData []byte) error {
	size, err := structOpsValidateMemberPair(m, km)
	if err != nil {
		return err
	}
	return structOpsCopyMemberBytes(m, km, data, kernVData, size)
}

// structOpsIsMemZeroed() checks whether all bytes in data are zero.
func structOpsIsMemZeroed(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

// structOpsSetAttachTo sets p.AttachTo in the expected "struct_name:memberName" format
// based on the struct definition.
//
// this relies on the assumption that each member in the
// `.struct_ops` section has a relocation at its starting byte offset.
func structOpsSetAttachTo(
	sec *elfSection,
	baseOff uint32,
	userSt *btf.Struct,
	progs map[string]*ProgramSpec) error {
	for _, m := range userSt.Members {
		memberOff := m.Offset
		sym, ok := sec.relocations[uint64(baseOff+memberOff.Bytes())]
		if !ok {
			continue
		}
		p, ok := progs[sym.Name]
		if !ok || p == nil {
			return fmt.Errorf("program %s not found", sym.Name)
		}

		if p.Type != StructOps {
			return fmt.Errorf("program %s is not StructOps", sym.Name)
		}
		p.AttachTo = userSt.Name + ":" + m.Name
	}
	return nil
}
