package types

import (
	"bytes"
	"compress/gzip"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io/ioutil"
)

// GzippedText is a []byte which transparently gzips data being submitted to
// a database and ungzips data being Scanned from a database.
type GzippedText []byte

// Value implements the driver.Valuer interface, gzipping the raw value of
// this GzippedText.
func (g GzippedText) Value() (driver.Value, error) {
	b := make([]byte, 0, len(g))
	buf := bytes.NewBuffer(b)
	w := gzip.NewWriter(buf)
	w.Write(g)
	w.Close()
	return buf.Bytes(), nil

}

// Scan implements the sql.Scanner interface, ungzipping the value coming off
// the wire and storing the raw result in the GzippedText.
func (g *GzippedText) Scan(src interface{}) error {
	var source []byte
	switch src := src.(type) {
	case string:
		source = []byte(src)
	case []byte:
		source = src
	default:
		//lint:ignore ST1005 changing this could break consumers of this package
		return errors.New("Incompatible type for GzippedText")
	}
	reader, err := gzip.NewReader(bytes.NewReader(source))
	if err != nil {
		return err
	}
	defer reader.Close()
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	*g = GzippedText(b)
	return nil
}

// JSONText is a json.RawMessage, which is a []byte underneath.
// Value() validates the json format in the source, and returns an error if
// the json is not valid.  Scan does no validation.  JSONText additionally
// implements `Unmarshal`, which unmarshals the json within to an interface{}
type JSONText json.RawMessage

var emptyJSON = JSONText("{}")

// MarshalJSON returns the *j as the JSON encoding of j.
func (j JSONText) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return emptyJSON, nil
	}
	return j, nil
}

// UnmarshalJSON sets *j to a copy of data
func (j *JSONText) UnmarshalJSON(data []byte) error {
	if j == nil {
		return errors.New("JSONText: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

// Value returns j as a value.  This does a validating unmarshal into another
// RawMessage.  If j is invalid json, it returns an error.
func (j JSONText) Value() (driver.Value, error) {
	var m json.RawMessage
	var err = j.Unmarshal(&m)
	if err != nil {
		return []byte{}, err
	}
	return []byte(j), nil
}

// Scan stores the src in *j.  No validation is done.
func (j *JSONText) Scan(src interface{}) error {
	var source []byte
	switch t := src.(type) {
	case string:
		source = []byte(t)
	case []byte:
		if len(t) == 0 {
			source = emptyJSON
		} else {
			source = t
		}
	case nil:
		*j = emptyJSON
	default:
		//lint:ignore ST1005 changing this could break consumers of this package
		return errors.New("Incompatible type for JSONText")
	}
	*j = append((*j)[0:0], source...)
	return nil
}

// Unmarshal unmarshal's the json in j to v, as in json.Unmarshal.
func (j *JSONText) Unmarshal(v interface{}) error {
	if len(*j) == 0 {
		*j = emptyJSON
	}
	return json.Unmarshal([]byte(*j), v)
}

// String supports pretty printing for JSONText types.
func (j JSONText) String() string {
	return string(j)
}

// NullJSONText represents a JSONText that may be null.
// NullJSONText implements the scanner interface so
// it can be used as a scan destination, similar to NullString.
type NullJSONText struct {
	JSONText
	Valid bool // Valid is true if JSONText is not NULL
}

// Scan implements the Scanner interface.
func (n *NullJSONText) Scan(value interface{}) error {
	if value == nil {
		n.JSONText, n.Valid = emptyJSON, false
		return nil
	}
	n.Valid = true
	return n.JSONText.Scan(value)
}

// Value implements the driver Valuer interface.
func (n NullJSONText) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.JSONText.Value()
}

// BitBool is an implementation of a bool for the MySQL type BIT(1).
// This type allows you to avoid wasting an entire byte for MySQL's boolean type TINYINT.
type BitBool bool

// Value implements the driver.Valuer interface,
// and turns the BitBool into a bitfield (BIT(1)) for MySQL storage.
func (b BitBool) Value() (driver.Value, error) {
	if b {
		return []byte{1}, nil
	}
	return []byte{0}, nil
}

// Scan implements the sql.Scanner interface,
// and turns the bitfield incoming from MySQL into a BitBool
func (b *BitBool) Scan(src interface{}) error {
	v, ok := src.([]byte)
	if !ok {
		return errors.New("bad []byte type assertion")
	}
	*b = v[0] == 1
	return nil
}
