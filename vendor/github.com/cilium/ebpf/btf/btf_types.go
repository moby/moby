package btf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unsafe"
)

//go:generate go tool stringer -linecomment -output=btf_types_string.go -type=FuncLinkage,VarLinkage,btfKind

// btfKind describes a Type.
type btfKind uint8

// Equivalents of the BTF_KIND_* constants.
const (
	kindUnknown  btfKind = iota // Unknown
	kindInt                     // Int
	kindPointer                 // Pointer
	kindArray                   // Array
	kindStruct                  // Struct
	kindUnion                   // Union
	kindEnum                    // Enum
	kindForward                 // Forward
	kindTypedef                 // Typedef
	kindVolatile                // Volatile
	kindConst                   // Const
	kindRestrict                // Restrict
	// Added ~4.20
	kindFunc      // Func
	kindFuncProto // FuncProto
	// Added ~5.1
	kindVar     // Var
	kindDatasec // Datasec
	// Added ~5.13
	kindFloat // Float
	// Added 5.16
	kindDeclTag // DeclTag
	// Added 5.17
	kindTypeTag // TypeTag
	// Added 6.0
	kindEnum64 // Enum64
)

// FuncLinkage describes BTF function linkage metadata.
type FuncLinkage int

// Equivalent of enum btf_func_linkage.
const (
	StaticFunc FuncLinkage = iota // static
	GlobalFunc                    // global
	ExternFunc                    // extern
)

// VarLinkage describes BTF variable linkage metadata.
type VarLinkage int

const (
	StaticVar VarLinkage = iota // static
	GlobalVar                   // global
	ExternVar                   // extern
)

const (
	btfTypeKindShift     = 24
	btfTypeKindLen       = 5
	btfTypeVlenShift     = 0
	btfTypeVlenMask      = 16
	btfTypeKindFlagShift = 31
	btfTypeKindFlagMask  = 1
)

var btfHeaderLen = binary.Size(&btfHeader{})

type btfHeader struct {
	Magic   uint16
	Version uint8
	Flags   uint8
	HdrLen  uint32

	TypeOff   uint32
	TypeLen   uint32
	StringOff uint32
	StringLen uint32
}

type btfLayout struct {
	Off uint32
	Len uint32
}

// parseBTFHeader parses the header of the .BTF section.
func parseBTFHeader(buf []byte) (*btfHeader, *btfLayout, binary.ByteOrder, error) {
	var header btfHeader
	var bo binary.ByteOrder
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		n, err := binary.Decode(buf, order, &header)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read header: %v", err)
		}

		if header.Magic != btfMagic {
			continue
		}

		buf = buf[n:]
		bo = order
		break
	}

	if bo == nil {
		return nil, nil, nil, fmt.Errorf("no valid BTF header")
	}

	if header.Version != 1 {
		return nil, nil, nil, fmt.Errorf("unexpected version %v", header.Version)
	}

	if header.Flags != 0 {
		return nil, nil, nil, fmt.Errorf("unsupported flags %v", header.Flags)
	}

	remainder := int64(header.HdrLen) - int64(binary.Size(&header))
	if remainder < 0 {
		return nil, nil, nil, errors.New("header length shorter than minimum BTF header size")
	}

	if len(buf) < int(remainder) {
		return nil, nil, nil, errors.New("header length exceeds available data")
	}

	var layout btfLayout
	if remainder >= int64(binary.Size(&btfLayout{})) {
		n, err := binary.Decode(buf, bo, &layout)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read layout offset and length: %v", err)
		}
		buf = buf[n:]
		remainder -= int64(n)
	}

	for _, b := range buf[:remainder] {
		if b != 0 {
			return nil, nil, nil, errors.New("header contains non-zero trailer")
		}
	}

	return &header, &layout, bo, nil
}

// btfType is equivalent to struct btf_type in Documentation/bpf/btf.rst.
type btfType struct {
	NameOff uint32
	/* "info" bits arrangement
	 * bits  0-15: vlen (e.g. # of struct's members), linkage
	 * bits 16-23: unused
	 * bits 24-28: kind (e.g. int, ptr, array...etc)
	 * bits 29-30: unused
	 * bit     31: kind_flag, currently used by
	 *             struct, union and fwd
	 */
	Info uint32
	/* "size" is used by INT, ENUM, STRUCT and UNION.
	 * "size" tells the size of the type it is describing.
	 *
	 * "type" is used by PTR, TYPEDEF, VOLATILE, CONST, RESTRICT,
	 * FUNC and FUNC_PROTO.
	 * "type" is a type_id referring to another type.
	 */
	SizeType uint32
}

var btfTypeSize = int(unsafe.Sizeof(btfType{}))

func unmarshalBtfType(bt *btfType, b []byte, bo binary.ByteOrder) (int, error) {
	if len(b) < btfTypeSize {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfType")
	}

	bt.NameOff = bo.Uint32(b[0:])
	bt.Info = bo.Uint32(b[4:])
	bt.SizeType = bo.Uint32(b[8:])
	return btfTypeSize, nil
}

func mask(len uint32) uint32 {
	return (1 << len) - 1
}

func readBits(value, len, shift uint32) uint32 {
	return (value >> shift) & mask(len)
}

func writeBits(value, len, shift, new uint32) uint32 {
	value &^= mask(len) << shift
	value |= (new & mask(len)) << shift
	return value
}

func (bt *btfType) info(len, shift uint32) uint32 {
	return readBits(bt.Info, len, shift)
}

func (bt *btfType) setInfo(value, len, shift uint32) {
	bt.Info = writeBits(bt.Info, len, shift, value)
}

func (bt *btfType) Kind() btfKind {
	return btfKind(bt.info(btfTypeKindLen, btfTypeKindShift))
}

func (bt *btfType) SetKind(kind btfKind) {
	bt.setInfo(uint32(kind), btfTypeKindLen, btfTypeKindShift)
}

func (bt *btfType) Vlen() int {
	return int(bt.info(btfTypeVlenMask, btfTypeVlenShift))
}

func (bt *btfType) SetVlen(vlen int) {
	bt.setInfo(uint32(vlen), btfTypeVlenMask, btfTypeVlenShift)
}

func (bt *btfType) kindFlagBool() bool {
	return bt.info(btfTypeKindFlagMask, btfTypeKindFlagShift) == 1
}

func (bt *btfType) setKindFlagBool(set bool) {
	var value uint32
	if set {
		value = 1
	}
	bt.setInfo(value, btfTypeKindFlagMask, btfTypeKindFlagShift)
}

// Bitfield returns true if the struct or union contain a bitfield.
func (bt *btfType) Bitfield() bool {
	return bt.kindFlagBool()
}

func (bt *btfType) SetBitfield(isBitfield bool) {
	bt.setKindFlagBool(isBitfield)
}

func (bt *btfType) FwdKind() FwdKind {
	return FwdKind(bt.info(btfTypeKindFlagMask, btfTypeKindFlagShift))
}

func (bt *btfType) SetFwdKind(kind FwdKind) {
	bt.setInfo(uint32(kind), btfTypeKindFlagMask, btfTypeKindFlagShift)
}

func (bt *btfType) Signed() bool {
	return bt.kindFlagBool()
}

func (bt *btfType) SetSigned(signed bool) {
	bt.setKindFlagBool(signed)
}

func (bt *btfType) Linkage() FuncLinkage {
	return FuncLinkage(bt.info(btfTypeVlenMask, btfTypeVlenShift))
}

func (bt *btfType) SetLinkage(linkage FuncLinkage) {
	bt.setInfo(uint32(linkage), btfTypeVlenMask, btfTypeVlenShift)
}

func (bt *btfType) Type() TypeID {
	// TODO: Panic here if wrong kind?
	return TypeID(bt.SizeType)
}

func (bt *btfType) SetType(id TypeID) {
	bt.SizeType = uint32(id)
}

func (bt *btfType) Size() uint32 {
	// TODO: Panic here if wrong kind?
	return bt.SizeType
}

func (bt *btfType) SetSize(size uint32) {
	bt.SizeType = size
}

func (bt *btfType) Encode(buf []byte, bo binary.ByteOrder) (int, error) {
	if len(buf) < btfTypeSize {
		return 0, fmt.Errorf("not enough bytes to marshal btfType")
	}
	bo.PutUint32(buf[0:], bt.NameOff)
	bo.PutUint32(buf[4:], bt.Info)
	bo.PutUint32(buf[8:], bt.SizeType)
	return btfTypeSize, nil
}

// DataLen returns the length of additional type specific data in bytes.
func (bt *btfType) DataLen() (int, error) {
	switch bt.Kind() {
	case kindInt:
		return int(unsafe.Sizeof(btfInt{})), nil
	case kindPointer:
	case kindArray:
		return int(unsafe.Sizeof(btfArray{})), nil
	case kindStruct:
		fallthrough
	case kindUnion:
		return int(unsafe.Sizeof(btfMember{})) * bt.Vlen(), nil
	case kindEnum:
		return int(unsafe.Sizeof(btfEnum{})) * bt.Vlen(), nil
	case kindForward:
	case kindTypedef:
	case kindVolatile:
	case kindConst:
	case kindRestrict:
	case kindFunc:
	case kindFuncProto:
		return int(unsafe.Sizeof(btfParam{})) * bt.Vlen(), nil
	case kindVar:
		return int(unsafe.Sizeof(btfVariable{})), nil
	case kindDatasec:
		return int(unsafe.Sizeof(btfVarSecinfo{})) * bt.Vlen(), nil
	case kindFloat:
	case kindDeclTag:
		return int(unsafe.Sizeof(btfDeclTag{})), nil
	case kindTypeTag:
	case kindEnum64:
		return int(unsafe.Sizeof(btfEnum64{})) * bt.Vlen(), nil
	default:
		return 0, fmt.Errorf("unknown kind: %v", bt.Kind())
	}

	return 0, nil
}

// btfInt encodes additional data for integers.
//
//	? ? ? ? e e e e o o o o o o o o ? ? ? ? ? ? ? ? b b b b b b b b
//	? = undefined
//	e = encoding
//	o = offset (bitfields?)
//	b = bits (bitfields)
type btfInt struct {
	Raw uint32
}

const (
	btfIntEncodingLen   = 4
	btfIntEncodingShift = 24
	btfIntOffsetLen     = 8
	btfIntOffsetShift   = 16
	btfIntBitsLen       = 8
	btfIntBitsShift     = 0
)

var btfIntLen = int(unsafe.Sizeof(btfInt{}))

func unmarshalBtfInt(bi *btfInt, b []byte, bo binary.ByteOrder) (int, error) {
	if len(b) < btfIntLen {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfInt")
	}

	bi.Raw = bo.Uint32(b[0:])
	return btfIntLen, nil
}

func (bi btfInt) Encoding() IntEncoding {
	return IntEncoding(readBits(bi.Raw, btfIntEncodingLen, btfIntEncodingShift))
}

func (bi *btfInt) SetEncoding(e IntEncoding) {
	bi.Raw = writeBits(uint32(bi.Raw), btfIntEncodingLen, btfIntEncodingShift, uint32(e))
}

func (bi btfInt) Offset() Bits {
	return Bits(readBits(bi.Raw, btfIntOffsetLen, btfIntOffsetShift))
}

func (bi *btfInt) SetOffset(offset uint32) {
	bi.Raw = writeBits(bi.Raw, btfIntOffsetLen, btfIntOffsetShift, offset)
}

func (bi btfInt) Bits() Bits {
	return Bits(readBits(bi.Raw, btfIntBitsLen, btfIntBitsShift))
}

func (bi *btfInt) SetBits(bits byte) {
	bi.Raw = writeBits(bi.Raw, btfIntBitsLen, btfIntBitsShift, uint32(bits))
}

type btfArray struct {
	Type      TypeID
	IndexType TypeID
	Nelems    uint32
}

var btfArrayLen = int(unsafe.Sizeof(btfArray{}))

func unmarshalBtfArray(ba *btfArray, b []byte, bo binary.ByteOrder) (int, error) {
	if len(b) < btfArrayLen {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfArray")
	}

	ba.Type = TypeID(bo.Uint32(b[0:]))
	ba.IndexType = TypeID(bo.Uint32(b[4:]))
	ba.Nelems = bo.Uint32(b[8:])
	return btfArrayLen, nil
}

type btfMember struct {
	NameOff uint32
	Type    TypeID
	Offset  uint32
}

var btfMemberLen = int(unsafe.Sizeof(btfMember{}))

func unmarshalBtfMember(bm *btfMember, b []byte, bo binary.ByteOrder) (int, error) {
	if btfMemberLen > len(b) {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfMember")
	}

	bm.NameOff = bo.Uint32(b[0:])
	bm.Type = TypeID(bo.Uint32(b[4:]))
	bm.Offset = bo.Uint32(b[8:])
	return btfMemberLen, nil
}

type btfVarSecinfo struct {
	Type   TypeID
	Offset uint32
	Size   uint32
}

var btfVarSecinfoLen = int(unsafe.Sizeof(btfVarSecinfo{}))

func unmarshalBtfVarSecInfo(bvsi *btfVarSecinfo, b []byte, bo binary.ByteOrder) (int, error) {
	if len(b) < btfVarSecinfoLen {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfVarSecinfo")
	}

	bvsi.Type = TypeID(bo.Uint32(b[0:]))
	bvsi.Offset = bo.Uint32(b[4:])
	bvsi.Size = bo.Uint32(b[8:])
	return btfVarSecinfoLen, nil
}

type btfVariable struct {
	Linkage uint32
}

var btfVariableLen = int(unsafe.Sizeof(btfVariable{}))

func unmarshalBtfVariable(bv *btfVariable, b []byte, bo binary.ByteOrder) (int, error) {
	if len(b) < btfVariableLen {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfVariable")
	}

	bv.Linkage = bo.Uint32(b[0:])
	return btfVariableLen, nil
}

type btfEnum struct {
	NameOff uint32
	Val     uint32
}

var btfEnumLen = int(unsafe.Sizeof(btfEnum{}))

func unmarshalBtfEnum(be *btfEnum, b []byte, bo binary.ByteOrder) (int, error) {
	if btfEnumLen > len(b) {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfEnum")
	}

	be.NameOff = bo.Uint32(b[0:])
	be.Val = bo.Uint32(b[4:])
	return btfEnumLen, nil
}

type btfEnum64 struct {
	NameOff uint32
	ValLo32 uint32
	ValHi32 uint32
}

var btfEnum64Len = int(unsafe.Sizeof(btfEnum64{}))

func unmarshalBtfEnum64(enum *btfEnum64, b []byte, bo binary.ByteOrder) (int, error) {
	if len(b) < btfEnum64Len {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfEnum64")
	}

	enum.NameOff = bo.Uint32(b[0:])
	enum.ValLo32 = bo.Uint32(b[4:])
	enum.ValHi32 = bo.Uint32(b[8:])

	return btfEnum64Len, nil
}

type btfParam struct {
	NameOff uint32
	Type    TypeID
}

var btfParamLen = int(unsafe.Sizeof(btfParam{}))

func unmarshalBtfParam(param *btfParam, b []byte, bo binary.ByteOrder) (int, error) {
	if len(b) < btfParamLen {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfParam")
	}

	param.NameOff = bo.Uint32(b[0:])
	param.Type = TypeID(bo.Uint32(b[4:]))

	return btfParamLen, nil
}

type btfDeclTag struct {
	ComponentIdx uint32
}

var btfDeclTagLen = int(unsafe.Sizeof(btfDeclTag{}))

func unmarshalBtfDeclTag(bdt *btfDeclTag, b []byte, bo binary.ByteOrder) (int, error) {
	if len(b) < btfDeclTagLen {
		return 0, fmt.Errorf("not enough bytes to unmarshal btfDeclTag")
	}

	bdt.ComponentIdx = bo.Uint32(b[0:])
	return btfDeclTagLen, nil
}
