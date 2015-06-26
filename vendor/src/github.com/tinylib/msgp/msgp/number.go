package msgp

import (
	"strconv"
)

// The portable parts of the Number implementation

// DecodeMsg implements msgp.Decodable
func (n *Number) DecodeMsg(r *Reader) error {
	typ, err := r.NextType()
	if err != nil {
		return err
	}
	switch typ {
	case Float32Type:
		f, err := r.ReadFloat32()
		if err != nil {
			return err
		}
		n.AsFloat32(f)
		return nil
	case Float64Type:
		f, err := r.ReadFloat64()
		if err != nil {
			return err
		}
		n.AsFloat64(f)
		return nil
	case IntType:
		i, err := r.ReadInt64()
		if err != nil {
			return err
		}
		n.AsInt(i)
		return nil
	case UintType:
		u, err := r.ReadUint64()
		if err != nil {
			return err
		}
		n.AsUint(u)
		return nil
	default:
		return TypeError{Encoded: typ, Method: IntType}
	}
}

// UnmarshalMsg implements msgp.Unmarshaler
func (n *Number) UnmarshalMsg(b []byte) ([]byte, error) {
	typ := NextType(b)
	switch typ {
	case IntType:
		i, o, err := ReadInt64Bytes(b)
		if err != nil {
			return b, err
		}
		n.AsInt(i)
		return o, nil
	case UintType:
		u, o, err := ReadUint64Bytes(b)
		if err != nil {
			return b, err
		}
		n.AsUint(u)
		return o, nil
	case Float64Type:
		f, o, err := ReadFloat64Bytes(b)
		if err != nil {
			return b, err
		}
		n.AsFloat64(f)
		return o, nil
	case Float32Type:
		f, o, err := ReadFloat32Bytes(b)
		if err != nil {
			return b, err
		}
		n.AsFloat32(f)
		return o, nil
	default:
		return b, TypeError{Method: IntType, Encoded: typ}
	}
}

// Msgsize implements msgp.Sizer
func (n *Number) Msgsize() int {
	switch n.typ {
	case Float32Type:
		return Float32Size
	case Float64Type:
		return Float64Size
	case IntType:
		return Int64Size
	case UintType:
		return Uint64Size
	default:
		return 1 // fixint(0)
	}
}

// MarshalJSON implements json.Marshaler
func (n *Number) MarshalJSON() ([]byte, error) {
	t := n.Type()
	if t == InvalidType {
		return []byte{'0'}, nil
	}
	out := make([]byte, 0, 32)
	switch t {
	case Float32Type, Float64Type:
		f, _ := n.Float()
		return strconv.AppendFloat(out, f, 'f', -1, 64), nil
	case IntType:
		i, _ := n.Int()
		return strconv.AppendInt(out, i, 10), nil
	case UintType:
		u, _ := n.Uint()
		return strconv.AppendUint(out, u, 10), nil
	default:
		panic("(*Number).typ is invalid")
	}
}

func (n *Number) String() string {
	switch n.typ {
	case InvalidType:
		return "0"
	case Float32Type, Float64Type:
		f, _ := n.Float()
		return strconv.FormatFloat(f, 'f', -1, 64)
	case IntType:
		i, _ := n.Int()
		return strconv.FormatInt(i, 10)
	case UintType:
		u, _ := n.Uint()
		return strconv.FormatUint(u, 10)
	default:
		panic("(*Number).typ is invalid")
	}
}
