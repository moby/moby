// +build appengine

package msgp

// let's just assume appengine
// uses 64-bit hardware...
const smallint = false

func UnsafeString(b []byte) string {
	return string(b)
}

func UnsafeBytes(s string) []byte {
	return []byte(s)
}

type Number struct {
	ibits uint64  // zero or bits
	fbits float64 // zero or bits
	typ   Type    // zero or type
}

func (n *Number) AsFloat64(f float64) {
	n.typ = Float64Type
	n.fbits = f
	n.ibits = 0
}

func (n *Number) AsFloat32(f float32) {
	n.typ = Float32Type
	n.fbits = float64(f)
	n.ibits = 0
}

func (n *Number) AsInt(i int64) {
	n.fbits = 0
	if i == 0 {
		n.typ = InvalidType
		n.ibits = 0
		return
	}
	n.ibits = uint64(i)
	n.typ = IntType
}

func (n *Number) AsUint(u uint64) {
	n.ibits = u
	n.fbits = 0
	n.typ = UintType
}

func (n *Number) Float() (float64, bool) {
	return n.fbits, n.typ == Float64Type || n.typ == Float32Type
}

func (n *Number) Int() (int64, bool) {
	return int64(n.ibits), n.typ == IntType
}

func (n *Number) Uint() (uint64, bool) {
	return n.ibits, n.typ == UintType
}

func (n *Number) Type() Type {
	if n.typ == InvalidType {
		return IntType
	}
	return n.typ
}

func (n *Number) MarshalMsg(o []byte) ([]byte, error) {
	switch n.typ {
	case InvalidType:
		return AppendInt64(o, 0), nil
	case IntType:
		return AppendInt64(o, int64(n.ibits)), nil
	case UintType:
		return AppendUint64(o, n.ibits), nil
	case Float32Type:
		return AppendFloat32(o, float32(n.fbits)), nil
	case Float64Type:
		return AppendFloat64(o, n.fbits), nil
	}
	panic("unreachable code!")
}

func (n *Number) EncodeMsg(w *Writer) error {
	switch n.typ {
	case InvalidType:
		return w.WriteInt64(0)
	case IntType:
		return w.WriteInt64(int64(n.ibits))
	case UintType:
		return w.WriteUint64(n.ibits)
	case Float32Type:
		return w.WriteFloat32(float32(n.fbits))
	case Float64Type:
		return w.WriteFloat64(n.fbits)
	}
	panic("unreachable code!")
}
