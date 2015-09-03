package data

import (
	"time"

	"github.com/jfrazelle/go/canonical/json"
)

type SignedRoot struct {
	Signatures []Signature
	Signed     Root
	Dirty      bool
}

type Root struct {
	Type    string    `json:"_type"`
	Version int       `json:"version"`
	Expires time.Time `json:"expires"`
	// These keys are public keys. We use TUFKey instead of PublicKey to
	// support direct JSON unmarshaling.
	Keys               map[string]*TUFKey   `json:"keys"`
	Roles              map[string]*RootRole `json:"roles"`
	ConsistentSnapshot bool                 `json:"consistent_snapshot"`
}

func NewRoot(keys map[string]PublicKey, roles map[string]*RootRole, consistent bool) (*SignedRoot, error) {
	signedRoot := &SignedRoot{
		Signatures: make([]Signature, 0),
		Signed: Root{
			Type:               TUFTypes["root"],
			Version:            0,
			Expires:            DefaultExpires("root"),
			Keys:               make(map[string]*TUFKey),
			Roles:              roles,
			ConsistentSnapshot: consistent,
		},
		Dirty: true,
	}

	// Convert PublicKeys to TUFKey structures
	// The Signed.Keys map needs to have *TUFKey values, since this
	// structure gets directly unmarshalled from JSON, and it's not
	// possible to unmarshal into an interface type. But this function
	// takes a map with PublicKey values to avoid exposing this ugliness.
	// The loop below converts to the TUFKey type.
	for k, v := range keys {
		signedRoot.Signed.Keys[k] = &TUFKey{
			Type: v.Algorithm(),
			Value: KeyPair{
				Public:  v.Public(),
				Private: nil,
			},
		}
	}

	return signedRoot, nil
}

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
