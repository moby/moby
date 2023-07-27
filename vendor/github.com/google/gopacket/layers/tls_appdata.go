// Copyright 2018 The GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"errors"

	"github.com/google/gopacket"
)

// TLSAppDataRecord contains all the information that each AppData Record types should have
type TLSAppDataRecord struct {
	TLSRecordHeader
	Payload []byte
}

// DecodeFromBytes decodes the slice into the TLS struct.
func (t *TLSAppDataRecord) decodeFromBytes(h TLSRecordHeader, data []byte, df gopacket.DecodeFeedback) error {
	// TLS Record Header
	t.ContentType = h.ContentType
	t.Version = h.Version
	t.Length = h.Length

	if len(data) != int(t.Length) {
		return errors.New("TLS Application Data length mismatch")
	}

	t.Payload = data
	return nil
}
