package data

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go/canonical/json"
)

// SignedSnapshot is a fully unpacked snapshot.json
type SignedSnapshot struct {
	Signatures []Signature
	Signed     Snapshot
	Dirty      bool
}

// Snapshot is the Signed component of a snapshot.json
type Snapshot struct {
	Type    string    `json:"_type"`
	Version int       `json:"version"`
	Expires time.Time `json:"expires"`
	Meta    Files     `json:"meta"`
}

// isValidSnapshotStructure returns an error, or nil, depending on whether the content of the
// struct is valid for snapshot metadata.  This does not check signatures or expiry, just that
// the metadata content is valid.
func isValidSnapshotStructure(s Snapshot) error {
	expectedType := TUFTypes[CanonicalSnapshotRole]
	if s.Type != expectedType {
		return ErrInvalidMetadata{
			role: CanonicalSnapshotRole, msg: fmt.Sprintf("expected type %s, not %s", expectedType, s.Type)}
	}

	for _, role := range []string{CanonicalRootRole, CanonicalTargetsRole} {
		// Meta is a map of FileMeta, so if the role isn't in the map it returns
		// an empty FileMeta, which has an empty map, and you can check on keys
		// from an empty map.
		if checksum, ok := s.Meta[role].Hashes["sha256"]; !ok || len(checksum) != sha256.Size {
			return ErrInvalidMetadata{
				role: CanonicalSnapshotRole,
				msg:  fmt.Sprintf("missing or invalid %s sha256 checksum information", role),
			}
		}
	}
	return nil
}

// NewSnapshot initilizes a SignedSnapshot with a given top level root
// and targets objects
func NewSnapshot(root *Signed, targets *Signed) (*SignedSnapshot, error) {
	logrus.Debug("generating new snapshot...")
	targetsJSON, err := json.Marshal(targets)
	if err != nil {
		logrus.Debug("Error Marshalling Targets")
		return nil, err
	}
	rootJSON, err := json.Marshal(root)
	if err != nil {
		logrus.Debug("Error Marshalling Root")
		return nil, err
	}
	rootMeta, err := NewFileMeta(bytes.NewReader(rootJSON), "sha256")
	if err != nil {
		return nil, err
	}
	targetsMeta, err := NewFileMeta(bytes.NewReader(targetsJSON), "sha256")
	if err != nil {
		return nil, err
	}
	return &SignedSnapshot{
		Signatures: make([]Signature, 0),
		Signed: Snapshot{
			Type:    TUFTypes["snapshot"],
			Version: 0,
			Expires: DefaultExpires("snapshot"),
			Meta: Files{
				CanonicalRootRole:    rootMeta,
				CanonicalTargetsRole: targetsMeta,
			},
		},
	}, nil
}

func (sp *SignedSnapshot) hashForRole(role string) []byte {
	return sp.Signed.Meta[role].Hashes["sha256"]
}

// ToSigned partially serializes a SignedSnapshot for further signing
func (sp *SignedSnapshot) ToSigned() (*Signed, error) {
	s, err := defaultSerializer.MarshalCanonical(sp.Signed)
	if err != nil {
		return nil, err
	}
	signed := json.RawMessage{}
	err = signed.UnmarshalJSON(s)
	if err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(sp.Signatures))
	copy(sigs, sp.Signatures)
	return &Signed{
		Signatures: sigs,
		Signed:     signed,
	}, nil
}

// AddMeta updates a role in the snapshot with new meta
func (sp *SignedSnapshot) AddMeta(role string, meta FileMeta) {
	sp.Signed.Meta[role] = meta
	sp.Dirty = true
}

// GetMeta gets the metadata for a particular role, returning an error if it's
// not found
func (sp *SignedSnapshot) GetMeta(role string) (*FileMeta, error) {
	if meta, ok := sp.Signed.Meta[role]; ok {
		return &meta, nil
	}
	return nil, ErrMissingMeta{Role: role}
}

// DeleteMeta removes a role from the snapshot. If the role doesn't
// exist in the snapshot, it's a noop.
func (sp *SignedSnapshot) DeleteMeta(role string) {
	if _, ok := sp.Signed.Meta[role]; ok {
		delete(sp.Signed.Meta, role)
		sp.Dirty = true
	}
}

// MarshalJSON returns the serialized form of SignedSnapshot as bytes
func (sp *SignedSnapshot) MarshalJSON() ([]byte, error) {
	signed, err := sp.ToSigned()
	if err != nil {
		return nil, err
	}
	return defaultSerializer.Marshal(signed)
}

// SnapshotFromSigned fully unpacks a Signed object into a SignedSnapshot
func SnapshotFromSigned(s *Signed) (*SignedSnapshot, error) {
	sp := Snapshot{}
	if err := defaultSerializer.Unmarshal(s.Signed, &sp); err != nil {
		return nil, err
	}
	if err := isValidSnapshotStructure(sp); err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedSnapshot{
		Signatures: sigs,
		Signed:     sp,
	}, nil
}
