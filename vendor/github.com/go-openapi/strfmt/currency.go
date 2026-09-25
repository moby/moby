// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package strfmt

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"golang.org/x/text/currency"
)

// Currency represents an ISO 4217 currency alpha-3 code as a string format.
//
// # Implementation
//
// golang.org/x/text/currency
//
// swagger:strfmt currency.
type Currency struct {
	currency.Unit
}

// IsCurrency checks if a string is a valid currency format.
func IsCurrency(str string) bool {
	_, err := ParseCurrency(str)

	return err == nil
}

// ParseCurrency parses a string that represents a valid [Currency].
func ParseCurrency(str string) (Currency, error) {
	cur, err := currency.ParseISO(str)

	return Currency{Unit: cur}, err
}

// MarshalText returns this instance into text.
func (u Currency) MarshalText() ([]byte, error) {
	return []byte(u.Unit.String()), nil
}

// UnmarshalText hydrates this instance from text.
func (u *Currency) UnmarshalText(data []byte) error { // validation is performed later on
	cur, err := ParseCurrency(string(data))
	if err != nil {
		return err
	}
	*u = cur

	return nil
}

// Scan read a value from a database driver.
func (u *Currency) Scan(raw any) error {
	switch v := raw.(type) {
	case []byte:
		cur, err := ParseCurrency(string(v))
		if err != nil {
			return err
		}
		*u = cur
	case string:
		cur, err := ParseCurrency(v)
		if err != nil {
			return err
		}
		*u = cur
	default:
		return fmt.Errorf("cannot sql.Scan() strfmt.Currency from: %#v: %w", v, ErrFormat)
	}

	return nil
}

// Value converts a value to a database driver value.
func (u Currency) Value() (driver.Value, error) {
	return driver.Value(u.String()), nil
}

func (u Currency) String() string {
	return u.Unit.String()
}

// MarshalJSON returns the [Currency] as JSON.
func (u Currency) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON sets the [Currency] from JSON.
func (u *Currency) UnmarshalJSON(data []byte) error {
	if string(data) == jsonNull {
		return nil
	}
	var ustr string
	if err := json.Unmarshal(data, &ustr); err != nil {
		return err
	}

	cur, err := ParseCurrency(ustr)
	if err != nil {
		return err
	}

	*u = cur

	return nil
}

// DeepCopyInto copies the receiver and writes its value into out.
func (u *Currency) DeepCopyInto(out *Currency) {
	*out = *u
}

// DeepCopy copies the receiver into a new [Currency].
func (u *Currency) DeepCopy() *Currency {
	if u == nil {
		return nil
	}

	out := new(Currency)
	u.DeepCopyInto(out)

	return out
}

// GobEncode implements the gob.GobEncoder interface.
func (u Currency) GobEncode() ([]byte, error) {
	return u.MarshalText()
}

// GobDecode implements the gob.GobDecoder interface.
func (u *Currency) GobDecode(data []byte) error {
	return u.UnmarshalText(data)
}

// MarshalBinary implements the encoding.[encoding.BinaryMarshaler] interface.
func (u Currency) MarshalBinary() ([]byte, error) {
	return u.MarshalText()
}

// UnmarshalBinary implements the encoding.[encoding.BinaryUnmarshaler] interface.
func (u *Currency) UnmarshalBinary(data []byte) error {
	return u.UnmarshalText(data)
}
