package ebpf

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"slices"

	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/internal/sysenc"
)

// VariableSpec is a convenience wrapper for modifying global variables of a
// CollectionSpec before loading it into the kernel.
//
// All operations on a VariableSpec's underlying MapSpec are performed in the
// host's native endianness.
type VariableSpec struct {
	Name string
	// Name of the section this variable was allocated in.
	SectionName string
	// Offset of the variable within the datasec.
	Offset uint32
	// Byte representation of the variable's value.
	Value []byte
	// Type information of the variable. Optional.
	Type *btf.Var
}

// Set sets the value of the VariableSpec to the provided input using the host's
// native endianness.
func (s *VariableSpec) Set(in any) error {
	size := int(s.Size())
	if size == 0 {
		bs := binary.Size(in)
		if bs < 0 {
			return fmt.Errorf("cannot determine binary size of value %v", in)
		}
		size = bs
	}

	if s.Value == nil {
		s.Value = make([]byte, size)
	}

	buf, err := sysenc.Marshal(in, size)
	if err != nil {
		return fmt.Errorf("marshaling value %s: %w", s.Name, err)
	}

	buf.CopyTo(s.Value)
	return nil
}

// Get writes the value of the VariableSpec to the provided output using the
// host's native endianness.
//
// Returns an error if the variable is not initialized or if the unmarshaling fails.
func (s *VariableSpec) Get(out any) error {
	if s.Value == nil {
		return fmt.Errorf("variable is not initialized")
	}

	if err := sysenc.Unmarshal(out, s.Value); err != nil {
		return fmt.Errorf("unmarshaling value: %w", err)
	}

	return nil
}

// Size returns the size of the variable in bytes.
func (s *VariableSpec) Size() uint32 {
	if s.Value != nil {
		return uint32(len(s.Value))
	}

	if s.Type != nil {
		size, err := btf.Sizeof(s.Type.Type)
		if err != nil {
			return 0
		}
		return uint32(size)
	}

	return 0
}

// Constant returns true if the variable is located in a data section intended
// for constant values.
func (s *VariableSpec) Constant() bool {
	return isConstantDataSection(s.SectionName)
}

func (s *VariableSpec) String() string {
	return fmt.Sprintf("%s (type=%v, section=%s, offset=%d, size=%d)", s.Name, s.Type, s.SectionName, s.Offset, s.Size())
}

// Copy the VariableSpec.
func (s *VariableSpec) Copy() *VariableSpec {
	cpy := *s
	cpy.Value = slices.Clone(s.Value)

	if s.Type != nil {
		cpy.Type = btf.Copy(s.Type).(*btf.Var)
	}

	return &cpy
}

// Variable is a convenience wrapper for modifying global variables of a
// Collection after loading it into the kernel. Operations on a Variable are
// performed using direct memory access, bypassing the BPF map syscall API.
//
// On kernels older than 5.5, most interactions with Variable return
// [ErrNotSupported].
type Variable struct {
	name   string
	offset uint32
	size   uint32
	t      *btf.Var

	mm *Memory
}

func newVariable(name string, offset, size uint32, t *btf.Var, mm *Memory) (*Variable, error) {
	if mm != nil {
		if !mm.bounds(offset, size) {
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
func (v *Variable) Size() uint32 {
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

	if !v.mm.bounds(v.offset, v.size) {
		return fmt.Errorf("variable %s: access out of bounds: %w", v.name, io.EOF)
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

func checkVariable[T any](v *Variable) error {
	if v.ReadOnly() {
		return ErrReadOnly
	}

	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Uintptr && v.size == 8 {
		// uintptr is 8 bytes on 64-bit and 4 on 32-bit. In BPF/BTF, pointers are
		// always 8 bytes. For the sake of portability, allow accessing 8-byte BPF
		// variables as uintptr on 32-bit systems, since the upper 32 bits of the
		// pointer should be zero anyway.
		return nil
	}
	if uintptr(v.size) != t.Size() {
		return fmt.Errorf("can't create %d-byte accessor to %d-byte variable: %w", t.Size(), v.size, ErrInvalidType)
	}

	return nil
}

// VariablePointer returns a pointer to a variable of type T backed by memory
// shared with the BPF program. Requires building the Go application with -tags
// ebpf_unsafe_memory_experiment.
//
// T must contain only fixed-size, non-Go-pointer types: bools, floats,
// (u)int[8-64], arrays, and structs containing them. Structs must embed
// [structs.HostLayout]. [ErrInvalidType] is returned if T is not a valid type.
func VariablePointer[T comparable](v *Variable) (*T, error) {
	if err := checkVariable[T](v); err != nil {
		return nil, fmt.Errorf("variable pointer %s: %w", v.name, err)
	}
	return memoryPointer[T](v.mm, v.offset)
}
