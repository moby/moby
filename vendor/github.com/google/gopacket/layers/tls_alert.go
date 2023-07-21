// Copyright 2018 The GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"errors"
	"fmt"

	"github.com/google/gopacket"
)

// TLSAlertLevel defines the alert level data type
type TLSAlertLevel uint8

// TLSAlertDescr defines the alert descrption data type
type TLSAlertDescr uint8

const (
	TLSAlertWarning      TLSAlertLevel = 1
	TLSAlertFatal        TLSAlertLevel = 2
	TLSAlertUnknownLevel TLSAlertLevel = 255

	TLSAlertCloseNotify               TLSAlertDescr = 0
	TLSAlertUnexpectedMessage         TLSAlertDescr = 10
	TLSAlertBadRecordMac              TLSAlertDescr = 20
	TLSAlertDecryptionFailedRESERVED  TLSAlertDescr = 21
	TLSAlertRecordOverflow            TLSAlertDescr = 22
	TLSAlertDecompressionFailure      TLSAlertDescr = 30
	TLSAlertHandshakeFailure          TLSAlertDescr = 40
	TLSAlertNoCertificateRESERVED     TLSAlertDescr = 41
	TLSAlertBadCertificate            TLSAlertDescr = 42
	TLSAlertUnsupportedCertificate    TLSAlertDescr = 43
	TLSAlertCertificateRevoked        TLSAlertDescr = 44
	TLSAlertCertificateExpired        TLSAlertDescr = 45
	TLSAlertCertificateUnknown        TLSAlertDescr = 46
	TLSAlertIllegalParameter          TLSAlertDescr = 47
	TLSAlertUnknownCa                 TLSAlertDescr = 48
	TLSAlertAccessDenied              TLSAlertDescr = 49
	TLSAlertDecodeError               TLSAlertDescr = 50
	TLSAlertDecryptError              TLSAlertDescr = 51
	TLSAlertExportRestrictionRESERVED TLSAlertDescr = 60
	TLSAlertProtocolVersion           TLSAlertDescr = 70
	TLSAlertInsufficientSecurity      TLSAlertDescr = 71
	TLSAlertInternalError             TLSAlertDescr = 80
	TLSAlertUserCanceled              TLSAlertDescr = 90
	TLSAlertNoRenegotiation           TLSAlertDescr = 100
	TLSAlertUnsupportedExtension      TLSAlertDescr = 110
	TLSAlertUnknownDescription        TLSAlertDescr = 255
)

//  TLS Alert
//  0  1  2  3  4  5  6  7  8
//  +--+--+--+--+--+--+--+--+
//  |         Level         |
//  +--+--+--+--+--+--+--+--+
//  |      Description      |
//  +--+--+--+--+--+--+--+--+

// TLSAlertRecord contains all the information that each Alert Record type should have
type TLSAlertRecord struct {
	TLSRecordHeader

	Level       TLSAlertLevel
	Description TLSAlertDescr

	EncryptedMsg []byte
}

// DecodeFromBytes decodes the slice into the TLS struct.
func (t *TLSAlertRecord) decodeFromBytes(h TLSRecordHeader, data []byte, df gopacket.DecodeFeedback) error {
	// TLS Record Header
	t.ContentType = h.ContentType
	t.Version = h.Version
	t.Length = h.Length

	if len(data) < 2 {
		df.SetTruncated()
		return errors.New("TLS Alert packet too short")
	}

	if t.Length == 2 {
		t.Level = TLSAlertLevel(data[0])
		t.Description = TLSAlertDescr(data[1])
	} else {
		t.Level = TLSAlertUnknownLevel
		t.Description = TLSAlertUnknownDescription
		t.EncryptedMsg = data
	}

	return nil
}

// Strings shows the TLS alert level nicely formatted
func (al TLSAlertLevel) String() string {
	switch al {
	default:
		return fmt.Sprintf("Unknown(%d)", al)
	case TLSAlertWarning:
		return "Warning"
	case TLSAlertFatal:
		return "Fatal"
	}
}

// Strings shows the TLS alert description nicely formatted
func (ad TLSAlertDescr) String() string {
	switch ad {
	default:
		return "Unknown"
	case TLSAlertCloseNotify:
		return "close_notify"
	case TLSAlertUnexpectedMessage:
		return "unexpected_message"
	case TLSAlertBadRecordMac:
		return "bad_record_mac"
	case TLSAlertDecryptionFailedRESERVED:
		return "decryption_failed_RESERVED"
	case TLSAlertRecordOverflow:
		return "record_overflow"
	case TLSAlertDecompressionFailure:
		return "decompression_failure"
	case TLSAlertHandshakeFailure:
		return "handshake_failure"
	case TLSAlertNoCertificateRESERVED:
		return "no_certificate_RESERVED"
	case TLSAlertBadCertificate:
		return "bad_certificate"
	case TLSAlertUnsupportedCertificate:
		return "unsupported_certificate"
	case TLSAlertCertificateRevoked:
		return "certificate_revoked"
	case TLSAlertCertificateExpired:
		return "certificate_expired"
	case TLSAlertCertificateUnknown:
		return "certificate_unknown"
	case TLSAlertIllegalParameter:
		return "illegal_parameter"
	case TLSAlertUnknownCa:
		return "unknown_ca"
	case TLSAlertAccessDenied:
		return "access_denied"
	case TLSAlertDecodeError:
		return "decode_error"
	case TLSAlertDecryptError:
		return "decrypt_error"
	case TLSAlertExportRestrictionRESERVED:
		return "export_restriction_RESERVED"
	case TLSAlertProtocolVersion:
		return "protocol_version"
	case TLSAlertInsufficientSecurity:
		return "insufficient_security"
	case TLSAlertInternalError:
		return "internal_error"
	case TLSAlertUserCanceled:
		return "user_canceled"
	case TLSAlertNoRenegotiation:
		return "no_renegotiation"
	case TLSAlertUnsupportedExtension:
		return "unsupported_extension"
	}
}
