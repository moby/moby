package data

import (
	"bytes"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go/canonical/json"
	"github.com/docker/notary"
)

// SignedSnapshot is a fully unpacked snapshot.json
type SignedSnapshot struct {
	Signatures []Signature
	Signed     Snapshot
	Dirty      bool
}

// Snapshot is the Signed component of a snapshot.json
type Snapshot struct {
	SignedCommon
	Meta Files `json:"meta"`
}

// IsValidSnapshotStructure returns an error, or nil, depending on whether the content of the
// struct is valid for snapshot metadata.  This does not check signatures or expiry, just that
// the metadata content is valid.
func IsValidSnapshotStructure(s Snapshot) error {
	expectedType := TUFTypes[CanonicalSnapshotRole]
	if s.Type != expectedType {
		return ErrInvalidMetadata{
			role: CanonicalSnapshotRole, msg: fmt.Sprintf("expected type %s, not %s", expectedType, s.Type)}
	}

	if s.Version < 1 {
		return ErrInvalidMetadata{
			role: CanonicalSnapshotRole, msg: "version cannot be less than one"}
	}

	for _, role := range []string{CanonicalRootRole, CanonicalTargetsRole} {
		// Meta is a map of FileMeta, so if the role isn't in the map it returns
		// an empty FileMeta, which has an empty map, and you can check on keys
		// from an empty map.
		//
		// For now sha256 is required and sha512 is not.
		if _, ok := s.Meta[role].Hashes[notary.SHA256]; !ok {
			return ErrInvalidMetadata{
				role: CanonicalSnapshotRole,
				msg:  fmt.Sprintf("missing %s sha256 checksum information", role),
			}
		}
		if err := CheckValidHashStructures(s.Meta[role].Hashes); err != nil {
			return ErrInvalidMetadata{
				role: CanonicalSnapshotRole,
				msg:  fmt.Sprintf("invalid %s checksum information, %v", role, err),
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
	rootMeta, err := NewFileMeta(bytes.NewReader(rootJSON), NotaryDefaultHashes...)
	if err != nil {
		return nil, err
	}
	targetsMeta, err := NewFileMeta(bytes.NewReader(targetsJSON), NotaryDefaultHashes...)
	if err != nil {
		return nil, err
	}
	return &SignedSnapshot{
		Signatures: make([]Signature, 0),
		Signed: Snapshot{
			SignedCommon: SignedCommon{
				Type:    TUFTypes[CanonicalSnapshotRole],
				Version: 0,
				Expires: DefaultExpires(CanonicalSnapshotRole),
			},
			Meta: Files{
				CanonicalRootRole:    rootMeta,
				CanonicalTargetsRole: targetsMeta,
			},
		},
	}, nil
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
		Signed:     &signed,
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
		if _, ok := meta.Hashes["sha256"]; ok {
			return &meta, nil
		}
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
	if err := defaultSerializer.Unmarshal(*s.Signed, &sp); err != nil {
		return nil, err
	}
	if err := IsValidSnapshotStructure(sp); err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedSnapshot{
		Signatures: sigs,
		Signed:     sp,
	}, nil
}
