package data

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/docker/go/canonical/json"
)

// SignedTimestamp is a fully unpacked timestamp.json
type SignedTimestamp struct {
	Signatures []Signature
	Signed     Timestamp
	Dirty      bool
}

// Timestamp is the Signed component of a timestamp.json
type Timestamp struct {
	Type    string    `json:"_type"`
	Version int       `json:"version"`
	Expires time.Time `json:"expires"`
	Meta    Files     `json:"meta"`
}

// isValidTimestampStructure returns an error, or nil, depending on whether the content of the struct
// is valid for timestamp metadata.  This does not check signatures or expiry, just that
// the metadata content is valid.
func isValidTimestampStructure(t Timestamp) error {
	expectedType := TUFTypes[CanonicalTimestampRole]
	if t.Type != expectedType {
		return ErrInvalidMetadata{
			role: CanonicalTimestampRole, msg: fmt.Sprintf("expected type %s, not %s", expectedType, t.Type)}
	}

	// Meta is a map of FileMeta, so if the role isn't in the map it returns
	// an empty FileMeta, which has an empty map, and you can check on keys
	// from an empty map.
	if cs, ok := t.Meta[CanonicalSnapshotRole].Hashes["sha256"]; !ok || len(cs) != sha256.Size {
		return ErrInvalidMetadata{
			role: CanonicalTimestampRole, msg: "missing or invalid snapshot sha256 checksum information"}
	}
	return nil
}

// NewTimestamp initializes a timestamp with an existing snapshot
func NewTimestamp(snapshot *Signed) (*SignedTimestamp, error) {
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	snapshotMeta, err := NewFileMeta(bytes.NewReader(snapshotJSON), "sha256")
	if err != nil {
		return nil, err
	}
	return &SignedTimestamp{
		Signatures: make([]Signature, 0),
		Signed: Timestamp{
			Type:    TUFTypes["timestamp"],
			Version: 0,
			Expires: DefaultExpires("timestamp"),
			Meta: Files{
				CanonicalSnapshotRole: snapshotMeta,
			},
		},
	}, nil
}

// ToSigned partially serializes a SignedTimestamp such that it can
// be signed
func (ts *SignedTimestamp) ToSigned() (*Signed, error) {
	s, err := defaultSerializer.MarshalCanonical(ts.Signed)
	if err != nil {
		return nil, err
	}
	signed := json.RawMessage{}
	err = signed.UnmarshalJSON(s)
	if err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(ts.Signatures))
	copy(sigs, ts.Signatures)
	return &Signed{
		Signatures: sigs,
		Signed:     signed,
	}, nil
}

// GetSnapshot gets the expected snapshot metadata hashes in the timestamp metadata,
// or nil if it doesn't exist
func (ts *SignedTimestamp) GetSnapshot() (*FileMeta, error) {
	snapshotExpected, ok := ts.Signed.Meta[CanonicalSnapshotRole]
	if !ok {
		return nil, ErrMissingMeta{Role: CanonicalSnapshotRole}
	}
	return &snapshotExpected, nil
}

// MarshalJSON returns the serialized form of SignedTimestamp as bytes
func (ts *SignedTimestamp) MarshalJSON() ([]byte, error) {
	signed, err := ts.ToSigned()
	if err != nil {
		return nil, err
	}
	return defaultSerializer.Marshal(signed)
}

// TimestampFromSigned parsed a Signed object into a fully unpacked
// SignedTimestamp
func TimestampFromSigned(s *Signed) (*SignedTimestamp, error) {
	ts := Timestamp{}
	if err := defaultSerializer.Unmarshal(s.Signed, &ts); err != nil {
		return nil, err
	}
	if err := isValidTimestampStructure(ts); err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedTimestamp{
		Signatures: sigs,
		Signed:     ts,
	}, nil
}
