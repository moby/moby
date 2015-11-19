package data

import (
	"bytes"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jfrazelle/go/canonical/json"
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
				ValidRoles["root"]:    rootMeta,
				ValidRoles["targets"]: targetsMeta,
			},
		},
	}, nil
}

func (sp *SignedSnapshot) hashForRole(role string) []byte {
	return sp.Signed.Meta[role].Hashes["sha256"]
}

// ToSigned partially serializes a SignedSnapshot for further signing
func (sp SignedSnapshot) ToSigned() (*Signed, error) {
	s, err := json.MarshalCanonical(sp.Signed)
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

// SnapshotFromSigned fully unpacks a Signed object into a SignedSnapshot
func SnapshotFromSigned(s *Signed) (*SignedSnapshot, error) {
	sp := Snapshot{}
	err := json.Unmarshal(s.Signed, &sp)
	if err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedSnapshot{
		Signatures: sigs,
		Signed:     sp,
	}, nil
}
