package ct

// This file contains selectively chosen snippets of
// github.com/google/certificate-transparency-go@ 5cfe585726ad9d990d4db524d6ce2567b13e2f80
//
// These snippets only perform deserialization for SCTs and are recreated here to prevent pulling in the whole of the ct
// which contains yet another version of x509,asn1 and tls

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// CTExtensions is a representation of the raw bytes of any CtExtension
// structure (see section 3.2)
type CTExtensions []byte

// SHA256Hash represents the output from the SHA256 hash function.
type SHA256Hash [sha256.Size]byte

// FromBase64String populates the SHA256 struct with the contents of the base64 data passed in.
func (s *SHA256Hash) FromBase64String(b64 string) error {
	bs, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("failed to unbase64 LogID: %v", err)
	}
	if len(bs) != sha256.Size {
		return fmt.Errorf("invalid SHA256 length, expected 32 but got %d", len(bs))
	}
	copy(s[:], bs)
	return nil
}

// Base64String returns the base64 representation of this SHA256Hash.
func (s SHA256Hash) Base64String() string {
	return base64.StdEncoding.EncodeToString(s[:])
}

// MarshalJSON implements the json.Marshaller interface for SHA256Hash.
func (s SHA256Hash) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.Base64String() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (s *SHA256Hash) UnmarshalJSON(b []byte) error {
	var content string
	if err := json.Unmarshal(b, &content); err != nil {
		return fmt.Errorf("failed to unmarshal SHA256Hash: %v", err)
	}
	return s.FromBase64String(content)
}

// HashAlgorithm from the DigitallySigned struct
type HashAlgorithm byte

// HashAlgorithm constants
const (
	None   HashAlgorithm = 0
	MD5    HashAlgorithm = 1
	SHA1   HashAlgorithm = 2
	SHA224 HashAlgorithm = 3
	SHA256 HashAlgorithm = 4
	SHA384 HashAlgorithm = 5
	SHA512 HashAlgorithm = 6
)

func (h HashAlgorithm) String() string {
	switch h {
	case None:
		return "None"
	case MD5:
		return "MD5"
	case SHA1:
		return "SHA1"
	case SHA224:
		return "SHA224"
	case SHA256:
		return "SHA256"
	case SHA384:
		return "SHA384"
	case SHA512:
		return "SHA512"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", h)
	}
}

// SignatureAlgorithm from the the DigitallySigned struct
type SignatureAlgorithm byte

// SignatureAlgorithm constants
const (
	Anonymous SignatureAlgorithm = 0
	RSA       SignatureAlgorithm = 1
	DSA       SignatureAlgorithm = 2
	ECDSA     SignatureAlgorithm = 3
)

func (s SignatureAlgorithm) String() string {
	switch s {
	case Anonymous:
		return "Anonymous"
	case RSA:
		return "RSA"
	case DSA:
		return "DSA"
	case ECDSA:
		return "ECDSA"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// DigitallySigned represents an RFC5246 DigitallySigned structure
type DigitallySigned struct {
	HashAlgorithm      HashAlgorithm
	SignatureAlgorithm SignatureAlgorithm
	Signature          []byte
}

// FromBase64String populates the DigitallySigned structure from the base64 data passed in.
// Returns an error if the base64 data is invalid.
func (d *DigitallySigned) FromBase64String(b64 string) error {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("failed to unbase64 DigitallySigned: %v", err)
	}
	ds, err := UnmarshalDigitallySigned(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("failed to unmarshal DigitallySigned: %v", err)
	}
	*d = *ds
	return nil
}

// Base64String returns the base64 representation of the DigitallySigned struct.
func (d DigitallySigned) Base64String() (string, error) {
	b, err := MarshalDigitallySigned(d)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// MarshalJSON implements the json.Marshaller interface.
func (d DigitallySigned) MarshalJSON() ([]byte, error) {
	b64, err := d.Base64String()
	if err != nil {
		return []byte{}, err
	}
	return []byte(`"` + b64 + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *DigitallySigned) UnmarshalJSON(b []byte) error {
	var content string
	if err := json.Unmarshal(b, &content); err != nil {
		return fmt.Errorf("failed to unmarshal DigitallySigned: %v", err)
	}
	return d.FromBase64String(content)
}

// Version represents the Version enum from section 3.2 of the RFC:
// enum { v1(0), (255) } Version;
type Version uint8

func (v Version) String() string {
	switch v {
	case V1:
		return "V1"
	default:
		return fmt.Sprintf("UnknownVersion(%d)", v)
	}
}

// CT Version constants, see section 3.2 of the RFC.
const (
	V1 Version = 0
)

// SignedCertificateTimestamp represents the structure returned by the
// add-chain and add-pre-chain methods after base64 decoding. (see RFC sections
// 3.2 ,4.1 and 4.2)
type SignedCertificateTimestamp struct {
	SCTVersion Version    `json:"version"` // The version of the protocol to which the SCT conforms
	LogID      SHA256Hash `json:"log_id"`  // the SHA-256 hash of the log's public key, calculated over
	// the DER encoding of the key represented as SubjectPublicKeyInfo.
	Timestamp  uint64          `json:"timestamp,omitempty"`  // Timestamp (in ms since unix epoc) at which the SCT was issued. NOTE: When this is serialized, the output is in seconds, not milliseconds.
	Extensions CTExtensions    `json:"extensions,omitempty"` // For future extensions to the protocol
	Signature  DigitallySigned `json:"signature"`            // The Log's signature for this SCT
}

// Copied from ct/types.go 2018/06/15 to deal with BQ timestamp overflow; output
// is expected to be seconds, not milliseconds.
type auxSignedCertificateTimestamp SignedCertificateTimestamp

const kMaxTimestamp = 253402300799

// MarshalJSON implements the JSON.Marshaller interface.
func (sct *SignedCertificateTimestamp) MarshalJSON() ([]byte, error) {
	aux := auxSignedCertificateTimestamp(*sct)
	aux.Timestamp = sct.Timestamp / 1000 // convert ms to sec
	if aux.Timestamp > kMaxTimestamp {
		aux.Timestamp = 0
	}
	return json.Marshal(&aux)
}

type sctError int

// Preallocate errors for performance
var (
	ErrInvalidVersion  error = sctError(1)
	ErrNotEnoughBuffer error = sctError(2)
)

func (e sctError) Error() string {
	switch e {
	case ErrInvalidVersion:
		return "invalid SCT version detected"
	case ErrNotEnoughBuffer:
		return "provided buffer was too small"
	default:
		return "unknown error"
	}
}
