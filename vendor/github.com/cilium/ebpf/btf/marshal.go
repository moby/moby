package btf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"sync"
	"unsafe"

	"github.com/cilium/ebpf/internal"
)

type MarshalOptions struct {
	// Target byte order. Defaults to the system's native endianness.
	Order binary.ByteOrder
	// Remove function linkage information for compatibility with <5.6 kernels.
	StripFuncLinkage bool
	// Replace decl tags with a placeholder for compatibility with <5.16 kernels.
	ReplaceDeclTags bool
	// Replace TypeTags with a placeholder for compatibility with <5.17 kernels.
	ReplaceTypeTags bool
	// Replace Enum64 with a placeholder for compatibility with <6.0 kernels.
	ReplaceEnum64 bool
	// Prevent the "No type found" error when loading BTF without any types.
	PreventNoTypeFound bool
}

// KernelMarshalOptions will generate BTF suitable for the current kernel.
func KernelMarshalOptions() *MarshalOptions {
	return &MarshalOptions{
		Order:              internal.NativeEndian,
		StripFuncLinkage:   haveFuncLinkage() != nil,
		ReplaceDeclTags:    haveDeclTags() != nil,
		ReplaceTypeTags:    haveTypeTags() != nil,
		ReplaceEnum64:      haveEnum64() != nil,
		PreventNoTypeFound: true, // All current kernels require this.
	}
}

// encoder turns Types into raw BTF.
type encoder struct {
	MarshalOptions

	pending internal.Deque[Type]
	strings *stringTableBuilder
	ids     map[Type]TypeID
	visited map[Type]struct{}
	lastID  TypeID
}

var bufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, btfHeaderLen+128)
		return &buf
	},
}

func getByteSlice() *[]byte {
	return bufferPool.Get().(*[]byte)
}

func putByteSlice(buf *[]byte) {
	*buf = (*buf)[:0]
	bufferPool.Put(buf)
}

// Builder turns Types into raw BTF.
//
// The default value may be used and represents an empty BTF blob. Void is
// added implicitly if necessary.
type Builder struct {
	// Explicitly added types.
	types []Type
	// IDs for all added types which the user knows about.
	stableIDs map[Type]TypeID
	// Explicitly added strings.
	strings *stringTableBuilder
	// Deduplication data structure.
	deduper *deduper
}

type BuilderOptions struct {
	// Deduplicate enables type deduplication.
	Deduplicate bool
}

// NewBuilder creates a Builder from a list of types.
//
// It is more efficient than calling [Add] individually.
//
// Returns an error if adding any of the types fails.
func NewBuilder(types []Type, opts *BuilderOptions) (*Builder, error) {
	if opts == nil {
		opts = &BuilderOptions{}
	}

	b := &Builder{
		make([]Type, 0, len(types)),
		make(map[Type]TypeID, len(types)),
		nil,
		nil,
	}

	if opts.Deduplicate {
		b.deduper = newDeduper()
	}

	for _, typ := range types {
		_, err := b.Add(typ)
		if err != nil {
			return nil, fmt.Errorf("add %s: %w", typ, err)
		}
	}

	return b, nil
}

// Empty returns true if neither types nor strings have been added.
func (b *Builder) Empty() bool {
	return len(b.types) == 0 && (b.strings == nil || b.strings.Length() == 0)
}

// Add a Type and allocate a stable ID for it.
//
// Adding the identical Type multiple times is valid and will return the same ID.
//
// See [Type] for details on identity.
func (b *Builder) Add(typ Type) (TypeID, error) {
	if _, ok := typ.(*Void); ok {
		// Equality is weird for void, since it is a zero sized type.
		return 0, nil
	}

	if err := internal.IsNilPointer(typ); err != nil {
		return 0, fmt.Errorf("invalid type: %w", err)
	}

	if b.stableIDs == nil {
		b.stableIDs = make(map[Type]TypeID)
	}

	if b.deduper != nil {
		var err error
		typ, err = b.deduper.deduplicate(typ)
		if err != nil {
			return 0, err
		}
	}

	if ds, ok := typ.(*Datasec); ok {
		if err := datasecResolveWorkaround(b, ds); err != nil {
			return 0, err
		}
	}

	id, ok := b.stableIDs[typ]
	if ok {
		return id, nil
	}

	b.types = append(b.types, typ)

	id = TypeID(len(b.types))
	if int(id) != len(b.types) {
		return 0, fmt.Errorf("no more type IDs")
	}

	b.stableIDs[typ] = id
	return id, nil
}

// Spec marshals the Builder's types and returns a new Spec to query them.
//
// The resulting Spec does not share any state with the Builder, subsequent
// additions to the Builder will not affect the Spec.
func (b *Builder) Spec() (*Spec, error) {
	buf, err := b.Marshal(make([]byte, 0), nil)
	if err != nil {
		return nil, err
	}

	return loadRawSpec(buf, nil)
}

// Marshal encodes all types in the Marshaler into BTF wire format.
//
// opts may be nil.
func (b *Builder) Marshal(buf []byte, opts *MarshalOptions) ([]byte, error) {
	stb := b.strings
	if stb == nil {
		// Assume that most types are named. This makes encoding large BTF like
		// vmlinux a lot cheaper.
		stb = newStringTableBuilder(len(b.types))
	} else {
		// Avoid modifying the Builder's string table.
		stb = b.strings.Copy()
	}

	if opts == nil {
		opts = &MarshalOptions{Order: internal.NativeEndian}
	}

	// Reserve space for the BTF header.
	buf = slices.Grow(buf, btfHeaderLen)[:btfHeaderLen]

	e := encoder{
		MarshalOptions: *opts,
		strings:        stb,
		lastID:         TypeID(len(b.types)),
		visited:        make(map[Type]struct{}, len(b.types)),
		ids:            maps.Clone(b.stableIDs),
	}

	if e.ids == nil {
		e.ids = make(map[Type]TypeID)
	}

	types := b.types
	if len(types) == 0 && stb.Length() > 0 && opts.PreventNoTypeFound {
		// We have strings that need to be written out,
		// but no types (besides the implicit Void).
		// Kernels as recent as v6.7 refuse to load such BTF
		// with a "No type found" error in the log.
		// Fix this by adding a dummy type.
		types = []Type{&Int{Size: 0}}
	}

	// Ensure that types are marshaled in the exact order they were Add()ed.
	// Otherwise the ID returned from Add() won't match.
	e.pending.Grow(len(types))
	for _, typ := range types {
		e.pending.Push(typ)
	}

	buf, err := e.deflatePending(buf)
	if err != nil {
		return nil, err
	}

	length := len(buf)
	typeLen := uint32(length - btfHeaderLen)

	stringLen := e.strings.Length()
	buf = e.strings.AppendEncoded(buf)

	// Fill out the header, and write it out.
	header := &btfHeader{
		Magic:     btfMagic,
		Version:   1,
		Flags:     0,
		HdrLen:    uint32(btfHeaderLen),
		TypeOff:   0,
		TypeLen:   typeLen,
		StringOff: typeLen,
		StringLen: uint32(stringLen),
	}

	_, err = binary.Encode(buf[:btfHeaderLen], e.Order, header)
	if err != nil {
		return nil, fmt.Errorf("write header: %v", err)
	}

	return buf, nil
}

// addString adds a string to the resulting BTF.
//
// Adding the same string multiple times will return the same result.
//
// Returns an identifier into the string table or an error if the string
// contains invalid characters.
func (b *Builder) addString(str string) (uint32, error) {
	if b.strings == nil {
		b.strings = newStringTableBuilder(0)
	}

	return b.strings.Add(str)
}

func (e *encoder) allocateIDs(root Type) error {
	for typ := range postorder(root, e.visited) {
		if _, ok := typ.(*Void); ok {
			continue
		}

		if _, ok := e.ids[typ]; ok {
			continue
		}

		id := e.lastID + 1
		if id < e.lastID {
			return errors.New("type ID overflow")
		}

		e.pending.Push(typ)
		e.ids[typ] = id
		e.lastID = id
	}

	return nil
}

// id returns the ID for the given type or panics with an error.
func (e *encoder) id(typ Type) TypeID {
	if _, ok := typ.(*Void); ok {
		return 0
	}

	id, ok := e.ids[typ]
	if !ok {
		panic(fmt.Errorf("no ID for type %v", typ))
	}

	return id
}

func (e *encoder) deflatePending(buf []byte) ([]byte, error) {
	// Declare root outside of the loop to avoid repeated heap allocations.
	var root Type

	for !e.pending.Empty() {
		root = e.pending.Shift()

		// Allocate IDs for all children of typ, including transitive dependencies.
		err := e.allocateIDs(root)
		if err != nil {
			return nil, err
		}

		buf, err = e.deflateType(buf, root)
		if err != nil {
			id := e.ids[root]
			return nil, fmt.Errorf("deflate %v with ID %d: %w", root, id, err)
		}
	}

	return buf, nil
}

func (e *encoder) deflateType(buf []byte, typ Type) (_ []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				panic(r)
			}
		}
	}()

	var raw btfType
	raw.NameOff, err = e.strings.Add(typ.TypeName())
	if err != nil {
		return nil, err
	}

	// Reserve space for the btfType header.
	start := len(buf)
	buf = append(buf, make([]byte, unsafe.Sizeof(raw))...)

	switch v := typ.(type) {
	case *Void:
		return nil, errors.New("Void is implicit in BTF wire format")

	case *Int:
		buf, err = e.deflateInt(buf, &raw, v)

	case *Pointer:
		raw.SetKind(kindPointer)
		raw.SetType(e.id(v.Target))

	case *Array:
		raw.SetKind(kindArray)
		buf, err = binary.Append(buf, e.Order, &btfArray{
			e.id(v.Type),
			e.id(v.Index),
			v.Nelems,
		})

	case *Struct:
		raw.SetKind(kindStruct)
		raw.SetSize(v.Size)
		buf, err = e.deflateMembers(buf, &raw, v.Members)

	case *Union:
		buf, err = e.deflateUnion(buf, &raw, v)

	case *Enum:
		if v.Size == 8 {
			buf, err = e.deflateEnum64(buf, &raw, v)
		} else {
			buf, err = e.deflateEnum(buf, &raw, v)
		}

	case *Fwd:
		raw.SetKind(kindForward)
		raw.SetFwdKind(v.Kind)

	case *Typedef:
		raw.SetKind(kindTypedef)
		raw.SetType(e.id(v.Type))

	case *Volatile:
		raw.SetKind(kindVolatile)
		raw.SetType(e.id(v.Type))

	case *Const:
		e.deflateConst(&raw, v)

	case *Restrict:
		raw.SetKind(kindRestrict)
		raw.SetType(e.id(v.Type))

	case *Func:
		raw.SetKind(kindFunc)
		raw.SetType(e.id(v.Type))
		if !e.StripFuncLinkage {
			raw.SetLinkage(v.Linkage)
		}

	case *FuncProto:
		raw.SetKind(kindFuncProto)
		raw.SetType(e.id(v.Return))
		raw.SetVlen(len(v.Params))
		buf, err = e.deflateFuncParams(buf, v.Params)

	case *Var:
		raw.SetKind(kindVar)
		raw.SetType(e.id(v.Type))
		buf, err = binary.Append(buf, e.Order, btfVariable{uint32(v.Linkage)})

	case *Datasec:
		raw.SetKind(kindDatasec)
		raw.SetSize(v.Size)
		raw.SetVlen(len(v.Vars))
		buf, err = e.deflateVarSecinfos(buf, v.Vars)

	case *Float:
		raw.SetKind(kindFloat)
		raw.SetSize(v.Size)

	case *declTag:
		buf, err = e.deflateDeclTag(buf, &raw, v)

	case *TypeTag:
		err = e.deflateTypeTag(&raw, v)

	default:
		return nil, fmt.Errorf("don't know how to deflate %T", v)
	}

	if err != nil {
		return nil, err
	}

	header := buf[start : start+int(unsafe.Sizeof(raw))]
	if _, err = raw.Encode(header, e.Order); err != nil {
		return nil, err
	}

	return buf, nil
}

func (e *encoder) deflateInt(buf []byte, raw *btfType, i *Int) ([]byte, error) {
	raw.SetKind(kindInt)
	raw.SetSize(i.Size)

	var bi btfInt
	bi.SetEncoding(i.Encoding)
	// We need to set bits in addition to size, since btf_type_int_is_regular
	// otherwise flags this as a bitfield.
	bi.SetBits(byte(i.Size) * 8)
	return binary.Append(buf, e.Order, bi)
}

func (e *encoder) deflateDeclTag(buf []byte, raw *btfType, tag *declTag) ([]byte, error) {
	// Replace a decl tag with an integer for compatibility with <5.16 kernels,
	// following libbpf behaviour.
	if e.ReplaceDeclTags {
		typ := &Int{"decl_tag_placeholder", 1, Unsigned}
		buf, err := e.deflateInt(buf, raw, typ)
		if err != nil {
			return nil, err
		}

		// Add the placeholder type name to the string table. The encoder added the
		// original type name before this call.
		raw.NameOff, err = e.strings.Add(typ.TypeName())
		return buf, err
	}

	var err error
	raw.SetKind(kindDeclTag)
	raw.SetType(e.id(tag.Type))
	raw.NameOff, err = e.strings.Add(tag.Value)
	if err != nil {
		return nil, err
	}

	return binary.Append(buf, e.Order, btfDeclTag{uint32(tag.Index)})
}

func (e *encoder) deflateConst(raw *btfType, c *Const) {
	raw.SetKind(kindConst)
	raw.SetType(e.id(c.Type))
}

func (e *encoder) deflateTypeTag(raw *btfType, tag *TypeTag) (err error) {
	// Replace a type tag with a const qualifier for compatibility with <5.17
	// kernels, following libbpf behaviour.
	if e.ReplaceTypeTags {
		e.deflateConst(raw, &Const{tag.Type})
		return nil
	}

	raw.SetKind(kindTypeTag)
	raw.SetType(e.id(tag.Type))
	raw.NameOff, err = e.strings.Add(tag.Value)
	return
}

func (e *encoder) deflateUnion(buf []byte, raw *btfType, union *Union) ([]byte, error) {
	raw.SetKind(kindUnion)
	raw.SetSize(union.Size)
	return e.deflateMembers(buf, raw, union.Members)
}

func (e *encoder) deflateMembers(buf []byte, header *btfType, members []Member) ([]byte, error) {
	var bm btfMember
	isBitfield := false

	buf = slices.Grow(buf, len(members)*int(unsafe.Sizeof(bm)))
	for _, member := range members {
		isBitfield = isBitfield || member.BitfieldSize > 0

		offset := member.Offset
		if isBitfield {
			offset = member.BitfieldSize<<24 | (member.Offset & 0xffffff)
		}

		nameOff, err := e.strings.Add(member.Name)
		if err != nil {
			return nil, err
		}

		bm = btfMember{
			nameOff,
			e.id(member.Type),
			uint32(offset),
		}

		buf, err = binary.Append(buf, e.Order, &bm)
		if err != nil {
			return nil, err
		}
	}

	header.SetVlen(len(members))
	header.SetBitfield(isBitfield)
	return buf, nil
}

func (e *encoder) deflateEnum(buf []byte, raw *btfType, enum *Enum) ([]byte, error) {
	raw.SetKind(kindEnum)
	raw.SetSize(enum.Size)
	raw.SetVlen(len(enum.Values))
	// Signedness appeared together with ENUM64 support.
	raw.SetSigned(enum.Signed && !e.ReplaceEnum64)
	return e.deflateEnumValues(buf, enum)
}

func (e *encoder) deflateEnumValues(buf []byte, enum *Enum) ([]byte, error) {
	var be btfEnum
	buf = slices.Grow(buf, len(enum.Values)*int(unsafe.Sizeof(be)))
	for _, value := range enum.Values {
		nameOff, err := e.strings.Add(value.Name)
		if err != nil {
			return nil, err
		}

		if enum.Signed {
			if signedValue := int64(value.Value); signedValue < math.MinInt32 || signedValue > math.MaxInt32 {
				return nil, fmt.Errorf("value %d of enum %q exceeds 32 bits", signedValue, value.Name)
			}
		} else {
			if value.Value > math.MaxUint32 {
				return nil, fmt.Errorf("value %d of enum %q exceeds 32 bits", value.Value, value.Name)
			}
		}

		be = btfEnum{
			nameOff,
			uint32(value.Value),
		}

		buf, err = binary.Append(buf, e.Order, &be)
		if err != nil {
			return nil, err
		}
	}

	return buf, nil
}

func (e *encoder) deflateEnum64(buf []byte, raw *btfType, enum *Enum) ([]byte, error) {
	if e.ReplaceEnum64 {
		// Replace the ENUM64 with a union of fields with the correct size.
		// This matches libbpf behaviour on purpose.
		placeholder := &Int{
			"enum64_placeholder",
			enum.Size,
			Unsigned,
		}
		if enum.Signed {
			placeholder.Encoding = Signed
		}
		if err := e.allocateIDs(placeholder); err != nil {
			return nil, fmt.Errorf("add enum64 placeholder: %w", err)
		}

		members := make([]Member, 0, len(enum.Values))
		for _, v := range enum.Values {
			members = append(members, Member{
				Name: v.Name,
				Type: placeholder,
			})
		}

		return e.deflateUnion(buf, raw, &Union{enum.Name, enum.Size, members, nil})
	}

	raw.SetKind(kindEnum64)
	raw.SetSize(enum.Size)
	raw.SetVlen(len(enum.Values))
	raw.SetSigned(enum.Signed)
	return e.deflateEnum64Values(buf, enum.Values)
}

func (e *encoder) deflateEnum64Values(buf []byte, values []EnumValue) ([]byte, error) {
	var be btfEnum64
	buf = slices.Grow(buf, len(values)*int(unsafe.Sizeof(be)))
	for _, value := range values {
		nameOff, err := e.strings.Add(value.Name)
		if err != nil {
			return nil, err
		}

		be = btfEnum64{
			nameOff,
			uint32(value.Value),
			uint32(value.Value >> 32),
		}

		buf, err = binary.Append(buf, e.Order, &be)
		if err != nil {
			return nil, err
		}
	}

	return buf, nil
}

func (e *encoder) deflateFuncParams(buf []byte, params []FuncParam) ([]byte, error) {
	var bp btfParam
	buf = slices.Grow(buf, len(params)*int(unsafe.Sizeof(bp)))
	for _, param := range params {
		nameOff, err := e.strings.Add(param.Name)
		if err != nil {
			return nil, err
		}

		bp = btfParam{
			nameOff,
			e.id(param.Type),
		}

		buf, err = binary.Append(buf, e.Order, &bp)
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func (e *encoder) deflateVarSecinfos(buf []byte, vars []VarSecinfo) ([]byte, error) {
	var vsi btfVarSecinfo
	var err error
	buf = slices.Grow(buf, len(vars)*int(unsafe.Sizeof(vsi)))
	for _, v := range vars {
		vsi = btfVarSecinfo{
			e.id(v.Type),
			v.Offset,
			v.Size,
		}

		buf, err = binary.Append(buf, e.Order, vsi)
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

// MarshalMapKV creates a BTF object containing a map key and value.
//
// The function is intended for the use of the ebpf package and may be removed
// at any point in time.
func MarshalMapKV(key, value Type) (_ *Handle, keyID, valueID TypeID, err error) {
	var b Builder

	if key != nil {
		keyID, err = b.Add(key)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("add key type: %w", err)
		}
	}

	if value != nil {
		valueID, err = b.Add(value)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("add value type: %w", err)
		}
	}

	handle, err := NewHandle(&b)
	if err != nil {
		// Check for 'full' map BTF support, since kernels between 4.18 and 5.2
		// already support BTF blobs for maps without Var or Datasec just fine.
		if err := haveMapBTF(); err != nil {
			return nil, 0, 0, err
		}
	}
	return handle, keyID, valueID, err
}
