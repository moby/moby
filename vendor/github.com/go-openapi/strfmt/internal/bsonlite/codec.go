// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package bsonlite provides a minimal BSON codec for strfmt types.
//
// This codec produces BSON output compatible with go.mongodb.org/mongo-driver/v2
// (v2.5.0). It handles only the exact BSON patterns used by strfmt:
// single-key {"data": value} documents with string, DateTime, or ObjectID values.
//
// This package is intended to provide a backward-compatible API to users of
// go-openapi/strfmt. It is not intended to be maintained or to follow the
// evolutions of the official MongoDB drivers. For up-to-date MongoDB support,
// import "github.com/go-openapi/strfmt/enable/mongodb" to replace this codec
// with one backed by the real driver.
package bsonlite

import "time"

// Codec provides BSON document marshal/unmarshal for strfmt types.
//
// MarshalDoc encodes a single-key BSON document {"data": value}.
// The value must be one of: string, time.Time, or [12]byte (ObjectID).
//
// UnmarshalDoc decodes a BSON document and returns the "data" field's value.
// Returns one of: string, time.Time, or [12]byte depending on the BSON type.
type Codec interface {
	MarshalDoc(value any) ([]byte, error)
	UnmarshalDoc(data []byte) (any, error)
}

// C is the active BSON codec.
//
//nolint:gochecknoglobals // replaceable codec, by design
var C Codec = liteCodec{}

// Replace swaps the active BSON codec with the provided implementation.
// This is intended to be called from enable/mongodb's init().
//
// Since [Replace] affects the global state of the package, it is not intended for concurrent use.
func Replace(c Codec) {
	C = c
}

// BSON type tags (from the BSON specification).
const (
	TypeString   byte = 0x02
	TypeObjectID byte = 0x07
	TypeDateTime byte = 0x09
	TypeNull     byte = 0x0A
)

// ObjectIDSize is the size of a BSON ObjectID in bytes.
const ObjectIDSize = 12

// DateTimeToMillis converts a time.Time to BSON DateTime milliseconds.
func DateTimeToMillis(t time.Time) int64 {
	const (
		millisec = 1000
		microsec = 1_000_000
	)
	return t.Unix()*millisec + int64(t.Nanosecond())/microsec
}

// MillisToTime converts BSON DateTime milliseconds to time.Time.
func MillisToTime(millis int64) time.Time {
	const (
		millisec   = 1000
		nanosPerMs = 1_000_000
	)
	return time.Unix(millis/millisec, millis%millisec*nanosPerMs)
}
