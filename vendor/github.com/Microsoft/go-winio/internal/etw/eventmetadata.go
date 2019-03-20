package etw

import (
	"bytes"
	"encoding/binary"
)

// InType indicates the type of data contained in the ETW event.
type InType byte

// Various InType definitions for TraceLogging. These must match the definitions
// found in TraceLoggingProvider.h in the Windows SDK.
const (
	InTypeNull InType = iota
	InTypeUnicodeString
	InTypeANSIString
	InTypeInt8
	InTypeUint8
	InTypeInt16
	InTypeUint16
	InTypeInt32
	InTypeUint32
	InTypeInt64
	InTypeUint64
	InTypeFloat
	InTypeDouble
	InTypeBool32
	InTypeBinary
	InTypeGUID
	InTypePointerUnsupported
	InTypeFileTime
	InTypeSystemTime
	InTypeSID
	InTypeHexInt32
	InTypeHexInt64
	InTypeCountedString
	InTypeCountedANSIString
	InTypeStruct
	InTypeCountedBinary
	InTypeCountedArray InType = 32
	InTypeArray        InType = 64
)

// OutType specifies a hint to the event decoder for how the value should be
// formatted.
type OutType byte

// Various OutType definitions for TraceLogging. These must match the
// definitions found in TraceLoggingProvider.h in the Windows SDK.
const (
	// OutTypeDefault indicates that the default formatting for the InType will
	// be used by the event decoder.
	OutTypeDefault OutType = iota
	OutTypeNoPrint
	OutTypeString
	OutTypeBoolean
	OutTypeHex
	OutTypePID
	OutTypeTID
	OutTypePort
	OutTypeIPv4
	OutTypeIPv6
	OutTypeSocketAddress
	OutTypeXML
	OutTypeJSON
	OutTypeWin32Error
	OutTypeNTStatus
	OutTypeHResult
	OutTypeFileTime
	OutTypeSigned
	OutTypeUnsigned
	OutTypeUTF8              OutType = 35
	OutTypePKCS7WithTypeInfo OutType = 36
	OutTypeCodePointer       OutType = 37
	OutTypeDateTimeUTC       OutType = 38
)

// EventMetadata maintains a buffer which builds up the metadata for an ETW
// event. It needs to be paired with EventData which describes the event.
type EventMetadata struct {
	buffer bytes.Buffer
}

// Bytes returns the raw binary data containing the event metadata. Before being
// returned, the current size of the buffer is written to the start of the
// buffer. The returned value is not copied from the internal buffer, so it can
// be mutated by the EventMetadata object after it is returned.
func (em *EventMetadata) Bytes() []byte {
	// Finalize the event metadata buffer by filling in the buffer length at the
	// beginning.
	binary.LittleEndian.PutUint16(em.buffer.Bytes(), uint16(em.buffer.Len()))
	return em.buffer.Bytes()
}

// WriteEventHeader writes the metadata for the start of an event to the buffer.
// This specifies the event name and tags.
func (em *EventMetadata) WriteEventHeader(name string, tags uint32) {
	binary.Write(&em.buffer, binary.LittleEndian, uint16(0)) // Length placeholder
	em.writeTags(tags)
	em.buffer.WriteString(name)
	em.buffer.WriteByte(0) // Null terminator for name
}

func (em *EventMetadata) writeField(name string, inType InType, outType OutType, tags uint32, arrSize uint16) {
	em.buffer.WriteString(name)
	em.buffer.WriteByte(0) // Null terminator for name

	if outType == OutTypeDefault && tags == 0 {
		em.buffer.WriteByte(byte(inType))
	} else {
		em.buffer.WriteByte(byte(inType | 128))
		if tags == 0 {
			em.buffer.WriteByte(byte(outType))
		} else {
			em.buffer.WriteByte(byte(outType | 128))
			em.writeTags(tags)
		}
	}

	if arrSize != 0 {
		binary.Write(&em.buffer, binary.LittleEndian, arrSize)
	}
}

// writeTags writes out the tags value to the event metadata. Tags is a 28-bit
// value, interpreted as bit flags, which are only relevant to the event
// consumer. The event consumer may choose to attribute special meaning to tags
// (e.g. 0x4 could mean the field contains PII). Tags are written as a series of
// bytes, each containing 7 bits of tag value, with the high bit set if there is
// more tag data in the following byte. This allows for a more compact
// representation when not all of the tag bits are needed.
func (em *EventMetadata) writeTags(tags uint32) {
	// Only use the top 28 bits of the tags value.
	tags &= 0xfffffff

	for {
		// Tags are written with the most significant bits (e.g. 21-27) first.
		val := tags >> 21

		if tags&0x1fffff == 0 {
			// If there is no more data to write after this, write this value
			// without the high bit set, and return.
			em.buffer.WriteByte(byte(val & 0x7f))
			return
		}

		em.buffer.WriteByte(byte(val | 0x80))

		tags <<= 7
	}
}

// WriteField writes the metadata for a simple field to the buffer.
func (em *EventMetadata) WriteField(name string, inType InType, outType OutType, tags uint32) {
	em.writeField(name, inType, outType, tags, 0)
}

// WriteArray writes the metadata for an array field to the buffer. The number
// of elements in the array must be written as a uint16 in the event data,
// immediately preceeding the event data.
func (em *EventMetadata) WriteArray(name string, inType InType, outType OutType, tags uint32) {
	em.writeField(name, inType|InTypeArray, outType, tags, 0)
}

// WriteCountedArray writes the metadata for an array field to the buffer. The
// size of a counted array is fixed, and the size is written into the metadata
// directly.
func (em *EventMetadata) WriteCountedArray(name string, count uint16, inType InType, outType OutType, tags uint32) {
	em.writeField(name, inType|InTypeCountedArray, outType, tags, count)
}

// WriteStruct writes the metadata for a nested struct to the buffer. The struct
// contains the next N fields in the metadata, where N is specified by the
// fieldCount argument.
func (em *EventMetadata) WriteStruct(name string, fieldCount uint8, tags uint32) {
	em.writeField(name, InTypeStruct, OutType(fieldCount), tags, 0)
}
