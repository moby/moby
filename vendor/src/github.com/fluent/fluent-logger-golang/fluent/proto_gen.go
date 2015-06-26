package fluent

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *Entry) DecodeMsg(dc *msgp.Reader) (err error) {
	var ssz uint32
	ssz, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if ssz != 2 {
		err = msgp.ArrayError{Wanted: 2, Got: ssz}
		return
	}
	z.Time, err = dc.ReadInt64()
	if err != nil {
		return
	}
	z.Record, err = dc.ReadIntf()
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Entry) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteArrayHeader(2)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Time)
	if err != nil {
		return
	}
	err = en.WriteIntf(z.Record)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Entry) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendArrayHeader(o, 2)
	o = msgp.AppendInt64(o, z.Time)
	o, err = msgp.AppendIntf(o, z.Record)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Entry) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var ssz uint32
		ssz, bts, err = msgp.ReadArrayHeaderBytes(bts)
		if err != nil {
			return
		}
		if ssz != 2 {
			err = msgp.ArrayError{Wanted: 2, Got: ssz}
			return
		}
	}
	z.Time, bts, err = msgp.ReadInt64Bytes(bts)
	if err != nil {
		return
	}
	z.Record, bts, err = msgp.ReadIntfBytes(bts)
	if err != nil {
		return
	}
	o = bts
	return
}

func (z Entry) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize + msgp.Int64Size + msgp.GuessSize(z.Record)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Forward) DecodeMsg(dc *msgp.Reader) (err error) {
	var ssz uint32
	ssz, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if ssz != 3 {
		err = msgp.ArrayError{Wanted: 3, Got: ssz}
		return
	}
	z.Tag, err = dc.ReadString()
	if err != nil {
		return
	}
	var xsz uint32
	xsz, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if cap(z.Entries) >= int(xsz) {
		z.Entries = z.Entries[:xsz]
	} else {
		z.Entries = make([]Entry, xsz)
	}
	for xvk := range z.Entries {
		var ssz uint32
		ssz, err = dc.ReadArrayHeader()
		if err != nil {
			return
		}
		if ssz != 2 {
			err = msgp.ArrayError{Wanted: 2, Got: ssz}
			return
		}
		z.Entries[xvk].Time, err = dc.ReadInt64()
		if err != nil {
			return
		}
		z.Entries[xvk].Record, err = dc.ReadIntf()
		if err != nil {
			return
		}
	}
	z.Option, err = dc.ReadIntf()
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Forward) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteArrayHeader(3)
	if err != nil {
		return
	}
	err = en.WriteString(z.Tag)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Entries)))
	if err != nil {
		return
	}
	for xvk := range z.Entries {
		err = en.WriteArrayHeader(2)
		if err != nil {
			return
		}
		err = en.WriteInt64(z.Entries[xvk].Time)
		if err != nil {
			return
		}
		err = en.WriteIntf(z.Entries[xvk].Record)
		if err != nil {
			return
		}
	}
	err = en.WriteIntf(z.Option)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Forward) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendArrayHeader(o, 3)
	o = msgp.AppendString(o, z.Tag)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Entries)))
	for xvk := range z.Entries {
		o = msgp.AppendArrayHeader(o, 2)
		o = msgp.AppendInt64(o, z.Entries[xvk].Time)
		o, err = msgp.AppendIntf(o, z.Entries[xvk].Record)
		if err != nil {
			return
		}
	}
	o, err = msgp.AppendIntf(o, z.Option)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Forward) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var ssz uint32
		ssz, bts, err = msgp.ReadArrayHeaderBytes(bts)
		if err != nil {
			return
		}
		if ssz != 3 {
			err = msgp.ArrayError{Wanted: 3, Got: ssz}
			return
		}
	}
	z.Tag, bts, err = msgp.ReadStringBytes(bts)
	if err != nil {
		return
	}
	var xsz uint32
	xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if cap(z.Entries) >= int(xsz) {
		z.Entries = z.Entries[:xsz]
	} else {
		z.Entries = make([]Entry, xsz)
	}
	for xvk := range z.Entries {
		{
			var ssz uint32
			ssz, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if ssz != 2 {
				err = msgp.ArrayError{Wanted: 2, Got: ssz}
				return
			}
		}
		z.Entries[xvk].Time, bts, err = msgp.ReadInt64Bytes(bts)
		if err != nil {
			return
		}
		z.Entries[xvk].Record, bts, err = msgp.ReadIntfBytes(bts)
		if err != nil {
			return
		}
	}
	z.Option, bts, err = msgp.ReadIntfBytes(bts)
	if err != nil {
		return
	}
	o = bts
	return
}

func (z *Forward) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize + msgp.StringPrefixSize + len(z.Tag) + msgp.ArrayHeaderSize
	for xvk := range z.Entries {
		s += msgp.ArrayHeaderSize + msgp.Int64Size + msgp.GuessSize(z.Entries[xvk].Record)
	}
	s += msgp.GuessSize(z.Option)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Message) DecodeMsg(dc *msgp.Reader) (err error) {
	var ssz uint32
	ssz, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if ssz != 4 {
		err = msgp.ArrayError{Wanted: 4, Got: ssz}
		return
	}
	z.Tag, err = dc.ReadString()
	if err != nil {
		return
	}
	z.Time, err = dc.ReadInt64()
	if err != nil {
		return
	}
	z.Record, err = dc.ReadIntf()
	if err != nil {
		return
	}
	z.Option, err = dc.ReadIntf()
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Message) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteArrayHeader(4)
	if err != nil {
		return
	}
	err = en.WriteString(z.Tag)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.Time)
	if err != nil {
		return
	}
	err = en.WriteIntf(z.Record)
	if err != nil {
		return
	}
	err = en.WriteIntf(z.Option)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Message) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendArrayHeader(o, 4)
	o = msgp.AppendString(o, z.Tag)
	o = msgp.AppendInt64(o, z.Time)
	o, err = msgp.AppendIntf(o, z.Record)
	if err != nil {
		return
	}
	o, err = msgp.AppendIntf(o, z.Option)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Message) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var ssz uint32
		ssz, bts, err = msgp.ReadArrayHeaderBytes(bts)
		if err != nil {
			return
		}
		if ssz != 4 {
			err = msgp.ArrayError{Wanted: 4, Got: ssz}
			return
		}
	}
	z.Tag, bts, err = msgp.ReadStringBytes(bts)
	if err != nil {
		return
	}
	z.Time, bts, err = msgp.ReadInt64Bytes(bts)
	if err != nil {
		return
	}
	z.Record, bts, err = msgp.ReadIntfBytes(bts)
	if err != nil {
		return
	}
	z.Option, bts, err = msgp.ReadIntfBytes(bts)
	if err != nil {
		return
	}
	o = bts
	return
}

func (z *Message) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize + msgp.StringPrefixSize + len(z.Tag) + msgp.Int64Size + msgp.GuessSize(z.Record) + msgp.GuessSize(z.Option)
	return
}
