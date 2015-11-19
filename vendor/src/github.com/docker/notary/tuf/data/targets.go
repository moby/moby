package data

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/jfrazelle/go/canonical/json"
)

// SignedTargets is a fully unpacked targets.json, or target delegation
// json file
type SignedTargets struct {
	Signatures []Signature
	Signed     Targets
	Dirty      bool
}

// Targets is the Signed components of a targets.json or delegation json file
type Targets struct {
	SignedCommon
	Targets     Files       `json:"targets"`
	Delegations Delegations `json:"delegations,omitempty"`
}

// NewTargets intiializes a new empty SignedTargets object
func NewTargets() *SignedTargets {
	return &SignedTargets{
		Signatures: make([]Signature, 0),
		Signed: Targets{
			SignedCommon: SignedCommon{
				Type:    TUFTypes["targets"],
				Version: 0,
				Expires: DefaultExpires("targets"),
			},
			Targets:     make(Files),
			Delegations: *NewDelegations(),
		},
		Dirty: true,
	}
}

// GetMeta attempts to find the targets entry for the path. It
// will return nil in the case of the target not being found.
func (t SignedTargets) GetMeta(path string) *FileMeta {
	for p, meta := range t.Signed.Targets {
		if p == path {
			return &meta
		}
	}
	return nil
}

// GetDelegations filters the roles and associated keys that may be
// the signers for the given target path. If no appropriate roles
// can be found, it will simply return nil for the return values.
// The returned slice of Role will have order maintained relative
// to the role slice on Delegations per TUF spec proposal on using
// order to determine priority.
func (t SignedTargets) GetDelegations(path string) []*Role {
	var roles []*Role
	pathHashBytes := sha256.Sum256([]byte(path))
	pathHash := hex.EncodeToString(pathHashBytes[:])
	for _, r := range t.Signed.Delegations.Roles {
		if !r.IsValid() {
			// Role has both Paths and PathHashPrefixes.
			continue
		}
		if r.CheckPaths(path) {
			roles = append(roles, r)
			continue
		}
		if r.CheckPrefixes(pathHash) {
			roles = append(roles, r)
			continue
		}
		//keysDB.AddRole(r)
	}
	return roles
}

// AddTarget adds or updates the meta for the given path
func (t *SignedTargets) AddTarget(path string, meta FileMeta) {
	t.Signed.Targets[path] = meta
	t.Dirty = true
}

// AddDelegation will add a new delegated role with the given keys,
// ensuring the keys either already exist, or are added to the map
// of delegation keys
func (t *SignedTargets) AddDelegation(role *Role, keys []*PublicKey) error {
	return nil
}

// ToSigned partially serializes a SignedTargets for further signing
func (t SignedTargets) ToSigned() (*Signed, error) {
	s, err := json.MarshalCanonical(t.Signed)
	if err != nil {
		return nil, err
	}
	signed := json.RawMessage{}
	err = signed.UnmarshalJSON(s)
	if err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(t.Signatures))
	copy(sigs, t.Signatures)
	return &Signed{
		Signatures: sigs,
		Signed:     signed,
	}, nil
}

// TargetsFromSigned fully unpacks a Signed object into a SignedTargets
func TargetsFromSigned(s *Signed) (*SignedTargets, error) {
	t := Targets{}
	err := json.Unmarshal(s.Signed, &t)
	if err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedTargets{
		Signatures: sigs,
		Signed:     t,
	}, nil
}
