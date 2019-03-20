package etw

import (
	"bytes"
	"encoding/binary"
)

// EventData maintains a buffer which builds up the data for an ETW event. It
// needs to be paired with EventMetadata which describes the event.
type EventData struct {
	buffer bytes.Buffer
}

// Bytes returns the raw binary data containing the event data. The returned
// value is not copied from the internal buffer, so it can be mutated by the
// EventData object after it is returned.
func (ed *EventData) Bytes() []byte {
	return ed.buffer.Bytes()
}

// WriteString appends a string, including the null terminator, to the buffer.
func (ed *EventData) WriteString(data string) {
	ed.buffer.WriteString(data)
	ed.buffer.WriteByte(0)
}

// WriteInt8 appends a int8 to the buffer.
func (ed *EventData) WriteInt8(value int8) {
	ed.buffer.WriteByte(uint8(value))
}

// WriteInt16 appends a int16 to the buffer.
func (ed *EventData) WriteInt16(value int16) {
	binary.Write(&ed.buffer, binary.LittleEndian, value)
}

// WriteInt32 appends a int32 to the buffer.
func (ed *EventData) WriteInt32(value int32) {
	binary.Write(&ed.buffer, binary.LittleEndian, value)
}

// WriteInt64 appends a int64 to the buffer.
func (ed *EventData) WriteInt64(value int64) {
	binary.Write(&ed.buffer, binary.LittleEndian, value)
}

// WriteUint8 appends a uint8 to the buffer.
func (ed *EventData) WriteUint8(value uint8) {
	ed.buffer.WriteByte(value)
}

// WriteUint16 appends a uint16 to the buffer.
func (ed *EventData) WriteUint16(value uint16) {
	binary.Write(&ed.buffer, binary.LittleEndian, value)
}

// WriteUint32 appends a uint32 to the buffer.
func (ed *EventData) WriteUint32(value uint32) {
	binary.Write(&ed.buffer, binary.LittleEndian, value)
}

// WriteUint64 appends a uint64 to the buffer.
func (ed *EventData) WriteUint64(value uint64) {
	binary.Write(&ed.buffer, binary.LittleEndian, value)
}
