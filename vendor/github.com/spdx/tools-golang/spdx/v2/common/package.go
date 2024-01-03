// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package common

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Supplier struct {
	// can be "NOASSERTION"
	Supplier string
	// SupplierType can be one of "Person", "Organization", or empty if Supplier is "NOASSERTION"
	SupplierType string
}

// UnmarshalJSON takes a supplier in the typical one-line format and parses it into a Supplier struct.
// This function is also used when unmarshalling YAML
func (s *Supplier) UnmarshalJSON(data []byte) error {
	// the value is just a string presented as a slice of bytes
	supplierStr := string(data)
	supplierStr = strings.Trim(supplierStr, "\"")

	if supplierStr == "NOASSERTION" {
		s.Supplier = supplierStr
		return nil
	}

	supplierFields := strings.SplitN(supplierStr, ": ", 2)

	if len(supplierFields) != 2 {
		return fmt.Errorf("failed to parse Supplier '%s'", supplierStr)
	}

	s.SupplierType = supplierFields[0]
	s.Supplier = supplierFields[1]

	return nil
}

// MarshalJSON converts the receiver into a slice of bytes representing a Supplier in string form.
// This function is also used when marshalling to YAML
func (s Supplier) MarshalJSON() ([]byte, error) {
	if s.Supplier == "NOASSERTION" {
		return json.Marshal(s.Supplier)
	} else if s.SupplierType != "" && s.Supplier != "" {
		return json.Marshal(fmt.Sprintf("%s: %s", s.SupplierType, s.Supplier))
	}

	return []byte{}, fmt.Errorf("failed to marshal invalid Supplier: %+v", s)
}

type Originator struct {
	// can be "NOASSERTION"
	Originator string
	// OriginatorType can be one of "Person", "Organization", or empty if Originator is "NOASSERTION"
	OriginatorType string
}

// UnmarshalJSON takes an originator in the typical one-line format and parses it into an Originator struct.
// This function is also used when unmarshalling YAML
func (o *Originator) UnmarshalJSON(data []byte) error {
	// the value is just a string presented as a slice of bytes
	originatorStr := string(data)
	originatorStr = strings.Trim(originatorStr, "\"")

	if originatorStr == "NOASSERTION" {
		o.Originator = originatorStr
		return nil
	}

	originatorFields := strings.SplitN(originatorStr, ": ", 2)

	if len(originatorFields) != 2 {
		return fmt.Errorf("failed to parse Originator '%s'", originatorStr)
	}

	o.OriginatorType = originatorFields[0]
	o.Originator = originatorFields[1]

	return nil
}

// MarshalJSON converts the receiver into a slice of bytes representing an Originator in string form.
// This function is also used when marshalling to YAML
func (o Originator) MarshalJSON() ([]byte, error) {
	if o.Originator == "NOASSERTION" {
		return json.Marshal(o.Originator)
	} else if o.Originator != "" {
		return json.Marshal(fmt.Sprintf("%s: %s", o.OriginatorType, o.Originator))
	}

	return []byte{}, nil
}

type PackageVerificationCode struct {
	// Cardinality: mandatory, one if filesAnalyzed is true / omitted;
	//              zero (must be omitted) if filesAnalyzed is false
	Value string `json:"packageVerificationCodeValue"`
	// Spec also allows specifying files to exclude from the
	// verification code algorithm; intended to enable exclusion of
	// the SPDX document file itself.
	ExcludedFiles []string `json:"packageVerificationCodeExcludedFiles,omitempty"`
}
