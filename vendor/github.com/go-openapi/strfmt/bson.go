// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package strfmt

import (
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func init() { //nolint:gochecknoinits // registers bsonobjectid format in the default registry
	var id ObjectId
	Default.Add("bsonobjectid", &id, IsBSONObjectID)
}

// IsBSONObjectID returns true when the string is a valid BSON [ObjectId].
func IsBSONObjectID(str string) bool {
	_, err := objectIDFromHex(str)
	return err == nil
}

// ObjectId represents a BSON object ID (a 12-byte unique identifier).
//
// swagger:strfmt bsonobjectid.
type ObjectId [12]byte //nolint:revive

// nilObjectID is the zero-value ObjectId.
var nilObjectID ObjectId //nolint:gochecknoglobals // package-level sentinel

// NewObjectId creates a [ObjectId] from a hexadecimal String.
func NewObjectId(hex string) ObjectId { //nolint:revive
	oid, err := objectIDFromHex(hex)
	if err != nil {
		panic(err)
	}
	return oid
}

// MarshalText turns this instance into text.
func (id ObjectId) MarshalText() ([]byte, error) {
	if id == nilObjectID {
		return nil, nil
	}
	return []byte(id.Hex()), nil
}

// UnmarshalText hydrates this instance from text.
func (id *ObjectId) UnmarshalText(data []byte) error { // validation is performed later on
	if len(data) == 0 {
		*id = nilObjectID
		return nil
	}
	oid, err := objectIDFromHex(string(data))
	if err != nil {
		return err
	}
	*id = oid
	return nil
}

// Scan read a value from a database driver.
func (id *ObjectId) Scan(raw any) error {
	var data []byte
	switch v := raw.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot sql.Scan() strfmt.URI from: %#v: %w", v, ErrFormat)
	}

	return id.UnmarshalText(data)
}

// Value converts a value to a database driver value.
func (id ObjectId) Value() (driver.Value, error) {
	return driver.Value(id.Hex()), nil
}

// Hex returns the hex string representation of the [ObjectId].
func (id ObjectId) Hex() string {
	return hex.EncodeToString(id[:])
}

func (id ObjectId) String() string {
	return id.Hex()
}

// MarshalJSON returns the [ObjectId] as JSON.
func (id ObjectId) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.Hex())
}

// UnmarshalJSON sets the [ObjectId] from JSON.
func (id *ObjectId) UnmarshalJSON(data []byte) error {
	var hexStr string
	if err := json.Unmarshal(data, &hexStr); err != nil {
		return err
	}
	oid, err := objectIDFromHex(hexStr)
	if err != nil {
		return err
	}
	*id = oid
	return nil
}

// DeepCopyInto copies the receiver and writes its value into out.
func (id *ObjectId) DeepCopyInto(out *ObjectId) {
	*out = *id
}

// DeepCopy copies the receiver into a new [ObjectId].
func (id *ObjectId) DeepCopy() *ObjectId {
	if id == nil {
		return nil
	}
	out := new(ObjectId)
	id.DeepCopyInto(out)
	return out
}

// objectIDFromHex parses a 24-character hex string into an [ObjectId].
func objectIDFromHex(s string) (ObjectId, error) {
	const objectIDHexLen = 24
	if len(s) != objectIDHexLen {
		return nilObjectID, fmt.Errorf("the provided hex string %q is not a valid ObjectID: %w", s, ErrFormat)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nilObjectID, fmt.Errorf("the provided hex string %q is not a valid ObjectID: %w", s, err)
	}
	var oid ObjectId
	copy(oid[:], b)
	return oid, nil
}
