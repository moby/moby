// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package bsonlite

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// liteCodec is a minimal BSON codec that handles only the patterns used by strfmt:
// single-key documents of the form {"data": <value>} where value is a string,
// BSON DateTime (time.Time), or BSON ObjectID ([12]byte).
type liteCodec struct{}

var _ Codec = liteCodec{}

func (liteCodec) MarshalDoc(value any) ([]byte, error) {
	switch v := value.(type) {
	case string:
		return marshalStringDoc(v), nil
	case time.Time:
		return marshalDateTimeDoc(v), nil
	case [ObjectIDSize]byte:
		return marshalObjectIDDoc(v), nil
	default:
		return nil, fmt.Errorf("bsonlite: unsupported value type %T: %w", value, errUnsupportedType)
	}
}

func (liteCodec) UnmarshalDoc(data []byte) (any, error) {
	return unmarshalDoc(data)
}

// BSON wire format helpers.
//
// Document: int32(size) + elements + 0x00
// Element:  byte(type) + cstring(key) + value
// String:   int32(len+1) + bytes + 0x00
// DateTime: int64 (LE, millis since epoch)
// ObjectID: [12]byte

const dataKey = "data\x00"

func marshalStringDoc(s string) []byte {
	sBytes := []byte(s)
	// doc_size(4) + type(1) + key("data\0"=5) + strlen(4) + string + \0(1) + doc_term(1)
	docSize := 4 + 1 + len(dataKey) + 4 + len(sBytes) + 1 + 1

	buf := make([]byte, docSize)
	pos := 0

	binary.LittleEndian.PutUint32(buf[pos:], uint32(docSize)) //nolint:gosec // size is computed from input, cannot overflow
	pos += 4

	buf[pos] = TypeString
	pos++

	pos += copy(buf[pos:], dataKey)

	binary.LittleEndian.PutUint32(buf[pos:], uint32(len(sBytes)+1)) //nolint:gosec // string length cannot overflow uint32
	pos += 4

	pos += copy(buf[pos:], sBytes)
	buf[pos] = 0 // string null terminator
	pos++

	buf[pos] = 0 // document terminator

	return buf
}

func marshalDateTimeDoc(t time.Time) []byte {
	// doc_size(4) + type(1) + key("data\0"=5) + int64(8) + doc_term(1)
	const docSize = 4 + 1 + 5 + 8 + 1

	buf := make([]byte, docSize)
	pos := 0

	binary.LittleEndian.PutUint32(buf[pos:], docSize)
	pos += 4

	buf[pos] = TypeDateTime
	pos++

	pos += copy(buf[pos:], dataKey)

	millis := DateTimeToMillis(t)
	binary.LittleEndian.PutUint64(buf[pos:], uint64(millis)) //nolint:gosec // negative datetime millis are valid
	// pos += 8

	buf[docSize-1] = 0 // document terminator

	return buf
}

func marshalObjectIDDoc(oid [ObjectIDSize]byte) []byte {
	// doc_size(4) + type(1) + key("data\0"=5) + objectid(12) + doc_term(1)
	const docSize = 4 + 1 + 5 + ObjectIDSize + 1

	buf := make([]byte, docSize)
	pos := 0

	binary.LittleEndian.PutUint32(buf[pos:], docSize)
	pos += 4

	buf[pos] = TypeObjectID
	pos++

	pos += copy(buf[pos:], dataKey)

	copy(buf[pos:], oid[:])
	// pos += ObjectIDSize

	buf[docSize-1] = 0 // document terminator

	return buf
}

var (
	errUnsupportedType = errors.New("bsonlite: unsupported type")
	errDocTooShort     = errors.New("bsonlite: document too short")
	errDocSize         = errors.New("bsonlite: document size mismatch")
	errNoTerminator    = errors.New("bsonlite: missing key terminator")
	errTruncated       = errors.New("bsonlite: truncated value")
	errDataNotFound    = errors.New("bsonlite: \"data\" field not found")
)

func unmarshalDoc(raw []byte) (any, error) {
	const minDocSize = 5 // int32(size) + terminator

	if len(raw) < minDocSize {
		return nil, errDocTooShort
	}

	docSize := int(binary.LittleEndian.Uint32(raw[:4]))
	if docSize != len(raw) {
		return nil, errDocSize
	}

	pos := 4

	for pos < docSize-1 {
		if pos >= len(raw) {
			return nil, errTruncated
		}
		typeByte := raw[pos]
		pos++

		// Read key (cstring: bytes until 0x00).
		keyStart := pos
		for pos < len(raw) && raw[pos] != 0 {
			pos++
		}
		if pos >= len(raw) {
			return nil, errNoTerminator
		}
		key := string(raw[keyStart:pos])
		pos++ // skip null terminator

		val, newPos, err := readValue(typeByte, raw, pos)
		if err != nil {
			return nil, err
		}
		pos = newPos

		if key == "data" {
			return val, nil
		}
	}

	return nil, errDataNotFound
}

func readValue(typeByte byte, raw []byte, pos int) (any, int, error) {
	switch typeByte {
	case TypeString:
		if pos+4 > len(raw) {
			return nil, 0, errTruncated
		}
		strLen := int(binary.LittleEndian.Uint32(raw[pos:]))
		pos += 4
		if pos+strLen > len(raw) || strLen < 1 {
			return nil, 0, errTruncated
		}
		s := string(raw[pos : pos+strLen-1]) // exclude null terminator
		return s, pos + strLen, nil

	case TypeObjectID:
		if pos+ObjectIDSize > len(raw) {
			return nil, 0, errTruncated
		}
		var oid [ObjectIDSize]byte
		copy(oid[:], raw[pos:pos+ObjectIDSize])
		return oid, pos + ObjectIDSize, nil

	case TypeDateTime:
		const dateTimeSize = 8
		if pos+dateTimeSize > len(raw) {
			return nil, 0, errTruncated
		}
		millis := int64(binary.LittleEndian.Uint64(raw[pos:])) //nolint:gosec // negative datetime millis are valid
		return MillisToTime(millis), pos + dateTimeSize, nil

	case TypeNull:
		return nil, pos, nil

	default:
		return nil, 0, fmt.Errorf("bsonlite: unsupported BSON type 0x%02x: %w", typeByte, errUnsupportedType)
	}
}
