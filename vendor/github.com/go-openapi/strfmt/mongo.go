// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package strfmt

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/go-openapi/strfmt/internal/bsonlite"
	"github.com/oklog/ulid/v2"
)

// bsonMarshaler is satisfied by types implementing MarshalBSON.
type bsonMarshaler interface {
	MarshalBSON() ([]byte, error)
}

// bsonUnmarshaler is satisfied by types implementing UnmarshalBSON.
type bsonUnmarshaler interface {
	UnmarshalBSON(data []byte) error
}

// bsonValueMarshaler is satisfied by types implementing MarshalBSONValue.
type bsonValueMarshaler interface {
	MarshalBSONValue() (byte, []byte, error)
}

// bsonValueUnmarshaler is satisfied by types implementing UnmarshalBSONValue.
type bsonValueUnmarshaler interface {
	UnmarshalBSONValue(tpe byte, data []byte) error
}

// Compile-time interface checks.
var (
	_ bsonMarshaler   = Date{}
	_ bsonUnmarshaler = &Date{}
	_ bsonMarshaler   = Base64{}
	_ bsonUnmarshaler = &Base64{}
	_ bsonMarshaler   = Duration(0)
	_ bsonUnmarshaler = (*Duration)(nil)
	_ bsonMarshaler   = DateTime{}
	_ bsonUnmarshaler = &DateTime{}
	_ bsonMarshaler   = ULID{}
	_ bsonUnmarshaler = &ULID{}
	_ bsonMarshaler   = URI("")
	_ bsonUnmarshaler = (*URI)(nil)
	_ bsonMarshaler   = Email("")
	_ bsonUnmarshaler = (*Email)(nil)
	_ bsonMarshaler   = Hostname("")
	_ bsonUnmarshaler = (*Hostname)(nil)
	_ bsonMarshaler   = IPv4("")
	_ bsonUnmarshaler = (*IPv4)(nil)
	_ bsonMarshaler   = IPv6("")
	_ bsonUnmarshaler = (*IPv6)(nil)
	_ bsonMarshaler   = CIDR("")
	_ bsonUnmarshaler = (*CIDR)(nil)
	_ bsonMarshaler   = MAC("")
	_ bsonUnmarshaler = (*MAC)(nil)
	_ bsonMarshaler   = Password("")
	_ bsonUnmarshaler = (*Password)(nil)
	_ bsonMarshaler   = UUID("")
	_ bsonUnmarshaler = (*UUID)(nil)
	_ bsonMarshaler   = UUID3("")
	_ bsonUnmarshaler = (*UUID3)(nil)
	_ bsonMarshaler   = UUID4("")
	_ bsonUnmarshaler = (*UUID4)(nil)
	_ bsonMarshaler   = UUID5("")
	_ bsonUnmarshaler = (*UUID5)(nil)
	_ bsonMarshaler   = UUID7("")
	_ bsonUnmarshaler = (*UUID7)(nil)
	_ bsonMarshaler   = ISBN("")
	_ bsonUnmarshaler = (*ISBN)(nil)
	_ bsonMarshaler   = ISBN10("")
	_ bsonUnmarshaler = (*ISBN10)(nil)
	_ bsonMarshaler   = ISBN13("")
	_ bsonUnmarshaler = (*ISBN13)(nil)
	_ bsonMarshaler   = CreditCard("")
	_ bsonUnmarshaler = (*CreditCard)(nil)
	_ bsonMarshaler   = SSN("")
	_ bsonUnmarshaler = (*SSN)(nil)
	_ bsonMarshaler   = HexColor("")
	_ bsonUnmarshaler = (*HexColor)(nil)
	_ bsonMarshaler   = RGBColor("")
	_ bsonUnmarshaler = (*RGBColor)(nil)
	_ bsonMarshaler   = ObjectId{}
	_ bsonUnmarshaler = &ObjectId{}

	_ bsonValueMarshaler   = DateTime{}
	_ bsonValueUnmarshaler = &DateTime{}
	_ bsonValueMarshaler   = ObjectId{}
	_ bsonValueUnmarshaler = &ObjectId{}
)

const (
	millisec         = 1000
	microsec         = 1_000_000
	bsonDateTimeSize = 8
)

func (d Date) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(d.String())
}

func (d *Date) UnmarshalBSON(data []byte) error {
	v, err := bsonlite.C.UnmarshalDoc(data)
	if err != nil {
		return err
	}

	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("couldn't unmarshal bson bytes value as Date: %w", ErrFormat)
	}

	rd, err := time.ParseInLocation(RFC3339FullDate, s, DefaultTimeLocation)
	if err != nil {
		return err
	}
	*d = Date(rd)
	return nil
}

// MarshalBSON document from this value.
func (b Base64) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(b.String())
}

// UnmarshalBSON document into this value.
func (b *Base64) UnmarshalBSON(data []byte) error {
	v, err := bsonlite.C.UnmarshalDoc(data)
	if err != nil {
		return err
	}

	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("couldn't unmarshal bson bytes as base64: %w", ErrFormat)
	}

	vb, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	*b = Base64(vb)
	return nil
}

func (d Duration) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(d.String())
}

func (d *Duration) UnmarshalBSON(data []byte) error {
	v, err := bsonlite.C.UnmarshalDoc(data)
	if err != nil {
		return err
	}

	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("couldn't unmarshal bson bytes value as Duration: %w", ErrFormat)
	}

	rd, err := ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(rd)
	return nil
}

// MarshalBSON renders the [DateTime] as a BSON document.
func (t DateTime) MarshalBSON() ([]byte, error) {
	tNorm := NormalizeTimeForMarshal(time.Time(t))
	return bsonlite.C.MarshalDoc(tNorm)
}

// UnmarshalBSON reads the [DateTime] from a BSON document.
func (t *DateTime) UnmarshalBSON(data []byte) error {
	v, err := bsonlite.C.UnmarshalDoc(data)
	if err != nil {
		return err
	}

	tv, ok := v.(time.Time)
	if !ok {
		return fmt.Errorf("couldn't unmarshal bson bytes value as DateTime: %w", ErrFormat)
	}
	*t = DateTime(tv)
	return nil
}

// MarshalBSONValue marshals a [DateTime] as a BSON DateTime value (type 0x09),
// an int64 representing milliseconds since epoch.
//
// MarshalBSONValue is an interface implemented by types that can marshal themselves
// into a BSON document represented as bytes.
//
// The bytes returned must be a valid BSON document if the error is nil.
func (t DateTime) MarshalBSONValue() (byte, []byte, error) {
	// UnixNano cannot be used directly, the result of calling UnixNano on the zero
	// Time is undefined. Thats why we use time.Nanosecond() instead.

	tNorm := NormalizeTimeForMarshal(time.Time(t))
	i64 := tNorm.Unix()*millisec + int64(tNorm.Nanosecond())/microsec
	buf := make([]byte, bsonDateTimeSize)
	binary.LittleEndian.PutUint64(buf, uint64(i64)) //nolint:gosec // it's okay to handle negative int64 this way

	return bsonlite.TypeDateTime, buf, nil
}

// UnmarshalBSONValue unmarshals a BSON DateTime value into this [DateTime].
func (t *DateTime) UnmarshalBSONValue(tpe byte, data []byte) error {
	if tpe == bsonlite.TypeNull {
		*t = DateTime{}
		return nil
	}

	if len(data) != bsonDateTimeSize {
		return fmt.Errorf("bson date field length not exactly %d bytes: %w", bsonDateTimeSize, ErrFormat)
	}

	i64 := int64(binary.LittleEndian.Uint64(data)) //nolint:gosec // it's okay if we overflow and get a negative datetime
	*t = DateTime(time.Unix(i64/millisec, i64%millisec*microsec))

	return nil
}

// MarshalBSON document from this value.
func (u ULID) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *ULID) UnmarshalBSON(data []byte) error {
	v, err := bsonlite.C.UnmarshalDoc(data)
	if err != nil {
		return err
	}

	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("couldn't unmarshal bson bytes as ULID: %w", ErrFormat)
	}

	id, err := ulid.ParseStrict(s)
	if err != nil {
		return fmt.Errorf("couldn't parse bson bytes as ULID: %w: %w", err, ErrFormat)
	}
	u.ULID = id
	return nil
}

// unmarshalBSONString is a helper for string-based strfmt types.
func unmarshalBSONString(data []byte, typeName string) (string, error) {
	v, err := bsonlite.C.UnmarshalDoc(data)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("couldn't unmarshal bson bytes as %s: %w", typeName, ErrFormat)
	}
	return s, nil
}

// MarshalBSON document from this value.
func (u URI) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *URI) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "uri")
	if err != nil {
		return err
	}
	*u = URI(s)
	return nil
}

// MarshalBSON document from this value.
func (e Email) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(e.String())
}

// UnmarshalBSON document into this value.
func (e *Email) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "email")
	if err != nil {
		return err
	}
	*e = Email(s)
	return nil
}

// MarshalBSON document from this value.
func (h Hostname) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(h.String())
}

// UnmarshalBSON document into this value.
func (h *Hostname) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "hostname")
	if err != nil {
		return err
	}
	*h = Hostname(s)
	return nil
}

// MarshalBSON document from this value.
func (u IPv4) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *IPv4) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "ipv4")
	if err != nil {
		return err
	}
	*u = IPv4(s)
	return nil
}

// MarshalBSON document from this value.
func (u IPv6) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *IPv6) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "ipv6")
	if err != nil {
		return err
	}
	*u = IPv6(s)
	return nil
}

// MarshalBSON document from this value.
func (u CIDR) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *CIDR) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "CIDR")
	if err != nil {
		return err
	}
	*u = CIDR(s)
	return nil
}

// MarshalBSON document from this value.
func (u MAC) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *MAC) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "MAC")
	if err != nil {
		return err
	}
	*u = MAC(s)
	return nil
}

// MarshalBSON document from this value.
func (r Password) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(r.String())
}

// UnmarshalBSON document into this value.
func (r *Password) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "Password")
	if err != nil {
		return err
	}
	*r = Password(s)
	return nil
}

// MarshalBSON document from this value.
func (u UUID) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *UUID) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "UUID")
	if err != nil {
		return err
	}
	*u = UUID(s)
	return nil
}

// MarshalBSON document from this value.
func (u UUID3) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *UUID3) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "UUID3")
	if err != nil {
		return err
	}
	*u = UUID3(s)
	return nil
}

// MarshalBSON document from this value.
func (u UUID4) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *UUID4) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "UUID4")
	if err != nil {
		return err
	}
	*u = UUID4(s)
	return nil
}

// MarshalBSON document from this value.
func (u UUID5) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *UUID5) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "UUID5")
	if err != nil {
		return err
	}
	*u = UUID5(s)
	return nil
}

// MarshalBSON document from this value.
func (u UUID7) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *UUID7) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "UUID7")
	if err != nil {
		return err
	}
	*u = UUID7(s)
	return nil
}

// MarshalBSON document from this value.
func (u ISBN) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *ISBN) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "ISBN")
	if err != nil {
		return err
	}
	*u = ISBN(s)
	return nil
}

// MarshalBSON document from this value.
func (u ISBN10) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *ISBN10) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "ISBN10")
	if err != nil {
		return err
	}
	*u = ISBN10(s)
	return nil
}

// MarshalBSON document from this value.
func (u ISBN13) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *ISBN13) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "ISBN13")
	if err != nil {
		return err
	}
	*u = ISBN13(s)
	return nil
}

// MarshalBSON document from this value.
func (u CreditCard) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *CreditCard) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "CreditCard")
	if err != nil {
		return err
	}
	*u = CreditCard(s)
	return nil
}

// MarshalBSON document from this value.
func (u SSN) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(u.String())
}

// UnmarshalBSON document into this value.
func (u *SSN) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "SSN")
	if err != nil {
		return err
	}
	*u = SSN(s)
	return nil
}

// MarshalBSON document from this value.
func (h HexColor) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(h.String())
}

// UnmarshalBSON document into this value.
func (h *HexColor) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "HexColor")
	if err != nil {
		return err
	}
	*h = HexColor(s)
	return nil
}

// MarshalBSON document from this value.
func (r RGBColor) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc(r.String())
}

// UnmarshalBSON document into this value.
func (r *RGBColor) UnmarshalBSON(data []byte) error {
	s, err := unmarshalBSONString(data, "RGBColor")
	if err != nil {
		return err
	}
	*r = RGBColor(s)
	return nil
}

// MarshalBSON renders the object id as a BSON document.
func (id ObjectId) MarshalBSON() ([]byte, error) {
	return bsonlite.C.MarshalDoc([12]byte(id))
}

// UnmarshalBSON reads the objectId from a BSON document.
func (id *ObjectId) UnmarshalBSON(data []byte) error {
	v, err := bsonlite.C.UnmarshalDoc(data)
	if err != nil {
		return err
	}

	oid, ok := v.([12]byte)
	if !ok {
		return fmt.Errorf("couldn't unmarshal bson bytes as ObjectId: %w", ErrFormat)
	}
	*id = ObjectId(oid)
	return nil
}

// MarshalBSONValue marshals the [ObjectId] as a raw BSON ObjectID value.
func (id ObjectId) MarshalBSONValue() (byte, []byte, error) {
	oid := [12]byte(id)
	return bsonlite.TypeObjectID, oid[:], nil
}

// UnmarshalBSONValue unmarshals a raw BSON ObjectID value into this [ObjectId].
func (id *ObjectId) UnmarshalBSONValue(_ byte, data []byte) error {
	var oid [12]byte
	copy(oid[:], data)
	*id = ObjectId(oid)
	return nil
}
