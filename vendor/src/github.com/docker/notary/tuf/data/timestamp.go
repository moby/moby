package data

import (
	"bytes"
	"time"

	"github.com/jfrazelle/go/canonical/json"
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
				ValidRoles["snapshot"]: snapshotMeta,
			},
		},
	}, nil
}

// ToSigned partially serializes a SignedTimestamp such that it can
// be signed
func (ts SignedTimestamp) ToSigned() (*Signed, error) {
	s, err := json.MarshalCanonical(ts.Signed)
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

// TimestampFromSigned parsed a Signed object into a fully unpacked
// SignedTimestamp
func TimestampFromSigned(s *Signed) (*SignedTimestamp, error) {
	ts := Timestamp{}
	err := json.Unmarshal(s.Signed, &ts)
	if err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedTimestamp{
		Signatures: sigs,
		Signed:     ts,
	}, nil
}
