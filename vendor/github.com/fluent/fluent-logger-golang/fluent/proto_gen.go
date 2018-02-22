package fluent

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *Entry) DecodeMsg(dc *msgp.Reader) (err error) {
	var zxvk uint32
	zxvk, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if zxvk != 2 {
		err = msgp.ArrayError{Wanted: 2, Got: zxvk}
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
	// array header, size 2
	err = en.Append(0x92)
	if err != nil {
		return err
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
	// array header, size 2
	o = append(o, 0x92)
	o = msgp.AppendInt64(o, z.Time)
	o, err = msgp.AppendIntf(o, z.Record)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Entry) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var zbzg uint32
	zbzg, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if zbzg != 2 {
		err = msgp.ArrayError{Wanted: 2, Got: zbzg}
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
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Entry) Msgsize() (s int) {
	s = 1 + msgp.Int64Size + msgp.GuessSize(z.Record)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Forward) DecodeMsg(dc *msgp.Reader) (err error) {
	var zcmr uint32
	zcmr, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if zcmr != 3 {
		err = msgp.ArrayError{Wanted: 3, Got: zcmr}
		return
	}
	z.Tag, err = dc.ReadString()
	if err != nil {
		return
	}
	var zajw uint32
	zajw, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if cap(z.Entries) >= int(zajw) {
		z.Entries = (z.Entries)[:zajw]
	} else {
		z.Entries = make([]Entry, zajw)
	}
	for zbai := range z.Entries {
		var zwht uint32
		zwht, err = dc.ReadArrayHeader()
		if err != nil {
			return
		}
		if zwht != 2 {
			err = msgp.ArrayError{Wanted: 2, Got: zwht}
			return
		}
		z.Entries[zbai].Time, err = dc.ReadInt64()
		if err != nil {
			return
		}
		z.Entries[zbai].Record, err = dc.ReadIntf()
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
	// array header, size 3
	err = en.Append(0x93)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Tag)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Entries)))
	if err != nil {
		return
	}
	for zbai := range z.Entries {
		// array header, size 2
		err = en.Append(0x92)
		if err != nil {
			return err
		}
		err = en.WriteInt64(z.Entries[zbai].Time)
		if err != nil {
			return
		}
		err = en.WriteIntf(z.Entries[zbai].Record)
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
	// array header, size 3
	o = append(o, 0x93)
	o = msgp.AppendString(o, z.Tag)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Entries)))
	for zbai := range z.Entries {
		// array header, size 2
		o = append(o, 0x92)
		o = msgp.AppendInt64(o, z.Entries[zbai].Time)
		o, err = msgp.AppendIntf(o, z.Entries[zbai].Record)
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
	var zhct uint32
	zhct, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if zhct != 3 {
		err = msgp.ArrayError{Wanted: 3, Got: zhct}
		return
	}
	z.Tag, bts, err = msgp.ReadStringBytes(bts)
	if err != nil {
		return
	}
	var zcua uint32
	zcua, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if cap(z.Entries) >= int(zcua) {
		z.Entries = (z.Entries)[:zcua]
	} else {
		z.Entries = make([]Entry, zcua)
	}
	for zbai := range z.Entries {
		var zxhx uint32
		zxhx, bts, err = msgp.ReadArrayHeaderBytes(bts)
		if err != nil {
			return
		}
		if zxhx != 2 {
			err = msgp.ArrayError{Wanted: 2, Got: zxhx}
			return
		}
		z.Entries[zbai].Time, bts, err = msgp.ReadInt64Bytes(bts)
		if err != nil {
			return
		}
		z.Entries[zbai].Record, bts, err = msgp.ReadIntfBytes(bts)
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

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *Forward) Msgsize() (s int) {
	s = 1 + msgp.StringPrefixSize + len(z.Tag) + msgp.ArrayHeaderSize
	for zbai := range z.Entries {
		s += 1 + msgp.Int64Size + msgp.GuessSize(z.Entries[zbai].Record)
	}
	s += msgp.GuessSize(z.Option)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Message) DecodeMsg(dc *msgp.Reader) (err error) {
	var zlqf uint32
	zlqf, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if zlqf != 4 {
		err = msgp.ArrayError{Wanted: 4, Got: zlqf}
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
	// array header, size 4
	err = en.Append(0x94)
	if err != nil {
		return err
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
	// array header, size 4
	o = append(o, 0x94)
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
	var zdaf uint32
	zdaf, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if zdaf != 4 {
		err = msgp.ArrayError{Wanted: 4, Got: zdaf}
		return
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

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *Message) Msgsize() (s int) {
	s = 1 + msgp.StringPrefixSize + len(z.Tag) + msgp.Int64Size + msgp.GuessSize(z.Record) + msgp.GuessSize(z.Option)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *MessageExt) DecodeMsg(dc *msgp.Reader) (err error) {
	var zpks uint32
	zpks, err = dc.ReadArrayHeader()
	if err != nil {
		return
	}
	if zpks != 4 {
		err = msgp.ArrayError{Wanted: 4, Got: zpks}
		return
	}
	z.Tag, err = dc.ReadString()
	if err != nil {
		return
	}
	err = dc.ReadExtension(&z.Time)
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
func (z *MessageExt) EncodeMsg(en *msgp.Writer) (err error) {
	// array header, size 4
	err = en.Append(0x94)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Tag)
	if err != nil {
		return
	}
	err = en.WriteExtension(&z.Time)
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
func (z *MessageExt) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// array header, size 4
	o = append(o, 0x94)
	o = msgp.AppendString(o, z.Tag)
	o, err = msgp.AppendExtension(o, &z.Time)
	if err != nil {
		return
	}
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
func (z *MessageExt) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var zjfb uint32
	zjfb, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if zjfb != 4 {
		err = msgp.ArrayError{Wanted: 4, Got: zjfb}
		return
	}
	z.Tag, bts, err = msgp.ReadStringBytes(bts)
	if err != nil {
		return
	}
	bts, err = msgp.ReadExtensionBytes(bts, &z.Time)
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

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *MessageExt) Msgsize() (s int) {
	s = 1 + msgp.StringPrefixSize + len(z.Tag) + msgp.ExtensionPrefixSize + z.Time.Len() + msgp.GuessSize(z.Record) + msgp.GuessSize(z.Option)
	return
}
