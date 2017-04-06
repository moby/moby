package data

import (
	"fmt"

	"github.com/docker/go/canonical/json"
)

// SignedRoot is a fully unpacked root.json
type SignedRoot struct {
	Signatures []Signature
	Signed     Root
	Dirty      bool
}

// Root is the Signed component of a root.json
type Root struct {
	SignedCommon
	Keys               Keys                 `json:"keys"`
	Roles              map[string]*RootRole `json:"roles"`
	ConsistentSnapshot bool                 `json:"consistent_snapshot"`
}

// isValidRootStructure returns an error, or nil, depending on whether the content of the struct
// is valid for root metadata.  This does not check signatures or expiry, just that
// the metadata content is valid.
func isValidRootStructure(r Root) error {
	expectedType := TUFTypes[CanonicalRootRole]
	if r.Type != expectedType {
		return ErrInvalidMetadata{
			role: CanonicalRootRole, msg: fmt.Sprintf("expected type %s, not %s", expectedType, r.Type)}
	}

	if r.Version < 1 {
		return ErrInvalidMetadata{
			role: CanonicalRootRole, msg: "version cannot be less than 1"}
	}

	// all the base roles MUST appear in the root.json - other roles are allowed,
	// but other than the mirror role (not currently supported) are out of spec
	for _, roleName := range BaseRoles {
		roleObj, ok := r.Roles[roleName]
		if !ok || roleObj == nil {
			return ErrInvalidMetadata{
				role: CanonicalRootRole, msg: fmt.Sprintf("missing %s role specification", roleName)}
		}
		if err := isValidRootRoleStructure(CanonicalRootRole, roleName, *roleObj, r.Keys); err != nil {
			return err
		}
	}
	return nil
}

func isValidRootRoleStructure(metaContainingRole, rootRoleName string, r RootRole, validKeys Keys) error {
	if r.Threshold < 1 {
		return ErrInvalidMetadata{
			role: metaContainingRole,
			msg:  fmt.Sprintf("invalid threshold specified for %s: %v ", rootRoleName, r.Threshold),
		}
	}
	for _, keyID := range r.KeyIDs {
		if _, ok := validKeys[keyID]; !ok {
			return ErrInvalidMetadata{
				role: metaContainingRole,
				msg:  fmt.Sprintf("key ID %s specified in %s without corresponding key", keyID, rootRoleName),
			}
		}
	}
	return nil
}

// NewRoot initializes a new SignedRoot with a set of keys, roles, and the consistent flag
func NewRoot(keys map[string]PublicKey, roles map[string]*RootRole, consistent bool) (*SignedRoot, error) {
	signedRoot := &SignedRoot{
		Signatures: make([]Signature, 0),
		Signed: Root{
			SignedCommon: SignedCommon{
				Type:    TUFTypes[CanonicalRootRole],
				Version: 0,
				Expires: DefaultExpires(CanonicalRootRole),
			},
			Keys:               keys,
			Roles:              roles,
			ConsistentSnapshot: consistent,
		},
		Dirty: true,
	}

	return signedRoot, nil
}

// BuildBaseRole returns a copy of a BaseRole using the information in this SignedRoot for the specified role name.
// Will error for invalid role name or key metadata within this SignedRoot
func (r SignedRoot) BuildBaseRole(roleName string) (BaseRole, error) {
	roleData, ok := r.Signed.Roles[roleName]
	if !ok {
		return BaseRole{}, ErrInvalidRole{Role: roleName, Reason: "role not found in root file"}
	}
	// Get all public keys for the base role from TUF metadata
	keyIDs := roleData.KeyIDs
	pubKeys := make(map[string]PublicKey)
	for _, keyID := range keyIDs {
		pubKey, ok := r.Signed.Keys[keyID]
		if !ok {
			return BaseRole{}, ErrInvalidRole{
				Role:   roleName,
				Reason: fmt.Sprintf("key with ID %s was not found in root metadata", keyID),
			}
		}
		pubKeys[keyID] = pubKey
	}

	return BaseRole{
		Name:      roleName,
		Keys:      pubKeys,
		Threshold: roleData.Threshold,
	}, nil
}

// ToSigned partially serializes a SignedRoot for further signing
func (r SignedRoot) ToSigned() (*Signed, error) {
	s, err := defaultSerializer.MarshalCanonical(r.Signed)
	if err != nil {
		return nil, err
	}
	// cast into a json.RawMessage
	signed := json.RawMessage{}
	err = signed.UnmarshalJSON(s)
	if err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(r.Signatures))
	copy(sigs, r.Signatures)
	return &Signed{
		Signatures: sigs,
		Signed:     &signed,
	}, nil
}

// MarshalJSON returns the serialized form of SignedRoot as bytes
func (r SignedRoot) MarshalJSON() ([]byte, error) {
	signed, err := r.ToSigned()
	if err != nil {
		return nil, err
	}
	return defaultSerializer.Marshal(signed)
}

// RootFromSigned fully unpacks a Signed object into a SignedRoot and ensures
// that it is a valid SignedRoot
func RootFromSigned(s *Signed) (*SignedRoot, error) {
	r := Root{}
	if s.Signed == nil {
		return nil, ErrInvalidMetadata{
			role: CanonicalRootRole,
			msg:  "root file contained an empty payload",
		}
	}
	if err := defaultSerializer.Unmarshal(*s.Signed, &r); err != nil {
		return nil, err
	}
	if err := isValidRootStructure(r); err != nil {
		return nil, err
	}
	sigs := make([]Signature, len(s.Signatures))
	copy(sigs, s.Signatures)
	return &SignedRoot{
		Signatures: sigs,
		Signed:     r,
	}, nil
}
