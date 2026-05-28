// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkix

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/zmap/zcrypto/encoding/asn1"
)

// AuxOID behaves similar to asn1.ObjectIdentifier, except encodes to JSON as a
// string in dot notation. It is a type synonym for []int, and can be converted
// to an asn1.ObjectIdentifier by going through []int and back.
type AuxOID []int

// AsSlice returns a slice over the inner-representation
func (aux *AuxOID) AsSlice() []int {
	return *aux
}

// CopyAsSlice returns a copy of the inter-representation as a slice
func (aux *AuxOID) CopyAsSlice() []int {
	out := make([]int, len(*aux))
	copy(out, *aux)
	return out
}

// Equal tests (deep) equality of two AuxOIDs
func (aux *AuxOID) Equal(other *AuxOID) bool {
	var a []int = *aux
	var b []int = *other
	if len(a) != len(b) {
		return false
	}
	for idx := range a {
		if a[idx] != b[idx] {
			return false
		}
	}
	return true
}

// MarshalJSON implements the json.Marshaler interface
func (aux *AuxOID) MarshalJSON() ([]byte, error) {
	var oid asn1.ObjectIdentifier
	oid = []int(*aux)
	return json.Marshal(oid.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (aux *AuxOID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parts := strings.Split(s, ".")
	if len(parts) < 1 {
		return fmt.Errorf("Invalid OID string %s", s)
	}
	slice := make([]int, len(parts))
	for idx := range parts {
		n, err := strconv.Atoi(parts[idx])
		if err != nil || n < 0 {
			return fmt.Errorf("Invalid OID integer %s", parts[idx])
		}
		slice[idx] = n
	}
	*aux = slice
	return nil
}
