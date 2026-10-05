package btf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/maphash"
	"io"
	"iter"
	"maps"
	"math"
	"slices"
	"sync"
)

// sharedBuf is a buffer which may be shared between multiple decoders.
//
// It must not be modified. Some sharedBuf may be backed by an mmap-ed file, in
// which case the sharedBuf has a finalizer. sharedBuf must therefore always be
// passed as a pointer.
type sharedBuf struct {
	raw []byte
}

type decoder struct {
	// Immutable fields, may be shared.

	base      *decoder
	byteOrder binary.ByteOrder
	*sharedBuf
	strings *stringTable
	// The ID for offsets[0].
	firstTypeID TypeID
	// Map from TypeID to offset of the marshaled data in raw. Contains an entry
	// for each TypeID, including 0 aka Void. The offset for Void is invalid.
	offsets  []int
	declTags map[TypeID][]TypeID
	// An index from essentialName to TypeID.
	namedTypes *fuzzyStringIndex

	// Protection for mutable fields below.
	mu              sync.Mutex
	types           map[TypeID]Type
	typeIDs         map[Type]TypeID
	legacyBitfields map[TypeID][2]Bits // offset, size
}

func newDecoder(raw []byte, bo binary.ByteOrder, strings *stringTable, base *decoder) (*decoder, error) {
	firstTypeID := TypeID(0)
	if base != nil {
		if base.byteOrder != bo {
			return nil, fmt.Errorf("can't use %v base with %v split BTF", base.byteOrder, bo)
		}

		if base.firstTypeID != 0 {
			return nil, fmt.Errorf("can't use split BTF as base")
		}

		firstTypeID = TypeID(len(base.offsets))
	}

	var header btfType
	var numTypes, numDeclTags, numNamedTypes int

	for _, err := range allBtfTypeOffsets(raw, bo, &header) {
		if err != nil {
			return nil, err
		}

		numTypes++

		if header.Kind() == kindDeclTag {
			numDeclTags++
		}

		if header.NameOff != 0 {
			numNamedTypes++
		}
	}

	if firstTypeID == 0 {
		// Allocate an extra slot for Void so we don't have to deal with
		// constant off by one issues.
		numTypes++
	}

	offsets := make([]int, 0, numTypes)
	declTags := make(map[TypeID][]TypeID, numDeclTags)
	namedTypes := newFuzzyStringIndex(numNamedTypes)

	if firstTypeID == 0 {
		// Add a sentinel for Void.
		offsets = append(offsets, math.MaxInt)
	}

	id := firstTypeID + TypeID(len(offsets))
	for offset := range allBtfTypeOffsets(raw, bo, &header) {
		if id < firstTypeID {
			return nil, fmt.Errorf("no more type IDs")
		}

		offsets = append(offsets, offset)

		if header.Kind() == kindDeclTag {
			declTags[header.Type()] = append(declTags[header.Type()], id)
		}

		// Build named type index.
		name, err := strings.LookupBytes(header.NameOff)
		if err != nil {
			return nil, fmt.Errorf("lookup type name for id %v: %w", id, err)
		}

		if len(name) > 0 {
			if i := bytes.Index(name, []byte("___")); i != -1 {
				// Flavours are rare. It's cheaper to find the first index for some
				// reason.
				i = bytes.LastIndex(name, []byte("___"))
				name = name[:i]
			}

			namedTypes.Add(name, id)
		}

		id++
	}

	namedTypes.Build()

	return &decoder{
		base,
		bo,
		&sharedBuf{raw},
		strings,
		firstTypeID,
		offsets,
		declTags,
		namedTypes,
		sync.Mutex{},
		make(map[TypeID]Type),
		make(map[Type]TypeID),
		make(map[TypeID][2]Bits),
	}, nil
}

func allBtfTypeOffsets(buf []byte, bo binary.ByteOrder, header *btfType) iter.Seq2[int, error] {
	return func(yield func(int, error) bool) {
		for offset := 0; offset < len(buf); {
			start := offset

			n, err := unmarshalBtfType(header, buf[offset:], bo)
			if err != nil {
				yield(-1, fmt.Errorf("unmarshal type header: %w", err))
				return
			}
			offset += n

			n, err = header.DataLen()
			if err != nil {
				yield(-1, err)
				return
			}
			offset += n

			if offset > len(buf) {
				yield(-1, fmt.Errorf("auxiliary type data: %w", io.ErrUnexpectedEOF))
				return
			}

			if !yield(start, nil) {
				return
			}
		}
	}
}

// Copy performs a deep copy of a decoder and its base.
func (d *decoder) Copy() *decoder {
	if d == nil {
		return nil
	}

	return d.copy(nil)
}

func (d *decoder) copy(copiedTypes map[Type]Type) *decoder {
	if d == nil {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if copiedTypes == nil {
		copiedTypes = make(map[Type]Type, len(d.types))
	}

	types := make(map[TypeID]Type, len(d.types))
	typeIDs := make(map[Type]TypeID, len(d.typeIDs))
	for id, typ := range d.types {
		types[id] = copyType(typ, d.typeIDs, copiedTypes, typeIDs)
	}

	return &decoder{
		d.base.copy(copiedTypes),
		d.byteOrder,
		d.sharedBuf,
		d.strings,
		d.firstTypeID,
		d.offsets,
		d.declTags,
		d.namedTypes,
		sync.Mutex{},
		types,
		typeIDs,
		maps.Clone(d.legacyBitfields),
	}
}

// TypeID returns the ID for a Type previously obtained via [TypeByID].
func (d *decoder) TypeID(typ Type) (TypeID, error) {
	if _, ok := typ.(*Void); ok {
		// Equality is weird for void, since it is a zero sized type.
		return 0, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	id, ok := d.typeIDs[typ]
	if !ok {
		return 0, fmt.Errorf("no ID for type %s: %w", typ, ErrNotFound)
	}

	return id, nil
}

// TypesByName returns all types which have the given essential name.
//
// Returns ErrNotFound if no matching Type exists.
func (d *decoder) TypesByName(name essentialName) ([]Type, error) {
	var types []Type
	for id := range d.namedTypes.Find(string(name)) {
		typ, err := d.TypeByID(id)
		if err != nil {
			return nil, err
		}

		if newEssentialName(typ.TypeName()) == name {
			// Deal with hash collisions by checking against the name.
			types = append(types, typ)
		}
	}

	if len(types) == 0 {
		// Return an unwrapped error because this is on the hot path
		// for CO-RE.
		return nil, ErrNotFound
	}

	return types, nil
}

// TypeByID decodes a type and any of its descendants.
func (d *decoder) TypeByID(id TypeID) (Type, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.inflateType(id)
}

func (d *decoder) inflateType(id TypeID) (typ Type, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}

		// err is the return value of the enclosing function, even if an explicit
		// return is used.
		// See https://go.dev/ref/spec#Defer_statements
		if err != nil {
			// Remove partially inflated type so that d.types only contains
			// fully inflated ones.
			delete(d.types, id)
		} else {
			// Populate reverse index.
			d.typeIDs[typ] = id
		}
	}()

	if id < d.firstTypeID {
		return d.base.inflateType(id)
	}

	if id == 0 {
		// Void is defined to always be type ID 0, and is thus omitted from BTF.
		// Fast-path because it is looked up frequently.
		return (*Void)(nil), nil
	}

	if typ, ok := d.types[id]; ok {
		return typ, nil
	}

	fixup := func(id TypeID, typ *Type) {
		fixup, err := d.inflateType(id)
		if err != nil {
			panic(err)
		}
		*typ = fixup
	}

	convertMembers := func(header *btfType, buf []byte) ([]Member, error) {
		var bm btfMember
		members := make([]Member, 0, header.Vlen())
		for i := range header.Vlen() {
			n, err := unmarshalBtfMember(&bm, buf, d.byteOrder)
			if err != nil {
				return nil, fmt.Errorf("unmarshal member: %w", err)
			}
			buf = buf[n:]

			name, err := d.strings.Lookup(bm.NameOff)
			if err != nil {
				return nil, fmt.Errorf("can't get name for member %d: %w", i, err)
			}

			members = append(members, Member{
				Name:   name,
				Offset: Bits(bm.Offset),
			})

			m := &members[i]
			fixup(bm.Type, &m.Type)

			if header.Bitfield() {
				m.BitfieldSize = Bits(bm.Offset >> 24)
				m.Offset &= 0xffffff
				// We ignore legacy bitfield definitions if the current composite
				// is a new-style bitfield. This is kind of safe since offset and
				// size on the type of the member must be zero if kindFlat is set
				// according to spec.
				continue
			}

			// This may be a legacy bitfield, try to fix it up.
			data, ok := d.legacyBitfields[bm.Type]
			if ok {
				// Bingo!
				m.Offset += data[0]
				m.BitfieldSize = data[1]
				continue
			}
		}
		return members, nil
	}

	idx := int(id - d.firstTypeID)
	if idx >= len(d.offsets) {
		return nil, fmt.Errorf("type id %v: %w", id, ErrNotFound)
	}

	offset := d.offsets[idx]
	if offset >= len(d.raw) {
		return nil, fmt.Errorf("offset out of bounds")
	}

	var (
		header    btfType
		bInt      btfInt
		bArr      btfArray
		bVariable btfVariable
		bDeclTag  btfDeclTag
		pos       = d.raw[offset:]
	)

	{
		if n, err := unmarshalBtfType(&header, pos, d.byteOrder); err != nil {
			return nil, fmt.Errorf("can't unmarshal type info for id %v: %v", id, err)
		} else {
			pos = pos[n:]
		}

		name, err := d.strings.Lookup(header.NameOff)
		if err != nil {
			return nil, fmt.Errorf("get name for type id %d: %w", id, err)
		}

		switch header.Kind() {
		case kindInt:
			size := header.Size()
			if _, err := unmarshalBtfInt(&bInt, pos, d.byteOrder); err != nil {
				return nil, fmt.Errorf("can't unmarshal btfInt, id: %d: %w", id, err)
			}
			if bInt.Offset() > 0 || bInt.Bits().Bytes() != size {
				d.legacyBitfields[id] = [2]Bits{bInt.Offset(), bInt.Bits()}
			}
			typ = &Int{name, header.Size(), bInt.Encoding()}
			d.types[id] = typ

		case kindPointer:
			ptr := &Pointer{nil}
			d.types[id] = ptr

			fixup(header.Type(), &ptr.Target)
			typ = ptr

		case kindArray:
			if _, err := unmarshalBtfArray(&bArr, pos, d.byteOrder); err != nil {
				return nil, fmt.Errorf("can't unmarshal btfArray, id: %d: %w", id, err)
			}

			arr := &Array{nil, nil, bArr.Nelems}
			d.types[id] = arr

			fixup(bArr.IndexType, &arr.Index)
			fixup(bArr.Type, &arr.Type)
			typ = arr

		case kindStruct:
			str := &Struct{name, header.Size(), nil, nil}
			d.types[id] = str
			typ = str

			str.Members, err = convertMembers(&header, pos)
			if err != nil {
				return nil, fmt.Errorf("struct %s (id %d): %w", name, id, err)
			}

		case kindUnion:
			uni := &Union{name, header.Size(), nil, nil}
			d.types[id] = uni
			typ = uni

			uni.Members, err = convertMembers(&header, pos)
			if err != nil {
				return nil, fmt.Errorf("union %s (id %d): %w", name, id, err)
			}

		case kindEnum:
			enum := &Enum{name, header.Size(), header.Signed(), nil}
			d.types[id] = enum
			typ = enum

			var be btfEnum
			enum.Values = make([]EnumValue, 0, header.Vlen())
			for i := range header.Vlen() {
				n, err := unmarshalBtfEnum(&be, pos, d.byteOrder)
				if err != nil {
					return nil, fmt.Errorf("unmarshal btfEnum %d, id: %d: %w", i, id, err)
				}
				pos = pos[n:]

				name, err := d.strings.Lookup(be.NameOff)
				if err != nil {
					return nil, fmt.Errorf("get name for enum value %d: %s", i, err)
				}

				value := uint64(be.Val)
				if enum.Signed {
					// Sign extend values to 64 bit.
					value = uint64(int32(be.Val))
				}
				enum.Values = append(enum.Values, EnumValue{name, value})
			}

		case kindForward:
			typ = &Fwd{name, header.FwdKind()}
			d.types[id] = typ

		case kindTypedef:
			typedef := &Typedef{name, nil, nil}
			d.types[id] = typedef

			fixup(header.Type(), &typedef.Type)
			typ = typedef

		case kindVolatile:
			volatile := &Volatile{nil}
			d.types[id] = volatile

			fixup(header.Type(), &volatile.Type)
			typ = volatile

		case kindConst:
			cnst := &Const{nil}
			d.types[id] = cnst

			fixup(header.Type(), &cnst.Type)
			typ = cnst

		case kindRestrict:
			restrict := &Restrict{nil}
			d.types[id] = restrict

			fixup(header.Type(), &restrict.Type)
			typ = restrict

		case kindFunc:
			fn := &Func{name, nil, header.Linkage(), nil, nil}
			d.types[id] = fn

			fixup(header.Type(), &fn.Type)
			typ = fn

		case kindFuncProto:
			fp := &FuncProto{}
			d.types[id] = fp

			params := make([]FuncParam, 0, header.Vlen())
			var bParam btfParam
			for i := range header.Vlen() {
				n, err := unmarshalBtfParam(&bParam, pos, d.byteOrder)
				if err != nil {
					return nil, fmt.Errorf("can't unmarshal btfParam %d, id: %d: %w", i, id, err)
				}
				pos = pos[n:]

				name, err := d.strings.Lookup(bParam.NameOff)
				if err != nil {
					return nil, fmt.Errorf("get name for func proto parameter %d: %s", i, err)
				}

				param := FuncParam{Name: name}
				fixup(bParam.Type, &param.Type)
				params = append(params, param)
			}

			fixup(header.Type(), &fp.Return)
			fp.Params = params
			typ = fp

		case kindVar:
			if _, err := unmarshalBtfVariable(&bVariable, pos, d.byteOrder); err != nil {
				return nil, fmt.Errorf("can't read btfVariable, id: %d: %w", id, err)
			}

			v := &Var{name, nil, VarLinkage(bVariable.Linkage), nil}
			d.types[id] = v

			fixup(header.Type(), &v.Type)
			typ = v

		case kindDatasec:
			ds := &Datasec{name, header.Size(), nil}
			d.types[id] = ds

			vlen := header.Vlen()
			vars := make([]VarSecinfo, 0, vlen)
			var bSecInfo btfVarSecinfo
			for i := range vlen {
				n, err := unmarshalBtfVarSecInfo(&bSecInfo, pos, d.byteOrder)
				if err != nil {
					return nil, fmt.Errorf("can't unmarshal btfVarSecinfo %d, id: %d: %w", i, id, err)
				}
				pos = pos[n:]

				vs := VarSecinfo{
					Offset: bSecInfo.Offset,
					Size:   bSecInfo.Size,
				}
				fixup(bSecInfo.Type, &vs.Type)
				vars = append(vars, vs)
			}
			ds.Vars = vars
			typ = ds

		case kindFloat:
			typ = &Float{name, header.Size()}
			d.types[id] = typ

		case kindDeclTag:
			if _, err := unmarshalBtfDeclTag(&bDeclTag, pos, d.byteOrder); err != nil {
				return nil, fmt.Errorf("can't read btfDeclTag, id: %d: %w", id, err)
			}

			btfIndex := bDeclTag.ComponentIdx
			if uint64(btfIndex) > math.MaxInt {
				return nil, fmt.Errorf("type id %d: index exceeds int", id)
			}

			dt := &declTag{nil, name, int(int32(btfIndex))}
			d.types[id] = dt

			fixup(header.Type(), &dt.Type)
			typ = dt

		case kindTypeTag:
			tt := &TypeTag{nil, name}
			d.types[id] = tt

			fixup(header.Type(), &tt.Type)
			typ = tt

		case kindEnum64:
			enum := &Enum{name, header.Size(), header.Signed(), nil}
			d.types[id] = enum
			typ = enum

			enum.Values = make([]EnumValue, 0, header.Vlen())
			var bEnum64 btfEnum64
			for i := range header.Vlen() {
				n, err := unmarshalBtfEnum64(&bEnum64, pos, d.byteOrder)
				if err != nil {
					return nil, fmt.Errorf("can't unmarshal btfEnum64 %d, id: %d: %w", i, id, err)
				}
				pos = pos[n:]

				name, err := d.strings.Lookup(bEnum64.NameOff)
				if err != nil {
					return nil, fmt.Errorf("get name for enum64 value %d: %s", i, err)
				}
				value := (uint64(bEnum64.ValHi32) << 32) | uint64(bEnum64.ValLo32)
				enum.Values = append(enum.Values, EnumValue{name, value})
			}

		default:
			return nil, fmt.Errorf("type id %d: unknown kind: %v", id, header.Kind())
		}
	}

	for _, tagID := range d.declTags[id] {
		dtType, err := d.inflateType(tagID)
		if err != nil {
			return nil, err
		}

		dt, ok := dtType.(*declTag)
		if !ok {
			return nil, fmt.Errorf("type id %v: not a declTag", tagID)
		}

		switch t := typ.(type) {
		case *Var:
			if dt.Index != -1 {
				return nil, fmt.Errorf("type %s: component idx %d is not -1", dt, dt.Index)
			}
			t.Tags = append(t.Tags, dt.Value)

		case *Typedef:
			if dt.Index != -1 {
				return nil, fmt.Errorf("type %s: component idx %d is not -1", dt, dt.Index)
			}
			t.Tags = append(t.Tags, dt.Value)

		case composite:
			if dt.Index >= 0 {
				members := t.members()
				if dt.Index >= len(members) {
					return nil, fmt.Errorf("type %s: component idx %d exceeds members of %s", dt, dt.Index, t)
				}

				members[dt.Index].Tags = append(members[dt.Index].Tags, dt.Value)
			} else if dt.Index == -1 {
				switch t2 := t.(type) {
				case *Struct:
					t2.Tags = append(t2.Tags, dt.Value)
				case *Union:
					t2.Tags = append(t2.Tags, dt.Value)
				}
			} else {
				return nil, fmt.Errorf("type %s: decl tag for type %s has invalid component idx", dt, t)
			}

		case *Func:
			fp, ok := t.Type.(*FuncProto)
			if !ok {
				return nil, fmt.Errorf("type %s: %s is not a FuncProto", dt, t.Type)
			}

			// Ensure the number of argument tag lists equals the number of arguments
			if len(t.ParamTags) == 0 {
				t.ParamTags = make([][]string, len(fp.Params))
			}

			if dt.Index >= 0 {
				if dt.Index >= len(fp.Params) {
					return nil, fmt.Errorf("type %s: component idx %d exceeds params of %s", dt, dt.Index, t)
				}

				t.ParamTags[dt.Index] = append(t.ParamTags[dt.Index], dt.Value)
			} else if dt.Index == -1 {
				t.Tags = append(t.Tags, dt.Value)
			} else {
				return nil, fmt.Errorf("type %s: decl tag for type %s has invalid component idx", dt, t)
			}

		default:
			return nil, fmt.Errorf("type %s: decl tag for type %s is not supported", dt, t)
		}
	}

	return typ, nil
}

// An index from string to TypeID.
//
// Fuzzy because it may return false positive matches.
type fuzzyStringIndex struct {
	seed    maphash.Seed
	entries []fuzzyStringIndexEntry
}

func newFuzzyStringIndex(capacity int) *fuzzyStringIndex {
	return &fuzzyStringIndex{
		maphash.MakeSeed(),
		make([]fuzzyStringIndexEntry, 0, capacity),
	}
}

// Add a string to the index.
//
// Calling the method with identical arguments will create duplicate entries.
func (idx *fuzzyStringIndex) Add(name []byte, id TypeID) {
	hash := uint32(maphash.Bytes(idx.seed, name))
	idx.entries = append(idx.entries, newFuzzyStringIndexEntry(hash, id))
}

// Build the index.
//
// Must be called after [Add] and before [Match].
func (idx *fuzzyStringIndex) Build() {
	slices.Sort(idx.entries)
}

// Find TypeIDs which may match the name.
//
// May return false positives, but is guaranteed to not have false negatives.
//
// You must call [Build] at least once before calling this method.
func (idx *fuzzyStringIndex) Find(name string) iter.Seq[TypeID] {
	return func(yield func(TypeID) bool) {
		hash := uint32(maphash.String(idx.seed, name))

		// We match only on the first 32 bits here, so ignore found.
		i, _ := slices.BinarySearch(idx.entries, fuzzyStringIndexEntry(hash)<<32)
		for i := i; i < len(idx.entries); i++ {
			if idx.entries[i].hash() != hash {
				break
			}

			if !yield(idx.entries[i].id()) {
				return
			}
		}
	}
}

// Tuple mapping the hash of an essential name to a type.
//
// Encoded in an uint64 so that it implements cmp.Ordered.
type fuzzyStringIndexEntry uint64

func newFuzzyStringIndexEntry(hash uint32, id TypeID) fuzzyStringIndexEntry {
	return fuzzyStringIndexEntry(hash)<<32 | fuzzyStringIndexEntry(id)
}

func (e fuzzyStringIndexEntry) hash() uint32 {
	return uint32(e >> 32)
}

func (e fuzzyStringIndexEntry) id() TypeID {
	return TypeID(e)
}
