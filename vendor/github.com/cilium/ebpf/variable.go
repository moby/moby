package ebpf

import (
	"fmt"
	"io"

	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/internal/sysenc"
)

// VariableSpec is a convenience wrapper for modifying global variables of a
// CollectionSpec before loading it into the kernel.
//
// All operations on a VariableSpec's underlying MapSpec are performed in the
// host's native endianness.
type VariableSpec struct {
	name   string
	offset uint64
	size   uint64

	m *MapSpec
	t *btf.Var
}

// Set sets the value of the VariableSpec to the provided input using the host's
// native endianness.
func (s *VariableSpec) Set(in any) error {
	buf, err := sysenc.Marshal(in, int(s.size))
	if err != nil {
		return fmt.Errorf("marshaling value %s: %w", s.name, err)
	}

	b, _, err := s.m.dataSection()
	if err != nil {
		return fmt.Errorf("getting data section of map %s: %w", s.m.Name, err)
	}

	if int(s.offset+s.size) > len(b) {
		return fmt.Errorf("offset %d(+%d) for variable %s is out of bounds", s.offset, s.size, s.name)
	}

	// MapSpec.Copy() performs a shallow copy. Fully copy the byte slice
	// to avoid any changes affecting other copies of the MapSpec.
	cpy := make([]byte, len(b))
	copy(cpy, b)

	buf.CopyTo(cpy[s.offset : s.offset+s.size])

	s.m.Contents[0] = MapKV{Key: uint32(0), Value: cpy}

	return nil
}

// Get writes the value of the VariableSpec to the provided output using the
// host's native endianness.
func (s *VariableSpec) Get(out any) error {
	b, _, err := s.m.dataSection()
	if err != nil {
		return fmt.Errorf("getting data section of map %s: %w", s.m.Name, err)
	}

	if int(s.offset+s.size) > len(b) {
		return fmt.Errorf("offset %d(+%d) for variable %s is out of bounds", s.offset, s.size, s.name)
	}

	if err := sysenc.Unmarshal(out, b[s.offset:s.offset+s.size]); err != nil {
		return fmt.Errorf("unmarshaling value: %w", err)
	}

	return nil
}

// Size returns the size of the variable in bytes.
func (s *VariableSpec) Size() uint64 {
	return s.size
}

// MapName returns the name of the underlying MapSpec.
func (s *VariableSpec) MapName() string {
	return s.m.Name
}

// Offset returns the offset of the variable in the underlying MapSpec.
func (s *VariableSpec) Offset() uint64 {
	return s.offset
}

// Constant returns true if the VariableSpec represents a variable that is
// read-only from the perspective of the BPF program.
func (s *VariableSpec) Constant() bool {
	return s.m.readOnly()
}

// Type returns the [btf.Var] representing the variable in its data section.
// This is useful for inspecting the variable's decl tags and the type
// information of the inner type.
//
// Returns nil if the original ELF object did not contain BTF information.
func (s *VariableSpec) Type() *btf.Var {
	return s.t
}

func (s *VariableSpec) String() string {
	return fmt.Sprintf("%s (type=%v, map=%s, offset=%d, size=%d)", s.name, s.t, s.m.Name, s.offset, s.size)
}

// copy returns a new VariableSpec with the same values as the original,
// but with a different underlying MapSpec. This is useful when copying a
// CollectionSpec. Returns nil if a MapSpec with the same name is not found.
func (s *VariableSpec) copy(cpy *CollectionSpec) *VariableSpec {
	out := &VariableSpec{
		name:   s.name,
		offset: s.offset,
		size:   s.size,
	}
	if s.t != nil {
		out.t = btf.Copy(s.t).(*btf.Var)
	}

	// Attempt to find a MapSpec with the same name in the copied CollectionSpec.
	for _, m := range cpy.Maps {
		if m.Name == s.m.Name {
			out.m = m
			return out
		}
	}

	return nil
}

// Variable is a convenience wrapper for modifying global variables of a
// Collection after loading it into the kernel. Operations on a Variable are
// performed using direct memory access, bypassing the BPF map syscall API.
//
// On kernels older than 5.5, most interactions with Variable return
// [ErrNotSupported].
type Variable struct {
	name   string
	offset uint64
	size   uint64
	t      *btf.Var

	mm *Memory
}

func newVariable(name string, offset, size uint64, t *btf.Var, mm *Memory) (*Variable, error) {
	if mm != nil {
		if int(offset+size) > mm.Size() {
			return nil, fmt.Errorf("offset %d(+%d) is out of bounds", offset, size)
		}
	}

	return &Variable{
		name:   name,
		offset: offset,
		size:   size,
		t:      t,
		mm:     mm,
	}, nil
}

// Size returns the size of the variable.
func (v *Variable) Size() uint64 {
	return v.size
}

// ReadOnly returns true if the Variable represents a variable that is read-only
// after loading the Collection into the kernel.
//
// On systems without BPF_F_MMAPABLE support, ReadOnly always returns true.
func (v *Variable) ReadOnly() bool {
	if v.mm == nil {
		return true
	}
	return v.mm.ReadOnly()
}

// Type returns the [btf.Var] representing the variable in its data section.
// This is useful for inspecting the variable's decl tags and the type
// information of the inner type.
//
// Returns nil if the original ELF object did not contain BTF information.
func (v *Variable) Type() *btf.Var {
	return v.t
}

func (v *Variable) String() string {
	return fmt.Sprintf("%s (type=%v)", v.name, v.t)
}

// Set the value of the Variable to the provided input. The input must marshal
// to the same length as the size of the Variable.
func (v *Variable) Set(in any) error {
	if v.mm == nil {
		return fmt.Errorf("variable %s: direct access requires Linux 5.5 or later: %w", v.name, ErrNotSupported)
	}

	if v.ReadOnly() {
		return fmt.Errorf("variable %s: %w", v.name, ErrReadOnly)
	}

	buf, err := sysenc.Marshal(in, int(v.size))
	if err != nil {
		return fmt.Errorf("marshaling value %s: %w", v.name, err)
	}

	if _, err := v.mm.WriteAt(buf.Bytes(), int64(v.offset)); err != nil {
		return fmt.Errorf("writing value to %s: %w", v.name, err)
	}

	return nil
}

// Get writes the value of the Variable to the provided output. The output must
// be a pointer to a value whose size matches the Variable.
func (v *Variable) Get(out any) error {
	if v.mm == nil {
		return fmt.Errorf("variable %s: direct access requires Linux 5.5 or later: %w", v.name, ErrNotSupported)
	}

	if !v.mm.bounds(v.offset, v.size) {
		return fmt.Errorf("variable %s: access out of bounds: %w", v.name, io.EOF)
	}

	if err := sysenc.Unmarshal(out, v.mm.b[v.offset:v.offset+v.size]); err != nil {
		return fmt.Errorf("unmarshaling value %s: %w", v.name, err)
	}

	return nil
}
