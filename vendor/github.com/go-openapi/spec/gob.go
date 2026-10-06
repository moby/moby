// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"bytes"
	"encoding/gob"
)

// Optional numbers and gob.
//
// gob omits any struct field that holds the zero value for its type, and flattens a pointer to
// the value it points at. An optional number that is present and zero - "minimum": 0, which the
// JSON Schema meta-schema spells for every positiveInteger, or "maximum": 0 on a parameter -
// therefore travels as the zero value, is omitted, and comes back as a nil pointer. The bound is
// dropped and nothing reports it.
//
// [Schema], [Parameter], [Header] and [Items] are the types that carry those numbers, so each
// sends which of them were present-and-zero alongside the struct, and puts the zero back on the
// way in. The encoders sit on the outer types rather than on CommonValidations or SchemaProps
// because a method on an embedded type is promoted to the type embedding it: gob would then call
// it for the whole value and drop everything the embedded type does not hold. [Swagger] and
// [Operation] already carry their own encoders for the same reason.

// zeroBounds records which optional numbers of a value were present and zero.
type zeroBounds uint16

const (
	zeroMaximum zeroBounds = 1 << iota
	zeroMinimum
	zeroMaxLength
	zeroMinLength
	zeroMaxItems
	zeroMinItems
	zeroMultipleOf
	zeroMaxProperties
	zeroMinProperties
)

func floatBound(value *float64, bit zeroBounds) zeroBounds {
	if value != nil && *value == 0 {
		return bit
	}

	return 0
}

func intBound(value *int64, bit zeroBounds) zeroBounds {
	if value != nil && *value == 0 {
		return bit
	}

	return 0
}

func restoreFloat(value **float64, bounds, bit zeroBounds) {
	if bounds&bit != 0 && *value == nil {
		*value = new(float64)
	}
}

func restoreInt(value **int64, bounds, bit zeroBounds) {
	if bounds&bit != 0 && *value == nil {
		*value = new(int64)
	}
}

func commonBounds(v CommonValidations) zeroBounds {
	return floatBound(v.Maximum, zeroMaximum) |
		floatBound(v.Minimum, zeroMinimum) |
		floatBound(v.MultipleOf, zeroMultipleOf) |
		intBound(v.MaxLength, zeroMaxLength) |
		intBound(v.MinLength, zeroMinLength) |
		intBound(v.MaxItems, zeroMaxItems) |
		intBound(v.MinItems, zeroMinItems)
}

func restoreCommon(v *CommonValidations, bounds zeroBounds) {
	restoreFloat(&v.Maximum, bounds, zeroMaximum)
	restoreFloat(&v.Minimum, bounds, zeroMinimum)
	restoreFloat(&v.MultipleOf, bounds, zeroMultipleOf)
	restoreInt(&v.MaxLength, bounds, zeroMaxLength)
	restoreInt(&v.MinLength, bounds, zeroMinLength)
	restoreInt(&v.MaxItems, bounds, zeroMaxItems)
	restoreInt(&v.MinItems, bounds, zeroMinItems)
}

// GobEncode provides a gob encoder for Schema that keeps its zero-valued bounds.
func (s Schema) GobEncode() ([]byte, error) {
	type plain Schema

	p := s.SchemaProps
	bounds := floatBound(p.Maximum, zeroMaximum) |
		floatBound(p.Minimum, zeroMinimum) |
		floatBound(p.MultipleOf, zeroMultipleOf) |
		intBound(p.MaxLength, zeroMaxLength) |
		intBound(p.MinLength, zeroMinLength) |
		intBound(p.MaxItems, zeroMaxItems) |
		intBound(p.MinItems, zeroMinItems) |
		intBound(p.MaxProperties, zeroMaxProperties) |
		intBound(p.MinProperties, zeroMinProperties)

	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(struct {
		Plain  plain
		Bounds zeroBounds
	}{Plain: plain(s), Bounds: bounds})

	return b.Bytes(), err
}

// GobDecode provides a gob decoder for Schema that keeps its zero-valued bounds.
func (s *Schema) GobDecode(b []byte) error {
	type plain Schema

	var raw struct {
		Plain  plain
		Bounds zeroBounds
	}
	if err := gob.NewDecoder(bytes.NewBuffer(b)).Decode(&raw); err != nil {
		return err
	}

	*s = Schema(raw.Plain)
	restoreFloat(&s.Maximum, raw.Bounds, zeroMaximum)
	restoreFloat(&s.Minimum, raw.Bounds, zeroMinimum)
	restoreFloat(&s.MultipleOf, raw.Bounds, zeroMultipleOf)
	restoreInt(&s.MaxLength, raw.Bounds, zeroMaxLength)
	restoreInt(&s.MinLength, raw.Bounds, zeroMinLength)
	restoreInt(&s.MaxItems, raw.Bounds, zeroMaxItems)
	restoreInt(&s.MinItems, raw.Bounds, zeroMinItems)
	restoreInt(&s.MaxProperties, raw.Bounds, zeroMaxProperties)
	restoreInt(&s.MinProperties, raw.Bounds, zeroMinProperties)

	return nil
}

// GobEncode provides a gob encoder for Parameter that keeps its zero-valued bounds.
func (p Parameter) GobEncode() ([]byte, error) {
	type plain Parameter

	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(struct {
		Plain  plain
		Bounds zeroBounds
	}{Plain: plain(p), Bounds: commonBounds(p.CommonValidations)})

	return b.Bytes(), err
}

// GobDecode provides a gob decoder for Parameter that keeps its zero-valued bounds.
func (p *Parameter) GobDecode(b []byte) error {
	type plain Parameter

	var raw struct {
		Plain  plain
		Bounds zeroBounds
	}
	if err := gob.NewDecoder(bytes.NewBuffer(b)).Decode(&raw); err != nil {
		return err
	}

	*p = Parameter(raw.Plain)
	restoreCommon(&p.CommonValidations, raw.Bounds)

	return nil
}

// GobEncode provides a gob encoder for Header that keeps its zero-valued bounds.
func (h Header) GobEncode() ([]byte, error) {
	type plain Header

	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(struct {
		Plain  plain
		Bounds zeroBounds
	}{Plain: plain(h), Bounds: commonBounds(h.CommonValidations)})

	return b.Bytes(), err
}

// GobDecode provides a gob decoder for Header that keeps its zero-valued bounds.
func (h *Header) GobDecode(b []byte) error {
	type plain Header

	var raw struct {
		Plain  plain
		Bounds zeroBounds
	}
	if err := gob.NewDecoder(bytes.NewBuffer(b)).Decode(&raw); err != nil {
		return err
	}

	*h = Header(raw.Plain)
	restoreCommon(&h.CommonValidations, raw.Bounds)

	return nil
}

// GobEncode provides a gob encoder for Items that keeps its zero-valued bounds.
func (i Items) GobEncode() ([]byte, error) {
	type plain Items

	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(struct {
		Plain  plain
		Bounds zeroBounds
	}{Plain: plain(i), Bounds: commonBounds(i.CommonValidations)})

	return b.Bytes(), err
}

// GobDecode provides a gob decoder for Items that keeps its zero-valued bounds.
func (i *Items) GobDecode(b []byte) error {
	type plain Items

	var raw struct {
		Plain  plain
		Bounds zeroBounds
	}
	if err := gob.NewDecoder(bytes.NewBuffer(b)).Decode(&raw); err != nil {
		return err
	}

	*i = Items(raw.Plain)
	restoreCommon(&i.CommonValidations, raw.Bounds)

	return nil
}
