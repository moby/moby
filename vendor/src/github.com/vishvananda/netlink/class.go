package netlink

import (
	"fmt"
)

type Class interface {
	Attrs() *ClassAttrs
	Type() string
}

// Class represents a netlink class. A filter is associated with a link,
// has a handle and a parent. The root filter of a device should have a
// parent == HANDLE_ROOT.
type ClassAttrs struct {
	LinkIndex int
	Handle    uint32
	Parent    uint32
	Leaf      uint32
}

func (q ClassAttrs) String() string {
	return fmt.Sprintf("{LinkIndex: %d, Handle: %s, Parent: %s, Leaf: %s}", q.LinkIndex, HandleStr(q.Handle), HandleStr(q.Parent), q.Leaf)
}

type HtbClassAttrs struct {
	// TODO handle all attributes
	Rate    uint64
	Ceil    uint64
	Buffer  uint32
	Cbuffer uint32
	Quantum uint32
	Level   uint32
	Prio    uint32
}

func (q HtbClassAttrs) String() string {
	return fmt.Sprintf("{Rate: %d, Ceil: %d, Buffer: %d, Cbuffer: %d}", q.Rate, q.Ceil, q.Buffer, q.Cbuffer)
}

// Htb class
type HtbClass struct {
	ClassAttrs
	Rate    uint64
	Ceil    uint64
	Buffer  uint32
	Cbuffer uint32
	Quantum uint32
	Level   uint32
	Prio    uint32
}

func NewHtbClass(attrs ClassAttrs, cattrs HtbClassAttrs) *HtbClass {
	mtu := 1600
	rate := cattrs.Rate / 8
	ceil := cattrs.Ceil / 8
	buffer := cattrs.Buffer
	cbuffer := cattrs.Cbuffer

	if ceil == 0 {
		ceil = rate
	}

	if buffer == 0 {
		buffer = uint32(float64(rate)/Hz() + float64(mtu))
	}
	buffer = uint32(Xmittime(rate, buffer))

	if cbuffer == 0 {
		cbuffer = uint32(float64(ceil)/Hz() + float64(mtu))
	}
	cbuffer = uint32(Xmittime(ceil, cbuffer))

	return &HtbClass{
		ClassAttrs: attrs,
		Rate:       rate,
		Ceil:       ceil,
		Buffer:     buffer,
		Cbuffer:    cbuffer,
		Quantum:    10,
		Level:      0,
		Prio:       0,
	}
}

func (q HtbClass) String() string {
	return fmt.Sprintf("{Rate: %d, Ceil: %d, Buffer: %d, Cbuffer: %d}", q.Rate, q.Ceil, q.Buffer, q.Cbuffer)
}

func (class *HtbClass) Attrs() *ClassAttrs {
	return &class.ClassAttrs
}

func (class *HtbClass) Type() string {
	return "htb"
}

// GenericClass classes represent types that are not currently understood
// by this netlink library.
type GenericClass struct {
	ClassAttrs
	ClassType string
}

func (class *GenericClass) Attrs() *ClassAttrs {
	return &class.ClassAttrs
}

func (class *GenericClass) Type() string {
	return class.ClassType
}
