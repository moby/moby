// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package strfmt

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/go-openapi/strfmt/internal/countries"
)

// Country represents an ISO 3166 country alpha-3 or alpha-2 code as a string format.
//
// swagger:strfmt country.
type Country struct {
	countries.Country

	l int
}

const (
	alpha2Len = 2
	alpha3Len = 3
)

// IsCountry checks if a string is a valid ISO 3166 country format.
func IsCountry(str string) bool {
	_, err := ParseCountry(str)

	return err == nil
}

// ParseCountry parses a string that represents a valid [Country].
func ParseCountry(str string) (Country, error) {
	l := len(str)
	switch l {
	case alpha3Len:
		c, ok := countries.CountriesISO3[str]
		if !ok {
			return Country{}, fmt.Errorf("unrecognized strfmt.Country %q: %w", str, ErrFormat)
		}

		return Country{Country: c, l: alpha3Len}, nil
	case alpha2Len:
		c, ok := countries.CountriesISO2[str]
		if !ok {
			return Country{}, fmt.Errorf("unrecognized strfmt.Country %q: %w", str, ErrFormat)
		}

		return Country{Country: c, l: alpha2Len}, nil
	default:
		return Country{}, fmt.Errorf("invalid length for strfmt.Country in: %q: %w", str, ErrFormat)
	}
}

// MarshalText returns this instance into text.
func (u Country) MarshalText() ([]byte, error) {
	return []byte(u.String()), nil
}

// UnmarshalText hydrates this instance from text.
func (u *Country) UnmarshalText(data []byte) error { // validation is performed later on
	c, err := ParseCountry(string(data))
	if err != nil {
		return err
	}

	*u = c

	return nil
}

// Scan read a value from a database driver.
func (u *Country) Scan(raw any) error {
	switch v := raw.(type) {
	case []byte:
		c, err := ParseCountry(string(v))
		if err != nil {
			return err
		}
		*u = c
	case string:
		c, err := ParseCountry(v)
		if err != nil {
			return err
		}
		*u = c
	default:
		return fmt.Errorf("cannot sql.Scan() strfmt.Country from: %#v: %w", v, ErrFormat)
	}

	return nil
}

// Value converts a value to a database driver value.
func (u Country) Value() (driver.Value, error) {
	return driver.Value(u.String()), nil
}

func (u Country) String() string {
	switch u.l {
	case alpha3Len:
		return u.ISOAlpha3
	case alpha2Len:
		return u.ISOAlpha2
	default:
		return ""
	}
}

// MarshalJSON returns the [Country] as JSON.
func (u Country) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON sets the [Country] from JSON.
func (u *Country) UnmarshalJSON(data []byte) error {
	if string(data) == jsonNull {
		return nil
	}
	var ustr string
	if err := json.Unmarshal(data, &ustr); err != nil {
		return err
	}

	c, err := ParseCountry(ustr)
	if err != nil {
		return err
	}

	*u = c

	return nil
}

// DeepCopyInto copies the receiver and writes its value into out.
func (u *Country) DeepCopyInto(out *Country) {
	*out = *u
}

// DeepCopy copies the receiver into a new [Country].
func (u *Country) DeepCopy() *Country {
	if u == nil {
		return nil
	}

	out := new(Country)
	u.DeepCopyInto(out)

	return out
}

// GobEncode implements the gob.GobEncoder interface.
func (u Country) GobEncode() ([]byte, error) {
	return u.MarshalText()
}

// GobDecode implements the gob.GobDecoder interface.
func (u *Country) GobDecode(data []byte) error {
	return u.UnmarshalText(data)
}

// MarshalBinary implements the encoding.[encoding.BinaryMarshaler] interface.
func (u Country) MarshalBinary() ([]byte, error) {
	return u.MarshalText()
}

// UnmarshalBinary implements the encoding.[encoding.BinaryUnmarshaler] interface.
func (u *Country) UnmarshalBinary(data []byte) error {
	return u.UnmarshalText(data)
}
