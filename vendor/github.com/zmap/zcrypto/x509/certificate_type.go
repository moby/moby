// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import "encoding/json"

// TODO: Automatically generate this file from a CSV

// CertificateType represents whether a certificate is a root, intermediate, or
// leaf.
type CertificateType int

// CertificateType constants. Values should not be considered significant aside
// from CertificateTypeUnknown is the zero value.
const (
	CertificateTypeUnknown      CertificateType = 0
	CertificateTypeLeaf         CertificateType = 1
	CertificateTypeIntermediate CertificateType = 2
	CertificateTypeRoot         CertificateType = 3
)

const (
	certificateTypeStringLeaf         = "leaf"
	certificateTypeStringIntermediate = "intermediate"
	certificateTypeStringRoot         = "root"
	certificateTypeStringUnknown      = "unknown"
)

// MarshalJSON implements the json.Marshaler interface. Any unknown integer
// value is considered the same as CertificateTypeUnknown.
func (t CertificateType) MarshalJSON() ([]byte, error) {
	switch t {
	case CertificateTypeLeaf:
		return json.Marshal(certificateTypeStringLeaf)
	case CertificateTypeIntermediate:
		return json.Marshal(certificateTypeStringIntermediate)
	case CertificateTypeRoot:
		return json.Marshal(certificateTypeStringRoot)
	default:
		return json.Marshal(certificateTypeStringUnknown)
	}
}

// UnmarshalJSON implements the json.Unmarshaler interface. Any unknown string
// is considered the same CertificateTypeUnknown.
func (t *CertificateType) UnmarshalJSON(b []byte) error {
	var certificateTypeString string
	if err := json.Unmarshal(b, &certificateTypeString); err != nil {
		return err
	}
	switch certificateTypeString {
	case certificateTypeStringLeaf:
		*t = CertificateTypeLeaf
	case certificateTypeStringIntermediate:
		*t = CertificateTypeIntermediate
	case certificateTypeStringRoot:
		*t = CertificateTypeRoot
	default:
		*t = CertificateTypeUnknown
	}
	return nil
}
