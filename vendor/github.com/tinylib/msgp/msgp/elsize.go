package msgp

func calcBytespec(v byte) bytespec {
	// single byte values
	switch v {

	case mnil:
		return bytespec{size: 1, extra: constsize, typ: NilType}
	case mfalse:
		return bytespec{size: 1, extra: constsize, typ: BoolType}
	case mtrue:
		return bytespec{size: 1, extra: constsize, typ: BoolType}
	case mbin8:
		return bytespec{size: 2, extra: extra8, typ: BinType}
	case mbin16:
		return bytespec{size: 3, extra: extra16, typ: BinType}
	case mbin32:
		return bytespec{size: 5, extra: extra32, typ: BinType}
	case mext8:
		return bytespec{size: 3, extra: extra8, typ: ExtensionType}
	case mext16:
		return bytespec{size: 4, extra: extra16, typ: ExtensionType}
	case mext32:
		return bytespec{size: 6, extra: extra32, typ: ExtensionType}
	case mfloat32:
		return bytespec{size: 5, extra: constsize, typ: Float32Type}
	case mfloat64:
		return bytespec{size: 9, extra: constsize, typ: Float64Type}
	case muint8:
		return bytespec{size: 2, extra: constsize, typ: UintType}
	case muint16:
		return bytespec{size: 3, extra: constsize, typ: UintType}
	case muint32:
		return bytespec{size: 5, extra: constsize, typ: UintType}
	case muint64:
		return bytespec{size: 9, extra: constsize, typ: UintType}
	case mint8:
		return bytespec{size: 2, extra: constsize, typ: IntType}
	case mint16:
		return bytespec{size: 3, extra: constsize, typ: IntType}
	case mint32:
		return bytespec{size: 5, extra: constsize, typ: IntType}
	case mint64:
		return bytespec{size: 9, extra: constsize, typ: IntType}
	case mfixext1:
		return bytespec{size: 3, extra: constsize, typ: ExtensionType}
	case mfixext2:
		return bytespec{size: 4, extra: constsize, typ: ExtensionType}
	case mfixext4:
		return bytespec{size: 6, extra: constsize, typ: ExtensionType}
	case mfixext8:
		return bytespec{size: 10, extra: constsize, typ: ExtensionType}
	case mfixext16:
		return bytespec{size: 18, extra: constsize, typ: ExtensionType}
	case mstr8:
		return bytespec{size: 2, extra: extra8, typ: StrType}
	case mstr16:
		return bytespec{size: 3, extra: extra16, typ: StrType}
	case mstr32:
		return bytespec{size: 5, extra: extra32, typ: StrType}
	case marray16:
		return bytespec{size: 3, extra: array16v, typ: ArrayType}
	case marray32:
		return bytespec{size: 5, extra: array32v, typ: ArrayType}
	case mmap16:
		return bytespec{size: 3, extra: map16v, typ: MapType}
	case mmap32:
		return bytespec{size: 5, extra: map32v, typ: MapType}
	}

	switch {

	// fixint
	case v >= mfixint && v < 0x80:
		return bytespec{size: 1, extra: constsize, typ: IntType}

	// fixstr gets constsize, since the prefix yields the size
	case v >= mfixstr && v < 0xc0:
		return bytespec{size: 1 + rfixstr(v), extra: constsize, typ: StrType}

	// fixmap
	case v >= mfixmap && v < 0x90:
		return bytespec{size: 1, extra: varmode(2 * rfixmap(v)), typ: MapType}

	// fixarray
	case v >= mfixarray && v < 0xa0:
		return bytespec{size: 1, extra: varmode(rfixarray(v)), typ: ArrayType}

	// nfixint
	case v >= mnfixint && uint16(v) < 0x100:
		return bytespec{size: 1, extra: constsize, typ: IntType}

	}

	// 0xC1 is unused per the spec and falls through to here,
	// everything else is covered above

	return bytespec{}
}

func getType(v byte) Type {
	return getBytespec(v).typ
}

// a valid bytespsec has
// non-zero 'size' and
// non-zero 'typ'
type bytespec struct {
	size  uint8   // prefix size information
	extra varmode // extra size information
	typ   Type    // type
	_     byte    // makes bytespec 4 bytes (yes, this matters)
}

// size mode
// if positive, # elements for composites
type varmode int8

const (
	constsize varmode = 0  // constant size (size bytes + uint8(varmode) objects)
	extra8    varmode = -1 // has uint8(p[1]) extra bytes
	extra16   varmode = -2 // has be16(p[1:]) extra bytes
	extra32   varmode = -3 // has be32(p[1:]) extra bytes
	map16v    varmode = -4 // use map16
	map32v    varmode = -5 // use map32
	array16v  varmode = -6 // use array16
	array32v  varmode = -7 // use array32
)
