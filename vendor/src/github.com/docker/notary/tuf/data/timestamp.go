package data

import (
	"bytes"
	"fmt"

	"github.com/docker/go/canonical/json"
	"github.com/docker/notary"
)

// SignedTimestamp is a fully unpacked timestamp.json
type SignedTimestamp struct {
	Signatures []Signature
	Signed     Timestamp
	Dirty      bool
}

// Timestamp is the Signed component of a timestamp.json
type Timestamp struct {
	SignedCommon
	Meta Files `json:"meta"`
}

// IsValidTimestampStructure returns an error, or nil, depending on whether the content of the struct
// is valid for timestamp metadata.  This does not check signatures or expiry, just that
// the metadata content is valid.
func IsValidTimestampStructure(t Timestamp) error {
	expectedType := TUFTypes[CanonicalTimestampRole]
	if t.Type != expectedType {
		return ErrInvalidMetadata{
			role: CanonicalTimestampRole, msg: fmt.Sprintf("expected type %s, not %s", expectedType, t.Type)}
	}

	if t.Version < 1 {
		return ErrInvalidMetadata{
			role: CanonicalTimestampRole, msg: "version cannot be less than one"}
	}

	// Meta is a map of FileMeta, so if the role isn't in the map it returns
	// an empty FileMeta, which has an empty map, and you can check on keys
	// from an empty map.
	//
	// For now sha256 is required and sha512 is not.
	if _, ok := t.Meta[CanonicalSnapshotRole].Hashes[notary.SHA256]; !ok {
		return ErrInvalidMetadata{
			role: CanonicalTimestampRole, msg: "missing snapshot sha256 checksum information"}
	}
	if err := CheckValidHashStructures(t.Meta[CanonicalSnapshotRole].Hashes); err != nil {
		return ErrInvalidMetadata{
			role: CanonicalTimestampRole, msg: fmt.Sprintf("invalid snapshot checksum information, %v", err)}
	}

	return nil
}

// NewTimestamp initializes a timestamp with an existing snapshot
func NewTimestamp(snapshot *Signed) (*SignedTimestamp, error) {
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	snapshotMeta, err := NewFileMeta(bytes.NewReader(snapshotJSON), NotaryDefaultHashes...)
	if err != nil {
		return nil, err
	}
	return &SignedTimestamp{
		Signatures: make([]Signature, 0),
		Signed: Timestamp{
			SignedCommon: SignedCommon{
				Type:    TUFTypes[CanonicalTimestampRole],
				Version: 0,
				Expires: DefaultExpires(CanonicalTimestampRole),
			},
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
		Signed:     &signed,
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
	if err := defaultSerializer.Unmarshal(*s.Signed, &ts); err != nil {
		return nil, err
	}
	if err := IsValidTimestampStructure(ts); err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedTimestamp{
		Signatures: sigs,
		Signed:     ts,
	}, nil
}
