package data

import (
	"errors"
	"fmt"
	"path"

	"github.com/docker/go/canonical/json"
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

// isValidTargetsStructure returns an error, or nil, depending on whether the content of the struct
// is valid for targets metadata.  This does not check signatures or expiry, just that
// the metadata content is valid.
func isValidTargetsStructure(t Targets, roleName string) error {
	if roleName != CanonicalTargetsRole && !IsDelegation(roleName) {
		return ErrInvalidRole{Role: roleName}
	}

	// even if it's a delegated role, the metadata type is "Targets"
	expectedType := TUFTypes[CanonicalTargetsRole]
	if t.Type != expectedType {
		return ErrInvalidMetadata{
			role: roleName, msg: fmt.Sprintf("expected type %s, not %s", expectedType, t.Type)}
	}

	if t.Version < 1 {
		return ErrInvalidMetadata{role: roleName, msg: "version cannot be less than one"}
	}

	for _, roleObj := range t.Delegations.Roles {
		if !IsDelegation(roleObj.Name) || path.Dir(roleObj.Name) != roleName {
			return ErrInvalidMetadata{
				role: roleName, msg: fmt.Sprintf("delegation role %s invalid", roleObj.Name)}
		}
		if err := isValidRootRoleStructure(roleName, roleObj.Name, roleObj.RootRole, t.Delegations.Keys); err != nil {
			return err
		}
	}
	return nil
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

// GetValidDelegations filters the delegation roles specified in the signed targets, and
// only returns roles that are direct children and restricts their paths
func (t SignedTargets) GetValidDelegations(parent DelegationRole) []DelegationRole {
	roles := t.buildDelegationRoles()
	result := []DelegationRole{}
	for _, r := range roles {
		validRole, err := parent.Restrict(r)
		if err != nil {
			continue
		}
		result = append(result, validRole)
	}
	return result
}

// BuildDelegationRole returns a copy of a DelegationRole using the information in this SignedTargets for the specified role name.
// Will error for invalid role name or key metadata within this SignedTargets.  Path data is not validated.
func (t *SignedTargets) BuildDelegationRole(roleName string) (DelegationRole, error) {
	for _, role := range t.Signed.Delegations.Roles {
		if role.Name == roleName {
			pubKeys := make(map[string]PublicKey)
			for _, keyID := range role.KeyIDs {
				pubKey, ok := t.Signed.Delegations.Keys[keyID]
				if !ok {
					// Couldn't retrieve all keys, so stop walking and return invalid role
					return DelegationRole{}, ErrInvalidRole{Role: roleName, Reason: "delegation does not exist with all specified keys"}
				}
				pubKeys[keyID] = pubKey
			}
			return DelegationRole{
				BaseRole: BaseRole{
					Name:      role.Name,
					Keys:      pubKeys,
					Threshold: role.Threshold,
				},
				Paths: role.Paths,
			}, nil
		}
	}
	return DelegationRole{}, ErrNoSuchRole{Role: roleName}
}

// helper function to create DelegationRole structures from all delegations in a SignedTargets,
// these delegations are read directly from the SignedTargets and not modified or validated
func (t SignedTargets) buildDelegationRoles() []DelegationRole {
	var roles []DelegationRole
	for _, roleData := range t.Signed.Delegations.Roles {
		delgRole, err := t.BuildDelegationRole(roleData.Name)
		if err != nil {
			continue
		}
		roles = append(roles, delgRole)
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
	return errors.New("Not Implemented")
}

// ToSigned partially serializes a SignedTargets for further signing
func (t *SignedTargets) ToSigned() (*Signed, error) {
	s, err := defaultSerializer.MarshalCanonical(t.Signed)
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
		Signed:     &signed,
	}, nil
}

// MarshalJSON returns the serialized form of SignedTargets as bytes
func (t *SignedTargets) MarshalJSON() ([]byte, error) {
	signed, err := t.ToSigned()
	if err != nil {
		return nil, err
	}
	return defaultSerializer.Marshal(signed)
}

// TargetsFromSigned fully unpacks a Signed object into a SignedTargets, given
// a role name (so it can validate the SignedTargets object)
func TargetsFromSigned(s *Signed, roleName string) (*SignedTargets, error) {
	t := Targets{}
	if err := defaultSerializer.Unmarshal(*s.Signed, &t); err != nil {
		return nil, err
	}
	if err := isValidTargetsStructure(t, roleName); err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedTargets{
		Signatures: sigs,
		Signed:     t,
	}, nil
}
