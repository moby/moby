package data

import (
	"time"

	"github.com/jfrazelle/go/canonical/json"
)

// SignedRoot is a fully unpacked root.json
type SignedRoot struct {
	Signatures []Signature
	Signed     Root
	Dirty      bool
}

// Root is the Signed component of a root.json
type Root struct {
	Type               string               `json:"_type"`
	Version            int                  `json:"version"`
	Expires            time.Time            `json:"expires"`
	Keys               Keys                 `json:"keys"`
	Roles              map[string]*RootRole `json:"roles"`
	ConsistentSnapshot bool                 `json:"consistent_snapshot"`
}

// NewRoot initializes a new SignedRoot with a set of keys, roles, and the consistent flag
func NewRoot(keys map[string]PublicKey, roles map[string]*RootRole, consistent bool) (*SignedRoot, error) {
	signedRoot := &SignedRoot{
		Signatures: make([]Signature, 0),
		Signed: Root{
			Type:               TUFTypes["root"],
			Version:            0,
			Expires:            DefaultExpires("root"),
			Keys:               keys,
			Roles:              roles,
			ConsistentSnapshot: consistent,
		},
		Dirty: true,
	}

	return signedRoot, nil
}

// ToSigned partially serializes a SignedRoot for further signing
func (r SignedRoot) ToSigned() (*Signed, error) {
	s, err := json.MarshalCanonical(r.Signed)
	if err != nil {
		return nil, err
	}
	signed := json.RawMessage{}
	err = signed.UnmarshalJSON(s)
	if err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(r.Signatures))
	copy(sigs, r.Signatures)
	return &Signed{
		Signatures: sigs,
		Signed:     signed,
	}, nil
}

// RootFromSigned fully unpacks a Signed object into a SignedRoot
func RootFromSigned(s *Signed) (*SignedRoot, error) {
	r := Root{}
	err := json.Unmarshal(s.Signed, &r)
	if err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedRoot{
		Signatures: sigs,
		Signed:     r,
	}, nil
}
